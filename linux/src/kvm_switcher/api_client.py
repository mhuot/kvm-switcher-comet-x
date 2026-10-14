"""Client for the GLKVM (PiKVM-derived kvmd) HTTP API. Standard library only.

Endpoints (see docs/API.md in the repository):
    POST /api/auth/login               form: user=admin&passwd=...  -> {token}
    GET  /api/switch                   header token: ...            -> switch state
    POST /api/switch/set_active?port=N (N is zero-based)
    GET  /api/system/get_config        -> config.port_names (names set in the web UI)
"""

from __future__ import annotations

import hashlib
import http.client
import json
import socket
import ssl
import threading
import time
import urllib.parse
from dataclasses import dataclass
from typing import Any, Callable

from kvm_switcher.settings import Settings

ADMIN_USER_NAME = "admin"
REQUEST_TIMEOUT_SECONDS = 8
SWITCH_SETTLE_SECONDS = 0.4

MESSAGE_NOT_CONFIGURED = "Set the KVM address and password in Settings."
MESSAGE_LOGIN_REJECTED = "Login rejected. Check the password in Settings."
MESSAGE_TWO_FACTOR_REQUIRED = "2FA is enabled on the KVM; this app does not support it."
MESSAGE_CERTIFICATE_CHANGED = (
    "The KVM's certificate changed. Reset the pinned certificate in Settings if expected."
)
MESSAGE_UNEXPECTED_RESPONSE = "Unexpected response from the KVM."


class KvmError(Exception):
    """An error whose message can be shown directly to the user."""


@dataclass(frozen=True)
class KvmPort:
    """One downstream port on the KVM's built-in switch."""

    port_index: int
    port_id: str
    device_name: str
    has_video_link: bool

    def display_name(self, settings: Settings) -> str:
        """Pick the name to show: KVM name, then local name, then "Port N"."""
        if self.device_name:
            return self.device_name
        return settings.local_name(self.port_index) or f"Port {self.port_index + 1}"


@dataclass(frozen=True)
class KvmSwitchState:
    """Snapshot of the switch: which port is active and what ports exist."""

    active_port_index: int
    ports: tuple[KvmPort, ...]

    def active_port(self) -> KvmPort | None:
        """Return the active port, if it is in the port list."""
        if 0 <= self.active_port_index < len(self.ports):
            return self.ports[self.active_port_index]
        return None


def certificate_fingerprint(certificate_der: bytes) -> str:
    """SHA-256 of a DER certificate as lowercase hex, matching the macOS and Windows apps."""
    return hashlib.sha256(certificate_der).hexdigest()


def parse_switch_state(switch_result: Any, web_ui_port_names: list[str]) -> KvmSwitchState:
    """Build a KvmSwitchState from the "result" of GET /api/switch."""
    try:
        active_port_index = int(switch_result["summary"]["active_port"])
        raw_ports = switch_result["model"]["ports"]
        video_links = switch_result.get("video", {}).get("links", [])
        ports = []
        for port_index, raw_port in enumerate(raw_ports):
            switch_port_name = str(raw_port.get("name") or "").strip()
            web_ui_port_name = (
                str(web_ui_port_names[port_index]).strip()
                if port_index < len(web_ui_port_names)
                else ""
            )
            ports.append(
                KvmPort(
                    port_index=port_index,
                    port_id=str(raw_port.get("id", port_index + 1)),
                    device_name=switch_port_name or web_ui_port_name,
                    has_video_link=(
                        bool(video_links[port_index]) if port_index < len(video_links) else True
                    ),
                )
            )
    except (KeyError, TypeError, ValueError, AttributeError) as parse_error:
        raise KvmError(MESSAGE_UNEXPECTED_RESPONSE) from parse_error
    return KvmSwitchState(active_port_index=active_port_index, ports=tuple(ports))


def unwrap_result(status_code: int, response_body: bytes) -> Any:
    """kvmd wraps every reply as {"ok": bool, "result": ...}; return "result" or raise."""
    try:
        envelope = json.loads(response_body)
    except ValueError as decode_error:
        if status_code >= 400:
            raise KvmError(f"KVM error: HTTP {status_code}") from decode_error
        raise KvmError(MESSAGE_UNEXPECTED_RESPONSE) from decode_error

    if not isinstance(envelope, dict):
        raise KvmError(MESSAGE_UNEXPECTED_RESPONSE)
    result_value = envelope.get("result", {})
    if envelope.get("ok") is not True:
        error_message = (
            result_value.get("error_msg") if isinstance(result_value, dict) else None
        ) or f"HTTP {status_code}"
        raise KvmError(f"KVM error: {error_message}")
    return result_value


class KvmClient:
    """Logs in, caches the session token and talks to one KVM.

    The KVM's self-signed certificate is pinned on first use: its SHA-256 is stored in
    settings and any later certificate that differs is refused.
    """

    def __init__(self, settings: Settings, password_provider: Callable[[], str | None]):
        self.settings = settings
        self._password_provider = password_provider
        self._session_token: str | None = None
        self._lock = threading.Lock()

    def forget_session(self) -> None:
        """Drop the cached token so the next request logs in again."""
        self._session_token = None

    def fetch_switch_state(self) -> KvmSwitchState:
        """Return the current switch state, with port names from the web UI config."""
        with self._lock:
            switch_result = self._authorized_request("GET", "/api/switch")
            # Names are cosmetic, so a failed config read should not block switching.
            try:
                web_ui_port_names = self._fetch_web_ui_port_names()
            except KvmError:
                web_ui_port_names = []
            return parse_switch_state(switch_result, web_ui_port_names)

    def set_active_port(self, port_index: int) -> None:
        """Switch to a zero-based port and give the switch a moment to settle."""
        with self._lock:
            self._authorized_request("POST", f"/api/switch/set_active?port={int(port_index)}")
        time.sleep(SWITCH_SETTLE_SECONDS)

    def _fetch_web_ui_port_names(self) -> list[str]:
        config_result = self._authorized_request("GET", "/api/system/get_config")
        web_ui_config = config_result.get("config", {}) if isinstance(config_result, dict) else {}
        port_names = web_ui_config.get("port_names", []) if isinstance(web_ui_config, dict) else []
        return [str(name) for name in port_names] if isinstance(port_names, list) else []

    # ---- Request plumbing ----

    def _authorized_request(self, method: str, path_and_query: str) -> Any:
        """Send with the cached token, logging in first or again after a 401/403."""
        if self._session_token is None:
            self._session_token = self._login()
        status_code, response_body = self._send(method, path_and_query, token=self._session_token)
        if status_code in (401, 403):
            self._session_token = self._login()
            status_code, response_body = self._send(
                method, path_and_query, token=self._session_token
            )
        return unwrap_result(status_code, response_body)

    def _login(self) -> str:
        password = self._password_provider()
        if not password:
            raise KvmError(MESSAGE_NOT_CONFIGURED)
        form_body = urllib.parse.urlencode({"user": ADMIN_USER_NAME, "passwd": password})
        status_code, response_body = self._send(
            "POST",
            "/api/auth/login",
            body=form_body.encode("utf-8"),
            content_type="application/x-www-form-urlencoded",
        )
        if status_code in (401, 403):
            raise KvmError(MESSAGE_LOGIN_REJECTED)
        login_result = unwrap_result(status_code, response_body)
        if isinstance(login_result, dict):
            token = login_result.get("token")
            if isinstance(token, str) and token:
                return token
            if "need_2fa" in login_result or "2fa" in login_result:
                raise KvmError(MESSAGE_TWO_FACTOR_REQUIRED)
        raise KvmError(MESSAGE_UNEXPECTED_RESPONSE)

    def _send(
        self,
        method: str,
        path_and_query: str,
        *,
        token: str | None = None,
        body: bytes | None = None,
        content_type: str | None = None,
    ) -> tuple[int, bytes]:
        host = self.settings.host
        if not host:
            raise KvmError(MESSAGE_NOT_CONFIGURED)

        # Verification is done by fingerprint pinning below, not by a CA chain: the KVM's
        # certificate is self-signed.
        tls_context = ssl.create_default_context()
        tls_context.check_hostname = False
        tls_context.verify_mode = ssl.CERT_NONE

        headers = {"Accept": "application/json"}
        if token:
            headers["token"] = token
        if content_type:
            headers["Content-Type"] = content_type

        connection = http.client.HTTPSConnection(
            host, timeout=REQUEST_TIMEOUT_SECONDS, context=tls_context
        )
        try:
            connection.connect()
            self._verify_pinned_certificate(connection.sock.getpeercert(binary_form=True))
            connection.request(method, path_and_query, body=body, headers=headers)
            response = connection.getresponse()
            return response.status, response.read()
        except (socket.timeout, TimeoutError) as timeout_error:
            raise KvmError("The KVM did not respond in time.") from timeout_error
        except (OSError, http.client.HTTPException) as connection_error:
            raise KvmError(f"Could not reach the KVM: {connection_error}") from connection_error
        finally:
            connection.close()

    def _verify_pinned_certificate(self, certificate_der: bytes | None) -> None:
        if not certificate_der:
            raise KvmError(MESSAGE_CERTIFICATE_CHANGED)
        fingerprint = certificate_fingerprint(certificate_der)
        pinned_fingerprint = self.settings.pinned_certificate_fingerprint
        if pinned_fingerprint is None:
            self.settings.pinned_certificate_fingerprint = fingerprint
            self.settings.save()
        elif pinned_fingerprint != fingerprint:
            raise KvmError(MESSAGE_CERTIFICATE_CHANGED)

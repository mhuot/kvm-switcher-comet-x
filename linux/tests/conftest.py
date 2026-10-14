"""Shared pytest fixtures: a local HTTPS server that answers like a GLKVM from fixture files."""

from __future__ import annotations

import json
import ssl
import threading
from dataclasses import dataclass, field
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from typing import Callable
from urllib.parse import urlsplit

import pytest
import trustme

FIXTURES_DIRECTORY = Path(__file__).resolve().parents[2] / "fixtures"


def fixture_text(file_name: str) -> str:
    """Return the text of a shared fixture file."""
    return (FIXTURES_DIRECTORY / file_name).read_text(encoding="utf-8")


@dataclass
class RecordedRequest:
    """One request the fake KVM received."""

    method: str
    path: str
    query: str
    token: str | None
    body: str


@dataclass
class FakeKvm:
    """Routes by path to (status, fixture file); `responder` overrides routing entirely."""

    routes: dict[str, tuple[int, str]] = field(
        default_factory=lambda: {
            "/api/auth/login": (200, "login_success.json"),
            "/api/switch": (200, "switch_state.json"),
            "/api/system/get_config": (200, "get_config.json"),
            "/api/switch/set_active": (200, "set_active_success.json"),
        }
    )
    responder: Callable[[RecordedRequest], tuple[int, str]] | None = None
    requests: list[RecordedRequest] = field(default_factory=list)
    host: str = ""

    def requests_to(self, path: str) -> list[RecordedRequest]:
        """Requests received for one path."""
        return [recorded for recorded in self.requests if recorded.path == path]

    def respond(self, recorded: RecordedRequest) -> tuple[int, str]:
        """Return (status, body text) for a request."""
        self.requests.append(recorded)
        if self.responder is not None:
            return self.responder(recorded)
        status_code, file_name = self.routes.get(recorded.path, (404, ""))
        return status_code, fixture_text(file_name) if file_name else ""


def _make_handler(kvm_backend: FakeKvm):
    class FakeKvmHandler(BaseHTTPRequestHandler):
        """Serves the FakeKvm routes."""

        def _serve(self) -> None:
            url_parts = urlsplit(self.path)
            body_length = int(self.headers.get("Content-Length") or 0)
            recorded = RecordedRequest(
                method=self.command,
                path=url_parts.path,
                query=url_parts.query,
                token=self.headers.get("token"),
                body=self.rfile.read(body_length).decode("utf-8"),
            )
            status_code, response_text = kvm_backend.respond(recorded)
            response_bytes = response_text.encode("utf-8")
            self.send_response(status_code)
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(response_bytes)))
            self.end_headers()
            self.wfile.write(response_bytes)

        do_GET = _serve  # pylint: disable=invalid-name
        do_POST = _serve  # pylint: disable=invalid-name

        def log_message(self, *_args) -> None:  # pylint: disable=arguments-differ
            """Keep test output quiet."""

    return FakeKvmHandler


class QuietHTTPSServer(ThreadingHTTPServer):
    """Does not print tracebacks when a client aborts the TLS handshake (pinning tests do)."""

    def handle_error(self, request, client_address) -> None:
        """Ignore per-connection errors."""


def _start_https_server(kvm_backend: FakeKvm, certificate_authority: trustme.CA):
    server_certificate = certificate_authority.issue_cert("127.0.0.1", "localhost")
    tls_context = ssl.create_default_context(ssl.Purpose.CLIENT_AUTH)
    server_certificate.configure_cert(tls_context)
    https_server = QuietHTTPSServer(("127.0.0.1", 0), _make_handler(kvm_backend))
    https_server.socket = tls_context.wrap_socket(https_server.socket, server_side=True)
    threading.Thread(
        target=https_server.serve_forever, kwargs={"poll_interval": 0.05}, daemon=True
    ).start()
    kvm_backend.host = f"127.0.0.1:{https_server.server_address[1]}"
    return https_server


@pytest.fixture(name="fake_kvm")
def fixture_fake_kvm():
    """A FakeKvm served over HTTPS with its own certificate; host is in fake_kvm.host."""
    kvm = FakeKvm()
    https_server = _start_https_server(kvm, trustme.CA())
    yield kvm
    https_server.shutdown()
    https_server.server_close()


@pytest.fixture(name="second_fake_kvm")
def fixture_second_fake_kvm():
    """Another KVM with a different certificate, for certificate-pinning tests."""
    kvm = FakeKvm()
    https_server = _start_https_server(kvm, trustme.CA())
    yield kvm
    https_server.shutdown()
    https_server.server_close()


@pytest.fixture(name="isolated_config", autouse=True)
def fixture_isolated_config(tmp_path, monkeypatch):
    """Keep settings, autostart entries and env passwords away from the real user's files."""
    monkeypatch.setenv("XDG_CONFIG_HOME", str(tmp_path / "config"))
    monkeypatch.delenv("KVM_SWITCHER_PASSWORD", raising=False)
    return tmp_path


def load_cases() -> list[dict]:
    """Return the cross-platform contract cases."""
    return json.loads(fixture_text("cases.json"))["cases"]

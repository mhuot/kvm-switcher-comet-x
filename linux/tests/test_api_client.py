"""KvmClient tests against a real local HTTPS server serving the shared fixtures."""

from __future__ import annotations

import pytest
from conftest import fixture_text, load_cases

from kvm_switcher import api_client
from kvm_switcher.api_client import KvmClient, KvmError
from kvm_switcher.settings import Settings

TEST_PASSWORD = "correct horse"


@pytest.fixture(name="no_settle_delay", autouse=True)
def fixture_no_settle_delay(monkeypatch):
    """Skip the post-switch pause so tests stay fast."""
    monkeypatch.setattr(api_client, "SWITCH_SETTLE_SECONDS", 0)


def make_client(host: str, password: str | None = TEST_PASSWORD, settings=None) -> KvmClient:
    """Client for a fake KVM with a fixed password."""
    client_settings = settings or Settings(host=host)
    return KvmClient(client_settings, lambda: password)


@pytest.mark.parametrize("contract_case", load_cases(), ids=lambda case: case["name"])
def test_shared_fixture_cases(fake_kvm, contract_case):
    """Every platform must parse the shared fixtures to the same result."""
    fake_kvm.routes["/api/switch"] = (200, contract_case["switch"])
    fake_kvm.routes["/api/system/get_config"] = (200, contract_case["config"])
    client = make_client(fake_kvm.host)

    switch_state = client.fetch_switch_state()

    expected = contract_case["expected"]
    assert switch_state.active_port_index == expected["active_port_index"]
    assert [port.display_name(client.settings) for port in switch_state.ports] == [
        port["display_name"] for port in expected["ports"]
    ]
    assert [port.has_video_link for port in switch_state.ports] == [
        port["has_video_link"] for port in expected["ports"]
    ]


def test_logs_in_once_and_sends_token_header(fake_kvm):
    """The token is cached and sent as the `token` header."""
    client = make_client(fake_kvm.host)
    client.fetch_switch_state()
    client.fetch_switch_state()

    assert len(fake_kvm.requests_to("/api/auth/login")) == 1
    switch_requests = fake_kvm.requests_to("/api/switch")
    assert len(switch_requests) == 2
    assert switch_requests[0].token == "0" * 64


def test_login_form_encodes_special_characters(fake_kvm):
    """Passwords with &, +, = and non-ASCII survive form encoding."""
    make_client(fake_kvm.host, password="p&ss+word= ü").fetch_switch_state()

    (login_request,) = fake_kvm.requests_to("/api/auth/login")
    assert login_request.method == "POST"
    assert login_request.body == "user=admin&passwd=p%26ss%2Bword%3D+%C3%BC"


def test_expired_token_triggers_one_relogin_and_retry(fake_kvm):
    """A 401 leads to exactly one new login and a retried request."""
    switch_call_count = 0

    def respond(recorded):
        nonlocal switch_call_count
        if recorded.path == "/api/switch":
            switch_call_count += 1
            if switch_call_count == 1:
                return 401, fixture_text("error_unauthorized.json")
            return 200, fixture_text("switch_state.json")
        file_name = {
            "/api/auth/login": "login_success.json",
            "/api/system/get_config": "get_config.json",
        }[recorded.path]
        return 200, fixture_text(file_name)

    fake_kvm.responder = respond
    switch_state = make_client(fake_kvm.host).fetch_switch_state()

    assert switch_state.active_port_index == 2
    assert len(fake_kvm.requests_to("/api/auth/login")) == 2


def test_wrong_password_reports_login_rejected(fake_kvm):
    """A 403 from login means the password is wrong."""
    fake_kvm.routes["/api/auth/login"] = (403, "error_forbidden.json")
    with pytest.raises(KvmError, match="Login rejected"):
        make_client(fake_kvm.host).fetch_switch_state()


def test_missing_password_reports_not_configured_without_network(fake_kvm):
    """No password means no request is sent."""
    with pytest.raises(KvmError, match="Set the KVM address"):
        make_client(fake_kvm.host, password=None).fetch_switch_state()
    assert not fake_kvm.requests


def test_empty_host_reports_not_configured():
    """No host configured is reported before any network access."""
    with pytest.raises(KvmError, match="Set the KVM address"):
        make_client("").fetch_switch_state()


def test_set_active_port_sends_zero_based_index(fake_kvm):
    """Ports are zero-based on the wire."""
    make_client(fake_kvm.host).set_active_port(3)

    (set_active_request,) = fake_kvm.requests_to("/api/switch/set_active")
    assert set_active_request.method == "POST"
    assert set_active_request.query == "port=3"


def test_config_failure_falls_back_to_default_names(fake_kvm):
    """Port names are cosmetic; a failing get_config must not break fetching state."""
    fake_kvm.routes["/api/system/get_config"] = (500, "error_forbidden.json")
    client = make_client(fake_kvm.host)
    switch_state = client.fetch_switch_state()
    assert [port.display_name(client.settings) for port in switch_state.ports] == [
        "Port 1",
        "Port 2",
        "Port 3",
        "Port 4",
    ]


def test_local_names_fill_gaps_but_kvm_names_win(fake_kvm):
    """Local names are used only where the KVM has no name."""
    settings = Settings(host=fake_kvm.host, local_port_names=["Local One", "", "Local Three"])
    fake_kvm.routes["/api/system/get_config"] = (200, "get_config_without_port_names.json")
    client = make_client(fake_kvm.host, settings=settings)
    assert [port.display_name(settings) for port in client.fetch_switch_state().ports] == [
        "Local One",
        "Port 2",
        "Local Three",
        "Port 4",
    ]

    fake_kvm.routes["/api/system/get_config"] = (200, "get_config.json")
    assert client.fetch_switch_state().ports[0].display_name(settings) == "Workstation"


def test_api_error_message_is_surfaced(fake_kvm):
    """kvmd's error_msg is shown to the user."""
    fake_kvm.routes["/api/switch/set_active"] = (200, "error_forbidden.json")
    with pytest.raises(KvmError, match="KVM error: Forbidden"):
        make_client(fake_kvm.host).set_active_port(1)


def test_certificate_is_pinned_on_first_use_and_saved(fake_kvm):
    """The first certificate seen is pinned and persisted."""
    settings = Settings(host=fake_kvm.host)
    make_client(fake_kvm.host, settings=settings).fetch_switch_state()

    assert settings.pinned_certificate_fingerprint
    assert len(settings.pinned_certificate_fingerprint) == 64
    reloaded = Settings.load(settings.settings_path)
    assert reloaded.pinned_certificate_fingerprint == settings.pinned_certificate_fingerprint


def test_changed_certificate_is_refused(fake_kvm, second_fake_kvm):
    """A different certificate at the same address is refused, and no password is sent."""
    settings = Settings(host=fake_kvm.host)
    make_client(fake_kvm.host, settings=settings).fetch_switch_state()

    settings.host = second_fake_kvm.host  # same settings, a KVM with another certificate
    with pytest.raises(KvmError, match="certificate changed"):
        make_client(second_fake_kvm.host, settings=settings).fetch_switch_state()
    assert not second_fake_kvm.requests


def test_unreachable_kvm_reports_connection_error():
    """A closed port is reported as an unreachable KVM."""
    with pytest.raises(KvmError, match="Could not reach the KVM"):
        make_client("127.0.0.1:9").fetch_switch_state()

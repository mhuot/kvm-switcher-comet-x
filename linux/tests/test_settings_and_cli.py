"""Tests for settings persistence, autostart entries and the CLI."""

from __future__ import annotations

import pytest

from kvm_switcher import api_client, autostart, cli, credentials
from kvm_switcher.settings import (
    LABEL_STYLE_ICON_ONLY,
    LABEL_STYLE_PORT_NAME,
    Settings,
    default_settings_path,
    normalize_host,
)


@pytest.mark.parametrize(
    ("raw_host", "expected_host"),
    [
        ("  https://glkvm.local/ ", "glkvm.local"),
        ("HTTP://10.0.0.5//", "10.0.0.5"),
        ("glkvm.local", "glkvm.local"),
    ],
)
def test_normalize_host(raw_host, expected_host):
    """Schemes and trailing slashes are stripped."""
    assert normalize_host(raw_host) == expected_host


def test_settings_round_trip(isolated_config):
    """Saved settings load back identically, under XDG_CONFIG_HOME."""
    settings = Settings(
        host="glkvm.local",
        local_port_names=["A", "B"],
        label_style=LABEL_STYLE_ICON_ONLY,
        pinned_certificate_fingerprint="ab" * 32,
    )
    settings.save()

    assert default_settings_path().is_relative_to(isolated_config)
    assert Settings.load() == settings


def test_damaged_settings_fall_back_to_defaults():
    """A corrupt file never stops the app."""
    default_settings_path().parent.mkdir(parents=True)
    default_settings_path().write_text("{not json", encoding="utf-8")
    assert Settings.load() == Settings()


def test_unknown_label_style_falls_back():
    """An unknown label style from a newer/older version is ignored."""
    default_settings_path().parent.mkdir(parents=True)
    default_settings_path().write_text('{"label_style": "sparkles"}', encoding="utf-8")
    assert Settings.load().label_style == LABEL_STYLE_PORT_NAME


def test_autostart_entry_created_and_removed():
    """The XDG autostart entry is written and removed."""
    assert not autostart.is_enabled()
    autostart.set_enabled(True)
    entry_text = autostart.autostart_path().read_text(encoding="utf-8")
    assert "[Desktop Entry]" in entry_text
    assert "Exec=" in entry_text
    assert autostart.is_enabled()
    autostart.set_enabled(False)
    assert not autostart.is_enabled()


def test_environment_password_overrides_keyring(monkeypatch):
    """KVM_SWITCHER_PASSWORD is used for scripts and headless machines."""
    monkeypatch.setenv(credentials.PASSWORD_ENVIRONMENT_VARIABLE, "from-env")
    assert credentials.read_password() == "from-env"


# ---- CLI ----


@pytest.fixture(name="configured_cli")
def fixture_configured_cli(fake_kvm, monkeypatch):
    """Point the CLI at the fake KVM, never at a running tray instance."""
    Settings(host=fake_kvm.host).save()
    monkeypatch.setenv(credentials.PASSWORD_ENVIRONMENT_VARIABLE, "correct horse")
    monkeypatch.setattr(cli, "_forward_to_running_tray", lambda _port_index: False)
    monkeypatch.setattr(api_client, "SWITCH_SETTLE_SECONDS", 0)
    return fake_kvm


def test_cli_switch_uses_one_based_numbers(configured_cli, capsys):
    """`switch 4` sends zero-based port 3."""
    assert cli.main(["switch", "4"]) == 0
    (set_active_request,) = configured_cli.requests_to("/api/switch/set_active")
    assert set_active_request.query == "port=3"
    assert "Switched to" in capsys.readouterr().out


@pytest.mark.parametrize("port_number", ["0", "5"])
def test_cli_switch_rejects_out_of_range_ports(configured_cli, port_number):
    """Port 0 and ports beyond the KVM's count are usage errors."""
    assert cli.main(["switch", port_number]) == 2
    assert not configured_cli.requests_to("/api/switch/set_active")


@pytest.mark.usefixtures("configured_cli")
def test_cli_status_prints_active_port_name(capsys):
    """`status` prints the active port's display name."""
    assert cli.main(["status"]) == 0
    assert capsys.readouterr().out.strip() == "Media PC & Games"


@pytest.mark.usefixtures("configured_cli")
def test_cli_list_marks_active_port(capsys):
    """`list` shows every port with * on the active one."""
    assert cli.main(["list"]) == 0
    output_lines = capsys.readouterr().out.splitlines()
    assert output_lines[0] == "  1  Workstation"
    assert output_lines[2] == "* 3  Media PC & Games"
    assert len(output_lines) == 4


def test_cli_reports_errors_on_stderr(capsys, monkeypatch):
    """Without configuration the CLI exits 1 with a readable message."""
    monkeypatch.setattr(cli, "_forward_to_running_tray", lambda _port_index: False)
    assert cli.main(["status"]) == 1
    assert "Set the KVM address" in capsys.readouterr().err


def test_cli_switch_is_forwarded_to_running_tray(monkeypatch, capsys):
    """With the tray running, the switch goes over D-Bus instead of the network."""
    forwarded_ports = []
    monkeypatch.setattr(
        cli,
        "_forward_to_running_tray",
        lambda port_index: forwarded_ports.append(port_index) or True,
    )
    assert cli.main(["switch", "2"]) == 0
    assert forwarded_ports == [1]
    assert "running KVM Switcher" in capsys.readouterr().out

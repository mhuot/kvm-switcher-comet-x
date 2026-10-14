"""Command line entry point: `kvm-switcher [tray|switch N|status|list|self-test]`.

`kvm-switcher switch N` is what users bind to a keyboard shortcut in their desktop settings,
since Wayland does not let apps grab global hotkeys themselves. When the tray app is running
the request is forwarded to it over D-Bus so its label updates at once.
"""

from __future__ import annotations

import argparse
import sys

from kvm_switcher import APPLICATION_ID, SWITCH_PORT_ACTION, __version__, credentials
from kvm_switcher.api_client import KvmClient, KvmError
from kvm_switcher.settings import Settings


def _make_client() -> KvmClient:
    return KvmClient(Settings.load(), credentials.read_password)


def _forward_to_running_tray(port_index: int) -> bool:
    """Ask a running tray instance to switch. Returns False when none is running."""
    try:
        import gi  # pylint: disable=import-outside-toplevel

        gi.require_version("Gio", "2.0")
        from gi.repository import Gio, GLib  # pylint: disable=import-outside-toplevel

        application_flags = getattr(
            Gio.ApplicationFlags, "DEFAULT_FLAGS", Gio.ApplicationFlags.FLAGS_NONE
        )
        remote_application = Gio.Application(application_id=APPLICATION_ID, flags=application_flags)
        remote_application.register(None)
        if not remote_application.get_is_remote():
            return False
        remote_application.activate_action(SWITCH_PORT_ACTION, GLib.Variant.new_int32(port_index))
        remote_application.get_dbus_connection().flush_sync(None)
        return True
    except Exception:  # pylint: disable=broad-exception-caught
        # No GObject, no session bus, or registration refused: switch directly instead.
        return False


def _command_switch(port_number: int) -> int:
    if port_number < 1:
        print("Port numbers start at 1.", file=sys.stderr)
        return 2
    port_index = port_number - 1
    if _forward_to_running_tray(port_index):
        print(f"Asked the running KVM Switcher to switch to port {port_number}.")
        return 0

    kvm_client = _make_client()
    try:
        switch_state = kvm_client.fetch_switch_state()
        if port_index >= len(switch_state.ports):
            print(f"The KVM has only {len(switch_state.ports)} ports.", file=sys.stderr)
            return 2
        kvm_client.set_active_port(port_index)
        switch_state = kvm_client.fetch_switch_state()
    except KvmError as kvm_error:
        print(kvm_error, file=sys.stderr)
        return 1

    active_port = switch_state.active_port()
    active_name = active_port.display_name(kvm_client.settings) if active_port else "?"
    print(f"Switched to {active_name}.")
    return 0


def _command_status(list_all_ports: bool) -> int:
    kvm_client = _make_client()
    try:
        switch_state = kvm_client.fetch_switch_state()
    except KvmError as kvm_error:
        print(kvm_error, file=sys.stderr)
        return 1

    if not list_all_ports:
        active_port = switch_state.active_port()
        print(active_port.display_name(kvm_client.settings) if active_port else "?")
        return 0

    for port in switch_state.ports:
        active_marker = "*" if port.port_index == switch_state.active_port_index else " "
        video_note = "" if port.has_video_link else "  (no video)"
        print(
            f"{active_marker} {port.port_index + 1}  "
            f"{port.display_name(kvm_client.settings)}{video_note}"
        )
    return 0


def build_argument_parser() -> argparse.ArgumentParser:
    """Build the CLI parser (exposed for tests)."""
    argument_parser = argparse.ArgumentParser(
        prog="kvm-switcher",
        description="Switch ports on a GL.iNet KVM. With no command, runs the tray icon.",
    )
    argument_parser.add_argument("--version", action="version", version=__version__)
    subcommands = argument_parser.add_subparsers(dest="command", metavar="command")
    subcommands.add_parser("tray", help="run the tray icon (default)")
    switch_parser = subcommands.add_parser("switch", help="switch to port N (1-based)")
    switch_parser.add_argument("port_number", type=int, metavar="N")
    subcommands.add_parser("status", help="print the active port's name")
    subcommands.add_parser("list", help="list ports; * marks the active one")
    subcommands.add_parser(
        "self-test", help="check GTK, tray and keyring support without a KVM (for CI)"
    )
    return argument_parser


def main(argv: list[str] | None = None) -> int:
    """Run the CLI and return a process exit code."""
    parsed_arguments = build_argument_parser().parse_args(argv)
    command = parsed_arguments.command or "tray"

    if command == "switch":
        return _command_switch(parsed_arguments.port_number)
    if command in ("status", "list"):
        return _command_status(list_all_ports=command == "list")

    # The GTK pieces are imported only here so the CLI works on headless machines.
    from kvm_switcher import tray  # pylint: disable=import-outside-toplevel

    if command == "self-test":
        return tray.run_self_test()
    return tray.run_tray()

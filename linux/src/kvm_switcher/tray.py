"""The panel (tray) app: an AppIndicator icon with a port menu, built on Gtk.Application.

Gtk.Application also gives single-instance behaviour: `kvm-switcher switch N` from a
keyboard shortcut reaches the running instance through the "switch-port" action.
"""

from __future__ import annotations

import sys
import threading
from typing import Any, Callable

from kvm_switcher import APPLICATION_ID, SWITCH_PORT_ACTION, credentials
from kvm_switcher.api_client import KvmClient, KvmError, KvmPort, KvmSwitchState
from kvm_switcher.gtk_loader import GLib, Gio, Gtk, load_app_indicator
from kvm_switcher.settings import LABEL_STYLE_ICON_ONLY, LABEL_STYLE_PORT_NUMBER, Settings
from kvm_switcher.settings_dialog import SettingsDialog

BACKGROUND_REFRESH_SECONDS = 15
DEFAULT_PORT_COUNT = 4
MAXIMUM_LABEL_LENGTH = 24
ICON_CONNECTED = "video-display-symbolic"
ICON_ERROR = "dialog-warning-symbolic"
MISSING_INDICATOR_MESSAGE = (
    "KVM Switcher needs AppIndicator support. Install the Ayatana AppIndicator GObject "
    "package (e.g. gir1.2-ayatanaappindicator3-0.1 or libayatana-appindicator-gtk3) and, "
    "on GNOME, the 'AppIndicator and KStatusNotifierItem Support' extension."
)


def run_in_background(work: Callable[[], Any], on_done: Callable[[Any, Exception | None], None]):
    """Run blocking network work off the GTK thread and deliver the result back on it."""

    def worker() -> None:
        try:
            work_result, work_error = work(), None
        except KvmError as kvm_error:
            work_result, work_error = None, kvm_error
        GLib.idle_add(lambda: on_done(work_result, work_error) and False)

    threading.Thread(target=worker, daemon=True).start()


def panel_label(switch_state: KvmSwitchState | None, settings: Settings) -> str:
    """Text shown beside the panel icon for the configured label style."""
    if switch_state is None or settings.label_style == LABEL_STYLE_ICON_ONLY:
        return ""
    if settings.label_style == LABEL_STYLE_PORT_NUMBER:
        return str(switch_state.active_port_index + 1)
    active_port = switch_state.active_port()
    if active_port is None:
        return str(switch_state.active_port_index + 1)
    port_name = active_port.display_name(settings)
    if len(port_name) > MAXIMUM_LABEL_LENGTH:
        return port_name[: MAXIMUM_LABEL_LENGTH - 1] + "…"
    return port_name


def menu_item_label(port: KvmPort, settings: Settings) -> str:
    """Menu text with a "_1" mnemonic; underscores in names are escaped."""
    escaped_name = port.display_name(settings).replace("_", "__")
    mnemonic = f"_{port.port_index + 1}   " if port.port_index < 9 else ""
    video_note = "" if port.has_video_link else "  (no video)"
    return f"{mnemonic}{escaped_name}{video_note}"


class TrayApplication(Gtk.Application):
    """Owns the indicator, the port menu and the settings dialog."""

    def __init__(self, app_indicator_module):
        application_flags = getattr(
            Gio.ApplicationFlags, "DEFAULT_FLAGS", Gio.ApplicationFlags.FLAGS_NONE
        )
        super().__init__(application_id=APPLICATION_ID, flags=application_flags)
        self._app_indicator_module = app_indicator_module
        self.settings = Settings.load()
        self.kvm_client = KvmClient(self.settings, credentials.read_password)
        self.switch_state: KvmSwitchState | None = None
        self.last_error_message: str | None = None
        self._indicator = None
        self._settings_dialog: SettingsDialog | None = None
        self._switch_in_progress = False
        self._refresh_in_progress = False
        self._updating_menu = False

    # ---- Gtk.Application lifecycle ----

    def do_startup(self, *args):  # pylint: disable=arguments-differ
        """Create the indicator and the D-Bus action; keep running without windows."""
        Gtk.Application.do_startup(self, *args)
        self.hold()

        switch_action = Gio.SimpleAction.new(SWITCH_PORT_ACTION, GLib.VariantType.new("i"))
        switch_action.connect(
            "activate", lambda _action, parameter: self.switch_to_port(parameter.get_int32())
        )
        self.add_action(switch_action)

        indicator_module = self._app_indicator_module
        self._indicator = indicator_module.Indicator.new(
            APPLICATION_ID, ICON_CONNECTED, indicator_module.IndicatorCategory.HARDWARE
        )
        self._indicator.set_status(indicator_module.IndicatorStatus.ACTIVE)
        self._indicator.set_title("KVM Switcher")
        self.rebuild_menu()

        GLib.timeout_add_seconds(BACKGROUND_REFRESH_SECONDS, self._periodic_refresh)
        self.refresh_state()

    def do_activate(self, *_args):  # pylint: disable=arguments-differ
        """Show settings on first run (nothing configured yet)."""
        if not self.settings.host or not credentials.read_password():
            self.show_settings()

    # ---- State ----

    def _periodic_refresh(self) -> bool:
        self.refresh_state()
        return GLib.SOURCE_CONTINUE

    def refresh_state(self, on_finished: Callable[[], None] | None = None) -> None:
        """Re-read the switch state in the background and update the panel."""
        if self._refresh_in_progress:
            if on_finished:
                GLib.timeout_add(200, lambda: self.refresh_state(on_finished) and False)
            return
        self._refresh_in_progress = True

        def apply_result(fetched_state: KvmSwitchState | None, fetch_error: Exception | None):
            self._refresh_in_progress = False
            if fetch_error is None:
                self.switch_state, self.last_error_message = fetched_state, None
            else:
                self.last_error_message = str(fetch_error)
            self.update_indicator()
            self.rebuild_menu()
            if on_finished:
                on_finished()

        run_in_background(self.kvm_client.fetch_switch_state, apply_result)

    def switch_to_port(self, port_index: int) -> None:
        """Switch ports (from the menu or a forwarded `kvm-switcher switch N`)."""
        if self._switch_in_progress:
            return
        if self.switch_state is not None and port_index >= len(self.switch_state.ports):
            return
        self._switch_in_progress = True
        self.update_indicator()

        def apply_result(_unused_result, switch_error: Exception | None) -> None:
            self._switch_in_progress = False
            if switch_error is not None:
                self.last_error_message = str(switch_error)
                self.update_indicator()
            self.refresh_state()

        run_in_background(lambda: self.kvm_client.set_active_port(port_index), apply_result)

    # ---- Panel ----

    def update_indicator(self) -> None:
        """Set the icon and label from the current state."""
        if self._indicator is None:
            return
        if self.last_error_message:
            self._indicator.set_icon_full(ICON_ERROR, self.last_error_message)
            self._indicator.set_label("", "")
            return
        self._indicator.set_icon_full(ICON_CONNECTED, "KVM Switcher")
        label_text = (
            "…" if self._switch_in_progress else panel_label(self.switch_state, self.settings)
        )
        self._indicator.set_label(label_text, label_text)

    def build_menu(self) -> Gtk.Menu:
        """Build the port menu from the current state (also used by the self-test)."""
        port_menu = Gtk.Menu()
        if self.last_error_message:
            port_menu.append(Gtk.MenuItem(label=self.last_error_message, sensitive=False))
            port_menu.append(Gtk.SeparatorMenuItem())

        ports = (
            self.switch_state.ports
            if self.switch_state
            else tuple(KvmPort(index, "", "", True) for index in range(DEFAULT_PORT_COUNT))
        )
        self._updating_menu = True
        for port in ports:
            port_item = Gtk.CheckMenuItem(
                label=menu_item_label(port, self.settings), use_underline=True
            )
            port_item.set_draw_as_radio(True)
            port_item.set_active(
                self.switch_state is not None
                and port.port_index == self.switch_state.active_port_index
            )
            port_item.set_sensitive(self.switch_state is not None)
            port_item.connect("activate", self._port_item_activated, port.port_index)
            port_menu.append(port_item)
        self._updating_menu = False

        port_menu.append(Gtk.SeparatorMenuItem())
        for item_label, item_handler in (
            ("_Refresh", lambda _item: self.refresh_state()),
            ("_Open KVM Web UI", lambda _item: self.open_web_ui()),
            ("_Settings…", lambda _item: self.show_settings()),
        ):
            action_item = Gtk.MenuItem(label=item_label, use_underline=True)
            action_item.connect("activate", item_handler)
            port_menu.append(action_item)
        port_menu.append(Gtk.SeparatorMenuItem())
        quit_item = Gtk.MenuItem(label="_Quit KVM Switcher", use_underline=True)
        quit_item.connect("activate", lambda _item: self.quit())
        port_menu.append(quit_item)
        port_menu.show_all()
        return port_menu

    def rebuild_menu(self) -> None:
        """Replace the indicator's menu (AppIndicator menus cannot report being opened)."""
        if self._indicator is not None:
            self._indicator.set_menu(self.build_menu())

    def _port_item_activated(self, _item, port_index: int) -> None:
        if not self._updating_menu:
            self.switch_to_port(port_index)

    def open_web_ui(self) -> None:
        """Open the KVM's own web page in the default browser."""
        if self.settings.host:
            Gio.AppInfo.launch_default_for_uri(f"https://{self.settings.host}/", None)

    # ---- Settings ----

    def _settings_saved(self) -> None:
        self.kvm_client.forget_session()
        self.update_indicator()
        self.refresh_state()

    def _test_connection(self, show_result: Callable[[str], None]) -> None:
        self.kvm_client.forget_session()

        def report() -> None:
            if self.last_error_message:
                show_result(f"Failed: {self.last_error_message}")
                return
            port_count = len(self.switch_state.ports) if self.switch_state else 0
            active_port = self.switch_state.active_port() if self.switch_state else None
            active_name = active_port.display_name(self.settings) if active_port else "?"
            show_result(f"Connected. {port_count} ports, active: {active_name}.")

        self.refresh_state(on_finished=report)

    def show_settings(self) -> None:
        """Show (or raise) the settings dialog."""
        if self._settings_dialog is None:
            self._settings_dialog = SettingsDialog(
                self.settings, self._settings_saved, self._test_connection
            )
            self._settings_dialog.set_application(self)
            self._settings_dialog.connect(
                "delete-event", lambda dialog, _event: dialog.hide_on_delete()
            )
        self._settings_dialog.show_all()
        self._settings_dialog.present()


def run_tray() -> int:
    """Start the panel app; returns the process exit code."""
    app_indicator_module = load_app_indicator()
    if app_indicator_module is None:
        print(MISSING_INDICATOR_MESSAGE, file=sys.stderr)
        return 1
    return TrayApplication(app_indicator_module).run([sys.argv[0]])


def run_self_test() -> int:
    """Check GTK, AppIndicator, menu/dialog construction and the keyring without a KVM.

    Needs a display (CI runs it under xvfb-run). Prints PASS/FAIL/SKIP lines.
    """
    results: list[tuple[str, str, str]] = []

    def check(check_name: str, check_action: Callable[[], str | None]) -> None:
        try:
            skip_reason = check_action()
            results.append(("SKIP" if skip_reason else "PASS", check_name, skip_reason or ""))
        except Exception as check_error:  # pylint: disable=broad-exception-caught
            results.append(("FAIL", check_name, f"{type(check_error).__name__}: {check_error}"))

    app_indicator_module = load_app_indicator()

    def check_indicator() -> None:
        if app_indicator_module is None:
            raise RuntimeError(MISSING_INDICATOR_MESSAGE)

    def check_menu_and_labels() -> None:
        tray_application = TrayApplication(app_indicator_module)
        tray_application.switch_state = KvmSwitchState(
            active_port_index=2,
            ports=tuple(
                KvmPort(index, f"1.{index + 1}", name, index != 3)
                for index, name in enumerate(["Work_station", "Laptop", "Media PC", ""])
            ),
        )
        port_menu = tray_application.build_menu()
        port_item_labels = [item.get_label() for item in port_menu.get_children()[:4]]
        if port_item_labels[0] != "_1   Work__station":
            raise AssertionError(f"Unexpected menu labels {port_item_labels}")
        if panel_label(tray_application.switch_state, tray_application.settings) != "Media PC":
            raise AssertionError("Unexpected panel label")

    def check_settings_dialog() -> None:
        settings_dialog = SettingsDialog(Settings(), lambda: None, lambda _show: None)
        _minimum_size, natural_size = settings_dialog.get_preferred_size()
        settings_dialog.destroy()
        if natural_size.height < 200:
            raise AssertionError(f"Settings dialog collapsed to {natural_size.height}px")

    def check_icon_theme() -> str | None:
        if Gtk.IconTheme.get_default().has_icon(ICON_CONNECTED):
            return None
        return f"icon {ICON_CONNECTED} not in the current theme (desktop themes provide it)"

    def check_keyring() -> str | None:
        probe_account = "self-test"
        probe_password = "self-test p&ss ü"
        try:
            credentials.save_password(probe_password, account=probe_account)
        except credentials.KeyringUnavailableError as keyring_error:
            return f"no Secret Service available ({keyring_error})"
        try:
            read_back = credentials.read_password(account=probe_account)
            if read_back != probe_password:
                raise AssertionError(f"read back {read_back!r}")
        finally:
            credentials.save_password(None, account=probe_account)
        return None

    check("AppIndicator available", check_indicator)
    if app_indicator_module is not None:
        check("build menu and panel label", check_menu_and_labels)
    check("build settings dialog", check_settings_dialog)
    check("icon theme", check_icon_theme)
    check("keyring round trip", check_keyring)

    for outcome, check_name, detail in results:
        print(f"{outcome} {check_name}" + (f": {detail}" if detail else ""))
    any_failed = any(outcome == "FAIL" for outcome, _name, _detail in results)
    print("RESULT FAIL" if any_failed else "RESULT PASS")
    return 1 if any_failed else 0

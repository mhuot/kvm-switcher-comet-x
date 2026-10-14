"""GTK settings dialog."""

from __future__ import annotations

from typing import Callable

from kvm_switcher import autostart, credentials
from kvm_switcher.gtk_loader import Gtk
from kvm_switcher.settings import (
    LABEL_STYLE_ICON_ONLY,
    LABEL_STYLE_PORT_NAME,
    LABEL_STYLE_PORT_NUMBER,
    Settings,
    normalize_host,
)

EDITABLE_PORT_COUNT = 4
_LABEL_STYLE_TITLES = {
    LABEL_STYLE_PORT_NAME: "Port name",
    LABEL_STYLE_PORT_NUMBER: "Port number",
    LABEL_STYLE_ICON_ONLY: "Icon only",
}
_SHORTCUT_HINT = (
    "Keyboard shortcuts: in your desktop's keyboard settings, add custom shortcuts that run\n"
    "<tt>kvm-switcher switch 1</tt>, <tt>kvm-switcher switch 2</tt> and so on."
)


class SettingsDialog(Gtk.Dialog):  # pylint: disable=too-few-public-methods
    """Edits host, password, local port names, label style and autostart."""

    def __init__(
        self,
        settings: Settings,
        on_save: Callable[[], None],
        on_test_connection: Callable[[Callable[[str], None]], None],
    ):
        super().__init__(title="KVM Switcher Settings")
        self.set_default_size(460, -1)
        self.set_resizable(False)
        self._settings = settings
        self._on_save = on_save
        self._on_test_connection = on_test_connection

        self._host_entry = Gtk.Entry(placeholder_text="glkvm.local or IP address", hexpand=True)
        self._password_entry = Gtk.Entry(visibility=False, input_purpose=Gtk.InputPurpose.PASSWORD)
        self._status_label = Gtk.Label(xalign=0, wrap=True, max_width_chars=50)
        self._status_label.get_style_context().add_class("dim-label")
        self._test_button = Gtk.Button(label="Test Connection")
        self._port_name_entries = [
            Gtk.Entry(placeholder_text=f"Port {port_number}")
            for port_number in range(1, EDITABLE_PORT_COUNT + 1)
        ]
        self._label_style_combo = Gtk.ComboBoxText()
        for style_id, style_title in _LABEL_STYLE_TITLES.items():
            self._label_style_combo.append(style_id, style_title)
        self._autostart_check = Gtk.CheckButton(label="Start KVM Switcher when I log in")

        self._build_layout()
        self._load_values()

    def _build_layout(self) -> None:
        layout_grid = Gtk.Grid(column_spacing=12, row_spacing=8, margin=16)
        next_row = 0

        def add_heading(heading_text: str) -> None:
            nonlocal next_row
            heading_label = Gtk.Label(xalign=0)
            heading_label.set_markup(f"<b>{heading_text}</b>")
            layout_grid.attach(heading_label, 0, next_row, 2, 1)
            next_row += 1

        def add_row(label_text: str, row_widget: Gtk.Widget) -> None:
            nonlocal next_row
            layout_grid.attach(Gtk.Label(label=label_text, xalign=0), 0, next_row, 1, 1)
            layout_grid.attach(row_widget, 1, next_row, 1, 1)
            next_row += 1

        def add_full_width(row_widget: Gtk.Widget) -> None:
            nonlocal next_row
            layout_grid.attach(row_widget, 0, next_row, 2, 1)
            next_row += 1

        add_heading("GL.iNet KVM")
        add_row("Address", self._host_entry)
        add_row("Admin password", self._password_entry)
        reset_certificate_button = Gtk.Button(label="Reset Pinned Certificate")
        reset_certificate_button.connect("clicked", self._reset_certificate)
        self._test_button.connect("clicked", self._test_connection)
        connection_buttons = Gtk.Box(spacing=8)
        connection_buttons.pack_start(self._test_button, False, False, 0)
        connection_buttons.pack_start(reset_certificate_button, False, False, 0)
        add_row("", connection_buttons)
        add_row("", self._status_label)

        add_heading("Port names (used when the KVM has none set)")
        for port_number, name_entry in enumerate(self._port_name_entries, start=1):
            add_row(f"Port {port_number}", name_entry)

        add_heading("General")
        add_row("Panel shows", self._label_style_combo)
        add_full_width(self._autostart_check)
        shortcut_hint = Gtk.Label(xalign=0, wrap=True)
        shortcut_hint.set_markup(_SHORTCUT_HINT)
        shortcut_hint.get_style_context().add_class("dim-label")
        add_full_width(shortcut_hint)

        self.get_content_area().add(layout_grid)
        self.add_button("Close", Gtk.ResponseType.CLOSE)
        save_button = self.add_button("Save", Gtk.ResponseType.APPLY)
        save_button.get_style_context().add_class("suggested-action")
        self.set_default_response(Gtk.ResponseType.APPLY)
        self.connect("response", self._handle_response)
        self.show_all()

    def _load_values(self) -> None:
        self._host_entry.set_text(self._settings.host)
        self._password_entry.set_text(credentials.read_password() or "")
        for port_index, name_entry in enumerate(self._port_name_entries):
            name_entry.set_text(self._settings.local_name(port_index))
        self._label_style_combo.set_active_id(self._settings.label_style)
        self._autostart_check.set_active(autostart.is_enabled())

    def _save_values(self) -> bool:
        self._settings.host = normalize_host(self._host_entry.get_text())
        self._settings.local_port_names = [
            name_entry.get_text().strip() for name_entry in self._port_name_entries
        ]
        self._settings.label_style = (
            self._label_style_combo.get_active_id() or LABEL_STYLE_PORT_NAME
        )
        try:
            self._settings.save()
            credentials.save_password(self._password_entry.get_text())
            autostart.set_enabled(self._autostart_check.get_active())
        except (OSError, credentials.KeyringUnavailableError) as save_error:
            self._status_label.set_text(f"Could not save: {save_error}")
            return False
        self._host_entry.set_text(self._settings.host)
        self._on_save()
        return True

    def _handle_response(self, _dialog, response_id: int) -> None:
        if response_id == Gtk.ResponseType.APPLY:
            if self._save_values():
                self._status_label.set_text("Saved.")
        else:
            self.hide()

    def _reset_certificate(self, _button) -> None:
        self._settings.pinned_certificate_fingerprint = None
        self._settings.save()
        self._status_label.set_text("Certificate will be re-pinned on next connection.")

    def _test_connection(self, _button) -> None:
        if not self._save_values():
            return
        self._test_button.set_sensitive(False)
        self._status_label.set_text("Connecting…")

        def show_result(result_text: str) -> None:
            self._status_label.set_text(result_text)
            self._test_button.set_sensitive(True)

        self._on_test_connection(show_result)

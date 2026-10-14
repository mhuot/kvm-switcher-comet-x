"""Loads GTK 3 and an AppIndicator implementation through GObject introspection."""

from __future__ import annotations

import importlib

import gi

gi.require_version("Gtk", "3.0")
from gi.repository import GLib, Gio, Gtk  # pylint: disable=wrong-import-position


def load_app_indicator():
    """Return the AppIndicator module: Ayatana (Debian/Ubuntu/Fedora/Arch) or legacy.

    Returns None when neither is installed; the caller explains what to install.
    """
    for module_name in ("AyatanaAppIndicator3", "AppIndicator3"):
        try:
            gi.require_version(module_name, "0.1")
            return importlib.import_module(f"gi.repository.{module_name}")
        except (ValueError, ImportError):
            continue
    return None


__all__ = ["GLib", "Gio", "Gtk", "load_app_indicator"]

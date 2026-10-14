"""Start-at-login via an XDG autostart desktop entry (works on GNOME, KDE, Xfce, etc.)."""

from __future__ import annotations

import shlex
import shutil
import sys
from pathlib import Path

from kvm_switcher import APPLICATION_ID
from kvm_switcher.settings import config_directory


def autostart_path() -> Path:
    """Return ~/.config/autostart/net.mhuot.KVMSwitcher.desktop (XDG_CONFIG_HOME aware)."""
    return config_directory().parent / "autostart" / f"{APPLICATION_ID}.desktop"


def launch_command() -> str:
    """Prefer the installed launcher; fall back to running the module with this interpreter."""
    launcher_path = shutil.which("kvm-switcher")
    if launcher_path:
        return shlex.quote(launcher_path)
    return f"{shlex.quote(sys.executable)} -m kvm_switcher"


def desktop_entry(exec_command: str) -> str:
    """Return the text of the autostart desktop entry."""
    return (
        "[Desktop Entry]\n"
        "Type=Application\n"
        "Name=KVM Switcher\n"
        "Comment=Switch GL.iNet KVM ports from the panel\n"
        f"Exec={exec_command}\n"
        "Icon=video-display\n"
        "Terminal=false\n"
        "X-GNOME-Autostart-enabled=true\n"
    )


def is_enabled() -> bool:
    """True when the autostart entry exists."""
    return autostart_path().exists()


def set_enabled(start_at_login: bool) -> None:
    """Create or remove the autostart entry."""
    entry_path = autostart_path()
    if start_at_login:
        entry_path.parent.mkdir(parents=True, exist_ok=True)
        entry_path.write_text(desktop_entry(launch_command()), encoding="utf-8")
    else:
        entry_path.unlink(missing_ok=True)

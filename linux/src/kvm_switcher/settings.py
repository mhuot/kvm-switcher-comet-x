"""Non-secret preferences, stored as JSON under $XDG_CONFIG_HOME/kvm-switcher/.

The password lives in the desktop keyring (see credentials.py).
"""

from __future__ import annotations

import json
import os
import threading
from dataclasses import asdict, dataclass, field
from pathlib import Path

LABEL_STYLE_PORT_NAME = "port_name"
LABEL_STYLE_PORT_NUMBER = "port_number"
LABEL_STYLE_ICON_ONLY = "icon_only"
LABEL_STYLES = (LABEL_STYLE_PORT_NAME, LABEL_STYLE_PORT_NUMBER, LABEL_STYLE_ICON_ONLY)

_SAVE_LOCK = threading.Lock()


def config_directory() -> Path:
    """Return the per-user config directory, honouring XDG_CONFIG_HOME."""
    xdg_config_home = os.environ.get("XDG_CONFIG_HOME") or str(Path.home() / ".config")
    return Path(xdg_config_home) / "kvm-switcher"


def default_settings_path() -> Path:
    """Return the settings file path."""
    return config_directory() / "settings.json"


def normalize_host(raw_host: str) -> str:
    """Accept "https://1.2.3.4/", "1.2.3.4" etc. and return just the host part."""
    trimmed_host = raw_host.strip()
    for scheme in ("https://", "http://"):
        if trimmed_host.lower().startswith(scheme):
            trimmed_host = trimmed_host[len(scheme) :]
    return trimmed_host.rstrip("/")


@dataclass
class Settings:
    """User preferences. Call save() after changing them."""

    host: str = ""
    local_port_names: list[str] = field(default_factory=list)
    label_style: str = LABEL_STYLE_PORT_NAME
    pinned_certificate_fingerprint: str | None = None
    settings_path: Path = field(default_factory=default_settings_path, repr=False, compare=False)

    @classmethod
    def load(cls, settings_path: Path | None = None) -> "Settings":
        """Load settings, falling back to defaults when the file is missing or damaged."""
        resolved_path = settings_path or default_settings_path()
        try:
            stored_values = json.loads(resolved_path.read_text(encoding="utf-8"))
        except (OSError, ValueError):
            return cls(settings_path=resolved_path)

        if not isinstance(stored_values, dict):
            return cls(settings_path=resolved_path)

        label_style = stored_values.get("label_style", LABEL_STYLE_PORT_NAME)
        local_port_names = stored_values.get("local_port_names", [])
        return cls(
            host=str(stored_values.get("host", "")),
            local_port_names=[str(name) for name in local_port_names if isinstance(name, str)],
            label_style=label_style if label_style in LABEL_STYLES else LABEL_STYLE_PORT_NAME,
            pinned_certificate_fingerprint=stored_values.get("pinned_certificate_fingerprint"),
            settings_path=resolved_path,
        )

    def save(self) -> None:
        """Write settings atomically so a crash never leaves a half-written file."""
        stored_values = asdict(self)
        del stored_values["settings_path"]
        with _SAVE_LOCK:
            self.settings_path.parent.mkdir(parents=True, exist_ok=True)
            temporary_path = self.settings_path.with_suffix(".json.tmp")
            temporary_path.write_text(json.dumps(stored_values, indent=2) + "\n", encoding="utf-8")
            temporary_path.replace(self.settings_path)

    def local_name(self, port_index: int) -> str:
        """Return the user's fallback name for a port, or ""."""
        if port_index < len(self.local_port_names):
            return self.local_port_names[port_index].strip()
        return ""

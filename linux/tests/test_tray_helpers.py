"""Tests for the tray's label helpers. Skipped where GTK/PyGObject is not installed."""

from __future__ import annotations

import pytest

pytest.importorskip("gi")
tray = pytest.importorskip("kvm_switcher.tray", exc_type=ImportError)

# pylint: disable=wrong-import-position
from kvm_switcher.api_client import KvmPort, KvmSwitchState
from kvm_switcher.settings import (
    LABEL_STYLE_ICON_ONLY,
    LABEL_STYLE_PORT_NAME,
    LABEL_STYLE_PORT_NUMBER,
    Settings,
)

SAMPLE_STATE = KvmSwitchState(
    active_port_index=1,
    ports=(
        KvmPort(0, "1.1", "Work_station", True),
        KvmPort(1, "1.2", "A very long port name that will not fit", False),
    ),
)


@pytest.mark.parametrize(
    ("label_style", "expected_label"),
    [
        (LABEL_STYLE_PORT_NAME, "A very long port name t…"),
        (LABEL_STYLE_PORT_NUMBER, "2"),
        (LABEL_STYLE_ICON_ONLY, ""),
    ],
)
def test_panel_label_styles(label_style, expected_label):
    """The panel label follows the chosen style and truncates long names."""
    assert tray.panel_label(SAMPLE_STATE, Settings(label_style=label_style)) == expected_label


def test_panel_label_empty_before_first_refresh():
    """Nothing is shown until the state is known."""
    assert tray.panel_label(None, Settings()) == ""


def test_menu_item_label_escapes_underscores_and_notes_missing_video():
    """Underscores would become mnemonics unless doubled."""
    assert tray.menu_item_label(SAMPLE_STATE.ports[0], Settings()) == "_1   Work__station"
    assert tray.menu_item_label(SAMPLE_STATE.ports[1], Settings()).endswith("(no video)")

package tray

import (
	"strings"
	"testing"

	"github.com/mhuot/kvm-switcher-comet-x/linux/internal/kvm"
	"github.com/mhuot/kvm-switcher-comet-x/linux/internal/settings"
)

var sampleState = &kvm.SwitchState{
	ActivePortIndex: 1,
	Ports: []kvm.Port{
		{Index: 0, ID: "1.1", DeviceName: "Work_station", HasVideoLink: true},
		{Index: 1, ID: "1.2", DeviceName: "A very long port name that will not fit", HasVideoLink: false},
	},
}

func TestPanelLabelStyles(t *testing.T) {
	for labelStyle, expectedLabel := range map[string]string{
		settings.LabelStylePortName: "A very long port name t…",
		settings.LabelStyleIconOnly: "",
	} {
		if gotLabel := PanelLabel(sampleState, settings.Settings{LabelStyle: labelStyle}); gotLabel != expectedLabel {
			t.Errorf("style %s: got %q, want %q", labelStyle, gotLabel, expectedLabel)
		}
	}
}

func TestPanelLabelTruncatesByCharacterNotByte(t *testing.T) {
	accentedState := &kvm.SwitchState{Ports: []kvm.Port{{DeviceName: strings.Repeat("é", 30)}}}
	gotLabel := PanelLabel(accentedState, settings.Settings{LabelStyle: settings.LabelStylePortName})
	if want := strings.Repeat("é", 23) + "…"; gotLabel != want {
		t.Fatalf("got %q, want %q", gotLabel, want)
	}
}

func TestPanelLabelEmptyBeforeFirstRefresh(t *testing.T) {
	if gotLabel := PanelLabel(nil, settings.Settings{}); gotLabel != "" {
		t.Fatalf("got %q", gotLabel)
	}
}

func TestMenuItemLabelEscapesUnderscoresAndNotesMissingVideo(t *testing.T) {
	if gotLabel := MenuItemLabel(sampleState.Ports[0], settings.Settings{}); gotLabel != "_1   Work__station" {
		t.Errorf("got %q", gotLabel)
	}
	if gotLabel := MenuItemLabel(sampleState.Ports[1], settings.Settings{}); !strings.HasSuffix(gotLabel, "(no video)") {
		t.Errorf("got %q", gotLabel)
	}
}

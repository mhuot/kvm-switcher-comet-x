package autostart_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mhuot/kvm-switcher-comet-x/linux/internal/autostart"
)

func TestEntryCreatedAndRemoved(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)
	if want := filepath.Join(configHome, "autostart", "net.mhuot.KVMSwitcher.desktop"); autostart.EntryPath() != want {
		t.Fatalf("EntryPath() = %q, want %q", autostart.EntryPath(), want)
	}
	if autostart.IsEnabled() {
		t.Fatal("enabled before SetEnabled")
	}
	if err := autostart.SetEnabled(true); err != nil {
		t.Fatal(err)
	}
	entryBytes, err := os.ReadFile(autostart.EntryPath())
	if err != nil {
		t.Fatal(err)
	}
	for _, wantLine := range []string{"[Desktop Entry]", "Exec=", "Type=Application"} {
		if !strings.Contains(string(entryBytes), wantLine) {
			t.Errorf("entry is missing %q:\n%s", wantLine, entryBytes)
		}
	}
	if !autostart.IsEnabled() {
		t.Fatal("not enabled after SetEnabled(true)")
	}
	if err := autostart.SetEnabled(false); err != nil {
		t.Fatal(err)
	}
	if autostart.IsEnabled() {
		t.Fatal("still enabled after SetEnabled(false)")
	}
	if err := autostart.SetEnabled(false); err != nil {
		t.Fatalf("disabling twice failed: %v", err)
	}
}

func TestExecPathWithSpacesIsQuoted(t *testing.T) {
	entryText := autostart.DesktopEntry(`"/home/me/My Apps/kvm-switcher"`)
	if !strings.Contains(entryText, "Exec=\"/home/me/My Apps/kvm-switcher\"\n") {
		t.Fatalf("unexpected entry:\n%s", entryText)
	}
}

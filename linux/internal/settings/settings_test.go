package settings_test

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/mhuot/kvm-switcher-comet-x/linux/internal/settings"
)

func TestNormalizeHost(t *testing.T) {
	for rawHost, expectedHost := range map[string]string{
		"  https://glkvm.local/ ": "glkvm.local",
		"HTTP://10.0.0.5//":       "10.0.0.5",
		"glkvm.local":             "glkvm.local",
	} {
		if gotHost := settings.NormalizeHost(rawHost); gotHost != expectedHost {
			t.Errorf("NormalizeHost(%q) = %q, want %q", rawHost, gotHost, expectedHost)
		}
	}
}

func TestDefaultPathHonoursXDGConfigHome(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)
	if want := filepath.Join(configHome, "kvm-switcher", "settings.json"); settings.DefaultPath() != want {
		t.Fatalf("DefaultPath() = %q, want %q", settings.DefaultPath(), want)
	}
}

func TestRoundTrip(t *testing.T) {
	settingsPath := filepath.Join(t.TempDir(), "kvm-switcher", "settings.json")
	saved := settings.Settings{
		Host:                         "glkvm.local",
		LocalPortNames:               []string{"A", "B"},
		LabelStyle:                   settings.LabelStyleIconOnly,
		PinnedCertificateFingerprint: "ab",
	}
	if err := settings.Open(settingsPath).Update(func(updated *settings.Settings) { *updated = saved }); err != nil {
		t.Fatal(err)
	}
	if loaded := settings.Open(settingsPath).Get(); !reflect.DeepEqual(loaded, saved) {
		t.Fatalf("loaded %+v, want %+v", loaded, saved)
	}
	fileInfo, err := os.Stat(settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	if fileInfo.Mode().Perm() != 0o600 {
		t.Errorf("settings file mode %v, want 0600", fileInfo.Mode().Perm())
	}
}

// Hand-edited files with nulls and stray types still load.
func TestToleratesNullsAndWrongTypes(t *testing.T) {
	settingsPath := filepath.Join(t.TempDir(), "settings.json")
	storedSettings := `{
  "host": "glkvm.local",
  "local_port_names": ["Desk", "", 7, "Lab"],
  "label_style": "port_number",
  "pinned_certificate_fingerprint": null
}`
	if err := os.WriteFile(settingsPath, []byte(storedSettings), 0o600); err != nil {
		t.Fatal(err)
	}
	loaded := settings.Open(settingsPath).Get()
	want := settings.Settings{
		Host:           "glkvm.local",
		LocalPortNames: []string{"Desk", "", "Lab"},
		LabelStyle:     settings.LabelStylePortNumber,
	}
	if !reflect.DeepEqual(loaded, want) {
		t.Fatalf("loaded %+v, want %+v", loaded, want)
	}
}

func TestDamagedFileFallsBackToDefaults(t *testing.T) {
	settingsPath := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(settingsPath, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if loaded := settings.Open(settingsPath).Get(); loaded.Host != "" || loaded.LabelStyle != settings.LabelStylePortName {
		t.Fatalf("loaded %+v, want defaults", loaded)
	}
}

func TestUnknownLabelStyleFallsBack(t *testing.T) {
	settingsPath := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(settingsPath, []byte(`{"label_style": "sparkles"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if loaded := settings.Open(settingsPath).Get(); loaded.LabelStyle != settings.LabelStylePortName {
		t.Fatalf("label style %q", loaded.LabelStyle)
	}
}

func TestReloadIfChangedPicksUpAnotherProcessesWrite(t *testing.T) {
	settingsPath := filepath.Join(t.TempDir(), "settings.json")
	trayStore := settings.Open(settingsPath)
	if trayStore.ReloadIfChanged() {
		t.Fatal("reloaded with no change")
	}
	if err := settings.Open(settingsPath).Update(func(updated *settings.Settings) { updated.Host = "glkvm.local" }); err != nil {
		t.Fatal(err)
	}
	if !trayStore.ReloadIfChanged() || trayStore.Get().Host != "glkvm.local" {
		t.Fatalf("did not reload: %+v", trayStore.Get())
	}
}

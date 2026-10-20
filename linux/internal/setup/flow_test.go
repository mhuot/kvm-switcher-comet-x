package setup_test

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mhuot/kvm-switcher-comet-x/linux/internal/autostart"
	"github.com/mhuot/kvm-switcher-comet-x/linux/internal/credentials"
	"github.com/mhuot/kvm-switcher-comet-x/linux/internal/fakekvm"
	"github.com/mhuot/kvm-switcher-comet-x/linux/internal/settings"
	"github.com/mhuot/kvm-switcher-comet-x/linux/internal/setup"
)

// isolate gives the test private config, no password sources and no keyring.
func isolate(t *testing.T) *settings.Store {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv(credentials.PasswordEnvironmentVariable, "")
	t.Setenv(credentials.PasswordFileEnvironmentVariable, "")
	t.Setenv("CREDENTIALS_DIRECTORY", "")
	t.Setenv("DBUS_SESSION_BUS_ADDRESS", "unix:path="+filepath.Join(t.TempDir(), "no-bus"))
	return settings.Open("")
}

func runScripted(t *testing.T, store *settings.Store, answers ...string) (string, int) {
	t.Helper()
	var output bytes.Buffer
	changeCount := 0
	session := setup.Session{
		Prompter:    setup.NewScriptedPrompter(strings.NewReader(strings.Join(answers, "\n")+"\n"), &output),
		Store:       store,
		OnChanged:   func() { changeCount++ },
		ShowDoneRow: true,
	}
	session.Run()
	return output.String(), changeCount
}

func TestFirstRunSavesAddressAndPasswordAndConnects(t *testing.T) {
	store := isolate(t)
	fake := fakekvm.Start()
	defer fake.Close()

	transcript, changeCount := runScripted(t, store,
		"https://"+fake.Host()+"/", // address, with scheme and slash to normalize
		"correct horse",            // password
		"y",                        // no keyring: accept the private password file
	)

	if gotHost := store.Get().Host; gotHost != fake.Host() {
		t.Fatalf("host %q, want %q", gotHost, fake.Host())
	}
	if password, source := credentials.Lookup(credentials.DefaultAccount); password != "correct horse" ||
		source != credentials.SourcePasswordFile {
		t.Fatalf("password %q from %v", password, source)
	}
	if !strings.Contains(transcript, "Connected. 4 ports, active: Media PC & Games.") {
		t.Fatalf("no connection summary in:\n%s", transcript)
	}
	if store.Get().PinnedCertificateFingerprint == "" {
		t.Fatal("certificate was not pinned")
	}
	if changeCount != 2 {
		t.Fatalf("%d change notifications, want 2", changeCount)
	}
}

func TestFirstRunDecliningPasswordFileSavesNothing(t *testing.T) {
	store := isolate(t)
	transcript, _ := runScripted(t, store, "glkvm.local", "secret", "n")
	if password, _ := credentials.Lookup(credentials.DefaultAccount); password != "" {
		t.Fatalf("saved %q after the file was declined", password)
	}
	if !strings.Contains(transcript, "Could not save the password") {
		t.Fatalf("no explanation in:\n%s", transcript)
	}
}

func TestMenuEditsPortNamesLabelStyleAndAutostart(t *testing.T) {
	store := isolate(t)
	t.Setenv(credentials.PasswordEnvironmentVariable, "from-env")
	if err := store.Update(func(updated *settings.Settings) {
		updated.Host = "glkvm.local"
		updated.LabelStyle = settings.LabelStylePortName
	}); err != nil {
		t.Fatal(err)
	}

	runScripted(t, store,
		"4", "Desk", "", "Lab", "", // port names
		"5", "2", // panel shows: icon only
		"6", "1", // start at login: on
		"", // Enter: done
	)

	current := store.Get()
	if strings.Join(current.LocalPortNames, ",") != "Desk,,Lab," {
		t.Errorf("port names %q", current.LocalPortNames)
	}
	if current.LabelStyle != settings.LabelStyleIconOnly {
		t.Errorf("label style %q", current.LabelStyle)
	}
	if !autostart.IsEnabled() {
		t.Error("start at login was not enabled")
	}
}

func TestChangingAddressForgetsThePinnedCertificate(t *testing.T) {
	store := isolate(t)
	t.Setenv(credentials.PasswordEnvironmentVariable, "from-env")
	if err := store.Update(func(updated *settings.Settings) {
		updated.Host = "old.local"
		updated.PinnedCertificateFingerprint = strings.Repeat("ab", 32)
	}); err != nil {
		t.Fatal(err)
	}
	runScripted(t, store, "1", "new.local", "")
	if current := store.Get(); current.Host != "new.local" || current.PinnedCertificateFingerprint != "" {
		t.Fatalf("settings %+v", current)
	}
}

func TestEnvironmentPasswordIsNotEditable(t *testing.T) {
	store := isolate(t)
	t.Setenv(credentials.PasswordEnvironmentVariable, "from-env")
	if err := store.Update(func(updated *settings.Settings) { updated.Host = "glkvm.local" }); err != nil {
		t.Fatal(err)
	}
	transcript, _ := runScripted(t, store, "2", "")
	if !strings.Contains(transcript, "from KVM_SWITCHER_PASSWORD") {
		t.Fatalf("no explanation in:\n%s", transcript)
	}
}

func TestClosedInputEndsTheSession(t *testing.T) {
	store := isolate(t)
	// No answers at all: the wizard must stop rather than loop.
	var output bytes.Buffer
	session := setup.Session{
		Prompter: setup.NewScriptedPrompter(strings.NewReader(""), &output),
		Store:    store,
	}
	session.Run()
	if store.Get().Host != "" {
		t.Fatal("saved settings from empty input")
	}
}

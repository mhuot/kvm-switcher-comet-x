package credentials_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mhuot/kvm-switcher-comet-x/linux/internal/credentials"
)

// isolate points every password source at empty, private locations.
func isolate(t *testing.T) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv(credentials.PasswordEnvironmentVariable, "")
	t.Setenv(credentials.PasswordFileEnvironmentVariable, "")
	t.Setenv("CREDENTIALS_DIRECTORY", "")
	// No session bus, so the keyring is never consulted.
	t.Setenv("DBUS_SESSION_BUS_ADDRESS", "unix:path="+filepath.Join(t.TempDir(), "no-bus"))
}

func writeFile(t *testing.T, path, contents string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), mode); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, mode); err != nil {
		t.Fatal(err)
	}
}

func TestNothingConfigured(t *testing.T) {
	isolate(t)
	if password, source := credentials.Lookup(credentials.DefaultAccount); password != "" || source != credentials.SourceNone {
		t.Fatalf("got %q from %v", password, source)
	}
}

func TestEnvironmentVariableWins(t *testing.T) {
	isolate(t)
	if err := credentials.SaveToFile("from-file"); err != nil {
		t.Fatal(err)
	}
	t.Setenv(credentials.PasswordEnvironmentVariable, "from-env")
	if password, source := credentials.Lookup(credentials.DefaultAccount); password != "from-env" || source != credentials.SourceEnvironment {
		t.Fatalf("got %q from %v", password, source)
	}
}

func TestPasswordFileEnvironmentVariable(t *testing.T) {
	isolate(t)
	passwordPath := filepath.Join(t.TempDir(), "secret")
	writeFile(t, passwordPath, "p&ss word ü\r\nignored second line\n", 0o640)
	t.Setenv(credentials.PasswordFileEnvironmentVariable, passwordPath)
	if password, source := credentials.Lookup(credentials.DefaultAccount); password != "p&ss word ü" || source != credentials.SourceEnvironmentFile {
		t.Fatalf("got %q from %v", password, source)
	}
}

func TestSystemdCredential(t *testing.T) {
	isolate(t)
	credentialsDirectory := t.TempDir()
	writeFile(t, filepath.Join(credentialsDirectory, credentials.SystemdCredentialName), "from-systemd", 0o400)
	t.Setenv("CREDENTIALS_DIRECTORY", credentialsDirectory)
	if password, source := credentials.Lookup(credentials.DefaultAccount); password != "from-systemd" || source != credentials.SourceSystemdCredential {
		t.Fatalf("got %q from %v", password, source)
	}
}

func TestSavedPasswordFileRoundTripIsPrivate(t *testing.T) {
	isolate(t)
	if err := credentials.SaveToFile("saved"); err != nil {
		t.Fatal(err)
	}
	fileInfo, err := os.Stat(credentials.PasswordFilePath())
	if err != nil {
		t.Fatal(err)
	}
	if fileInfo.Mode().Perm() != 0o600 {
		t.Fatalf("mode %v, want 0600", fileInfo.Mode().Perm())
	}
	if password, source := credentials.Lookup(credentials.DefaultAccount); password != "saved" || source != credentials.SourcePasswordFile {
		t.Fatalf("got %q from %v", password, source)
	}
	if err := credentials.RemovePasswordFile(); err != nil {
		t.Fatal(err)
	}
	if password, _ := credentials.Lookup(credentials.DefaultAccount); password != "" {
		t.Fatalf("still found %q after removal", password)
	}
}

func TestReadablePasswordFileIsRefused(t *testing.T) {
	isolate(t)
	writeFile(t, credentials.PasswordFilePath(), "too-open\n", 0o644)
	if password, source := credentials.Lookup(credentials.DefaultAccount); password != "" || source != credentials.SourceNone {
		t.Fatalf("got %q from %v; a world-readable file must be ignored", password, source)
	}
}

func TestOverriddenSources(t *testing.T) {
	for source, wantOverridden := range map[credentials.Source]bool{
		credentials.SourceEnvironment:       true,
		credentials.SourceEnvironmentFile:   true,
		credentials.SourceSystemdCredential: true,
		credentials.SourceKeyring:           false,
		credentials.SourcePasswordFile:      false,
		credentials.SourceNone:              false,
	} {
		if source.Overridden() != wantOverridden {
			t.Errorf("%v.Overridden() = %v", source, !wantOverridden)
		}
	}
}

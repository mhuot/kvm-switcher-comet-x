package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mhuot/kvm-switcher-comet-x/linux/internal/credentials"
	"github.com/mhuot/kvm-switcher-comet-x/linux/internal/fakekvm"
	"github.com/mhuot/kvm-switcher-comet-x/linux/internal/kvm"
	"github.com/mhuot/kvm-switcher-comet-x/linux/internal/settings"
)

func init() {
	kvm.SwitchSettleDelay = 0
}

// isolate gives the test private config and no session bus, so neither the keyring nor a
// running tray is ever reached.
func isolate(t *testing.T) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv(credentials.PasswordEnvironmentVariable, "")
	t.Setenv(credentials.PasswordFileEnvironmentVariable, "")
	t.Setenv("CREDENTIALS_DIRECTORY", "")
	t.Setenv("DBUS_SESSION_BUS_ADDRESS", "unix:path="+filepath.Join(t.TempDir(), "no-bus"))
}

// configured points the CLI at a fake KVM.
func configured(t *testing.T) *fakekvm.Server {
	t.Helper()
	isolate(t)
	fake := fakekvm.Start()
	t.Cleanup(fake.Close)
	if err := settings.Open("").Update(func(updated *settings.Settings) { updated.Host = fake.Host() }); err != nil {
		t.Fatal(err)
	}
	t.Setenv(credentials.PasswordEnvironmentVariable, "correct horse")
	return fake
}

func runCLI(t *testing.T, input string, arguments ...string) (int, string, string) {
	t.Helper()
	var output, errorOutput bytes.Buffer
	exitCode := run(arguments, strings.NewReader(input), &output, &errorOutput)
	return exitCode, output.String(), errorOutput.String()
}

func TestSwitchUsesOneBasedNumbers(t *testing.T) {
	fake := configured(t)
	exitCode, output, errorOutput := runCLI(t, "", "switch", "4")
	if exitCode != 0 {
		t.Fatalf("exit %d: %s", exitCode, errorOutput)
	}
	setActiveRequests := fake.RequestsTo("/api/switch/set_active")
	if len(setActiveRequests) != 1 || setActiveRequests[0].Query != "port=3" {
		t.Fatalf("set_active requests %+v", setActiveRequests)
	}
	if !strings.Contains(output, "Switched to") {
		t.Fatalf("output %q", output)
	}
}

func TestSwitchRejectsOutOfRangePorts(t *testing.T) {
	for _, portArgument := range []string{"0", "5", "x"} {
		fake := configured(t)
		if exitCode, _, _ := runCLI(t, "", "switch", portArgument); exitCode != 2 {
			t.Errorf("switch %s exited %d, want 2", portArgument, exitCode)
		}
		if requests := fake.RequestsTo("/api/switch/set_active"); len(requests) != 0 {
			t.Errorf("switch %s sent set_active", portArgument)
		}
	}
}

func TestStatusPrintsActivePortName(t *testing.T) {
	configured(t)
	exitCode, output, _ := runCLI(t, "", "status")
	if exitCode != 0 || strings.TrimSpace(output) != "Media PC & Games" {
		t.Fatalf("exit %d, output %q", exitCode, output)
	}
}

func TestListMarksActivePort(t *testing.T) {
	configured(t)
	exitCode, output, _ := runCLI(t, "", "list")
	outputLines := strings.Split(strings.TrimRight(output, "\n"), "\n")
	if exitCode != 0 || len(outputLines) != 4 || outputLines[0] != "  1  Workstation" ||
		outputLines[2] != "* 3  Media PC & Games" {
		t.Fatalf("exit %d, output:\n%s", exitCode, output)
	}
}

func TestErrorsGoToStderr(t *testing.T) {
	isolate(t)
	exitCode, _, errorOutput := runCLI(t, "", "status")
	if exitCode != 1 || !strings.Contains(errorOutput, "Set the KVM address") {
		t.Fatalf("exit %d, stderr %q", exitCode, errorOutput)
	}
}

func TestSwitchIsForwardedToRunningTray(t *testing.T) {
	isolate(t)
	var forwardedPorts []int
	forwardToRunningTray = func(portIndex int) bool {
		forwardedPorts = append(forwardedPorts, portIndex)
		return true
	}
	t.Cleanup(func() { forwardToRunningTray = func(int) bool { return false } })
	exitCode, output, _ := runCLI(t, "", "switch", "2")
	if exitCode != 0 || len(forwardedPorts) != 1 || forwardedPorts[0] != 1 ||
		!strings.Contains(output, "running KVM Switcher") {
		t.Fatalf("exit %d, forwarded %v, output %q", exitCode, forwardedPorts, output)
	}
}

func TestNonInteractiveSetupForHeadlessMachines(t *testing.T) {
	isolate(t)
	fake := fakekvm.Start()
	defer fake.Close()
	exitCode, output, errorOutput := runCLI(t, "correct horse\n",
		"setup", "--address", "https://"+fake.Host()+"/", "--password-stdin", "--save-password-to-file", "--test")
	if exitCode != 0 {
		t.Fatalf("exit %d: %s", exitCode, errorOutput)
	}
	if settings.Open("").Get().Host != fake.Host() {
		t.Fatal("address not saved")
	}
	if password, source := credentials.Lookup(credentials.DefaultAccount); password != "correct horse" ||
		source != credentials.SourcePasswordFile {
		t.Fatalf("password %q from %v", password, source)
	}
	if !strings.Contains(output, "Connected. 4 ports") {
		t.Fatalf("output %q", output)
	}
}

func TestNonInteractiveSetupWithoutKeyringExplainsOptions(t *testing.T) {
	isolate(t)
	exitCode, _, errorOutput := runCLI(t, "secret\n", "setup", "--password-stdin")
	if exitCode != 1 || !strings.Contains(errorOutput, "--save-password-to-file") {
		t.Fatalf("exit %d, stderr %q", exitCode, errorOutput)
	}
}

func TestVersionAndUnknownCommand(t *testing.T) {
	if exitCode, output, _ := runCLI(t, "", "--version"); exitCode != 0 || strings.TrimSpace(output) != version {
		t.Fatalf("exit %d, output %q", exitCode, output)
	}
	if exitCode, _, _ := runCLI(t, "", "sparkles"); exitCode != 2 {
		t.Fatalf("unknown command exited %d", exitCode)
	}
}

// Command kvm-switcher switches ports on a GL.iNet Comet X (GL-RM4PE) KVM from the Linux
// desktop panel or the command line.
//
//	kvm-switcher                 run the panel icon
//	kvm-switcher switch N        switch to port N (1-based); bind this to a keyboard shortcut
//	kvm-switcher status | list   show the active port, or every port
//	kvm-switcher setup           change the settings (works over SSH, too)
//	kvm-switcher self-test       check the panel and keyring support without a KVM
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/mhuot/kvm-switcher-comet-x/linux/internal/credentials"
	"github.com/mhuot/kvm-switcher-comet-x/linux/internal/instance"
	"github.com/mhuot/kvm-switcher-comet-x/linux/internal/kvm"
	"github.com/mhuot/kvm-switcher-comet-x/linux/internal/selftest"
	"github.com/mhuot/kvm-switcher-comet-x/linux/internal/settings"
	"github.com/mhuot/kvm-switcher-comet-x/linux/internal/setup"
	"github.com/mhuot/kvm-switcher-comet-x/linux/internal/tray"
)

// version is set at build time with -ldflags "-X main.version=…"; the release workflow
// checks it against the tag.
var version = "1.0.0"

const usageText = `Usage: kvm-switcher [command]

Switch ports on a GL.iNet Comet X (GL-RM4PE) KVM. With no command, runs the panel icon.

Commands:
  tray          run the panel icon (the default)
  switch N      switch to port N (1-based); bind this to a keyboard shortcut
  status        print the active port's name
  list          list the ports; * marks the active one
  setup         change the address, password and other settings
  self-test     check panel icon and keyring support without a KVM
  version       print the version

Run "kvm-switcher setup --help" for setting things up from a script.
`

// Exit codes.
const (
	exitSuccess = 0
	exitFailure = 1
	exitUsage   = 2
)

// forwardToRunningTray is replaced in tests.
var forwardToRunningTray = instance.ForwardSwitch

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(arguments []string, input io.Reader, output, errorOutput io.Writer) int {
	command := "tray"
	if len(arguments) > 0 {
		command = arguments[0]
	}
	switch command {
	case "tray":
		return tray.Run()
	case "switch":
		if len(arguments) != 2 {
			fmt.Fprintln(errorOutput, "Usage: kvm-switcher switch N")
			return exitUsage
		}
		return commandSwitch(arguments[1], output, errorOutput)
	case "status", "list":
		return commandStatus(command == "list", output, errorOutput)
	case "setup":
		return commandSetup(arguments[1:], input, output, errorOutput)
	case "self-test":
		return selftest.Run(output)
	case "version", "--version", "-V":
		fmt.Fprintln(output, version)
		return exitSuccess
	case "help", "--help", "-h":
		fmt.Fprint(output, usageText)
		return exitSuccess
	default:
		fmt.Fprintf(errorOutput, "Unknown command %q.\n\n%s", command, usageText)
		return exitUsage
	}
}

func newClient() (*kvm.Client, *settings.Store) {
	store := settings.Open("")
	return kvm.NewClient(store, credentials.ReadPassword), store
}

func commandSwitch(portArgument string, output, errorOutput io.Writer) int {
	portNumber, err := strconv.Atoi(portArgument)
	if err != nil || portNumber < 1 {
		fmt.Fprintln(errorOutput, "Port numbers start at 1.")
		return exitUsage
	}
	portIndex := portNumber - 1
	if forwardToRunningTray(portIndex) {
		fmt.Fprintf(output, "Asked the running KVM Switcher to switch to port %d.\n", portNumber)
		return exitSuccess
	}

	client, store := newClient()
	switchState, err := client.FetchSwitchState()
	if err != nil {
		fmt.Fprintln(errorOutput, err)
		return exitFailure
	}
	if portIndex >= len(switchState.Ports) {
		fmt.Fprintf(errorOutput, "The KVM has only %d ports.\n", len(switchState.Ports))
		return exitUsage
	}
	if err := client.SetActivePort(portIndex); err != nil {
		fmt.Fprintln(errorOutput, err)
		return exitFailure
	}
	if switchState, err = client.FetchSwitchState(); err != nil {
		fmt.Fprintln(errorOutput, err)
		return exitFailure
	}
	activeName := "?"
	if activePort, found := switchState.ActivePort(); found {
		activeName = activePort.DisplayName(store.Get())
	}
	fmt.Fprintf(output, "Switched to %s.\n", activeName)
	return exitSuccess
}

func commandStatus(listAllPorts bool, output, errorOutput io.Writer) int {
	client, store := newClient()
	switchState, err := client.FetchSwitchState()
	if err != nil {
		fmt.Fprintln(errorOutput, err)
		return exitFailure
	}
	current := store.Get()
	if !listAllPorts {
		activeName := "?"
		if activePort, found := switchState.ActivePort(); found {
			activeName = activePort.DisplayName(current)
		}
		fmt.Fprintln(output, activeName)
		return exitSuccess
	}
	for _, port := range switchState.Ports {
		activeMarker := " "
		if port.Index == switchState.ActivePortIndex {
			activeMarker = "*"
		}
		videoNote := ""
		if !port.HasVideoLink {
			videoNote = "  (no video)"
		}
		fmt.Fprintf(output, "%s %d  %s%s\n", activeMarker, port.Index+1, port.DisplayName(current), videoNote)
	}
	return exitSuccess
}

func commandSetup(arguments []string, input io.Reader, output, errorOutput io.Writer) int {
	setupFlags := flag.NewFlagSet("setup", flag.ContinueOnError)
	setupFlags.SetOutput(errorOutput)
	address := setupFlags.String("address", "", "the KVM's address, e.g. glkvm.local")
	passwordFromStdin := setupFlags.Bool("password-stdin", false, "read the admin password from standard input")
	passwordToFile := setupFlags.Bool("save-password-to-file", false,
		"store the password in "+credentials.PasswordFilePath()+" (mode 0600) instead of the desktop keyring")
	testAfterwards := setupFlags.Bool("test", false, "test the connection after saving")
	waitAtEnd := setupFlags.Bool("wait-at-end", false, "wait for Enter before exiting (used by the panel's terminal window)")
	setupFlags.Usage = func() {
		fmt.Fprint(errorOutput, `Usage: kvm-switcher setup [flags]

With no flags, asks questions in the terminal. With flags, sets things up without asking,
for scripts and headless machines:

  echo "$PASSWORD" | kvm-switcher setup --address glkvm.local --password-stdin --test

Flags:
`)
		setupFlags.PrintDefaults()
	}
	if err := setupFlags.Parse(arguments); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return exitSuccess
		}
		return exitUsage
	}
	store := settings.Open("")

	if *address == "" && !*passwordFromStdin && !*testAfterwards {
		session := setup.Session{
			Prompter:    setup.NewTerminalPrompter(),
			Store:       store,
			OnChanged:   func() { instance.NotifyReload() },
			ShowDoneRow: true,
		}
		session.Run()
		if *waitAtEnd {
			fmt.Fprint(output, "\nPress Enter to close this window.")
			_, _ = fmt.Fscanln(input)
		}
		return exitSuccess
	}

	if *address != "" {
		newHost := settings.NormalizeHost(*address)
		if err := store.Update(func(updated *settings.Settings) {
			if updated.Host != newHost {
				updated.PinnedCertificateFingerprint = ""
			}
			updated.Host = newHost
		}); err != nil {
			fmt.Fprintln(errorOutput, "Could not save the settings:", err)
			return exitFailure
		}
		fmt.Fprintf(output, "Address set to %s.\n", newHost)
	}
	if *passwordFromStdin {
		passwordBytes, err := io.ReadAll(io.LimitReader(input, 4096))
		if err != nil {
			fmt.Fprintln(errorOutput, "Could not read the password:", err)
			return exitFailure
		}
		password, _, _ := strings.Cut(string(passwordBytes), "\n")
		password = strings.TrimSuffix(password, "\r")
		if password == "" {
			fmt.Fprintln(errorOutput, "The password on standard input is empty.")
			return exitUsage
		}
		if *passwordToFile {
			err = credentials.SaveToFile(password)
		} else {
			err = credentials.SaveToKeyring(credentials.DefaultAccount, password)
		}
		if err != nil {
			fmt.Fprintln(errorOutput, "Could not save the password:", err)
			if errors.Is(err, credentials.ErrKeyringUnavailable) {
				fmt.Fprintln(errorOutput, "On a machine without a desktop keyring, add --save-password-to-file, or set "+
					credentials.PasswordEnvironmentVariable+" or "+credentials.PasswordFileEnvironmentVariable+".")
			}
			return exitFailure
		}
		_, source := credentials.Lookup(credentials.DefaultAccount)
		fmt.Fprintf(output, "Password saved (%s).\n", source.Description())
	}
	instance.NotifyReload()
	if *testAfterwards {
		summary, err := setup.TestConnection(store)
		if err != nil {
			fmt.Fprintln(errorOutput, err)
			return exitFailure
		}
		fmt.Fprintln(output, summary)
	}
	return exitSuccess
}

// Package desktop opens URLs and terminals and shows notifications, using only tools and
// D-Bus services every Linux desktop already has.
package desktop

import (
	"errors"
	"os"
	"os/exec"
	"strings"

	"github.com/godbus/dbus/v5"
)

// HasDisplay reports whether a graphical session is available.
func HasDisplay() bool {
	return os.Getenv("WAYLAND_DISPLAY") != "" || os.Getenv("DISPLAY") != ""
}

// startDetached runs a program without waiting for it, reaping it when it exits.
func startDetached(programPath string, arguments ...string) error {
	command := exec.Command(programPath, arguments...)
	command.Stdin, command.Stdout, command.Stderr = nil, nil, nil
	if err := command.Start(); err != nil {
		return err
	}
	go func() { _ = command.Wait() }()
	return nil
}

// OpenURL opens a web page in the default browser.
func OpenURL(pageURL string) error {
	for _, opener := range [][]string{{"xdg-open"}, {"gio", "open"}, {"kde-open"}} {
		if programPath, err := exec.LookPath(opener[0]); err == nil {
			return startDetached(programPath, append(opener[1:], pageURL)...)
		}
	}
	// The OpenURI portal works even inside sandboxes and minimal desktops.
	connection, err := dbus.ConnectSessionBus()
	if err != nil {
		return err
	}
	defer connection.Close()
	var requestHandle dbus.ObjectPath
	return connection.Object("org.freedesktop.portal.Desktop", "/org/freedesktop/portal/desktop").
		Call("org.freedesktop.portal.OpenURI.OpenURI", 0, "", pageURL, map[string]dbus.Variant{}).
		Store(&requestHandle)
}

// terminalLaunchers lists terminal emulators and the argument that runs a command in them,
// in the order to try them.
var terminalLaunchers = [][]string{
	{"xdg-terminal-exec"},
	{"x-terminal-emulator", "-e"},
	{"ptyxis", "--"},
	{"gnome-terminal", "--"},
	{"konsole", "-e"},
	{"xfce4-terminal", "-x"},
	{"mate-terminal", "-x"},
	{"tilix", "-e"},
	{"terminator", "-x"},
	{"lxterminal", "-e"},
	{"qterminal", "-e"},
	{"kitty"},
	{"alacritty", "-e"},
	{"foot"},
	{"wezterm", "start", "--"},
	{"xterm", "-e"},
}

// ErrNoTerminal means no known terminal emulator is installed.
var ErrNoTerminal = errors.New("no terminal emulator found")

// RunInTerminal opens a terminal window running the given command.
func RunInTerminal(command ...string) error {
	desktopName := strings.ToUpper(os.Getenv("XDG_CURRENT_DESKTOP"))
	launchers := terminalLaunchers
	// Prefer the desktop's own terminal over a generic alternative.
	if strings.Contains(desktopName, "KDE") {
		launchers = append([][]string{{"konsole", "-e"}}, launchers...)
	}
	for _, launcher := range launchers {
		programPath, err := exec.LookPath(launcher[0])
		if err != nil {
			continue
		}
		return startDetached(programPath, append(append([]string{}, launcher[1:]...), command...)...)
	}
	return ErrNoTerminal
}

// Notify shows a desktop notification. Failures are ignored: notifications are a courtesy.
func Notify(summary, body string) {
	connection, err := dbus.ConnectSessionBus()
	if err != nil {
		return
	}
	defer connection.Close()
	_ = connection.Object("org.freedesktop.Notifications", "/org/freedesktop/Notifications").
		Call("org.freedesktop.Notifications.Notify", 0,
			"KVM Switcher", uint32(0), "video-display", summary, body,
			[]string{}, map[string]dbus.Variant{}, int32(-1)).Err
}

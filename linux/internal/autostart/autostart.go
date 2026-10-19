// Package autostart manages start-at-login through an XDG autostart desktop entry, which
// GNOME, KDE Plasma, Xfce, Cinnamon, MATE and Budgie all honour.
package autostart

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/mhuot/kvm-switcher-comet-x/linux/internal/settings"
)

// ApplicationID names the desktop entries and the D-Bus service.
const ApplicationID = "net.mhuot.KVMSwitcher"

// EntryPath returns ~/.config/autostart/net.mhuot.KVMSwitcher.desktop (XDG_CONFIG_HOME aware).
func EntryPath() string {
	return filepath.Join(filepath.Dir(settings.ConfigDirectory()), "autostart", ApplicationID+".desktop")
}

// LaunchCommand prefers kvm-switcher on PATH, falling back to this executable's path.
func LaunchCommand() string {
	if launcherPath, err := exec.LookPath("kvm-switcher"); err == nil {
		if absolutePath, err := filepath.Abs(launcherPath); err == nil {
			return quoteExecArgument(absolutePath)
		}
	}
	executablePath, err := os.Executable()
	if err != nil {
		return "kvm-switcher"
	}
	return quoteExecArgument(executablePath)
}

// quoteExecArgument quotes a path for a desktop entry's Exec key when it needs it.
func quoteExecArgument(argument string) string {
	if !strings.ContainsAny(argument, " \t\"'\\$`") {
		return argument
	}
	escaped := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "$", `\$`, "`", "\\`").Replace(argument)
	return `"` + escaped + `"`
}

// DesktopEntry returns the text of the autostart entry.
func DesktopEntry(execCommand string) string {
	return "[Desktop Entry]\n" +
		"Type=Application\n" +
		"Name=KVM Switcher\n" +
		"Comment=Switch GL.iNet KVM ports from the panel\n" +
		"Exec=" + execCommand + "\n" +
		"Icon=video-display\n" +
		"Terminal=false\n" +
		"X-GNOME-Autostart-enabled=true\n"
}

// IsEnabled reports whether the autostart entry exists.
func IsEnabled() bool {
	_, err := os.Stat(EntryPath())
	return err == nil
}

// SetEnabled creates or removes the autostart entry.
func SetEnabled(startAtLogin bool) error {
	entryPath := EntryPath()
	if !startAtLogin {
		if err := os.Remove(entryPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(entryPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(entryPath, []byte(DesktopEntry(LaunchCommand())), 0o644)
}

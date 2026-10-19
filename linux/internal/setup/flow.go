// Package setup is KVM Switcher's settings, as a terminal wizard (`kvm-switcher setup`,
// which also works over SSH on headless machines) or as desktop dialogs through zenity or
// kdialog. Both run the same steps through the Prompter interface.
package setup

import (
	"errors"
	"fmt"
	"strings"

	"github.com/mhuot/kvm-switcher-comet-x/linux/internal/autostart"
	"github.com/mhuot/kvm-switcher-comet-x/linux/internal/credentials"
	"github.com/mhuot/kvm-switcher-comet-x/linux/internal/kvm"
	"github.com/mhuot/kvm-switcher-comet-x/linux/internal/settings"
)

// EditablePortCount is how many local port names setup offers.
const EditablePortCount = 4

const windowTitle = "KVM Switcher Settings"

// ShortcutHint explains keyboard shortcuts, which Linux desktops own.
const ShortcutHint = "Keyboard shortcuts: in your desktop's keyboard settings, add custom shortcuts " +
	"that run \"kvm-switcher switch 1\", \"kvm-switcher switch 2\" and so on."

// MenuRow is one line of the settings menu.
type MenuRow struct {
	Key   string
	Label string
	Value string
}

// Option is one choice in a list.
type Option struct {
	Key   string
	Label string
}

// Prompter asks the user things. Every method's false/"" result means "cancelled".
type Prompter interface {
	Menu(title, text string, rows []MenuRow) (string, bool)
	Text(title, prompt, current string) (string, bool)
	Password(title, prompt string) (string, bool)
	Choice(title, prompt string, options []Option, selectedKey string) (string, bool)
	YesNo(title, question string) bool
	Info(title, message string)
	Error(title, message string)
	// Busy shows that something is in progress and returns a function that hides it.
	Busy(title, message string) func()
}

// Session is one run of the settings flow.
type Session struct {
	Prompter Prompter
	Store    *settings.Store
	// OnChanged runs after every saved change (the tray reloads; a separate setup process
	// tells the running tray).
	OnChanged func()
	// ShowDoneRow adds a "Done" row to the menu. Desktop dialogs leave it out because their
	// Close button ends the session.
	ShowDoneRow bool
}

func (session *Session) changed() {
	if session.OnChanged != nil {
		session.OnChanged()
	}
}

// NeedsFirstRun reports whether the address or password is missing.
func NeedsFirstRun(current settings.Settings) bool {
	password, _ := credentials.Lookup(credentials.DefaultAccount)
	return current.Host == "" || password == ""
}

// Run shows the guided first-run steps when nothing is configured, then the settings menu.
func (session *Session) Run() {
	if NeedsFirstRun(session.Store.Get()) {
		if session.firstRun() {
			return
		}
	}
	session.menuLoop()
}

// firstRun asks for the address and password and tests them. It returns true when setup is
// complete (or the user cancelled), false to continue to the menu so they can fix things.
func (session *Session) firstRun() bool {
	if !session.editAddress() {
		return true
	}
	if !session.editPassword() {
		return true
	}
	summary, err := session.testConnection()
	if err != nil {
		session.Prompter.Error(windowTitle, "Could not connect: "+err.Error()+
			"\n\nCheck the address and password in the next window.")
		return false
	}
	session.Prompter.Info(windowTitle, summary+"\n\nKVM Switcher is set up. Open Settings from "+
		"the panel icon, or run \"kvm-switcher setup\", to change it later.\n\n"+ShortcutHint)
	return true
}

func onOff(enabled bool) string {
	if enabled {
		return "On"
	}
	return "Off"
}

func (session *Session) menuRows() []MenuRow {
	current := session.Store.Get()
	_, passwordSource := credentials.Lookup(credentials.DefaultAccount)
	var portNames []string
	for portIndex := range EditablePortCount {
		localName := current.LocalName(portIndex)
		if localName == "" {
			localName = "—"
		}
		portNames = append(portNames, localName)
	}
	address := current.Host
	if address == "" {
		address = "not set"
	}
	pinState := "none yet"
	if current.PinnedCertificateFingerprint != "" {
		pinState = "SHA-256 " + current.PinnedCertificateFingerprint[:min(16, len(current.PinnedCertificateFingerprint))] + "…"
	}
	rows := []MenuRow{
		{"address", "Address", address},
		{"password", "Admin password", passwordSource.Description()},
		{"test", "Test connection", ""},
		{"ports", "Port names", strings.Join(portNames, ", ")},
		{"label", "Panel shows", settings.LabelStyleTitles[current.LabelStyle]},
		{"autostart", "Start when I log in", onOff(autostart.IsEnabled())},
		{"certificate", "Reset pinned certificate", pinState},
	}
	if session.ShowDoneRow {
		rows = append(rows, MenuRow{"done", "Done", ""})
	}
	return rows
}

func (session *Session) menuLoop() {
	for {
		chosenKey, chosen := session.Prompter.Menu(windowTitle,
			"Choose a setting to change. "+ShortcutHint, session.menuRows())
		if !chosen || chosenKey == "done" {
			return
		}
		switch chosenKey {
		case "address":
			session.editAddress()
		case "password":
			session.editPassword()
		case "test":
			if summary, err := session.testConnection(); err != nil {
				session.Prompter.Error(windowTitle, "Failed: "+err.Error())
			} else {
				session.Prompter.Info(windowTitle, summary)
			}
		case "ports":
			session.editPortNames()
		case "label":
			session.editLabelStyle()
		case "autostart":
			session.editAutostart()
		case "certificate":
			session.resetCertificate()
		}
	}
}

func (session *Session) save(change func(*settings.Settings)) bool {
	if err := session.Store.Update(change); err != nil {
		session.Prompter.Error(windowTitle, "Could not save: "+err.Error())
		return false
	}
	session.changed()
	return true
}

// editAddress returns false when cancelled.
func (session *Session) editAddress() bool {
	current := session.Store.Get()
	enteredAddress, entered := session.Prompter.Text(windowTitle,
		"KVM address: the name or IP address you type into your browser to reach the KVM, "+
			"for example glkvm.local.", current.Host)
	if !entered {
		return false
	}
	newHost := settings.NormalizeHost(enteredAddress)
	if newHost == "" {
		session.Prompter.Error(windowTitle, "The address can't be empty.")
		return session.editAddress()
	}
	if newHost == current.Host {
		return true
	}
	// A different address is a different KVM, so trust its certificate afresh.
	return session.save(func(updated *settings.Settings) {
		updated.Host = newHost
		updated.PinnedCertificateFingerprint = ""
	})
}

// editPassword returns false when cancelled.
func (session *Session) editPassword() bool {
	currentPassword, passwordSource := credentials.Lookup(credentials.DefaultAccount)
	if passwordSource.Overridden() {
		session.Prompter.Info(windowTitle, "The password comes "+passwordSource.Description()+
			". Change it there, or unset it to use a saved password.")
		return true
	}
	prompt := "KVM admin password: the one you use to log in to the KVM's web page."
	if currentPassword != "" {
		prompt += " Leave it empty to keep the saved password."
	}
	enteredPassword, entered := session.Prompter.Password(windowTitle, prompt)
	if !entered {
		return false
	}
	if enteredPassword == "" {
		if currentPassword == "" {
			session.Prompter.Error(windowTitle, "The password can't be empty.")
			return session.editPassword()
		}
		return true
	}
	if err := SavePassword(enteredPassword, session.Prompter.YesNo); err != nil {
		session.Prompter.Error(windowTitle, "Could not save the password: "+err.Error())
		return false
	}
	session.changed()
	return true
}

// errPasswordNotSaved means there is no keyring and the user declined the password file.
var errPasswordNotSaved = errors.New("not saved; there is no desktop keyring and the password file was declined")

// SavePassword stores the password in the keyring, or (after asking) in the private
// password file on machines without one.
func SavePassword(password string, askYesNo func(title, question string) bool) error {
	err := credentials.SaveToKeyring(credentials.DefaultAccount, password)
	if err == nil {
		return nil
	}
	if !errors.Is(err, credentials.ErrKeyringUnavailable) {
		return err
	}
	question := "There's no desktop keyring to store the password in (" + err.Error() + ").\n\n" +
		"Save it in " + credentials.PasswordFilePath() + " instead? Only your user account can " +
		"read that file, but it isn't encrypted."
	if !askYesNo(windowTitle, question) {
		return errPasswordNotSaved
	}
	return credentials.SaveToFile(password)
}

func (session *Session) editPortNames() {
	current := session.Store.Get()
	updatedNames := make([]string, EditablePortCount)
	for portIndex := range EditablePortCount {
		enteredName, entered := session.Prompter.Text(windowTitle,
			fmt.Sprintf("Name for port %d, used only when the KVM has none. Leave it empty for "+
				"\"Port %d\". Names set in the KVM's web page show on every computer.",
				portIndex+1, portIndex+1),
			current.LocalName(portIndex))
		if !entered {
			return
		}
		updatedNames[portIndex] = strings.TrimSpace(enteredName)
	}
	session.save(func(updated *settings.Settings) { updated.LocalPortNames = updatedNames })
}

func (session *Session) editLabelStyle() {
	var options []Option
	for _, labelStyle := range settings.LabelStyles {
		options = append(options, Option{labelStyle, settings.LabelStyleTitles[labelStyle]})
	}
	chosenStyle, chosen := session.Prompter.Choice(windowTitle, "What should appear beside the panel icon?",
		options, session.Store.Get().LabelStyle)
	if chosen {
		session.save(func(updated *settings.Settings) { updated.LabelStyle = chosenStyle })
	}
}

func (session *Session) editAutostart() {
	currentKey := "off"
	if autostart.IsEnabled() {
		currentKey = "on"
	}
	chosenKey, chosen := session.Prompter.Choice(windowTitle, "Start KVM Switcher when you log in?",
		[]Option{{"on", "On"}, {"off", "Off"}}, currentKey)
	if !chosen {
		return
	}
	if err := autostart.SetEnabled(chosenKey == "on"); err != nil {
		session.Prompter.Error(windowTitle, "Could not change start at login: "+err.Error())
	}
}

func (session *Session) resetCertificate() {
	if !session.Prompter.YesNo(windowTitle, "Forget the KVM's pinned certificate? Do this only "+
		"after a factory reset, a firmware update that changed it, or moving to another KVM. "+
		"The next connection trusts whatever certificate the KVM shows.") {
		return
	}
	if session.save(func(updated *settings.Settings) { updated.PinnedCertificateFingerprint = "" }) {
		session.Prompter.Info(windowTitle, "The certificate will be pinned again on the next connection.")
	}
}

// testConnection connects with the saved settings and summarizes the result.
func (session *Session) testConnection() (string, error) {
	hideBusy := session.Prompter.Busy(windowTitle, "Connecting to the KVM…")
	defer hideBusy()
	return TestConnection(session.Store)
}

// TestConnection logs in with the saved settings and describes the switch.
func TestConnection(store *settings.Store) (string, error) {
	client := kvm.NewClient(store, credentials.ReadPassword)
	switchState, err := client.FetchSwitchState()
	if err != nil {
		return "", err
	}
	activeName := "?"
	if activePort, found := switchState.ActivePort(); found {
		activeName = activePort.DisplayName(store.Get())
	}
	return fmt.Sprintf("Connected. %d ports, active: %s.", len(switchState.Ports), activeName), nil
}

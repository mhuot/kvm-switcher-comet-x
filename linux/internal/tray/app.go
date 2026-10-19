// Package tray is the panel app: an icon with the active port's name beside it and a menu
// of ports, kept current in the background.
package tray

import (
	"errors"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/godbus/dbus/v5"

	"github.com/mhuot/kvm-switcher-comet-x/linux/internal/autostart"
	"github.com/mhuot/kvm-switcher-comet-x/linux/internal/credentials"
	"github.com/mhuot/kvm-switcher-comet-x/linux/internal/desktop"
	"github.com/mhuot/kvm-switcher-comet-x/linux/internal/instance"
	"github.com/mhuot/kvm-switcher-comet-x/linux/internal/kvm"
	"github.com/mhuot/kvm-switcher-comet-x/linux/internal/settings"
	"github.com/mhuot/kvm-switcher-comet-x/linux/internal/setup"
	"github.com/mhuot/kvm-switcher-comet-x/linux/internal/statusnotifier"
)

const (
	backgroundRefreshInterval = 15 * time.Second
	// Opening the menu refreshes it, unless it was refreshed this recently.
	menuOpenRefreshInterval = 3 * time.Second
	defaultPortCount        = 4
	applicationTitle        = "KVM Switcher"
)

// MissingPanelMessage explains what to do when no panel hosts the icon.
const MissingPanelMessage = "KVM Switcher is running, but your desktop isn't showing panel icons. " +
	"On GNOME, install and turn on the \"AppIndicator and KStatusNotifierItem Support\" extension."

// Menu entry IDs. Ports use 1 to the port count.
const (
	entryError int32 = 100 + iota
	entryErrorSeparator
	entryPortsSeparator
	entryActionsSeparator
	entryRefresh
	entryOpenWebUI
	entrySettings
	entryQuit
)

// MenuActions are what the menu entries do.
type MenuActions struct {
	SwitchPort   func(portIndex int)
	Refresh      func()
	OpenWebUI    func()
	ShowSettings func()
	Quit         func()
}

// BuildMenu lays out the menu for a state. It is a pure function so the self-test and unit
// tests can check exactly what the panel will show.
func BuildMenu(switchState *kvm.SwitchState, lastError string, current settings.Settings, actions MenuActions) []statusnotifier.MenuEntry {
	var entries []statusnotifier.MenuEntry
	if lastError != "" {
		entries = append(entries,
			statusnotifier.MenuEntry{ID: entryError, Label: escapeMnemonics(lastError), Enabled: false},
			statusnotifier.MenuEntry{ID: entryErrorSeparator, Separator: true})
	}
	ports := make([]kvm.Port, 0, defaultPortCount)
	if switchState != nil {
		ports = switchState.Ports
	} else {
		for portIndex := range defaultPortCount {
			ports = append(ports, kvm.Port{Index: portIndex, HasVideoLink: true})
		}
	}
	for _, port := range ports {
		portIndex := port.Index
		entries = append(entries, statusnotifier.MenuEntry{
			ID:      int32(portIndex + 1),
			Label:   MenuItemLabel(port, current),
			Enabled: switchState != nil,
			Radio:   true,
			Checked: switchState != nil && portIndex == switchState.ActivePortIndex,
			OnClick: func() { actions.SwitchPort(portIndex) },
		})
	}
	return append(entries,
		statusnotifier.MenuEntry{ID: entryPortsSeparator, Separator: true},
		statusnotifier.MenuEntry{ID: entryRefresh, Label: "_Refresh", Enabled: true, OnClick: actions.Refresh},
		statusnotifier.MenuEntry{ID: entryOpenWebUI, Label: "_Open KVM Web UI", Enabled: current.Host != "",
			OnClick: actions.OpenWebUI},
		statusnotifier.MenuEntry{ID: entrySettings, Label: "_Settings…", Enabled: true, OnClick: actions.ShowSettings},
		statusnotifier.MenuEntry{ID: entryActionsSeparator, Separator: true},
		statusnotifier.MenuEntry{ID: entryQuit, Label: "_Quit KVM Switcher", Enabled: true, OnClick: actions.Quit},
	)
}

// App is the running panel app.
type App struct {
	connection    *dbus.Conn
	settingsStore *settings.Store
	client        *kvm.Client
	item          *statusnotifier.Item

	mutex            sync.Mutex
	switchState      *kvm.SwitchState
	lastError        string
	switchInProgress bool
	settingsOpen     bool
	lastRefresh      time.Time

	refreshRequests chan struct{}
	quitRequests    chan struct{}
	quitOnce        sync.Once
}

// Run starts the panel app and blocks until it quits. It returns the process exit code.
func Run() int {
	connection, err := dbus.ConnectSessionBus()
	if err != nil {
		fmt.Fprintln(os.Stderr, "KVM Switcher needs a desktop session (no D-Bus session bus):", err)
		return 1
	}
	defer connection.Close()

	app := &App{
		connection:      connection,
		settingsStore:   settings.Open(""),
		refreshRequests: make(chan struct{}, 1),
		quitRequests:    make(chan struct{}),
	}
	app.client = kvm.NewClient(app.settingsStore, credentials.ReadPassword)

	// The item is exported before the instance name is claimed, so requests from other
	// processes always find it; the panel only learns about it in Register below.
	app.item, err = statusnotifier.New(connection, autostart.ApplicationID, applicationTitle)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Could not create the panel icon:", err)
		return 1
	}
	defer app.item.Close()

	if err := instance.Claim(connection, app); err != nil {
		if errors.Is(err, instance.ErrAlreadyRunning) {
			// Like opening the app again: show its settings.
			instance.ForwardShowSettings()
			return 0
		}
		fmt.Fprintln(os.Stderr, "Could not start KVM Switcher:", err)
		return 1
	}
	app.item.OnMenuOpened(app.menuOpened)
	app.updatePanel()
	if err := app.item.Register(func() {
		fmt.Fprintln(os.Stderr, MissingPanelMessage)
		desktop.Notify(applicationTitle, MissingPanelMessage)
	}); err != nil {
		fmt.Fprintln(os.Stderr, "Could not register the panel icon:", err)
		return 1
	}

	go app.refreshLoop()
	app.Refresh()
	if setup.NeedsFirstRun(app.settingsStore.Get()) {
		go app.ShowSettings()
	}

	stopSignals := make(chan os.Signal, 1)
	signal.Notify(stopSignals, syscall.SIGINT, syscall.SIGTERM)
	select {
	case <-stopSignals:
	case <-app.quitRequests:
	}
	return 0
}

// Quit ends the app.
func (app *App) Quit() {
	app.quitOnce.Do(func() { close(app.quitRequests) })
}

// Refresh re-reads the switch state in the background. Requests made while one is running
// are merged into a single follow-up refresh.
func (app *App) Refresh() {
	select {
	case app.refreshRequests <- struct{}{}:
	default:
	}
}

func (app *App) refreshLoop() {
	ticker := time.NewTicker(backgroundRefreshInterval)
	defer ticker.Stop()
	for {
		select {
		case <-app.refreshRequests:
		case <-ticker.C:
		case <-app.quitRequests:
			return
		}
		if app.settingsStore.ReloadIfChanged() {
			app.client.ForgetSession()
		}
		switchState, err := app.client.FetchSwitchState()
		app.mutex.Lock()
		app.lastRefresh = time.Now()
		if err != nil {
			app.lastError = err.Error()
		} else {
			app.switchState, app.lastError = &switchState, ""
		}
		app.mutex.Unlock()
		app.updatePanel()
	}
}

func (app *App) menuOpened() {
	app.mutex.Lock()
	refreshedRecently := time.Since(app.lastRefresh) < menuOpenRefreshInterval
	app.mutex.Unlock()
	if !refreshedRecently {
		app.Refresh()
	}
}

// SwitchPort switches to a zero-based port, from the menu or from `kvm-switcher switch N`.
func (app *App) SwitchPort(portIndex int) {
	app.mutex.Lock()
	outOfRange := app.switchState != nil && portIndex >= len(app.switchState.Ports)
	if app.switchInProgress || outOfRange {
		app.mutex.Unlock()
		return
	}
	app.switchInProgress = true
	app.mutex.Unlock()
	app.updatePanel()

	err := app.client.SetActivePort(portIndex)
	app.mutex.Lock()
	app.switchInProgress = false
	if err != nil {
		app.lastError = err.Error()
	}
	app.mutex.Unlock()
	app.updatePanel()
	app.Refresh()
}

// ReloadSettings is called after `kvm-switcher setup` in another process saved changes.
func (app *App) ReloadSettings() {
	app.settingsStore.ReloadIfChanged()
	app.client.ForgetSession()
	app.updatePanel()
	app.Refresh()
}

// ShowSettings opens the settings: desktop dialogs when zenity or kdialog is installed,
// otherwise the setup wizard in a terminal window.
func (app *App) ShowSettings() {
	app.mutex.Lock()
	if app.settingsOpen {
		app.mutex.Unlock()
		return
	}
	app.settingsOpen = true
	app.mutex.Unlock()
	defer func() {
		app.mutex.Lock()
		app.settingsOpen = false
		app.mutex.Unlock()
	}()

	if dialogPrompter, found := setup.FindDialogPrompter(); found {
		session := setup.Session{
			Prompter:  dialogPrompter,
			Store:     app.settingsStore,
			OnChanged: app.ReloadSettings,
		}
		session.Run()
		return
	}
	executablePath, err := os.Executable()
	if err != nil {
		executablePath = "kvm-switcher"
	}
	if err := desktop.RunInTerminal(executablePath, "setup", "--wait-at-end"); err != nil {
		desktop.Notify(applicationTitle, "Run \"kvm-switcher setup\" in a terminal to change the settings. "+
			"(Install zenity or kdialog for settings windows.)")
	}
}

func (app *App) openWebUI() {
	if host := app.settingsStore.Get().Host; host != "" {
		if err := desktop.OpenURL("https://" + host + "/"); err != nil {
			desktop.Notify(applicationTitle, "Could not open a browser: "+err.Error())
		}
	}
}

func (app *App) updatePanel() {
	app.mutex.Lock()
	switchState, lastError, switchInProgress := app.switchState, app.lastError, app.switchInProgress
	app.mutex.Unlock()
	current := app.settingsStore.Get()

	if lastError != "" {
		app.item.SetIcon(IconError, errorPixmaps)
		app.item.SetLabel("", "")
		app.item.SetToolTip(applicationTitle, lastError)
	} else {
		app.item.SetIcon(IconConnected, connectedPixmaps)
		labelText := PanelLabel(switchState, current)
		if switchInProgress {
			labelText = "…"
		}
		app.item.SetLabel(labelText, labelText)
		toolTip := "Not connected yet"
		if switchState != nil {
			if activePort, found := switchState.ActivePort(); found {
				toolTip = "Active: " + activePort.DisplayName(current)
			}
		}
		app.item.SetToolTip(applicationTitle, toolTip)
	}
	app.item.SetMenu(BuildMenu(switchState, lastError, current, MenuActions{
		SwitchPort:   app.SwitchPort,
		Refresh:      app.Refresh,
		OpenWebUI:    app.openWebUI,
		ShowSettings: app.ShowSettings,
		Quit:         app.Quit,
	}))
}

// Package selftest checks, without a KVM, that the panel icon, its menu and the keyring work
// on this machine. CI runs it in every distribution's container; users can run
// `kvm-switcher self-test` when the icon doesn't appear.
package selftest

import (
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"

	"github.com/godbus/dbus/v5"

	"github.com/mhuot/kvm-switcher-comet-x/linux/internal/credentials"
	"github.com/mhuot/kvm-switcher-comet-x/linux/internal/desktop"
	"github.com/mhuot/kvm-switcher-comet-x/linux/internal/kvm"
	"github.com/mhuot/kvm-switcher-comet-x/linux/internal/settings"
	"github.com/mhuot/kvm-switcher-comet-x/linux/internal/setup"
	"github.com/mhuot/kvm-switcher-comet-x/linux/internal/statusnotifier"
	"github.com/mhuot/kvm-switcher-comet-x/linux/internal/tray"
)

const registrationTimeout = 5 * time.Second

type outcome string

const (
	outcomePass outcome = "PASS"
	outcomeFail outcome = "FAIL"
	outcomeSkip outcome = "SKIP"
)

type result struct {
	outcome outcome
	name    string
	detail  string
}

// skipError marks a check that could not run here, which is not a failure.
type skipError string

func (reason skipError) Error() string { return string(reason) }

// Run prints PASS/FAIL/SKIP lines and returns the exit code (1 if anything failed).
func Run(output io.Writer) int {
	var results []result
	check := func(checkName string, checkAction func() error) {
		err := checkAction()
		switch typedError := err.(type) {
		case nil:
			results = append(results, result{outcomePass, checkName, ""})
		case skipError:
			results = append(results, result{outcomeSkip, checkName, string(typedError)})
		default:
			results = append(results, result{outcomeFail, checkName, err.Error()})
		}
	}

	check("menu and panel label", checkMenuAndLabel)
	check("panel icon over D-Bus", checkPanelItemOverDBus)
	check("desktop panel present", checkPanelPresent)
	check("keyring round trip", checkKeyring)
	check("settings dialogs", checkDialogTool)

	anyFailed := false
	for _, checkResult := range results {
		line := string(checkResult.outcome) + " " + checkResult.name
		if checkResult.detail != "" {
			line += ": " + checkResult.detail
		}
		fmt.Fprintln(output, line)
		anyFailed = anyFailed || checkResult.outcome == outcomeFail
	}
	if anyFailed {
		fmt.Fprintln(output, "RESULT FAIL")
		return 1
	}
	fmt.Fprintln(output, "RESULT PASS")
	return 0
}

var sampleState = kvm.SwitchState{
	ActivePortIndex: 2,
	Ports: []kvm.Port{
		{Index: 0, ID: "1.1", DeviceName: "Work_station", HasVideoLink: true},
		{Index: 1, ID: "1.2", DeviceName: "Laptop", HasVideoLink: true},
		{Index: 2, ID: "1.3", DeviceName: "Media PC", HasVideoLink: true},
		{Index: 3, ID: "1.4", DeviceName: "", HasVideoLink: false},
	},
}

var sampleSettings = settings.Settings{Host: "glkvm.local", LabelStyle: settings.LabelStylePortName}

func checkMenuAndLabel() error {
	entries := tray.BuildMenu(&sampleState, "", sampleSettings, tray.MenuActions{})
	if entries[0].Label != "_1   Work__station" {
		return fmt.Errorf("first entry %q", entries[0].Label)
	}
	if entries[3].Label != "_4   Port 4  (no video)" || !entries[2].Checked {
		return fmt.Errorf("unexpected port entries %+v", entries[:4])
	}
	if panelLabel := tray.PanelLabel(&sampleState, sampleSettings); panelLabel != "Media PC" {
		return fmt.Errorf("panel label %q", panelLabel)
	}
	return nil
}

// checkPanelItemOverDBus exports a real panel item, registers it with a stand-in panel and
// reads it back over the bus the way KDE or the GNOME extension does, clicking a port.
func checkPanelItemOverDBus() error {
	itemConnection, err := dbus.ConnectSessionBus()
	if err != nil {
		return skipError("no D-Bus session bus (" + err.Error() + ")")
	}
	defer itemConnection.Close()
	panelConnection, err := dbus.ConnectSessionBus()
	if err != nil {
		return err
	}
	defer panelConnection.Close()

	registrations := (<-chan string)(nil)
	if !statusnotifier.WatcherAvailable(panelConnection) {
		watcher, err := statusnotifier.StartTestWatcher(panelConnection)
		if err != nil {
			return fmt.Errorf("start stand-in panel: %w", err)
		}
		registrations = watcher.Registrations()
	}

	item, err := statusnotifier.New(itemConnection, "net.mhuot.KVMSwitcher.SelfTest", "KVM Switcher self-test")
	if err != nil {
		return err
	}
	defer item.Close()
	clickedPorts := make(chan int, 1)
	item.SetIcon("", tray.GlyphPixmaps(tray.GlyphConnected, sampleState.ActivePortIndex+1))
	item.SetLabel(tray.PanelLabel(&sampleState, sampleSettings), "")
	item.SetToolTip("KVM Switcher", "Active: Media PC")
	item.SetMenu(tray.BuildMenu(&sampleState, "", sampleSettings, tray.MenuActions{
		SwitchPort: func(portIndex int) { clickedPorts <- portIndex },
	}))
	if err := item.Register(nil); err != nil {
		return err
	}
	if registrations != nil {
		select {
		case registeredService := <-registrations:
			if registeredService != item.ServiceName() {
				return fmt.Errorf("registered %q, want %q", registeredService, item.ServiceName())
			}
		case <-time.After(registrationTimeout):
			return fmt.Errorf("the item never registered with the panel")
		}
	}

	itemObject := panelConnection.Object(item.ServiceName(), statusnotifier.ItemPath)
	for propertyName, wantValue := range map[string]any{
		"IconName":      "",
		"XAyatanaLabel": "Media PC",
		"ItemIsMenu":    true,
		"Menu":          statusnotifier.MenuPath,
	} {
		propertyValue, err := itemObject.GetProperty(statusnotifier.ItemInterface + "." + propertyName)
		if err != nil {
			return fmt.Errorf("read %s: %w", propertyName, err)
		}
		if propertyValue.Value() != wantValue {
			return fmt.Errorf("%s is %v, want %v", propertyName, propertyValue.Value(), wantValue)
		}
	}

	iconPixmapValue, err := itemObject.GetProperty(statusnotifier.ItemInterface + ".IconPixmap")
	if err != nil {
		return fmt.Errorf("read IconPixmap: %w", err)
	}
	var iconPixmaps []statusnotifier.Pixmap
	if err := dbus.Store([]any{iconPixmapValue.Value()}, &iconPixmaps); err != nil {
		return fmt.Errorf("decode IconPixmap: %w", err)
	}
	if len(iconPixmaps) == 0 || iconPixmaps[0].Width != 16 || len(iconPixmaps[0].Data) != 16*16*4 {
		return fmt.Errorf("unexpected icon pixmaps (%d of them)", len(iconPixmaps))
	}

	menuObject := panelConnection.Object(item.ServiceName(), statusnotifier.MenuPath)
	var revision uint32
	var layout struct {
		ID         int32
		Properties map[string]dbus.Variant
		Children   []dbus.Variant
	}
	if err := menuObject.Call(statusnotifier.MenuInterface+".GetLayout", 0, int32(0), int32(-1), []string{}).
		Store(&revision, &layout); err != nil {
		return fmt.Errorf("GetLayout: %w", err)
	}
	var menuLabels []string
	for _, child := range layout.Children {
		childFields, isStruct := child.Value().([]any)
		if !isStruct || len(childFields) < 2 {
			return fmt.Errorf("menu entry has the wrong shape: %v", child)
		}
		childProperties, _ := childFields[1].(map[string]dbus.Variant)
		if childLabel, hasLabel := childProperties["label"]; hasLabel {
			menuLabels = append(menuLabels, fmt.Sprint(childLabel.Value()))
		}
	}
	if len(menuLabels) < 4 || menuLabels[0] != "_1   Work__station" || menuLabels[2] != "_3   Media PC" {
		return fmt.Errorf("menu labels %q", menuLabels)
	}

	if err := menuObject.Call(statusnotifier.MenuInterface+".Event", 0, int32(2), "clicked",
		dbus.MakeVariant(""), uint32(0)).Err; err != nil {
		return fmt.Errorf("click: %w", err)
	}
	select {
	case clickedPort := <-clickedPorts:
		if clickedPort != 1 {
			return fmt.Errorf("clicking port 2 switched to index %d", clickedPort)
		}
	case <-time.After(registrationTimeout):
		return fmt.Errorf("clicking a port did nothing")
	}
	return nil
}

func checkPanelPresent() error {
	connection, err := dbus.ConnectSessionBus()
	if err != nil {
		return skipError("no D-Bus session bus")
	}
	defer connection.Close()
	if !statusnotifier.WatcherAvailable(connection) {
		return skipError("no panel is hosting icons right now; " + tray.MissingPanelMessage)
	}
	return nil
}

func checkKeyring() error {
	const probeAccount = "self-test"
	const probePassword = "self-test p&ss ü"
	if err := credentials.SaveToKeyring(probeAccount, probePassword); err != nil {
		return skipError(err.Error())
	}
	defer func() { _ = credentials.SaveToKeyring(probeAccount, "") }()
	readBack, source := credentials.Lookup(probeAccount)
	if readBack != probePassword || source != credentials.SourceKeyring {
		return fmt.Errorf("read back %q from %v", readBack, source)
	}
	return nil
}

func checkDialogTool() error {
	if dialogPrompter, found := setup.FindDialogPrompter(); found {
		return skipError("uses " + dialogPrompter.ToolName() + " (not opened by the self-test)")
	}
	if !desktop.HasDisplay() {
		return skipError("no display; use \"kvm-switcher setup\" in a terminal")
	}
	var installedTerminals []string
	for _, terminalName := range []string{"x-terminal-emulator", "gnome-terminal", "konsole", "xterm"} {
		if _, err := exec.LookPath(terminalName); err == nil {
			installedTerminals = append(installedTerminals, terminalName)
		}
	}
	return skipError("no zenity or kdialog; Settings opens a terminal (" + strings.Join(installedTerminals, ", ") + ")")
}

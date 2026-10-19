// Package statusnotifier puts an icon, a text label and a menu in the desktop panel using
// the StatusNotifierItem and com.canonical.dbusmenu D-Bus protocols directly, with no GTK
// or libappindicator. KDE Plasma, the GNOME "AppIndicator and KStatusNotifierItem Support"
// extension, Cinnamon, MATE, Xfce, Budgie, LXQt, waybar and others host these items.
package statusnotifier

import (
	"fmt"
	"os"
	"sync"

	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/introspect"
	"github.com/godbus/dbus/v5/prop"
)

// D-Bus names used by the StatusNotifierItem protocol.
const (
	ItemInterface      = "org.kde.StatusNotifierItem"
	ItemPath           = dbus.ObjectPath("/StatusNotifierItem")
	MenuInterface      = "com.canonical.dbusmenu"
	MenuPath           = dbus.ObjectPath("/MenuBar")
	WatcherName        = "org.kde.StatusNotifierWatcher"
	WatcherPath        = dbus.ObjectPath("/StatusNotifierWatcher")
	WatcherInterface   = "org.kde.StatusNotifierWatcher"
	nameOwnerChanged   = "NameOwnerChanged"
	freedesktopService = "org.freedesktop.DBus"
)

// Pixmap is one ARGB32 icon image, pixels in network byte order, as the protocol wants.
type Pixmap struct {
	Width  int32
	Height int32
	Data   []byte
}

// toolTip is the protocol's (sa(iiay)ss) tooltip structure.
type toolTip struct {
	IconName    string
	IconPixmap  []Pixmap
	Title       string
	Description string
}

// Item is one panel icon with its menu.
type Item struct {
	connection  *dbus.Conn
	serviceName string
	properties  *prop.Properties
	menu        *menu
	mutex       sync.Mutex
	stopWatch   chan struct{}
}

type itemMethods struct{}

// The host calls these on clicks. The menu is the whole interface (ItemIsMenu), so they do
// nothing; hosts that ignore ItemIsMenu open the menu themselves.
func (itemMethods) Activate(_, _ int32) *dbus.Error                { return nil }
func (itemMethods) SecondaryActivate(_, _ int32) *dbus.Error       { return nil }
func (itemMethods) ContextMenu(_, _ int32) *dbus.Error             { return nil }
func (itemMethods) Scroll(_ int32, _ string) *dbus.Error           { return nil }
func (itemMethods) ProvideXdgActivationToken(_ string) *dbus.Error { return nil }

// New exports a panel item on connection. Call Register once the icon, label and menu are set.
func New(connection *dbus.Conn, itemID, title string) (*Item, error) {
	item := &Item{
		connection:  connection,
		serviceName: fmt.Sprintf("org.kde.StatusNotifierItem-%d-1", os.Getpid()),
		stopWatch:   make(chan struct{}),
	}
	readOnly := func(value any) *prop.Prop {
		return &prop.Prop{Value: value, Writable: false, Emit: prop.EmitFalse}
	}
	itemProperties := map[string]*prop.Prop{
		"Category":              readOnly("Hardware"),
		"Id":                    readOnly(itemID),
		"Title":                 readOnly(title),
		"Status":                readOnly("Active"),
		"WindowId":              readOnly(int32(0)),
		"IconThemePath":         readOnly(""),
		"IconName":              readOnly(""),
		"IconPixmap":            readOnly([]Pixmap{}),
		"OverlayIconName":       readOnly(""),
		"OverlayIconPixmap":     readOnly([]Pixmap{}),
		"AttentionIconName":     readOnly(""),
		"AttentionIconPixmap":   readOnly([]Pixmap{}),
		"AttentionMovieName":    readOnly(""),
		"ToolTip":               readOnly(toolTip{IconPixmap: []Pixmap{}, Title: title}),
		"ItemIsMenu":            readOnly(true),
		"Menu":                  readOnly(MenuPath),
		"XAyatanaLabel":         readOnly(""),
		"XAyatanaLabelGuide":    readOnly(""),
		"XAyatanaOrderingIndex": readOnly(uint32(0)),
	}
	if err := connection.Export(itemMethods{}, ItemPath, ItemInterface); err != nil {
		return nil, err
	}
	item.menu = newMenu(connection)
	exportedProperties, err := prop.Export(connection, ItemPath, prop.Map{ItemInterface: itemProperties})
	if err != nil {
		return nil, err
	}
	item.properties = exportedProperties
	if err := item.menu.export(); err != nil {
		return nil, err
	}
	if err := connection.Export(introspect.Introspectable(itemIntrospection), ItemPath,
		"org.freedesktop.DBus.Introspectable"); err != nil {
		return nil, err
	}
	return item, nil
}

func (item *Item) emit(signalName string, arguments ...any) {
	_ = item.connection.Emit(ItemPath, ItemInterface+"."+signalName, arguments...)
}

// SetIcon sets the theme icon name and the pixmaps hosts use when the theme lacks it.
func (item *Item) SetIcon(iconName string, pixmaps []Pixmap) {
	item.mutex.Lock()
	defer item.mutex.Unlock()
	item.properties.SetMust(ItemInterface, "IconName", iconName)
	item.properties.SetMust(ItemInterface, "IconPixmap", pixmaps)
	item.emit("NewIcon")
}

// SetLabel sets the text beside the icon (the Ayatana label). guide is the widest text
// expected, so the panel can reserve space.
func (item *Item) SetLabel(label, guide string) {
	item.mutex.Lock()
	defer item.mutex.Unlock()
	item.properties.SetMust(ItemInterface, "XAyatanaLabel", label)
	item.properties.SetMust(ItemInterface, "XAyatanaLabelGuide", guide)
	item.emit("XAyatanaNewLabel", label, guide)
}

// SetToolTip sets the hover text.
func (item *Item) SetToolTip(title, description string) {
	item.mutex.Lock()
	defer item.mutex.Unlock()
	item.properties.SetMust(ItemInterface, "ToolTip",
		toolTip{IconPixmap: []Pixmap{}, Title: title, Description: description})
	item.emit("NewToolTip")
}

// SetMenu replaces the menu.
func (item *Item) SetMenu(entries []MenuEntry) {
	item.menu.setEntries(entries)
}

// OnMenuOpened is called (on its own goroutine) when the user opens the menu.
func (item *Item) OnMenuOpened(onOpened func()) {
	item.menu.setOnOpened(onOpened)
}

// WatcherAvailable reports whether a panel is hosting StatusNotifierItems right now.
func WatcherAvailable(connection *dbus.Conn) bool {
	var hasOwner bool
	err := connection.BusObject().Call(freedesktopService+".NameHasOwner", 0, WatcherName).Store(&hasOwner)
	return err == nil && hasOwner
}

// Register takes the item's bus name, registers with the panel's watcher and registers
// again whenever the panel restarts. It returns an error only if the bus name cannot be
// taken; with no panel running yet, onWatcherMissing is called and the item waits for one.
func (item *Item) Register(onWatcherMissing func()) error {
	reply, err := item.connection.RequestName(item.serviceName, dbus.NameFlagDoNotQueue)
	if err != nil {
		return err
	}
	if reply != dbus.RequestNameReplyPrimaryOwner {
		return fmt.Errorf("could not own %s", item.serviceName)
	}
	matchOptions := []dbus.MatchOption{
		dbus.WithMatchSender(freedesktopService),
		dbus.WithMatchInterface(freedesktopService),
		dbus.WithMatchMember(nameOwnerChanged),
		dbus.WithMatchArg(0, WatcherName),
	}
	if err := item.connection.AddMatchSignal(matchOptions...); err != nil {
		return err
	}
	ownerSignals := make(chan *dbus.Signal, 8)
	item.connection.Signal(ownerSignals)
	go func() {
		for {
			select {
			case ownerSignal := <-ownerSignals:
				if ownerSignal.Name != freedesktopService+"."+nameOwnerChanged || len(ownerSignal.Body) < 3 {
					continue
				}
				if changedName, _ := ownerSignal.Body[0].(string); changedName != WatcherName {
					continue
				}
				if newOwner, _ := ownerSignal.Body[2].(string); newOwner != "" {
					_ = item.registerWithWatcher()
				}
			case <-item.stopWatch:
				return
			}
		}
	}()
	if err := item.registerWithWatcher(); err != nil && onWatcherMissing != nil {
		onWatcherMissing()
	}
	return nil
}

func (item *Item) registerWithWatcher() error {
	return item.connection.Object(WatcherName, WatcherPath).
		Call(WatcherInterface+".RegisterStatusNotifierItem", 0, item.serviceName).Err
}

// ServiceName is the bus name the item registered under.
func (item *Item) ServiceName() string {
	return item.serviceName
}

// Close stops watching for panel restarts.
func (item *Item) Close() {
	select {
	case <-item.stopWatch:
	default:
		close(item.stopWatch)
	}
}

const itemIntrospection = `<node>
  <interface name="org.kde.StatusNotifierItem">
    <property name="Category" type="s" access="read"/>
    <property name="Id" type="s" access="read"/>
    <property name="Title" type="s" access="read"/>
    <property name="Status" type="s" access="read"/>
    <property name="WindowId" type="i" access="read"/>
    <property name="IconThemePath" type="s" access="read"/>
    <property name="IconName" type="s" access="read"/>
    <property name="IconPixmap" type="a(iiay)" access="read"/>
    <property name="OverlayIconName" type="s" access="read"/>
    <property name="OverlayIconPixmap" type="a(iiay)" access="read"/>
    <property name="AttentionIconName" type="s" access="read"/>
    <property name="AttentionIconPixmap" type="a(iiay)" access="read"/>
    <property name="AttentionMovieName" type="s" access="read"/>
    <property name="ToolTip" type="(sa(iiay)ss)" access="read"/>
    <property name="ItemIsMenu" type="b" access="read"/>
    <property name="Menu" type="o" access="read"/>
    <property name="XAyatanaLabel" type="s" access="read"/>
    <property name="XAyatanaLabelGuide" type="s" access="read"/>
    <property name="XAyatanaOrderingIndex" type="u" access="read"/>
    <method name="ContextMenu"><arg name="x" type="i" direction="in"/><arg name="y" type="i" direction="in"/></method>
    <method name="Activate"><arg name="x" type="i" direction="in"/><arg name="y" type="i" direction="in"/></method>
    <method name="SecondaryActivate"><arg name="x" type="i" direction="in"/><arg name="y" type="i" direction="in"/></method>
    <method name="Scroll"><arg name="delta" type="i" direction="in"/><arg name="orientation" type="s" direction="in"/></method>
    <method name="ProvideXdgActivationToken"><arg name="token" type="s" direction="in"/></method>
    <signal name="NewTitle"/>
    <signal name="NewIcon"/>
    <signal name="NewAttentionIcon"/>
    <signal name="NewOverlayIcon"/>
    <signal name="NewToolTip"/>
    <signal name="NewStatus"><arg name="status" type="s"/></signal>
    <signal name="XAyatanaNewLabel"><arg name="label" type="s"/><arg name="guide" type="s"/></signal>
  </interface>` + prop.IntrospectDataString + introspect.IntrospectDeclarationString + `</node>`

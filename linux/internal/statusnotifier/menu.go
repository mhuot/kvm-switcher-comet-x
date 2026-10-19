package statusnotifier

import (
	"sync"

	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/introspect"
	"github.com/godbus/dbus/v5/prop"
)

// MenuEntry is one row of the panel menu. A flat list is all this app needs.
type MenuEntry struct {
	// ID must be unique and non-zero (0 is the menu itself).
	ID        int32
	Label     string
	Enabled   bool
	Separator bool
	// Radio draws the entry as a radio item; Checked selects it.
	Radio   bool
	Checked bool
	OnClick func()
}

// menuLayout is dbusmenu's (ia{sv}av) layout node.
type menuLayout struct {
	ID         int32
	Properties map[string]dbus.Variant
	Children   []dbus.Variant
}

// menuItemProperties is dbusmenu's (ia{sv}) entry.
type menuItemProperties struct {
	ID         int32
	Properties map[string]dbus.Variant
}

// menuEvent is dbusmenu's (isvu) event.
type menuEvent struct {
	ID        int32
	EventID   string
	Data      dbus.Variant
	Timestamp uint32
}

type menu struct {
	connection *dbus.Conn
	mutex      sync.Mutex
	revision   uint32
	entries    []MenuEntry
	onOpened   func()
}

func newMenu(connection *dbus.Conn) *menu {
	return &menu{connection: connection, revision: 1}
}

func (exported *menu) export() error {
	if err := exported.connection.Export(menuMethods{exported}, MenuPath, MenuInterface); err != nil {
		return err
	}
	readOnly := func(value any) *prop.Prop {
		return &prop.Prop{Value: value, Writable: false, Emit: prop.EmitFalse}
	}
	if _, err := prop.Export(exported.connection, MenuPath, prop.Map{MenuInterface: {
		"Version":       readOnly(uint32(3)),
		"TextDirection": readOnly("ltr"),
		"Status":        readOnly("normal"),
		"IconThemePath": readOnly([]string{}),
	}}); err != nil {
		return err
	}
	return exported.connection.Export(introspect.Introspectable(menuIntrospection), MenuPath,
		"org.freedesktop.DBus.Introspectable")
}

func (exported *menu) setEntries(entries []MenuEntry) {
	exported.mutex.Lock()
	exported.entries = append([]MenuEntry(nil), entries...)
	exported.revision++
	revision := exported.revision
	exported.mutex.Unlock()
	_ = exported.connection.Emit(MenuPath, MenuInterface+".LayoutUpdated", revision, int32(0))
}

func (exported *menu) setOnOpened(onOpened func()) {
	exported.mutex.Lock()
	defer exported.mutex.Unlock()
	exported.onOpened = onOpened
}

func (exported *menu) menuOpened() {
	exported.mutex.Lock()
	onOpened := exported.onOpened
	exported.mutex.Unlock()
	if onOpened != nil {
		go onOpened()
	}
}

func entryProperties(entry MenuEntry) map[string]dbus.Variant {
	if entry.Separator {
		return map[string]dbus.Variant{
			"type":    dbus.MakeVariant("separator"),
			"visible": dbus.MakeVariant(true),
		}
	}
	properties := map[string]dbus.Variant{
		"label":   dbus.MakeVariant(entry.Label),
		"enabled": dbus.MakeVariant(entry.Enabled),
		"visible": dbus.MakeVariant(true),
	}
	if entry.Radio {
		toggleState := int32(0)
		if entry.Checked {
			toggleState = 1
		}
		properties["toggle-type"] = dbus.MakeVariant("radio")
		properties["toggle-state"] = dbus.MakeVariant(toggleState)
	}
	return properties
}

func filterProperties(properties map[string]dbus.Variant, propertyNames []string) map[string]dbus.Variant {
	if len(propertyNames) == 0 {
		return properties
	}
	filtered := make(map[string]dbus.Variant, len(propertyNames))
	for _, propertyName := range propertyNames {
		if value, present := properties[propertyName]; present {
			filtered[propertyName] = value
		}
	}
	return filtered
}

func (exported *menu) findEntry(entryID int32) (MenuEntry, bool) {
	for _, entry := range exported.entries {
		if entry.ID == entryID {
			return entry, true
		}
	}
	return MenuEntry{}, false
}

type menuMethods struct {
	exported *menu
}

func (methods menuMethods) GetLayout(parentID, recursionDepth int32, propertyNames []string) (uint32, menuLayout, *dbus.Error) {
	exported := methods.exported
	exported.mutex.Lock()
	defer exported.mutex.Unlock()
	if parentID != 0 {
		entry, found := exported.findEntry(parentID)
		if !found {
			return 0, menuLayout{}, dbus.MakeFailedError(errUnknownMenuID)
		}
		return exported.revision, menuLayout{
			ID:         entry.ID,
			Properties: filterProperties(entryProperties(entry), propertyNames),
			Children:   []dbus.Variant{},
		}, nil
	}
	root := menuLayout{
		ID:         0,
		Properties: map[string]dbus.Variant{"children-display": dbus.MakeVariant("submenu")},
		Children:   []dbus.Variant{},
	}
	if recursionDepth != 0 {
		for _, entry := range exported.entries {
			root.Children = append(root.Children, dbus.MakeVariant(menuLayout{
				ID:         entry.ID,
				Properties: filterProperties(entryProperties(entry), propertyNames),
				Children:   []dbus.Variant{},
			}))
		}
	}
	return exported.revision, root, nil
}

func (methods menuMethods) GetGroupProperties(entryIDs []int32, propertyNames []string) ([]menuItemProperties, *dbus.Error) {
	exported := methods.exported
	exported.mutex.Lock()
	defer exported.mutex.Unlock()
	groupProperties := []menuItemProperties{}
	for _, entry := range exported.entries {
		requested := len(entryIDs) == 0
		for _, entryID := range entryIDs {
			requested = requested || entryID == entry.ID
		}
		if requested {
			groupProperties = append(groupProperties, menuItemProperties{
				ID:         entry.ID,
				Properties: filterProperties(entryProperties(entry), propertyNames),
			})
		}
	}
	return groupProperties, nil
}

func (methods menuMethods) GetProperty(entryID int32, propertyName string) (dbus.Variant, *dbus.Error) {
	exported := methods.exported
	exported.mutex.Lock()
	defer exported.mutex.Unlock()
	entry, found := exported.findEntry(entryID)
	if !found {
		return dbus.Variant{}, dbus.MakeFailedError(errUnknownMenuID)
	}
	value, present := entryProperties(entry)[propertyName]
	if !present {
		return dbus.Variant{}, dbus.MakeFailedError(errUnknownProperty)
	}
	return value, nil
}

func (methods menuMethods) handleEvent(entryID int32, eventID string) {
	exported := methods.exported
	if entryID == 0 && eventID == "opened" {
		exported.menuOpened()
		return
	}
	if eventID != "clicked" {
		return
	}
	exported.mutex.Lock()
	entry, found := exported.findEntry(entryID)
	exported.mutex.Unlock()
	if found && entry.Enabled && entry.OnClick != nil {
		go entry.OnClick()
	}
}

func (methods menuMethods) Event(entryID int32, eventID string, _ dbus.Variant, _ uint32) *dbus.Error {
	methods.handleEvent(entryID, eventID)
	return nil
}

func (methods menuMethods) EventGroup(events []menuEvent) ([]int32, *dbus.Error) {
	for _, event := range events {
		methods.handleEvent(event.ID, event.EventID)
	}
	return []int32{}, nil
}

func (methods menuMethods) AboutToShow(entryID int32) (bool, *dbus.Error) {
	if entryID == 0 {
		methods.exported.menuOpened()
	}
	return false, nil
}

func (methods menuMethods) AboutToShowGroup(entryIDs []int32) ([]int32, []int32, *dbus.Error) {
	for _, entryID := range entryIDs {
		if entryID == 0 {
			methods.exported.menuOpened()
		}
	}
	return []int32{}, []int32{}, nil
}

type menuError string

func (message menuError) Error() string { return string(message) }

const (
	errUnknownMenuID   = menuError("unknown menu item")
	errUnknownProperty = menuError("unknown property")
)

const menuIntrospection = `<node>
  <interface name="com.canonical.dbusmenu">
    <property name="Version" type="u" access="read"/>
    <property name="TextDirection" type="s" access="read"/>
    <property name="Status" type="s" access="read"/>
    <property name="IconThemePath" type="as" access="read"/>
    <method name="GetLayout">
      <arg name="parentId" type="i" direction="in"/>
      <arg name="recursionDepth" type="i" direction="in"/>
      <arg name="propertyNames" type="as" direction="in"/>
      <arg name="revision" type="u" direction="out"/>
      <arg name="layout" type="(ia{sv}av)" direction="out"/>
    </method>
    <method name="GetGroupProperties">
      <arg name="ids" type="ai" direction="in"/>
      <arg name="propertyNames" type="as" direction="in"/>
      <arg name="properties" type="a(ia{sv})" direction="out"/>
    </method>
    <method name="GetProperty">
      <arg name="id" type="i" direction="in"/>
      <arg name="name" type="s" direction="in"/>
      <arg name="value" type="v" direction="out"/>
    </method>
    <method name="Event">
      <arg name="id" type="i" direction="in"/>
      <arg name="eventId" type="s" direction="in"/>
      <arg name="data" type="v" direction="in"/>
      <arg name="timestamp" type="u" direction="in"/>
    </method>
    <method name="EventGroup">
      <arg name="events" type="a(isvu)" direction="in"/>
      <arg name="idErrors" type="ai" direction="out"/>
    </method>
    <method name="AboutToShow">
      <arg name="id" type="i" direction="in"/>
      <arg name="needUpdate" type="b" direction="out"/>
    </method>
    <method name="AboutToShowGroup">
      <arg name="ids" type="ai" direction="in"/>
      <arg name="updatesNeeded" type="ai" direction="out"/>
      <arg name="idErrors" type="ai" direction="out"/>
    </method>
    <signal name="ItemsPropertiesUpdated">
      <arg name="updatedProps" type="a(ia{sv})"/>
      <arg name="removedProps" type="a(ias)"/>
    </signal>
    <signal name="LayoutUpdated">
      <arg name="revision" type="u"/>
      <arg name="parent" type="i"/>
    </signal>
    <signal name="ItemActivationRequested">
      <arg name="id" type="i"/>
      <arg name="timestamp" type="u"/>
    </signal>
  </interface>` + prop.IntrospectDataString + introspect.IntrospectDeclarationString + `</node>`

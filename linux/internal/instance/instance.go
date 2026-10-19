// Package instance keeps one tray running per session and lets other processes reach it.
//
// The tray owns the session bus name net.mhuot.KVMSwitcher. `kvm-switcher switch N` (bound
// to a desktop keyboard shortcut, since Wayland does not let apps grab global hotkeys) asks
// the running tray to switch so its panel label updates at once, and `kvm-switcher setup`
// asks it to reload the settings it just saved.
package instance

import (
	"context"
	"errors"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/introspect"
)

// The D-Bus address of the running tray.
const (
	BusName     = "net.mhuot.KVMSwitcher"
	ObjectPath  = dbus.ObjectPath("/net/mhuot/KVMSwitcher")
	Interface   = "net.mhuot.KVMSwitcher"
	callTimeout = 3 * time.Second
)

// Handler is what the running tray does when another process asks.
type Handler interface {
	SwitchPort(portIndex int)
	ShowSettings()
	ReloadSettings()
}

type exportedService struct {
	handler Handler
}

func (service exportedService) SwitchPort(portIndex int32) *dbus.Error {
	if portIndex < 0 {
		return dbus.MakeFailedError(errors.New("port index must not be negative"))
	}
	go service.handler.SwitchPort(int(portIndex))
	return nil
}

func (service exportedService) ShowSettings() *dbus.Error {
	go service.handler.ShowSettings()
	return nil
}

func (service exportedService) ReloadSettings() *dbus.Error {
	go service.handler.ReloadSettings()
	return nil
}

const introspection = `<node>
  <interface name="` + Interface + `">
    <method name="SwitchPort"><arg name="port_index" type="i" direction="in"/></method>
    <method name="ShowSettings"/>
    <method name="ReloadSettings"/>
  </interface>` + introspect.IntrospectDeclarationString + `</node>`

// ErrAlreadyRunning means another tray already owns the bus name.
var ErrAlreadyRunning = errors.New("KVM Switcher is already running")

// Claim exports the service on connection and takes the bus name. It returns
// ErrAlreadyRunning when another tray has it.
func Claim(connection *dbus.Conn, handler Handler) error {
	if err := connection.Export(exportedService{handler: handler}, ObjectPath, Interface); err != nil {
		return err
	}
	if err := connection.Export(introspect.Introspectable(introspection), ObjectPath,
		"org.freedesktop.DBus.Introspectable"); err != nil {
		return err
	}
	reply, err := connection.RequestName(BusName, dbus.NameFlagDoNotQueue)
	if err != nil {
		return err
	}
	if reply != dbus.RequestNameReplyPrimaryOwner {
		return ErrAlreadyRunning
	}
	return nil
}

// call invokes a method on the running tray. It returns false when no tray is running or
// there is no session bus.
func call(method string, arguments ...any) bool {
	connection, err := dbus.ConnectSessionBus()
	if err != nil {
		return false
	}
	defer connection.Close()
	var hasOwner bool
	if connection.BusObject().Call("org.freedesktop.DBus.NameHasOwner", 0, BusName).
		Store(&hasOwner) != nil || !hasOwner {
		return false
	}
	callContext, cancel := context.WithTimeout(context.Background(), callTimeout)
	defer cancel()
	return connection.Object(BusName, ObjectPath).
		CallWithContext(callContext, Interface+"."+method, 0, arguments...).Err == nil
}

// ForwardSwitch asks a running tray to switch to a zero-based port.
func ForwardSwitch(portIndex int) bool {
	return call("SwitchPort", int32(portIndex))
}

// ForwardShowSettings asks a running tray to open its settings.
func ForwardShowSettings() bool {
	return call("ShowSettings")
}

// NotifyReload tells a running tray that the settings or password changed.
func NotifyReload() bool {
	return call("ReloadSettings")
}

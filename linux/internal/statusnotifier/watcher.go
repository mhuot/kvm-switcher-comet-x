package statusnotifier

import (
	"strings"
	"sync"

	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/prop"
)

// TestWatcher is a minimal StatusNotifierWatcher, standing in for a desktop panel in the
// self-test and in CI containers that have a session bus but no desktop.
type TestWatcher struct {
	mutex      sync.Mutex
	registered []string
	onRegister chan string
}

// StartTestWatcher owns org.kde.StatusNotifierWatcher on connection. It fails if a real
// panel already owns the name.
func StartTestWatcher(connection *dbus.Conn) (*TestWatcher, error) {
	watcher := &TestWatcher{onRegister: make(chan string, 8)}
	if err := connection.Export(watcherMethods{watcher}, WatcherPath, WatcherInterface); err != nil {
		return nil, err
	}
	if _, err := prop.Export(connection, WatcherPath, prop.Map{WatcherInterface: {
		"RegisteredStatusNotifierItems":  {Value: []string{}, Emit: prop.EmitFalse},
		"IsStatusNotifierHostRegistered": {Value: true, Emit: prop.EmitFalse},
		"ProtocolVersion":                {Value: int32(0), Emit: prop.EmitFalse},
	}}); err != nil {
		return nil, err
	}
	reply, err := connection.RequestName(WatcherName, dbus.NameFlagDoNotQueue)
	if err != nil {
		return nil, err
	}
	if reply != dbus.RequestNameReplyPrimaryOwner {
		return nil, errWatcherTaken
	}
	return watcher, nil
}

// Registrations returns the item services registered so far.
func (watcher *TestWatcher) Registrations() <-chan string {
	return watcher.onRegister
}

type watcherMethods struct {
	watcher *TestWatcher
}

func (methods watcherMethods) RegisterStatusNotifierItem(service string, sender dbus.Sender) *dbus.Error {
	// Items may register a bus name or just an object path (then the sender is the service).
	if strings.HasPrefix(service, "/") {
		service = string(sender) + service
	}
	methods.watcher.mutex.Lock()
	methods.watcher.registered = append(methods.watcher.registered, service)
	methods.watcher.mutex.Unlock()
	select {
	case methods.watcher.onRegister <- service:
	default:
	}
	return nil
}

func (methods watcherMethods) RegisterStatusNotifierHost(string) *dbus.Error {
	return nil
}

const errWatcherTaken = menuError("a StatusNotifierWatcher (a desktop panel) is already running")

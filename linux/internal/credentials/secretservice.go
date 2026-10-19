package credentials

import (
	"errors"
	"fmt"
	"time"

	"github.com/godbus/dbus/v5"
)

// A minimal client for the freedesktop Secret Service D-Bus API, which GNOME Keyring,
// KWallet (Plasma 5.97+) and KeePassXC all provide. It stores items the way libsecret does,
// with an "xdg:schema" attribute, so it finds the password the earlier Python app saved.

const (
	secretServiceName      = "org.freedesktop.secrets"
	secretServicePath      = dbus.ObjectPath("/org/freedesktop/secrets")
	secretServiceInterface = "org.freedesktop.Secret.Service"
	collectionInterface    = "org.freedesktop.Secret.Collection"
	itemInterface          = "org.freedesktop.Secret.Item"
	promptInterface        = "org.freedesktop.Secret.Prompt"
	noPrompt               = dbus.ObjectPath("/")
	promptTimeout          = 2 * time.Minute
	secretContentType      = "text/plain; charset=utf8"
)

// ErrKeyringUnavailable means no Secret Service is running, or it refused the request.
var ErrKeyringUnavailable = errors.New("no desktop keyring (Secret Service) is available")

// secret is the Secret Service's (oayays) secret structure.
type secret struct {
	Session     dbus.ObjectPath
	Parameters  []byte
	Value       []byte
	ContentType string
}

type secretService struct {
	connection *dbus.Conn
	service    dbus.BusObject
	session    dbus.ObjectPath
}

func openSecretService() (*secretService, error) {
	connection, err := dbus.ConnectSessionBus()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrKeyringUnavailable, err)
	}
	opened := &secretService{
		connection: connection,
		service:    connection.Object(secretServiceName, secretServicePath),
	}
	var ignoredOutput dbus.Variant
	err = opened.service.Call(secretServiceInterface+".OpenSession", 0,
		"plain", dbus.MakeVariant("")).Store(&ignoredOutput, &opened.session)
	if err != nil {
		connection.Close()
		return nil, fmt.Errorf("%w: %v", ErrKeyringUnavailable, err)
	}
	return opened, nil
}

func (opened *secretService) close() {
	_ = opened.connection.Object(secretServiceName, opened.session).
		Call("org.freedesktop.Secret.Session.Close", 0).Err
	opened.connection.Close()
}

// waitForPrompt runs a Secret Service prompt (for example "unlock your keyring") and waits
// for the user to answer it.
func (opened *secretService) waitForPrompt(promptPath dbus.ObjectPath) (dbus.Variant, error) {
	if promptPath == noPrompt || promptPath == "" {
		return dbus.Variant{}, nil
	}
	matchOptions := []dbus.MatchOption{
		dbus.WithMatchObjectPath(promptPath),
		dbus.WithMatchInterface(promptInterface),
		dbus.WithMatchMember("Completed"),
	}
	if err := opened.connection.AddMatchSignal(matchOptions...); err != nil {
		return dbus.Variant{}, err
	}
	defer func() { _ = opened.connection.RemoveMatchSignal(matchOptions...) }()
	completedSignals := make(chan *dbus.Signal, 4)
	opened.connection.Signal(completedSignals)
	defer opened.connection.RemoveSignal(completedSignals)

	if err := opened.connection.Object(secretServiceName, promptPath).
		Call(promptInterface+".Prompt", 0, "").Err; err != nil {
		return dbus.Variant{}, err
	}
	timeout := time.After(promptTimeout)
	for {
		select {
		case completedSignal := <-completedSignals:
			if completedSignal.Path != promptPath || len(completedSignal.Body) < 2 {
				continue
			}
			if dismissed, _ := completedSignal.Body[0].(bool); dismissed {
				return dbus.Variant{}, fmt.Errorf("%w: the keyring prompt was dismissed", ErrKeyringUnavailable)
			}
			promptResult, _ := completedSignal.Body[1].(dbus.Variant)
			return promptResult, nil
		case <-timeout:
			return dbus.Variant{}, fmt.Errorf("%w: the keyring prompt timed out", ErrKeyringUnavailable)
		}
	}
}

func (opened *secretService) unlock(objectPaths []dbus.ObjectPath) error {
	var unlockedPaths []dbus.ObjectPath
	var promptPath dbus.ObjectPath
	if err := opened.service.Call(secretServiceInterface+".Unlock", 0, objectPaths).
		Store(&unlockedPaths, &promptPath); err != nil {
		return err
	}
	_, err := opened.waitForPrompt(promptPath)
	return err
}

func (opened *secretService) searchItems(attributes map[string]string) ([]dbus.ObjectPath, error) {
	var unlockedItems, lockedItems []dbus.ObjectPath
	if err := opened.service.Call(secretServiceInterface+".SearchItems", 0, attributes).
		Store(&unlockedItems, &lockedItems); err != nil {
		return nil, err
	}
	if len(lockedItems) > 0 {
		if err := opened.unlock(lockedItems); err != nil {
			return nil, err
		}
	}
	return append(unlockedItems, lockedItems...), nil
}

func (opened *secretService) readSecret(itemPath dbus.ObjectPath) (string, error) {
	var itemSecret secret
	if err := opened.connection.Object(secretServiceName, itemPath).
		Call(itemInterface+".GetSecret", 0, opened.session).Store(&itemSecret); err != nil {
		return "", err
	}
	return string(itemSecret.Value), nil
}

// collectionExists reports whether a collection object answers at path.
func (opened *secretService) collectionExists(collectionPath dbus.ObjectPath) bool {
	_, err := opened.connection.Object(secretServiceName, collectionPath).
		GetProperty(collectionInterface + ".Locked")
	return err == nil
}

// defaultCollection returns the default collection, unlocking it when needed. Like
// libsecret's COLLECTION_DEFAULT it tries the "default" alias object first, then the alias
// lookup, then the "login" collection, and only then asks to create one (which needs a
// password prompt).
func (opened *secretService) defaultCollection() (dbus.ObjectPath, error) {
	collectionPath := dbus.ObjectPath("/org/freedesktop/secrets/aliases/default")
	if !opened.collectionExists(collectionPath) {
		collectionPath = noPrompt
		if err := opened.service.Call(secretServiceInterface+".ReadAlias", 0, "default").
			Store(&collectionPath); err != nil {
			return "", err
		}
	}
	if collectionPath == noPrompt || collectionPath == "" {
		if loginPath := dbus.ObjectPath("/org/freedesktop/secrets/collection/login"); opened.collectionExists(loginPath) {
			collectionPath = loginPath
		}
	}
	if collectionPath == noPrompt || collectionPath == "" {
		collectionProperties := map[string]dbus.Variant{
			collectionInterface + ".Label": dbus.MakeVariant("Login"),
		}
		var promptPath dbus.ObjectPath
		if err := opened.service.Call(secretServiceInterface+".CreateCollection", 0,
			collectionProperties, "default").Store(&collectionPath, &promptPath); err != nil {
			return "", err
		}
		if collectionPath == noPrompt {
			promptResult, err := opened.waitForPrompt(promptPath)
			if err != nil {
				return "", err
			}
			collectionPath, _ = promptResult.Value().(dbus.ObjectPath)
		}
		if collectionPath == noPrompt || collectionPath == "" {
			return "", fmt.Errorf("%w: the keyring has no default collection", ErrKeyringUnavailable)
		}
	}
	lockedProperty, err := opened.connection.Object(secretServiceName, collectionPath).
		GetProperty(collectionInterface + ".Locked")
	if err == nil {
		if isLocked, _ := lockedProperty.Value().(bool); isLocked {
			if err := opened.unlock([]dbus.ObjectPath{collectionPath}); err != nil {
				return "", err
			}
		}
	}
	return collectionPath, nil
}

func (opened *secretService) storeSecret(label string, attributes map[string]string, value string) error {
	collectionPath, err := opened.defaultCollection()
	if err != nil {
		return err
	}
	itemProperties := map[string]dbus.Variant{
		itemInterface + ".Label":      dbus.MakeVariant(label),
		itemInterface + ".Attributes": dbus.MakeVariant(attributes),
	}
	itemSecret := secret{Session: opened.session, Value: []byte(value), ContentType: secretContentType}
	var itemPath, promptPath dbus.ObjectPath
	if err := opened.connection.Object(secretServiceName, collectionPath).
		Call(collectionInterface+".CreateItem", 0, itemProperties, itemSecret, true).
		Store(&itemPath, &promptPath); err != nil {
		return err
	}
	_, err = opened.waitForPrompt(promptPath)
	return err
}

func (opened *secretService) deleteItems(attributes map[string]string) error {
	itemPaths, err := opened.searchItems(attributes)
	if err != nil {
		return err
	}
	for _, itemPath := range itemPaths {
		var promptPath dbus.ObjectPath
		if err := opened.connection.Object(secretServiceName, itemPath).
			Call(itemInterface+".Delete", 0).Store(&promptPath); err != nil {
			return err
		}
		if _, err := opened.waitForPrompt(promptPath); err != nil {
			return err
		}
	}
	return nil
}

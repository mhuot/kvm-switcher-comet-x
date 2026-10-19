// Package settings stores the non-secret preferences as JSON under
// $XDG_CONFIG_HOME/kvm-switcher/settings.json. The file format is the one the earlier
// Python app wrote, so existing settings and the pinned certificate carry over.
// The password lives in the desktop keyring (see package credentials).
package settings

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Label styles for the text beside the panel icon.
const (
	LabelStylePortName   = "port_name"
	LabelStylePortNumber = "port_number"
	LabelStyleIconOnly   = "icon_only"
)

// LabelStyles lists every valid label style in display order.
var LabelStyles = []string{LabelStylePortName, LabelStylePortNumber, LabelStyleIconOnly}

// LabelStyleTitles are the names shown to the user for each label style.
var LabelStyleTitles = map[string]string{
	LabelStylePortName:   "Port name",
	LabelStylePortNumber: "Port number",
	LabelStyleIconOnly:   "Icon only",
}

// Settings are the user's preferences.
type Settings struct {
	Host                         string   `json:"host"`
	LocalPortNames               []string `json:"local_port_names"`
	LabelStyle                   string   `json:"label_style"`
	PinnedCertificateFingerprint string   `json:"pinned_certificate_fingerprint"`
}

// LocalName returns the user's fallback name for a zero-based port, or "".
func (current Settings) LocalName(portIndex int) string {
	if portIndex >= 0 && portIndex < len(current.LocalPortNames) {
		return strings.TrimSpace(current.LocalPortNames[portIndex])
	}
	return ""
}

// ConfigDirectory returns the per-user config directory, honouring XDG_CONFIG_HOME.
func ConfigDirectory() string {
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		homeDirectory, _ := os.UserHomeDir()
		configHome = filepath.Join(homeDirectory, ".config")
	}
	return filepath.Join(configHome, "kvm-switcher")
}

// DefaultPath returns the settings file path.
func DefaultPath() string {
	return filepath.Join(ConfigDirectory(), "settings.json")
}

// NormalizeHost accepts "https://1.2.3.4/", "1.2.3.4" and so on and returns the host part.
func NormalizeHost(rawHost string) string {
	trimmedHost := strings.TrimSpace(rawHost)
	for _, scheme := range []string{"https://", "http://"} {
		if strings.HasPrefix(strings.ToLower(trimmedHost), scheme) {
			trimmedHost = trimmedHost[len(scheme):]
		}
	}
	return strings.TrimRight(trimmedHost, "/")
}

// Store loads, saves and shares one settings file between goroutines.
type Store struct {
	path         string
	mutex        sync.Mutex
	current      Settings
	loadedChange time.Time
}

// Open loads the settings file at path (DefaultPath when empty). A missing or damaged file
// gives defaults rather than an error, as before.
func Open(path string) *Store {
	if path == "" {
		path = DefaultPath()
	}
	store := &Store{path: path}
	store.current, store.loadedChange = readFile(path)
	return store
}

// Path returns the settings file path.
func (store *Store) Path() string {
	return store.path
}

// Get returns a copy of the current settings.
func (store *Store) Get() Settings {
	store.mutex.Lock()
	defer store.mutex.Unlock()
	return copySettings(store.current)
}

// Update changes the settings and saves them atomically.
func (store *Store) Update(change func(*Settings)) error {
	store.mutex.Lock()
	defer store.mutex.Unlock()
	updated := copySettings(store.current)
	change(&updated)
	if err := writeFile(store.path, updated); err != nil {
		return err
	}
	store.current = updated
	store.loadedChange = modificationTime(store.path)
	return nil
}

// ReloadIfChanged re-reads the file when another process (such as `kvm-switcher setup` in a
// terminal) has changed it. It reports whether anything was reloaded.
func (store *Store) ReloadIfChanged() bool {
	store.mutex.Lock()
	defer store.mutex.Unlock()
	if modificationTime(store.path).Equal(store.loadedChange) {
		return false
	}
	store.current, store.loadedChange = readFile(store.path)
	return true
}

func copySettings(original Settings) Settings {
	duplicate := original
	duplicate.LocalPortNames = append([]string(nil), original.LocalPortNames...)
	return duplicate
}

func modificationTime(path string) time.Time {
	fileInfo, err := os.Stat(path)
	if err != nil {
		return time.Time{}
	}
	return fileInfo.ModTime()
}

func readFile(path string) (Settings, time.Time) {
	defaults := Settings{LabelStyle: LabelStylePortName}
	fileChange := modificationTime(path)
	fileBytes, err := os.ReadFile(path)
	if err != nil {
		return defaults, fileChange
	}
	var stored struct {
		Host                         any `json:"host"`
		LocalPortNames               any `json:"local_port_names"`
		LabelStyle                   any `json:"label_style"`
		PinnedCertificateFingerprint any `json:"pinned_certificate_fingerprint"`
	}
	if json.Unmarshal(fileBytes, &stored) != nil {
		return defaults, fileChange
	}
	loaded := defaults
	loaded.Host, _ = stored.Host.(string)
	loaded.PinnedCertificateFingerprint, _ = stored.PinnedCertificateFingerprint.(string)
	if labelStyle, isString := stored.LabelStyle.(string); isString && validLabelStyle(labelStyle) {
		loaded.LabelStyle = labelStyle
	}
	if storedNames, isList := stored.LocalPortNames.([]any); isList {
		for _, storedName := range storedNames {
			if portName, isString := storedName.(string); isString {
				loaded.LocalPortNames = append(loaded.LocalPortNames, portName)
			}
		}
	}
	return loaded, fileChange
}

func validLabelStyle(labelStyle string) bool {
	for _, knownStyle := range LabelStyles {
		if labelStyle == knownStyle {
			return true
		}
	}
	return false
}

// writeFile saves atomically so a crash never leaves a half-written file.
func writeFile(path string, values Settings) error {
	if values.LocalPortNames == nil {
		values.LocalPortNames = []string{}
	}
	stored := map[string]any{
		"host":                           values.Host,
		"local_port_names":               values.LocalPortNames,
		"label_style":                    values.LabelStyle,
		"pinned_certificate_fingerprint": nil,
	}
	if values.PinnedCertificateFingerprint != "" {
		stored["pinned_certificate_fingerprint"] = values.PinnedCertificateFingerprint
	}
	fileBytes, err := json.MarshalIndent(stored, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	temporaryPath := path + ".tmp"
	if err := os.WriteFile(temporaryPath, append(fileBytes, '\n'), 0o600); err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return errors.Join(err, os.Remove(temporaryPath))
	}
	return nil
}

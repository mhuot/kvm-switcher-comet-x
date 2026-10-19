// Package credentials finds and stores the KVM admin password.
//
// On a desktop the password lives in the desktop keyring through the Secret Service API.
// For scripts and headless machines it can instead come from, in this order:
//
//  1. the KVM_SWITCHER_PASSWORD environment variable
//  2. a file named by KVM_SWITCHER_PASSWORD_FILE
//  3. a systemd credential named kvm-switcher-password ($CREDENTIALS_DIRECTORY)
//  4. the desktop keyring
//  5. $XDG_CONFIG_HOME/kvm-switcher/password, which `kvm-switcher setup` writes (mode 0600)
//     only when there is no keyring and the user agrees
package credentials

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mhuot/kvm-switcher-comet-x/linux/internal/settings"
)

// Environment variables and names that can supply the password.
const (
	PasswordEnvironmentVariable     = "KVM_SWITCHER_PASSWORD"
	PasswordFileEnvironmentVariable = "KVM_SWITCHER_PASSWORD_FILE"
	SystemdCredentialName           = "kvm-switcher-password"
	DefaultAccount                  = "glkvm-admin"
	schemaName                      = "net.mhuot.KVMSwitcher"
	itemLabel                       = "KVM Switcher (GL.iNet KVM admin password)"
	passwordFileName                = "password"
)

// Source says where a password came from.
type Source int

// Where a password can come from, in lookup order.
const (
	SourceNone Source = iota
	SourceEnvironment
	SourceEnvironmentFile
	SourceSystemdCredential
	SourceKeyring
	SourcePasswordFile
)

// Description is shown in setup, e.g. "Password: saved in the desktop keyring".
func (source Source) Description() string {
	switch source {
	case SourceEnvironment:
		return "from " + PasswordEnvironmentVariable
	case SourceEnvironmentFile:
		return "from the file in " + PasswordFileEnvironmentVariable
	case SourceSystemdCredential:
		return "from the systemd credential " + SystemdCredentialName
	case SourceKeyring:
		return "saved in the desktop keyring"
	case SourcePasswordFile:
		return "saved in " + PasswordFilePath()
	default:
		return "not set"
	}
}

// Overridden reports whether the password comes from the environment, so editing the
// saved password would have no effect.
func (source Source) Overridden() bool {
	return source == SourceEnvironment || source == SourceEnvironmentFile ||
		source == SourceSystemdCredential
}

// PasswordFilePath is the fallback password file for machines without a keyring.
func PasswordFilePath() string {
	return filepath.Join(settings.ConfigDirectory(), passwordFileName)
}

// ReadPassword returns the password for the default account, or "".
func ReadPassword() string {
	password, _ := Lookup(DefaultAccount)
	return password
}

// Lookup returns the password and where it came from. Failures to reach the keyring count
// as "no password" so the app can explain what to set up.
func Lookup(account string) (string, Source) {
	if account == DefaultAccount {
		if password := os.Getenv(PasswordEnvironmentVariable); password != "" {
			return password, SourceEnvironment
		}
		if passwordPath := os.Getenv(PasswordFileEnvironmentVariable); passwordPath != "" {
			if password, err := readPasswordFile(passwordPath, false); err == nil && password != "" {
				return password, SourceEnvironmentFile
			}
		}
		if credentialsDirectory := os.Getenv("CREDENTIALS_DIRECTORY"); credentialsDirectory != "" {
			credentialPath := filepath.Join(credentialsDirectory, SystemdCredentialName)
			if password, err := readPasswordFile(credentialPath, false); err == nil && password != "" {
				return password, SourceSystemdCredential
			}
		}
	}
	if password, err := readFromKeyring(account); err == nil && password != "" {
		return password, SourceKeyring
	}
	if account == DefaultAccount {
		if password, err := readPasswordFile(PasswordFilePath(), true); err == nil && password != "" {
			return password, SourcePasswordFile
		}
	}
	return "", SourceNone
}

// KeyringAvailable reports whether a Secret Service answers on the session bus.
func KeyringAvailable() bool {
	opened, err := openSecretService()
	if err != nil {
		return false
	}
	opened.close()
	return true
}

func keyringAttributes(account string) map[string]string {
	return map[string]string{"xdg:schema": schemaName, "account": account}
}

func readFromKeyring(account string) (string, error) {
	opened, err := openSecretService()
	if err != nil {
		return "", err
	}
	defer opened.close()
	itemPaths, err := opened.searchItems(keyringAttributes(account))
	if err != nil || len(itemPaths) == 0 {
		return "", err
	}
	return opened.readSecret(itemPaths[0])
}

// SaveToKeyring stores the password in the desktop keyring, or removes it when empty.
// A saved keyring password replaces any fallback password file.
func SaveToKeyring(account, password string) error {
	opened, err := openSecretService()
	if err != nil {
		return err
	}
	defer opened.close()
	if password == "" {
		err = opened.deleteItems(keyringAttributes(account))
	} else {
		err = opened.storeSecret(itemLabel, keyringAttributes(account), password)
	}
	if err != nil {
		if errors.Is(err, ErrKeyringUnavailable) {
			return err
		}
		return fmt.Errorf("%w: %v", ErrKeyringUnavailable, err)
	}
	if account == DefaultAccount && password != "" {
		return RemovePasswordFile()
	}
	return nil
}

// SaveToFile writes the fallback password file with mode 0600.
func SaveToFile(password string) error {
	passwordPath := PasswordFilePath()
	if err := os.MkdirAll(filepath.Dir(passwordPath), 0o700); err != nil {
		return err
	}
	temporaryPath := passwordPath + ".tmp"
	if err := os.WriteFile(temporaryPath, []byte(password+"\n"), 0o600); err != nil {
		return err
	}
	return os.Rename(temporaryPath, passwordPath)
}

// RemovePasswordFile deletes the fallback password file if it exists.
func RemovePasswordFile() error {
	if err := os.Remove(PasswordFilePath()); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// readPasswordFile returns the first line of a password file. With requirePrivate, a file
// that other users can read is refused, as ssh does for private keys.
func readPasswordFile(passwordPath string, requirePrivate bool) (string, error) {
	if requirePrivate {
		fileInfo, err := os.Stat(passwordPath)
		if err != nil {
			return "", err
		}
		if fileInfo.Mode().Perm()&0o077 != 0 {
			fmt.Fprintf(os.Stderr, "kvm-switcher: ignoring %s because other users can read it; run: chmod 600 %s\n",
				passwordPath, passwordPath)
			return "", os.ErrPermission
		}
	}
	fileBytes, err := os.ReadFile(passwordPath)
	if err != nil {
		return "", err
	}
	firstLine, _, _ := strings.Cut(string(fileBytes), "\n")
	return strings.TrimSuffix(firstLine, "\r"), nil
}

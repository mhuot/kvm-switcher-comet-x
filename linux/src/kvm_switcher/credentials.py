"""Stores the KVM admin password in the desktop keyring via libsecret.

libsecret talks to whatever Secret Service the desktop runs: GNOME Keyring, KWallet
(Plasma 5.97+) or KeePassXC. KVM_SWITCHER_PASSWORD overrides it for scripts and headless use.
"""

from __future__ import annotations

import os

PASSWORD_ENVIRONMENT_VARIABLE = "KVM_SWITCHER_PASSWORD"
DEFAULT_ACCOUNT = "glkvm-admin"
_SCHEMA_NAME = "net.mhuot.KVMSwitcher"
_ITEM_LABEL = "KVM Switcher (GL.iNet KVM admin password)"


class KeyringUnavailableError(Exception):
    """Raised when no libsecret / Secret Service is available."""


def _secret_module():
    """Import libsecret lazily so the CLI and tests work without GObject installed."""
    try:
        import gi  # pylint: disable=import-outside-toplevel

        gi.require_version("Secret", "1")
        from gi.repository import Secret  # pylint: disable=import-outside-toplevel
    except (ImportError, ValueError) as import_error:
        raise KeyringUnavailableError(
            "libsecret is not installed (package gir1.2-secret-1 / libsecret)."
        ) from import_error
    return Secret


def _schema(secret_module):
    return secret_module.Schema.new(
        _SCHEMA_NAME,
        secret_module.SchemaFlags.NONE,
        {"account": secret_module.SchemaAttributeType.STRING},
    )


def read_password(account: str = DEFAULT_ACCOUNT) -> str | None:
    """Return the stored password, preferring the environment override."""
    environment_password = os.environ.get(PASSWORD_ENVIRONMENT_VARIABLE)
    if environment_password:
        return environment_password
    try:
        secret_module = _secret_module()
        return secret_module.password_lookup_sync(
            _schema(secret_module), {"account": account}, None
        )
    except KeyringUnavailableError:
        return None
    except Exception:  # pylint: disable=broad-exception-caught
        # GLib.Error when no Secret Service is running or the keyring stays locked.
        return None


def save_password(new_password: str | None, account: str = DEFAULT_ACCOUNT) -> None:
    """Store the password, or remove it when empty. Raises KeyringUnavailableError."""
    secret_module = _secret_module()
    try:
        if new_password:
            secret_module.password_store_sync(
                _schema(secret_module),
                {"account": account},
                secret_module.COLLECTION_DEFAULT,
                _ITEM_LABEL,
                new_password,
                None,
            )
        else:
            secret_module.password_clear_sync(_schema(secret_module), {"account": account}, None)
    except Exception as keyring_error:  # pylint: disable=broad-exception-caught
        raise KeyringUnavailableError(f"Could not use the keyring: {keyring_error}") from None

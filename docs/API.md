# GLKVM HTTP API (as used by KVM Switcher)

The three KVM Switcher apps (macOS, Windows, Linux) share no code. This document and the
[`fixtures/`](../fixtures) folder are the contract they all implement and test against.

The GL.iNet KVM firmware ("GLKVM") is derived from PiKVM's kvmd. These endpoints were found
in the KVM's web UI JavaScript bundle and a browser HAR capture, and checked against an
RM-series KVM with the built-in 4-port switch.

## Transport

- HTTPS on port 443 with a **self-signed certificate**. Apps pin the SHA-256 fingerprint of
  the leaf certificate (DER bytes) on first connection (trust on first use) and refuse a
  different certificate afterwards until the user resets the pin.
- Every reply is a JSON envelope:

  ```json
  { "ok": true,  "result": { … } }
  { "ok": false, "result": { "error": "ForbiddenError", "error_msg": "Forbidden" } }
  ```

## Authentication

```
POST /api/auth/login
Content-Type: application/x-www-form-urlencoded

user=admin&passwd=<url-encoded password>
```

- Success: `{"ok": true, "result": {"token": "<64 hex chars>"}}` ([fixture](../fixtures/login_success.json))
- Wrong password: HTTP **403**, `ForbiddenError` ([fixture](../fixtures/error_forbidden.json))
- The user name is always `admin`.

Send the token on every other request as a header named `token`:

```
token: <token>
```

A missing or expired token returns HTTP **401** `UnauthorizedError`
([fixture](../fixtures/error_unauthorized.json)). Clients log in again once and retry the
request.

2FA: `GET /api/2fa/is_enabled` (no auth) → `{"enabled": bool}`. When enabled, login needs a
second step (`/api/auth/login_with_2fa`) that the apps do not implement.

## Switch state

```
GET /api/switch
```

([fixture](../fixtures/switch_state.json)) Fields the apps use:

| Field | Meaning |
|---|---|
| `result.summary.active_port` | Active port, **zero-based** |
| `result.model.ports[]` | One entry per port: `id` ("1.1" = unit 1 port 1), `channel`, `name` |
| `result.video.links[]` | `true` if that port currently has a video signal |

## Change port

```
POST /api/switch/set_active?port=<zero-based index>
```

Empty body. Returns `{"ok": true, "result": {}}`. Right afterwards `summary.synced` may be
`false` for a moment, so apps wait ~400 ms before reading the state back.

## Port names

The names users type in the GL.iNet web UI are **not** stored in `model.ports[].name` (that
field is usually empty). The web UI keeps them in its own config:

```
GET /api/system/get_config
```

→ `result.config.port_names[]`, one string per port ([fixture](../fixtures/get_config.json)).

### Display name rules

Every app picks a port's display name in this order:

1. `model.ports[i].name`, trimmed, if not empty
2. `config.port_names[i]`, trimmed, if not empty
3. The local name the user set in the app's own settings
4. `Port <i+1>`

A failed `get_config` call must not block switching; the app falls back to rules 1, 3 and 4.
[`fixtures/cases.json`](../fixtures/cases.json) lists the expected results that every test
suite checks.

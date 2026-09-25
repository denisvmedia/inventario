# Logging

The backend logs through the standard library's `log/slog`, configured once in
`cmd/inventario/main.go`:

- `INVENTARIO_LOG_LEVEL` picks the level. Empty or unparseable means `info`,
  and an unparseable value says so on stderr rather than silently falling back.
  `slog.Level.UnmarshalText` takes the four names case-insensitively plus the
  offset form (`debug+2`), so verbosity between the named levels needs no
  extra syntax.
- `INVENTARIO_LOG_FORMAT=json` swaps the text handler for the JSON one.
- `AddSource` is on, so every line carries its file and line.

Everything below is convention rather than configuration.

## Levels

Pick the level from who has to act on the line, not from how bad it sounds.

| Level | Use it when | Who reads it |
| --- | --- | --- |
| `Error` | An operation failed and the caller could not be served. | On call. Every line should be actionable. |
| `Warn` | Something recovered, degraded, or was refused on purpose — a rejected login, a retry, a deprecated path. | Whoever is investigating. |
| `Info` | A state change worth reconstructing later: startup, shutdown, a worker starting, a background job finishing. | Anyone reading a timeline. |
| `Debug` | Detail that only helps while reproducing a specific problem. | Whoever turned it on. |

A failed request that returns 4xx is `Warn`, not `Error`: the caller sent
something the server correctly refused, and nobody needs to be paged. Reserve
`Error` for the cases where the server itself could not do its job.

Do not log an error and also return it up a stack that logs again. The layer
that decides what the user sees is the layer that logs.

## Attribute keys

Keys are `snake_case`, and the same concept keeps one spelling everywhere. The
established vocabulary, by frequency:

```
error  user_id  group_id  tenant_id  file_id  commodity_id  job_id  export_id
path   method   remote_addr  ip  provider  email
```

Put the failure under `error`, never `err`. Identifiers end in `_id`. When a
key needs qualifying, prefix rather than invent: `file_tenant_id`,
`existing_user_id`, `created_by_user_id`.

An event name that groups related lines goes in the **value** of an `event`
key, dotted, as the orphan-file GC does:

```go
slog.Info("orphan gc pass finished", "event", "orphan_gc.tick", "removed", n)
```

## What must never reach a log

- **Passwords, tokens, secrets, API keys, private keys, TOTP seeds, backup
  codes.** Not at any level, not even truncated.
- **A DSN with its credentials.** Use `shared.RedactDSN` (a thin alias for
  `dsnutil.Redact`), which replaces the password with `xxxxxx` and keeps the
  user, host and database so the line is still useful. `logStartupInfo` masks
  the same value a different way, by rewriting the parsed URL's userinfo.
- **Authorization headers and cookies**, including inside a dumped request.
- **Environment variable values.** `logInventarioEnv` deliberately logs the
  names of `INVENTARIO_`-prefixed variables and not their contents.

Opaque row identifiers are not secrets. `token_id`, `session_id` and `jti`
name a row or a claim; they do not grant anything, and they are what makes a
revocation traceable.

Email addresses are personal data rather than credentials. They are logged on
authentication failure paths on purpose — a failed-login line without the
address it was aimed at cannot answer "who was targeted" — but do not add them
anywhere that reason does not apply.

## Request correlation

The API router installs chi's `middleware.RequestID`, so every request carries
an ID. Include it on any line you would want to line up with a specific
request:

```go
slog.Warn("Refresh token reuse detected",
    "user_id", refreshToken.UserID,
    "request_id", middleware.GetReqID(r.Context()))
```

Coverage is currently thin — the ID exists on every request but appears on
only a handful of lines. Adding it to a line you are touching anyway is
always welcome; see #521 for the broader pass.

## Writing a log call

```go
slog.Error("Failed to revoke session", "user_id", user.ID, "session_id", id, "error", err)
```

The message is a short constant phrase. It never interpolates values — those
are attributes, which is what makes the line searchable and groupable. Write
it so that the message alone says what failed and the attributes say to whom.

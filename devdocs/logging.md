# Logging

This is the logging guideline for the Go backend: which level a line gets,
what its fields are called, what never goes into a log, and how a line gets
the id of the request that wrote it. A pull request that changes log lines
cites the section it follows.

## Setup

The backend logs through the default logger of the standard library's
`log/slog`, configured once in `cmd/inventario/main.go`:

- `INVENTARIO_LOG_LEVEL` picks the level. Empty or unparseable means `info`,
  and an unparseable value says so on stderr rather than silently falling back.
  `slog.Level.UnmarshalText` takes the four names case-insensitively plus the
  offset form (`debug+2`), so verbosity between the named levels needs no
  extra syntax.
- `INVENTARIO_LOG_FORMAT=json` swaps the text handler for the JSON one.
- `AddSource` is on, so every line carries its file and line.
- The handler is wrapped by `internal/logctx`, which adds `request_id` to a
  record whose context carries one (see [Request IDs](#request-ids)).

Nothing hands a handler, a service or a worker a logger of its own. Code calls
the package-level functions (`slog.WarnContext(ctx, ...)`) and reaches the
default logger that way. The few components that keep a `*slog.Logger` field,
such as the migrator, the business metrics collector and the restore security
validator, are given `slog.Default()`, so their lines go through the same
handler. What carries request-scoped data from the HTTP edge down to the code
that logs is the `context.Context`, and nothing else.

chi's access log (`middleware.Logger`) is the exception: it writes plain text
through the standard `log` package, not through slog. It shows the request id
in brackets at the start of each line.

## Levels

Pick the level from who has to act on the line, not from how bad it sounds.

| Level | Use it when | Who reads it |
| --- | --- | --- |
| `Error` | An operation failed and the server could not do its job. | On call. Every line should be actionable. |
| `Warn` | Something recovered, degraded, or was refused on purpose: a rejected login, a retry, a fallback. | Whoever is investigating. |
| `Info` | A state change worth reconstructing later: startup, shutdown, a security event, a background job finishing. | Anyone reading a timeline. |
| `Debug` | Detail that only helps while reproducing a specific problem. | Whoever turned it on. |

Examples from the code:

- **Error.** `"Failed to write audit log entry"` (`services/audit_service.go`):
  the request went through, but the record of it did not.
  `"Failed to revoke all sessions on refresh-token reuse"` (`apiserver/auth.go`):
  a security response the server meant to make and could not.
- **Warn.** `"Failed login attempt: user not found"` (`apiserver/auth.go`) and
  `"API token rejected"` (`apiserver/api_token_middleware.go`): the server
  refused something, correctly. `"Email delivery failed; retry scheduled"`
  (`services/email_async_service.go`): it failed and recovered.
  `"Redis auth rate limiter unreachable at startup; rate limiting will fail
  open until Redis becomes available"` (`services/auth_rate_limiter.go`): it
  runs, degraded.
- **Info.** `"Starting API server"` (`cmd/inventario/run/bootstrap`).
  `"Successful user login"`, `"Password changed successfully"` and
  `"Impersonation session started"`: security events an operator wants a
  durable record of. `"group purged"` (`services/group_purge_service.go`): what
  a background job did.
- **Debug.** `"Getting item"` (`registry/memory/registry.go`), which fires on
  every read, and `"RLS Security: User context validated"`
  (`apiserver/apiserver.go`), which fires on every authenticated request.

Three rules follow from the table:

- A request that ends in a 4xx is `Warn` at most, not `Error`: the caller sent
  something the server correctly refused, and nobody needs to be paged.
- A line that fires on every request or every read is `Debug`. The access log
  already records the request.
- Log an error once. Do not log it and also return it up a stack that logs it
  again; the layer that decides what the user sees is the layer that logs.

Lines that do not fit the table yet are moved one package or area at a time,
in pull requests that cite this section.

## Field names

Keys are `snake_case`, and one concept keeps one spelling everywhere. sloglint
checks the case (`key-naming-case: snake` in `.golangci.yml`); the spelling is
up to the author and the reviewer.

| Key | Holds |
| --- | --- |
| `request_id` | The request's correlation id. Added by the handler; never pass it yourself. |
| `error` | The error. Never `err`. A second error on the same line is qualified: `send_error`, `retry_error`. |
| `user_id` | The tenant user acting or acted on. |
| `admin_id` | A back-office user, which is not a tenant user. |
| `tenant_id` | The tenant. `tenant_slug` when the value is the slug. |
| `group_id` | The location group. |
| `<entity>_id` | Any other row: `commodity_id`, `file_id`, `export_id`, `job_id`, `token_id`, `session_id`, `worker_id`. `jti` for a JWT id. |
| `email` | The address a sign-in, registration or recovery line is about. See [what never goes into a log](#what-never-goes-into-a-log). |
| `ip` | The client address as the code resolves it from the forwarding headers (`clientIPForGlobalRateLimit`, `clientIPTruncated`). |
| `remote_addr` | `r.RemoteAddr` verbatim. Behind an ingress that is the proxy, so prefer `ip` on new lines. |
| `method`, `path` | The HTTP method and `r.URL.Path`. Never the query string. |
| `user_agent` | `r.UserAgent()`. |
| `provider` | The OAuth or OIDC provider name. |
| `duration_ms` | A measured elapsed time, as an integer number of milliseconds. |
| `event` | A dotted event name that groups related lines, such as `orphan_gc.tick`. |

When a key needs qualifying, prefix rather than invent: `file_tenant_id`,
`existing_user_id`, `created_by_user_id`, `operator_id` and `target_id` for the
two sides of an impersonation.

Spellings found in the code that do not follow the table, and what replaces
them as each area is touched:

| Found | Lines | Use instead |
| --- | --- | --- |
| `err` | 12 | `error` |
| `user_email` | 8 | `email` |
| `to` for an email recipient (`apiserver/groups.go`) | 1 | `email` on a sign-in, registration or recovery line; otherwise `user_id`, or no recipient at all |
| `user_id` for a back-office user (`apiserver/backoffice_auth*.go`) | 22 | `admin_id` |
| `user_tenant`, `callback_tenant` (tenant ids, `apiserver/oauth.go`, `apiserver/tenant_context.go`) | 7 | `user_tenant_id`, `callback_tenant_id` |
| `tenant` (a slug, `debug/seeddata`) | 2 | `tenant_slug` |
| `item_id` (`registry/memory/registry.go`) | 1 | `commodity_id` |
| `id` | 3 | the entity's own key, such as `verification_id` |

## What never goes into a log

Not at any level, and not truncated:

- **Credentials.** Passwords and password hashes, TOTP seeds, MFA backup codes,
  private keys, the JWT secret, API keys, OAuth client secrets, SMTP and
  storage credentials.
- **Tokens.** JWT access and refresh tokens, personal API tokens
  (`inv_pat_...`), CSRF tokens, the tokens in verification, password-reset,
  magic-link and invite links, signed-URL signatures, OAuth codes and state.
- **A URL with credentials in it.** A database DSN, a Redis URL, an upload
  location. Pass a DSN through `shared.RedactDSN` (a thin alias for
  `dsnutil.Redact`), which replaces the password with `xxxxxx` and keeps the
  user, host and database so the line is still useful.
- **A full URL or a query string.** Signed URLs and emailed links carry their
  token in the query. Log `path`.
- **Authorization headers and cookies**, including inside a dumped request.
- **Whole request or response bodies, whole header maps, whole structs.** A
  `models.User` or a request DTO holds fields nobody decided to log. Pick the
  fields.
- **Personal data beyond identifiers.** Names, postal addresses, phone
  numbers, item names and descriptions, notes, feedback text, file contents.
- **Environment variable values.** `logInventarioEnv` logs the names of
  `INVENTARIO_`-prefixed variables and not their contents.
- **A value the client sent that was refused for its shape.** Log its length or
  the reason, as the request id middleware does for an id it replaces.

Identifiers are not secrets. `user_id`, `token_id`, `session_id` and `jti` name
a row or a claim; they grant nothing, and they are what makes an event
traceable.

Email addresses and IP addresses are personal data rather than credentials.
They are logged on authentication and abuse paths on purpose: a failed-login
line without the address it was aimed at cannot answer "who was targeted", and
a rate-limit line without the client address cannot answer "from where". Do not
add them where that reason does not apply.

The one sanctioned exception is local development: `--log-email-urls` makes the
stub email service log the full links it would have sent, tokens included.
Never set it on a shared environment.

### The DSN log guard

`go/internal/logguard` is a test that walks the module's source and fails when
a slog call, with or without the `Context` suffix, takes a DSN-named value
(`dsn`, `dbDSN`, `connString`, `databaseURL` and similar) directly. A DSN is a
plain string with a predictable name, which is what makes a source-level guard
possible. Wrapping the value in `shared.RedactDSN(dsn)` satisfies it. The rest
of this list has no such shape to match and relies on review.

## Request IDs

Every API request has a correlation id. `RequestIDMiddleware`
(`apiserver/request_id_middleware.go`) is the first middleware on the router:

- An incoming `X-Request-ID` is kept when it is at most 128 bytes of letters,
  digits and `-_.:/+=`. That covers UUIDs, hex and base64 ids and chi's
  `host/prefix-000001` form, so an id set by a proxy or a sibling service
  survives. Anything else, and a missing header, gets a fresh UUID.
- The id is returned in the `X-Request-ID` response header on every response,
  including a CORS preflight, a request refused before it reaches a handler,
  and a failed authentication. CORS exposes the header, so a cross-origin
  frontend can read it.
- The id goes on the request context. `internal/logctx` adds it as
  `request_id` to every record logged with that context; the audit service
  writes it to `audit_logs.request_id`; chi's access log prints it.

A client chooses the id it sends, so the id correlates; it proves nothing. Do
not make a security decision from it.

A log line carries `request_id` only when its call passes a context that came
from the request:

```go
slog.WarnContext(r.Context(), "API token rejected", "reason", reason, "ip", ip)
slog.ErrorContext(ctx, "Failed to write audit log entry", "action", ev.Action, "error", err)
```

`slog.Warn(...)` without a context reaches the same handler with nothing to
read. Wherever a `context.Context` or an `*http.Request` is in scope, use the
`Context` variant; sloglint (`context: scope`) fails the lint otherwise, and
`golangci-lint run --fix` rewrites the call. Code with no context in scope,
such as startup and a worker loop, logs without one, and its lines carry no
`request_id`.

Work that outlives the request keeps the id when its context derives from the
request's, as with `context.WithoutCancel(r.Context())`. A goroutine started
with `context.Background()` loses it.

To follow one request, take the id from the response (or from the browser's
network panel) and search the logs for it; `audit_logs.request_id` holds the
same value for the audit rows that request wrote.

## Writing a log call

```go
slog.ErrorContext(ctx, "Failed to revoke session", "user_id", user.ID, "session_id", id, "error", err)
```

The message is a short constant phrase. It never interpolates values; those
are attributes, which is what makes the line searchable and groupable. Write
it so that the message alone says what failed and the attributes say to whom.

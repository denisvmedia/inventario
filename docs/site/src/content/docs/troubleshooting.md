---
title: Troubleshooting
description: Symptoms and fixes for the things that actually go wrong, on your own server or on someone else's.
---

The problems below are the ones people hit first. Each starts with what you
see, not with what is wrong, because the symptom is the only part you have
when you arrive.

If you run the server yourself, the log is worth reading before anything else:
configuration mistakes almost always name the setting they are about.

```bash
docker compose logs inventario                       # Docker Compose
kubectl -n <namespace> logs deploy/<release>-inventario   # Kubernetes
```

## Sign-in and email

### The verification email never arrived

In order of how often each is the cause:

1. **The server is not sending mail at all.** The default email provider is a
   stub that accepts every message and delivers none. It exists so a
   development instance needs no mail server, and it is the single most common
   reason mail "does not arrive" on a fresh install. Ask whoever runs the
   server whether a real provider is configured.
2. **It went to spam.** Check the spam folder before assuming it was never
   sent. Then check it for the next one too — it is not a one-off.
3. **The sending domain is not authenticated.** Operators: publish SPF, DKIM
   and DMARC for the `email.from` domain. The
   [deliverability runbook](https://github.com/denisvmedia/inventario/blob/master/devdocs/email-deliverability.md)
   walks through it and shows how to read the headers of a test message.

A password-reset or invitation mail that does not arrive has the same three
causes in the same order.

### The verification link says it is invalid

Verification links are single use. If you clicked one, got signed in, and then
opened the same link again — from a mail client that prefetches links, or from
a second tab — the second click reports an invalid token even though the first
one worked. Try signing in normally before requesting a new link.

If you have never signed in successfully, request a fresh verification email
from the sign-in page.

### Sign-in works, then immediately signs me out

Usually the server is behind a proxy that is not forwarding
`X-Forwarded-Proto: https`. Session cookies are marked `Secure`, the browser
declines to store them over what looks like plain HTTP, and the next request
arrives unauthenticated. Operators: enable the header on your ingress
controller or reverse proxy.

## Adding and editing items

### "Fill with AI" is not offered, or says it is not enabled

AI scanning is off unless the operator configures a provider and an API key —
it spends real money per scan, so it is opt-in. On a server you do not run,
this is not something you can turn on.

Operators: set `aivision.provider` to `anthropic` or `openai` and supply the
matching key. `mock` gives a deterministic canned result with no network call,
which is useful for trying the flow out.

### The AI scan says I have been rate-limited

Scans are capped per user and per day to bound the cost. The limit resets;
fill the form by hand in the meantime. Nothing is lost — the manual form has
every field the scan would have filled.

### A file upload fails

There is a size cap on uploads (1 GB by default, and your operator may have
lowered it). A file over the cap is refused rather than truncated.

If the file is comfortably under the cap and the upload still fails, the usual
cause is a proxy in front of the server with its own, smaller limit —
nginx defaults to 1 MB. Operators: raise
`nginx.ingress.kubernetes.io/proxy-body-size` or your proxy's equivalent to
match the application's cap.

## Backups and restore

### The restore did not do what I expected

Restore has two modes and they differ in the part that matters:

- **Merge** adds and updates, leaving everything else alone.
- **Full replace** wipes the destination scope first. Everything: items with
  no area, files attached to a location or an area, standalone files. Then it
  restores from the backup.

Full replace is the right choice when you want the backup to become the truth,
and the wrong one when you meant to add to what is there.

**Dry run is on by default — leave it on the first time.** It produces the same
step-by-step report the real run would, changes nothing, and is the only way
to find out what "full replace" means for your data before it means it. Read
the report, then turn dry run off.

See [Backup & restore](../backup-and-restore/) for the whole flow.

### A restored file will not download

The archive carries the file metadata and the bytes separately. If the bytes
were missing from the original storage when the backup was taken, the restore
recreates a row pointing at nothing. The restore log flags these; it is worth
reading rather than skimming.

## Running your own server

### Pods will not start: ImagePullBackOff

The image tag the chart resolves does not exist. Check the chart's
`appVersion` against the [published releases](https://github.com/denisvmedia/inventario/releases),
or pass the tag explicitly:

```bash
helm upgrade --install inventario ./helm/inventario \
  -n inventario -f values-prod.yaml --set image.tag=vX.Y.Z
```

### The container starts and exits, complaining about the admin password

The initial admin password goes through the same validation as any other: at
least 8 characters, with an upper-case letter, a lower-case letter and a
digit. A password that fails it stops the setup before the first user exists,
which reads as a container that will not boot.

### Everything works, then every restart loses the data

The server fell back to the in-memory database. That backend exists for trying
things out and forgets everything when the process stops.

It happens when the DSN environment variable is not set under the name the
binary reads. Check the startup log — it says which backend it opened — and
confirm the variable name against
[`.env.example`](https://github.com/denisvmedia/inventario/blob/master/.env.example)
rather than assuming.

### A migration failed and the deploy is stuck

Read the migration log first: the error usually names the statement. Two
specific cases:

- **A missing PostgreSQL extension.** `pg_trgm` backs the search indexes and
  is created by `inventario db bootstrap`, not by the migration chain. Skipping
  bootstrap, or running it as a role without the rights, leaves the chain to
  fail on it. The migrator checks up front and names the extension.
- **A migration recorded as dirty.** A previous attempt left the schema
  between two states and the migrator will not retry it on its own. This one
  needs a human to reconcile the schema against the migration file; retrying
  will not clear it.

The [chart README](https://github.com/denisvmedia/inventario/blob/master/helm/inventario/README.md#database-migrations)
covers what runs when, how to preview pending migrations before you deploy,
and why rolling a schema back is rarely the right move.

### Log warns about an in-memory blacklist or rate limiter

You are running more than one replica without Redis, so each replica keeps its
own state: a token revoked on one is still accepted by another until it
expires. Configure Redis, or run a single replica.

## Still stuck

[SUPPORT.md](https://github.com/denisvmedia/inventario/blob/master/SUPPORT.md)
says where to ask and what to include so the answer is useful — briefly: which
install path, which version, and the server log around the failure, with
secrets removed.

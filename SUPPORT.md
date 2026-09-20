# Support

Inventario is in alpha, maintained by a small team. Everything below is
answered as time allows — there is no support contract behind it.

## Before you ask

Two things resolve most first-run problems on their own:

- **The server log.** Configuration mistakes almost always name the setting
  they are about. `docker compose logs inventario`, or
  `kubectl -n <namespace> logs deploy/<release>-inventario`.
- **The docs.** The [user guide](https://denisvmedia.github.io/inventario/)
  covers the application; [QUICKSTART.md](QUICKSTART.md) covers Docker Compose;
  [PRODUCTION.md](PRODUCTION.md) covers Kubernetes, including a troubleshooting
  table of the failures people actually hit.

## Where to go

| What you have | Where it goes |
| --- | --- |
| A bug | [Open an issue](https://github.com/denisvmedia/inventario/issues/new/choose) using the bug template |
| A feature idea | [Open an issue](https://github.com/denisvmedia/inventario/issues/new/choose) using the feature template |
| A question, or a deployment that will not come up | [Open an issue](https://github.com/denisvmedia/inventario/issues/new/choose) — a blank one is fine |
| **A security vulnerability** | **Never an issue.** Follow [SECURITY.md](SECURITY.md) |
| Something you would rather not post publicly | The in-app **Contact support** form, under Settings → Help |

The in-app form is disabled unless the operator has configured a destination
address, so on a deployment you do not run yourself it may not be available.
If you run the deployment, see `SUPPORT_EMAIL` in
[`.env.example`](.env.example) or `email.supportEmail` in the Helm chart.

## What makes a report answerable

Whatever the channel, these four turn a guess into a diagnosis:

1. **Which install path** — Docker Compose, the Helm chart, or built from
   source. They fail differently.
2. **The version** — `inventario version`, or the image tag.
3. **The server log** around the failure. Redact secrets first: a DSN carries a
   password and an access token is a live credential.
4. **What you expected** instead of what happened. It is often the fastest way
   to find out that something works as designed but is documented badly, which
   is still a bug worth fixing.

## What to expect

Issues are triaged as time allows. A reproducible bug with a log attached gets
looked at first, because it can be. There is no response-time commitment.

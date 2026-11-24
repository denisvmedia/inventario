# Changelog

All notable changes to Inventario are recorded here, newest first. The format
follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and versions
follow [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

This file is the source of truth for release notes: the release workflow reads
the section matching the tag and publishes it as the GitHub Release body. See
[CONTRIBUTING.md](CONTRIBUTING.md#changelog) for when to add an entry.

## [Unreleased]

### Security

- Signing in with Google or GitHub now follows the tenant's registration mode.
  Before, a provider account that matched no user created an active account
  even when the mode was `closed` or `approval`. Now `closed` refuses it,
  `approval` creates the account inactive until an administrator activates it,
  and `open` creates an active one as before
  ([#2788](https://github.com/denisvmedia/inventario/pull/2788)).

### Fixed

- The sign-in page now shows a message for every refusal the Google and GitHub
  sign-in can end with: a disabled account, an email the provider has not
  verified, a profile Inventario cannot use, an account on another site, closed
  registration, and an account waiting for approval. Before, it showed nothing
  for them ([#2788](https://github.com/denisvmedia/inventario/pull/2788)).
- A deactivated user whose account is linked to Google or GitHub now lands on
  the sign-in page with that message instead of a bare "User account is not
  available" page ([#2788](https://github.com/denisvmedia/inventario/pull/2788)).
- The version badge in Settings → Help and the version a feedback report
  carries now show the release you run. Before, every release showed 0.1.0.
  A build without a release number, such as the `edge` image, shows no badge
  ([#2790](https://github.com/denisvmedia/inventario/pull/2790)).

## [0.2.2] - 2026-11-23

### Added

- `setupJob.bootstrap.superuserSecret` in the Helm chart: a Secret of your own
  with the superuser DSN for the database bootstrap, which only the setup Job
  reads ([#2783](https://github.com/denisvmedia/inventario/pull/2783)).
- A `SETUP_ADMIN_EMAIL` key in the chart's `secrets.existingSecret`, which wins
  over `setupJob.initData.adminEmail`, so the first admin's real address need
  not be in the values ([#2785](https://github.com/denisvmedia/inventario/pull/2785)).

### Security

- The chart's runtime Secret no longer carries `SETUP_SUPERUSER_DSN` when
  `setupJob.bootstrap.superuserDsn` is set. Every app pod loads that Secret, so
  the superuser DSN reached them all, although only the setup Job uses it. A
  `SETUP_SUPERUSER_DSN` key in `secrets.existingSecret` still reaches the app
  pods; move it to `superuserSecret`
  ([#2783](https://github.com/denisvmedia/inventario/pull/2783)).

## [0.2.1] - 2026-11-23

### Changed

- An item that leaves out `converted_original_price` is no longer rejected with
  "cannot be blank". The value counts as 0, and the price rule still decides
  which prices an item needs: 0 for an item bought in the group currency, and a
  converted or current price for one bought in another currency
  ([#2781](https://github.com/denisvmedia/inventario/pull/2781)).

### Security

- Prices are read with `shopspring/decimal` 1.6.0. A price with a huge exponent,
  like `1e1000000000`, is now rejected. Before, such a value in a request could
  hang the server or use gigabytes of memory
  ([#2781](https://github.com/denisvmedia/inventario/pull/2781)).

## [0.2.0] - 2026-11-23

### Added

- `extraVolumes` and `extraVolumeMounts` in the Helm chart, for example to
  mount the database server's CA for `sslmode=verify-full`. The volumes go into
  every pod of the chart except the demo ones, and the mounts into every
  container that runs Inventario or reaches the database, the `migrate` init
  container and the backup included
  ([#2774](https://github.com/denisvmedia/inventario/pull/2774)).
- Privacy Policy and Terms of Service pages, linked from the registration
  consent checkbox ([#2148](https://github.com/denisvmedia/inventario/issues/2148)).
- `setupJob.migrateInInitContainer` makes the in-pod migration ordering
  available without ArgoCD, so a plain Helm or Helmfile install no longer has
  to migrate before the rollout starts
  ([#2086](https://github.com/denisvmedia/inventario/issues/2086)).
- End-to-end coverage of the onboarding loop through the browser: register,
  verify by email, sign in, reset the password, sign in again
  ([#2114](https://github.com/denisvmedia/inventario/issues/2114)).
- k6 load profile (`load/k6/api-load.js`) and a weekly OWASP ZAP baseline scan
  of the public surface ([#848](https://github.com/denisvmedia/inventario/issues/848)).
- Opt-in nightly `pg_dump` CronJob in the Helm chart (`backup.enabled`), with
  retention, a read-back check on each dump, and alerts on backup age
  ([#845](https://github.com/denisvmedia/inventario/issues/845)).
- Each release publishes the Helm chart to
  `oci://ghcr.io/denisvmedia/charts/inventario` and attaches the package to
  the GitHub Release. The chart version is the release version without the
  "v": this release ships chart 0.2.0. Before, the chart had its own numbers,
  which reached 0.6.0 but were never published, so the number goes down once;
  `helm upgrade` from 0.6.0 to 0.2.0 works as usual
  ([#2775](https://github.com/denisvmedia/inventario/pull/2775),
  [#2777](https://github.com/denisvmedia/inventario/pull/2777)).

### Fixed

- A fresh `helm install` with an external database no longer hangs. The setup
  Job was a `pre-install` hook that needs the chart's ServiceAccount, ConfigMap
  and Secret, which Helm creates only after its hooks, so the install hung
  until it timed out. It is now an ordinary release object on install, and
  stays a `pre-upgrade` hook on upgrade
  ([#2773](https://github.com/denisvmedia/inventario/pull/2773)).
- The chart's pods pass Pod Security `restricted`: `podSecurityContext` now
  sets the `RuntimeDefault` seccomp profile, without which a `restricted`
  namespace refused every pod but the demo ones
  ([#2774](https://github.com/denisvmedia/inventario/pull/2774)).
- `helm install --wait` no longer deadlocks with `demo.postgresql.enabled`: the
  setup Job is applied alongside the database rather than as a post-install
  hook that Helm cannot reach until the app pod is ready
  ([#2540](https://github.com/denisvmedia/inventario/issues/2540)).
- Every response now carries a Content-Security-Policy
  ([#2524](https://github.com/denisvmedia/inventario/issues/2524)).
- Every response now carries `X-Content-Type-Options`, `X-Frame-Options` and
  `Referrer-Policy`, and `Strict-Transport-Security` over TLS. A stock install
  serves the SPA from the binary itself, with no proxy to add them
  ([#2523](https://github.com/denisvmedia/inventario/issues/2523)).
- The concurrent-upload cap is enforced again when two requests race: the
  middleware compared error messages instead of unwrapping the sentinel, so the
  losing request went through above the cap
  ([#2531](https://github.com/denisvmedia/inventario/issues/2531)).
- Bootstrap now grants the app, worker and admin roles access to tables created
  by a migration login named anything other than `inventario_migrator`. With a
  custom name, every migrated table came out unreadable to the application
  ([#2520](https://github.com/denisvmedia/inventario/issues/2520)).
- `/metrics` no longer listens on the public port, where the default ingress
  rule exposed installation-wide gauges to the Internet
  ([#2244](https://github.com/denisvmedia/inventario/issues/2244)).
- The chart no longer mounts a Kubernetes API token into application pods that
  never use one ([#2245](https://github.com/denisvmedia/inventario/issues/2245)).
- Client-facing error bodies no longer carry stack traces
  ([#2178](https://github.com/denisvmedia/inventario/issues/2178)).
- A default `helm install` resolves an image again: the chart's `appVersion`
  named a tag that was never published
  ([#2035](https://github.com/denisvmedia/inventario/issues/2035),
  [#2036](https://github.com/denisvmedia/inventario/issues/2036)).
- The email-verification page no longer reports a fresh token as expired
  ([#2096](https://github.com/denisvmedia/inventario/issues/2096)).
- List pages show a failure as a failure instead of an empty state
  ([#2098](https://github.com/denisvmedia/inventario/issues/2098),
  [#2127](https://github.com/denisvmedia/inventario/issues/2127)).
- Filtered lists and dashboard counts no longer present one capped page as the
  whole set ([#2128](https://github.com/denisvmedia/inventario/issues/2128)).
- Search reported the page size as the total, hiding every result past the
  first page ([#2129](https://github.com/denisvmedia/inventario/issues/2129)).
- A magic link is no longer consumed by a request that is then refused, and a
  remount no longer spends the token twice
  ([#2131](https://github.com/denisvmedia/inventario/issues/2131),
  [#2132](https://github.com/denisvmedia/inventario/issues/2132)).
- `inventario version` prints to stdout
  ([#2455](https://github.com/denisvmedia/inventario/pull/2455)).

### Changed

- The admin, back-office and impersonation end-to-end suite runs again. It had
  been skipped entirely since the back-office auth plane landed
  ([#2100](https://github.com/denisvmedia/inventario/issues/2100)).

### Security

- The image is built with Go 1.27.4, and `golang.org/x/net`,
  `go-jose`, `klauspost/compress` and `andybalholm/brotli` are updated. The
  shipped binary had a man-in-the-middle flaw in `crypto/x509` and
  denial-of-service flaws in the HTML parser and in JOSE token parsing.

## [0.1.0] - 2026-09-19

First tagged release: an alpha for a small private cohort, not a public
launch. The full announcement, including what the release does, what it needs,
and the known gaps it ships with, is in
[the v0.1.0 release notes](.github/release-notes/v0.1.0.md).

[Unreleased]: https://github.com/denisvmedia/inventario/compare/v0.2.2...HEAD
[0.2.2]: https://github.com/denisvmedia/inventario/compare/v0.2.1...v0.2.2
[0.2.1]: https://github.com/denisvmedia/inventario/compare/v0.2.0...v0.2.1
[0.2.0]: https://github.com/denisvmedia/inventario/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/denisvmedia/inventario/releases/tag/v0.1.0

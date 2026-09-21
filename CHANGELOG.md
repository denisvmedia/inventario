# Changelog

All notable changes to Inventario are recorded here, newest first. The format
follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and versions
follow [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

This file is the source of truth for release notes: the release workflow reads
the section matching the tag and publishes it as the GitHub Release body. See
[CONTRIBUTING.md](CONTRIBUTING.md#changelog) for when to add an entry.

## [Unreleased]

### Added

- Privacy Policy and Terms of Service pages, linked from the registration
  consent checkbox ([#2148](https://github.com/denisvmedia/inventario/issues/2148)).
- End-to-end coverage of the onboarding loop through the browser: register,
  verify by email, sign in, reset the password, sign in again
  ([#2114](https://github.com/denisvmedia/inventario/issues/2114)).
- k6 load profile (`load/k6/api-load.js`) and a weekly OWASP ZAP baseline scan
  of the public surface ([#848](https://github.com/denisvmedia/inventario/issues/848)).
- Opt-in nightly `pg_dump` CronJob in the Helm chart (`backup.enabled`), with
  retention, a read-back check on each dump, and alerts on backup age
  ([#845](https://github.com/denisvmedia/inventario/issues/845)).

### Fixed

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

## [0.1.0] - 2026-09-19

First tagged release: an alpha for a small private cohort, not a public
launch. The full announcement, including what the release does, what it needs,
and the known gaps it ships with, is in
[the v0.1.0 release notes](.github/release-notes/v0.1.0.md).

[Unreleased]: https://github.com/denisvmedia/inventario/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/denisvmedia/inventario/releases/tag/v0.1.0

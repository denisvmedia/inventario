# Changelog

All notable changes to Inventario are recorded here, newest first. The format
follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and versions
follow [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

This file is the source of truth for release notes: the release workflow reads
the section matching the tag and publishes it as the GitHub Release body. See
[CONTRIBUTING.md](CONTRIBUTING.md#changelog) for when to add an entry.

## [Unreleased]

### Added

- A search field in the top bar of every page, an icon button on phones,
  opens the search window that `Ctrl+K` / `⌘K` opened before. It lists the
  group's matching items, locations, areas, files and tags by kind. An area or
  a tag offers "Show items" (a file tag "Show files"), the items group links to
  the items list with the same search, and an empty search lists the results
  opened recently. The search page lists tags too, has the same "Show items"
  links, and fills its recent list. `GET /search` now accepts `type=locations`,
  `type=areas` and `type=tags`
  ([#2801](https://github.com/denisvmedia/inventario/issues/2801)).

- Item reminders and an Upcoming page. The Reminders card on an item's
  Details tab takes a title, a date, up to five "remind me N days before"
  values and an optional repeat (daily, weekly, monthly or yearly, every N
  units). A monthly reminder on the 31st falls on the last day of shorter
  months, and a yearly one on 29 February on 28 February in other years. On a
  repeating reminder each lead time must be shorter than the repeat interval,
  and the date's year must be between 1900 and 2200. The emails go to whoever
  created the reminder, at most once per date and lead time, and can be
  turned off with the new "Item reminders" toggle in Settings →
  Notifications. Deleting an account deletes the reminders its owner
  created. The Upcoming page in the sidebar lists item reminders, warranty
  expiries, loans due back and maintenance due across the group by date, for
  the next 30 days, 90 days or 12 months. The worker runs every
  hour (`--item-reminder-interval`) and can be paused as `item-reminder`.
  Reminders on custom date fields are not part of this
  ([#2805](https://github.com/denisvmedia/inventario/issues/2805)).
- Saved views. **Views** on the items list keeps the current filters, search,
  sort and layout under a name, in the sidebar with an item count, on the
  Dashboard as a card with the count and the first items, or both. An open
  view tells when the list no longer matches it and offers **Save changes**
  and **Revert to saved**; a view naming a deleted area or tag still opens and
  says what no longer exists. Views are personal unless shared with the group
  (User role or higher), and sharing a view grants no access to items. Each
  member chooses which views, their own or shared, show in their sidebar and on
  their Dashboard. Warranty
  filters stay relative to the day the view is opened. The Dashboard suggests a
  few views to start with, and **Saved views** (`/g/:slug/views`) lists, edits,
  reorders and deletes them. New endpoints: `/g/{slug}/saved-views` and
  `/g/{slug}/saved-views/counts`
  ([#2793](https://github.com/denisvmedia/inventario/issues/2793)).
- Trash. Deleting an item, alone or in bulk, moves it to the group's Trash
  with the files attached only to it; deleting a file moves it there too, and
  so does "Delete everything" on a location or area for the items inside. The
  new Trash page in the sidebar restores entries under the same id and asset
  ID, deletes them for good, or empties the trash. Viewers see it, users
  restore, admins and owners delete. A restored item gets back its history,
  loans, services, schedules and the files that went with it, but not a file
  deleted on its own earlier; an item whose area was deleted in the meantime
  comes back without a location. While in Trash an item is left out of lists,
  search, reports, totals and reminders. Trashed files still count toward
  storage, shown as a separate Trash share on the storage card. Entries are
  deleted for good after `--trash-retention` (default 30 days) by the new
  `trash-purge` worker; `--trash-purge-interval` sets its cadence. Item delete
  toasts have an Undo button. Backups leave Trash out, a full-replace restore
  empties it, and a merge restore skips what is in it and puts a backup file
  that only trashed items use in Trash with them
  ([#2799](https://github.com/denisvmedia/inventario/issues/2799)).
- "New location" and "New area" at the end of the Location and Area pickers
  on the item's edit form, its All fields dialog and the location quick edit.
  They open the Locations page's create dialog over the item and select what
  it created, so a place that does not exist yet no longer means leaving the
  item. Only owners and administrators see them, as only they can create
  locations and areas
  ([#2796](https://github.com/denisvmedia/inventario/issues/2796)).

- Item search covers serial numbers, extra serial numbers, part numbers, tags
  (by slug and by name), notes, warranty notes and the status note, besides the
  name, short name and asset ID. Every word of a search must match, in any
  field, and double quotes keep a phrase together. A result that matched on
  something other than its name says where ("Serial number: SN-1234"). The
  items list, the area page and the search page use the same search, done in
  SQL on an indexed column
  ([#2800](https://github.com/denisvmedia/inventario/issues/2800)).
- Filter items by tag. The **Tags** menu on the items list has a search box,
  each tag with its color and item count, **All tags** / **Any tag**, and
  **No tags**; clicking a selected tag again excludes it. Tags on item cards
  and on the item page are colored pills that open the list filtered by the
  tag, and so are tag names on the Tags page; the area and the type on the
  item page link the same way. The file browser gets a **More tags** picker
  for any file tag. The new `GET /commodities/tag-counts` endpoint returns the
  per-tag counts for a query
  ([#2792](https://github.com/denisvmedia/inventario/issues/2792)).

- Sign-in with an OpenID Connect provider you run yourself, such as Authelia,
  Keycloak or Authentik. Set `OAUTH_OIDC_ISSUER_URL`, `OAUTH_OIDC_CLIENT_ID`,
  `OAUTH_OIDC_CLIENT_SECRET` and, for the button, `OAUTH_OIDC_DISPLAY_NAME`;
  the endpoints come from the issuer's discovery document.
  `OAUTH_OIDC_BACKCHANNEL_URL` covers a server that cannot reach the issuer's
  public address. The Helm chart gets `oauth.*` values and `secrets.oauth*`
  keys for all three providers. Setup is described on the new "Sign-in
  providers" page of the user guide
  ([#2789](https://github.com/denisvmedia/inventario/pull/2789)).
- A "Restart product tour" row in Preferences → Help & support, next to the
  entry in the user menu. It is greyed out, with the reason, while the tour
  cannot run: without a group, or in a window too narrow to show the sidebar
  ([#2816](https://github.com/denisvmedia/inventario/pull/2816)).
- Asset IDs and QR labels. Every item gets a running number per group, shown
  as `A-0042` on the item page and found by the items search and the search
  page. Existing items get their numbers when the server first starts after
  the upgrade, in the order they were added, and a restore keeps the numbers. A new label page
  prints the asset ID, the name and a QR code that opens the item, or the area
  with its contents, after sign-in. It prints one label, an A4 sheet (Avery
  L7160) or a Letter sheet (Avery 5160) at real size, and is reached from the
  item page, the items bulk bar and the area page
  ([#2807](https://github.com/denisvmedia/inventario/issues/2807)).
- Profile photos. Upload a still JPG, PNG or WebP picture of up to 2 MB from
  the Edit profile page, or from the camera on your Profile page. It is
  cropped to a square, scaled to at most 512 pixels, turned upright when a
  phone stored it sideways, and saved without its metadata, so a photo's GPS
  position is not kept. The photo replaces your initials in the sidebar, on
  the profile pages, in Preferences and in the members list of every group you
  are in. Only you and the members of your groups can load it, it does not
  count toward a group's storage, and deleting your account deletes it
  ([#1382](https://github.com/denisvmedia/inventario/issues/1382)).
- A file edit page with the preview beside the details form. Opened from the
  Files page, it keeps the list you came from: the header shows the file's
  place in it with previous and next arrows, "Save and next" saves and opens
  the next file, and Back returns to the same filtered page with the file's
  panel open. Unsaved changes ask to be saved or discarded before you leave
  ([#2814](https://github.com/denisvmedia/inventario/issues/2814)).
- Personal API tokens for scripts and other apps, under Preferences → Privacy
  & security → API tokens. A token has a name, read-only or read-write access,
  an optional expiry and optionally a single group, and is shown once. Scripts
  send it as `Authorization: Bearer inv_pat_…` with no CSRF token. A token
  carries the owner's current role in each group, works on group data and can
  read groups and their members, and is refused on account, security, session,
  token, invitation, group-management and admin endpoints. Changing or
  resetting the password, turning off two-factor authentication, and an
  administrator blocking or signing out the account revoke all of its tokens.
  Creation, revocation and use are written to the audit log. The new "API
  tokens for scripts" page of the user guide lists the endpoints worth
  scripting against
  ([#2808](https://github.com/denisvmedia/inventario/issues/2808)).
- One file can be attached to several items, locations and areas, so a receipt
  for three items is uploaded once. "Attach an existing file" on an item's
  Files tab (and "Attach existing" on a location or area) attaches a file the
  group already holds, and the file panel lists every place under "Linked to",
  each with its own Remove attachment. Removing the last attachment leaves a
  standalone file; deleting a file stays a separate action whose confirmation
  names every place it disappears from. Deleting an item keeps the files
  another item still uses. Each attached file counts toward storage once.
  Uploads compute a SHA-256, and when the group already holds the same content
  the upload dialog offers to attach the existing file instead. Existing
  attachments move to the new shape when the server first starts after the
  upgrade, and existing files get their checksum in the background
  ([#2806](https://github.com/denisvmedia/inventario/issues/2806)).
- Editing an item in place. Edit on the item page and in its side panel opens
  a form with the item's main fields where the details were, with a tag
  palette that adds or removes a tag in one click; "All fields" opens the
  step-by-step dialog, which in edit mode lets every step be opened at once
  and saves from any of them. The Details tab edits the location, the tags
  or the notes on their own. Opened from the Items list, an item shows its
  place in that list with previous and next arrows, Back returns to the list
  as it was left, and "Save and next" on the edit form saves and opens the
  next item's form
  ([#2795](https://github.com/denisvmedia/inventario/issues/2795)).
- A finished item can keep its purchase date and prices unknown, for a gift
  or an inherited object. A blank price is unknown and `0` is a known zero;
  the original price, the converted price and the current value can each be
  unknown on their own. Unknown shows as "—", lists sorted by purchase date
  or price put such items last, and the dashboard, area and insurance report
  totals leave them out and say how many they exclude. The insurance report's
  estimated value now follows the dashboard: an item's current value, else
  its purchase price
  ([#2813](https://github.com/denisvmedia/inventario/issues/2813)).

### Changed

- Item API: `original_price`, `converted_original_price` and `current_price`
  are `null` when unknown, and a request that omits one or sends `null`
  stores it as unknown rather than as zero. A `current_price` of `0` is a
  known zero and is the item's value in every total, ahead of the purchase
  price; send `null` or leave it out when the value is not known. A
  `converted_original_price` sent for an item bought in the group currency is
  stored as `null`. `GET /commodities/values` adds `valued_count` and
  `unvalued_count`, and each location and area total an `unvalued_count`.
  After the upgrade the server rewrites, in the background, the zero current
  and converted prices that older versions stored for "not entered" as
  unknown; a zero original price is kept, since it may be a free item.
  Backups are written in INB format 3.1, which keeps unknown and zero apart;
  older backups still restore, but a 3.1 backup restores only into this
  version or a later one. v0.2.x cannot read an unknown price: upgrade
  without old and new pods overlapping (see PRODUCTION.md, Appendix C), and
  do not roll back to v0.2.x once an item has an unknown price
  ([#2813](https://github.com/denisvmedia/inventario/issues/2813)).

- File API: a file carries a `links` list instead of `linked_entity_type`,
  `linked_entity_id` and `linked_entity_meta`, and `PUT /files/{id}` no
  longer accepts those fields. Attach and detach with
  `POST /files/{id}/links` and `DELETE /files/{id}/links/{linkID}`;
  `GET /files/{id}/duplicates` lists files with the same content. The
  `linked_entity_type` / `linked_entity_id` list filter is unchanged. Backups
  are written in INB format 3.0, which stores a shared file once; 2.x backups
  still restore
  ([#2806](https://github.com/denisvmedia/inventario/issues/2806)).
- `DELETE /commodities/{id}`, `POST /commodities/bulk-delete`,
  `DELETE /files/{id}` and `POST /files/bulk-delete` move the entries to Trash
  instead of removing them. Lists, search and the other live reads leave
  trashed entries out; `GET /trash/commodities` and `GET /trash/files` list
  them, with restore and purge endpoints under `/trash`. `GET /commodities`,
  `GET /commodities/tag-counts` and `GET /search?type=commodities` take
  `trashed=true` to apply the same filters to the items in Trash. The
  maintenance schedules and reminders of an item in Trash answer 404 under
  `/maintenance/{id}` and `/reminders/{id}` until the item is restored. The
  storage usage breakdown gains a `trash` bucket
  ([#2799](https://github.com/denisvmedia/inventario/issues/2799)).

### Changed

- `GET /commodities` and `GET /search?type=commodities` read one shared set of
  filter parameters, including the new `tags`, `tag_mode`, `tags_exclude` and
  `no_tags`. An unknown parameter, an unreadable value or a contradictory
  combination is answered with 400 and the code `query.invalid_parameter`
  instead of being ignored, and the `operator` parameter of `/search` is gone
  (`tag_mode` replaces it). The search page's link to all matching items now
  includes inactive items, as the search does
  ([#2800](https://github.com/denisvmedia/inventario/issues/2800)).

### Fixed

- Settings saved on the server show up in the app again. The settings
  endpoint sent its keys capitalized (`NotificationsWarrantyExpiry`) while the
  app reads camelCase (`notificationsWarrantyExpiry`), so after a reload the
  notification toggles showed their defaults, and the saved language and
  default items view were not applied
  ([#2805](https://github.com/denisvmedia/inventario/issues/2805)).
- The item delete dialog and the area delete dialog showed raw keys such as
  "delete.title" instead of their text
  ([#2799](https://github.com/denisvmedia/inventario/issues/2799)).
- The search page renders item results. It expected each result wrapped in
  JSON:API `attributes`, which the search endpoint does not send, and failed to
  render as soon as a search matched an item
  ([#2807](https://github.com/denisvmedia/inventario/issues/2807)).
- Opening or closing a file's panel on the Files page keeps the search, tag
  filter, page and layout of the list
  ([#2814](https://github.com/denisvmedia/inventario/issues/2814)).
- Files uploaded in the same instant keep one fixed order in the Files list,
  so paging through the list no longer shows a file twice or skips one
  ([#2814](https://github.com/denisvmedia/inventario/issues/2814)).
- In Safari, **Save** in the unsaved-changes prompt saves when the prompt
  opens again right after **Keep editing**. Before, the click closed the
  prompt instead ([#2830](https://github.com/denisvmedia/inventario/pull/2830)).
- An item opened from a searched Items list, or from an area's items, stays
  open. Before, an item clicked within a moment of the list loading closed
  again and the list came back
  ([#2825](https://github.com/denisvmedia/inventario/pull/2825)).
- The Files page, opened on a page past the last one (for example after a save
  took the last file off it), moves to the last page with files instead of an
  empty page without pagination
  ([#2825](https://github.com/denisvmedia/inventario/pull/2825)).
- Tags written in Cyrillic, Czech, Chinese or any other non-Latin script are
  kept on save. Before, their letters were stripped from the slug, so a tag
  like "кухня" disappeared and "USB кабель" and "USB хаб" became one tag.
  Slugs now keep letters and digits of every script, a tag created by typing
  it on an item or file keeps the typed text as its label, and item cards,
  item pages and file views show tag labels instead of slugs. Existing tags
  keep their slugs
  ([#2791](https://github.com/denisvmedia/inventario/issues/2791)).
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
- Creating your first group through **Create new group** in the group switcher
  now opens the new group. Before, the page stayed on the filled-in form, so
  it looked as if nothing had happened and a second click created another
  group ([#2815](https://github.com/denisvmedia/inventario/issues/2815)).
- The onboarding tour opened by itself for a user without a group, such as the
  first administrator of a new install. None of the sidebar entries it points
  at exist then, so every step was an empty centered card, and finishing it
  hid the tour for good. The tour now starts the first time a group is active,
  and it skips a step whose element is not on the page
  ([#2816](https://github.com/denisvmedia/inventario/pull/2816)).
- The onboarding tour started over from its welcome card, or closed under
  browser automation, when the window was narrower than the mobile breakpoint
  for a moment only: while Chromium takes a full-page screenshot, or when a
  window is dragged across that width and back. The tour now hides while the
  window is narrow and comes back on the step it was on. It closes only when
  the window stays narrow for a second
  ([#2829](https://github.com/denisvmedia/inventario/pull/2829)).

### Security

- Signing in with Google or GitHub now follows the tenant's registration mode.
  Before, a provider account that matched no user created an active account
  even when the mode was `closed` or `approval`. Now `closed` refuses it,
  `approval` creates the account inactive until an administrator activates it,
  and `open` creates an active one as before
  ([#2788](https://github.com/denisvmedia/inventario/pull/2788)).

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

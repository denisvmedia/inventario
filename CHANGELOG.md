# Changelog

All notable changes to Inventario are recorded here, newest first. The format
follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and versions
follow [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

This file is the source of truth for release notes: the release workflow reads
the section matching the tag and publishes it as the GitHub Release body.
Changes for the people who run the server are under **Operators**, the last
heading of each release. See [CONTRIBUTING.md](CONTRIBUTING.md#changelog) for
when to add an entry and where it goes.

## [Unreleased]

### Added

- Personal data export (GDPR Art. 20). `POST /users/me/data-exports` asks for
  a copy of the caller's own data: their profile, settings and group
  memberships, and in each group the items, files with their bytes, loans,
  maintenance schedules, service records, reminders, supply links, open
  drafts, saved views and history entries they created. A history entry
  holds only the fields it changed, with the values it set. An item's
  location, area, seller, type and tags are given by name; other members
  appear only as ids, never by name or email. A file whose bytes are no
  longer in storage is listed as missing. The archive is a plain `tar.gz` of
  JSON documents and files whose manifest has kind `personal_data_export`; it
  is not a group backup, and the restore upload refuses one with 422
  `restore.personal_data_export`. The export worker builds it in the
  background and emails the user when it is ready; a build cut short by a
  restart is queued again and built once more. One request is allowed per
  day (429 `data_export.rate_limited` with `Retry-After` and `retry_at`), the
  archive is deleted after 7 days or with the account, only its owner can
  list and download it, an impersonation session can neither request nor
  download one, and each request and download is recorded in the audit log.
  `GET /users/me/data-exports` lists the exports and when the next request is
  allowed; `POST /users/me/data-exports/{id}/download-url` returns a signed
  download link that works only in the browser session that asked for it
  (403 `data_export.session_required` without the session cookie)
  ([#2177](https://github.com/denisvmedia/inventario/issues/2177)).
- The AI scan suggests a type (invoice, receipt, manual, warranty, product
  photo or other) and a short readable title for each file it reads, in the
  language of the document. The scan response gains `files`, one entry per
  file with its `index`, `document_type` and `title`, each with a
  confidence; the scan-level `document_type` stays and is derived from the
  files when the model gives none. **Add items from files** sets the
  suggestions on the files once they are scanned: the **Invoice** tag for an
  invoice or a receipt, **Manual**, **Warranty** or **Photo** for the
  others, and the title. Only a file that still has the title and the tags
  of its upload changes, so a file you renamed or tagged keeps what you
  gave it and a rescan does not change it again (#1626).
- Two item statuses for every group: **Donated** and **Stolen**, next to
  Sold, Lost, Disposed and Written Off. They are set like the other end
  statuses, from the item page or the bulk **Set status** menu, with a date
  and an optional note and no sale price. The status filter, saved views,
  search and the API's `status` parameter take `donated` and `stolen`. Like
  the other end statuses, such an item leaves the default items list, the
  inbox counts, the upcoming warranties, the location insurance report and
  every total; its single-item insurance report still opens from its page.
  Backups carry the new statuses, and older backups restore as before.
- Hidden sections. A group admin or owner can hide Warranties, Maintenance
  and repairs (with In service), Loans, and Upcoming for every member, from
  the new **Sections** part of the group settings. A hidden section leaves
  the sidebar, the command palette, the Dashboard, the item form and the
  item's actions and tabs; a link to its page shows a notice with a way to
  the settings. Items keep the data they hold and still show it, and no
  write is refused. While a section is hidden, its reminder emails stop and
  the weekly digest leaves its lines out; showing it again resumes them.
  Existing groups hide nothing. The group resource gains `hidden_sections`
  (`warranties`, `maintenance`, `loans`, `upcoming`), which every member
  reads; `PUT /groups/{groupID}/hidden-sections` replaces the list and is
  admin-only, and creating or updating a group takes `hidden_sections` too.
  An unknown section is refused with 422 and the validation code
  `validation_invalid_group_section` on `hidden_sections`; duplicates are
  dropped. Like the group name and icon, the setting is not part of a backup
  ([#1607](https://github.com/denisvmedia/inventario/issues/1607)).
- Custom fields. A group keeps up to 200 fields of its own, each with a name
  (unique in the group, ignoring case) and a type that never changes: text,
  long text, number, money, date, yes/no, select, URL or a link to another
  item of the group. The new **Custom fields** page in the sidebar adds,
  renames and deletes them; the item forms fill them in (an **Add field**
  menu offers the group's fields), the item page and the single-item
  insurance report list an item's values, and a link shows the linked item
  by name, marked when it is in the Trash. Items store a value per field;
  zero, `false` and empty text are values, distinct from no value. A number
  is returned as a decimal string, so no digit is lost; a write takes a
  string or a JSON number. Renaming a field or a select option keeps its
  values, and deleting a field that items have values for asks to confirm
  with the number of items that lose a value, and asks again when that
  number changed meanwhile (the API needs `delete_values=true`, 409
  `custom_field.in_use` otherwise, with the item counts, and refuses a
  delete whose `expected_values` is not the current count). A field can be
  offered on some item types, each with an optional default: picking the
  type in an item form shows the field and fills in the default where the
  item has no value. On **All Items**, the **Fields** menu filters by custom
  field values (typed comparisons, ranges with "at least" and "at most"),
  **Sort by** lists the custom fields, the list layout shows up to four of
  them as **Columns**, and search finds their text and names the field it
  matched. A saved view or link that names a deleted field marks it
  "Deleted field".
  The API gains `/g/{slug}/custom-fields`; item writes take `custom_fields`
  and refuse a value its field cannot hold with 422
  `custom_field.invalid_value`, the reasons under
  `data.attributes.custom_fields.<field id>`; the items list filters
  with `cf=<field id>:<op>[:<value>]` and sorts with `sort=cf.<field id>`,
  names linked items in `meta.linked_items`, saved views keep both, and
  search reads text fields and option labels. AI
  scan fills the group's fields except money and links, after the member
  reviews them. A draft of a new item keeps the values typed in the Add item
  dialog, and promoting it checks and writes them with the new item. Backups
  carry the fields, templates and values, also those of drafts (format
  3.6), and a restore matches a field by the field it was backed up from,
  then by name and type, and an option by the option, then by label,
  creating the missing ones
  ([#2803](https://github.com/denisvmedia/inventario/issues/2803)).
- Documents from other systems: `POST /g/{groupSlug}/intake` turns a document
  that another system such as Paperless-ngx sends into a draft in the inbox,
  with the document attached when it is sent, its address among the draft's
  links, and the title, seller, purchase date and amount it names. A value it
  does not send stays unknown, and no AI scan runs. The endpoint takes a
  form, a multipart form with the document in `file`, or a flat JSON object,
  and accepts only an API token that may write, for a member who is not a
  viewer. The source system, its instance and the document id identify a
  document in the group: sending it again with the same values answers 200
  and makes nothing, and with other values answers 409
  `intake.document_changed` and changes nothing, also after the draft was
  discarded or became an item. The address is stored and shown, never
  fetched. A draft's inbox row says **From Paperless-ngx** and links to the
  document there, and the item made from it shows the same in a **Source**
  row; `GET /intake/documents` lists the delivered documents by draft or
  item. Full database backups carry the record (format 3.7; an older reader
  restores without it). The user guide has a Paperless-ngx recipe
  ([#2809](https://github.com/denisvmedia/inventario/issues/2809)).

- Add items from files: drop receipts, invoices and product photos on the
  Items page, or pick them with **Add from files**, and the AI scan reads them
  on the server in the background. Each file is a separate purchase unless
  **All files show one item** is on. The review page lists each purchase with
  the products found; the member keeps some of them, adjusts the quantity and
  the fields each draft takes, and gets exactly one draft per kept product. A
  draft takes the amount of its own receipt line, never the receipt total, and
  a value the scan did not read stays unknown. Every draft links the files of
  its purchase, so one receipt serves all its items. Choices and outcomes are
  stored on the server: the review shows the same state on any device, opening
  it never starts a scan, retrying only queues unfinished files, and repeating
  the request after a lost response makes no second draft. Scans count against
  the member's hourly scan limit and wait in the queue when it is reached.
  Batches are listed in an **Added from files** card on the Items page with a
  summary such as "2 drafts created, 1 file unreadable", and the Inbox lists
  every batch that is not finished, with its progress, until each file is
  scanned and each product has a draft or is left out; a file that could not
  be read keeps its batch there until it is retried or the batch is removed.
  The dialog refuses a HEIC photo up front and says to save it as JPEG.
  Batches are not in backups; a full replace removes them. The new API is
  `/intake` (limits, sessions with a client key, candidate review, drafts,
  cancel, resume, rescan, delete), `GET /inbox` lists the open batches
  (reason `intake`, kind `intake_session`, with their progress in `intake`),
  and the AI scan now reports the quantity of a product line
  ([#2810](https://github.com/denisvmedia/inventario/issues/2810)).

- An Inbox page lists what in the group is still unfinished: the drafts of
  new items and the items marked as draft. Each row says why it is there and
  offers the step that clears it: Continue opens a draft in the Add item
  dialog over the inbox, Discard deletes it, and Mark as reviewed turns an
  item's draft switch off, with Undo. Tabs filter by reason. The sidebar entry
  shows the number of records that need attention, counting each record once,
  and the dashboard shows a "Needs attention" card while there are any. A
  Missing details section links the No location and untagged item lists,
  which stay out of the inbox: a missing detail is not unfinished work. The
  Items page shows a line with an Open inbox button in place of the Drafts
  card. The new API is `GET /inbox` (paged, filtered by `reason`) and
  `GET /inbox/count`
  ([#2794](https://github.com/denisvmedia/inventario/issues/2794)).

- The Items page has a Missing filter for items without a photo, without a
  purchase price or without a purchase date. An unknown price or date counts
  as missing even after the item was reviewed with it; a price of 0 is a
  price. The inbox's Missing details section lists these three next to No
  location and No tags, each with the number of items its link shows, and
  none of them adds to the inbox count. Saved views keep the new filters. The
  item query API takes `no_photo`, `no_purchase_price` and
  `no_purchase_date`, and `GET /inbox/checks` returns the counts
  ([#2794](https://github.com/denisvmedia/inventario/issues/2794)).

- The items to review can be checked one after another. Review one by one
  on the inbox opens the first of them, and an item opened from the inbox
  shows a review bar: Mark as reviewed turns its draft switch off and opens
  the next item, or returns to the inbox after the last one; Skip opens the
  next item unchanged. Previous, Next and Save and next in the edit form walk
  the same list. Continue on a draft opens the Add item dialog on the first
  step that still needs input instead of Basics, with the field to correct
  marked. Mark as reviewed, on the inbox and in the review bar, changes the
  draft switch alone, so an edit of the item saved at the same moment is
  kept; the new API is `PATCH /commodities/{id}/draft`
  ([#2794](https://github.com/denisvmedia/inventario/issues/2794)).

- Save as draft in the Add item dialog saves the form and its files on the
  server as a draft of the group, so an item started on one device can be
  finished on another. Nothing is required to save a draft; a **Save draft**
  button is on every step while the switch is on. The Inbox lists the
  group's drafts with Continue and Discard. Finishing a
  draft creates exactly one item and moves the draft's files to it, and a save
  repeated after a lost response does not make a second draft. Discarding
  sends the files nothing else uses to the Trash. A save made on another
  device since the draft was opened is not overwritten: the dialog offers to
  load it or save as a new draft, which gets the old draft's files too. A
  draft finished or discarded on another device is not saved over: the dialog
  offers to open the item or save as a new draft. Drafts stay out of item
  lists, counts, search, reports, saved views and reminders. A draft's type
  must be one of the group's item types. A whole-group backup carries the open
  drafts with their files (format 3.5; a 3.4 reader restores it without them).
  A restore links a draft to the restored area, seller and files and maps its
  type as it maps an item's; a merge keeps a draft the group already has and
  reports it in the restore log, and a full replace replaces the group's
  drafts with the backup's. Deleting an account deletes the drafts its user started; files
  other members saved with them stay as standalone files. The short name of a
  new item follows the name until one is typed. The new API is `/drafts`
  (list, create with a client key, save with a revision, discard, promote with
  a promotion key that makes a retry return the same item). The dialog no
  longer creates items flagged as drafts; editing an item keeps that switch
  ([#2812](https://github.com/denisvmedia/inventario/issues/2812)).

- An item has a brand and a model, both optional text of up to 100
  characters. Cards, list rows and the item page show "Brand · Model" under
  the name, the insurance reports put the brand in the item's title and the
  model under it, and search finds items by either. The brand input suggests
  the brands the group already uses, most used first, from the new
  `GET /commodities/brands` endpoint. AI scan fills both, and backups carry
  them (format 3.2; a 3.1 archive restores with them empty)
  ([#2804](https://github.com/denisvmedia/inventario/issues/2804)).

- Sellers. A group keeps the shops and people it buys from, each with a name
  (unique in the group, ignoring case and extra spaces), a website, a contact
  and a note, on a new Sellers page that counts the items bought from each
  and links to them. An item names its seller in the Purchase step and the
  edit form, where "New seller" creates one without leaving the form. The
  items list filters by one or more sellers (`seller_id` on
  `GET /commodities`), the item page and the insurance reports show the
  seller, and deleting a seller keeps its items without a seller. The item
  History records a change of seller. AI scan fills the seller when the group
  has one with the name it read and otherwise offers to create it, instead of
  writing the name into the comments; a draft from the public landing page,
  which has no group yet, still gets the seller in its comments. Backups
  carry the sellers (format 3.3) and a restore links items to the group's
  sellers by name, creating the missing ones
  ([#2804](https://github.com/denisvmedia/inventario/issues/2804)).

- Bulk edits on the items list. The bulk-action bar has an Edit menu that adds
  and removes tags (an item keeps the tags the change does not name), sets the
  type, and changes the status with the same date and note dialog as a single
  change; sold is left out, since each item needs its own sale price. After the
  whole page is selected, "Select all N matching items" selects every item the
  search and filters match, on every page, for every bulk action but label
  printing. It is offered when 1 to 500 items match and the list could tell
  which items those are; otherwise a bulk action works on the items selected
  one by one or a page at a time. Each item goes through the checks of a
  single change and gets its own history event. When some items fail, a dialog
  lists them with the reason, keeps them selected and offers a retry of those
  items alone. The new
  `POST /commodities/bulk-update` takes ids or a query of the item query
  contract with the number and the digest of the items it was seen to match
  (`meta.selection_digest` of `GET /commodities`), and refuses with 409 when
  the query selects other items by then, even as many; `bulk-move` and
  `bulk-delete` take the same selection, and all three answer with the items
  that changed, were skipped as unchanged, or failed with a reason (a
  validation failure names the field and the rule, so the app can say it in
  the user's language). A query may select up to 500 items. `bulk-move` to an
  area that does not exist is refused with 422 before any item changes.
  `POST /trash/commodities/restore` gives each failure a code as well:
  `not_found` for an item that is not in the trash, `failed` otherwise. The
  Files page's bulk bar gains **Tags…**, which adds and removes tags on the
  selected files the same way, through the new `POST /files/bulk-update`
  ([#2798](https://github.com/denisvmedia/inventario/issues/2798)).

- Item types are a list each group keeps, instead of a fixed set of six. Every
  group starts with the six built-in types, seeded at startup for existing
  groups and on first use for new ones; members see their names in their own
  language until someone renames them. A new Item types page adds, renames and
  deletes types, each with a name (up to 50 characters, unique in the group)
  and one of 38 icons, and counts the items of each type, Trash included.
  Deleting a type that items have moves them to a type the member picks, which
  is also how two types are merged; a group keeps at least one type and at
  most 100 (409 `item_type.limit`). A type stays with the group when the
  member who added it deletes their account. The item forms pick from the
  group's types and create one with "New type". Items keep their type:
  `commodities.type` stays the type's slug, so the `type` filter, saved views,
  the reports and the icons work as before. The API gains
  `/g/{slug}/item-types` (`DELETE` takes `reassign_to`), and an item write
  with a type the group does not have fails with 422 `item_type.unknown`; a
  bulk type change picks from the group's types and refuses another with 422
  on `type` before any item changes. AI scan offers the group's types. Backups
  carry the types (format 3.4) and a restore matches them by slug, then by
  name, creating the missing ones
  ([#2802](https://github.com/denisvmedia/inventario/issues/2802)).

- Item photos large enough to recognize an item by. The items list opens on
  photo cards: the cover photo on top at 4:3, then the name and key facts,
  from two cards a row on a phone to six on a wide screen. The previous cards
  stay as **Compact cards** next to **List**, and Settings → Appearance →
  Default view offers all three. The list shows 24, 48 or 96 items per page,
  remembered on the device. On the item page the cover sits beside the details
  and opens the full-screen viewer, which goes through the item's photos.
  Thumbnails gain a `large` size (600 px) for these surfaces; a photo uploaded
  earlier gets it the first time a card shows it
  ([#2797](https://github.com/denisvmedia/inventario/issues/2797)).

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
  their Dashboard. Warranty filters stay relative to the day the view is
  opened. The Dashboard suggests a few views to start with, and **Saved views**
  (`/g/:slug/views`) lists, edits, reorders and deletes them. New endpoints: `/g/{slug}/saved-views` and
  `/g/{slug}/saved-views/counts`
  ([#2793](https://github.com/denisvmedia/inventario/issues/2793)).
- Your own lead times for warranty, maintenance and loan reminders. Under
  Settings → Notifications, the Warranty expiry and Maintenance reminders rows
  take up to five "days before" values from 1 to 365, and the Loan reminders
  row takes one value from 1 to 60 for the items you lend. Each member of a
  group keeps their own choice. Unset, the defaults stay as they were: 60, 30
  and 7 days for warranties, 14, 7 and 1 for maintenance, and
  `--loan-reminder-due-soon-days` for loans, which `/system` now reports as
  `loan_reminder_due_soon_days`
  ([#2805](https://github.com/denisvmedia/inventario/issues/2805)).
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
- Failed background jobs. Group admins and owners see the jobs of their
  group that stopped after their last attempt (exports, imports, restores,
  thumbnails, document scans and currency migrations) in **Group settings →
  Data & storage**, with the error and a **Retry** button. A restore and a
  currency migration cannot be retried; the card says so, and a new one has
  to be started. System admins get a **Failed jobs** page in the back office
  with every tenant's failed jobs, type and tenant filters, **Retry** and
  **Discard**. Discarding takes a job off the list but keeps its record and
  error until the normal cleanup removes them. Every retry and discard is
  written to the audit log. The API gains `GET /g/{groupSlug}/failed-jobs`
  and `POST /g/{groupSlug}/failed-jobs/{jobType}/{jobID}/retry` for group
  admins, and `GET /admin/failed-jobs` with `retry` and `discard` under
  `/admin/failed-jobs/{jobType}/{jobID}/` for the back office
  ([#1307](https://github.com/denisvmedia/inventario/issues/1307)).

### Changed

- Subscription plans are hidden. The feature is on hold, so the profile and
  Preferences pages show no "Free plan" badge or Upgrade link, `/plans`
  redirects to the group home, the group settings card is titled **Usage**
  and shows the group's items, locations and file storage without limits,
  and the admin tenant page has no Plan tile. `GET /g/{slug}/plan` reports
  the unlimited plan whatever the tenant's `plan_id` is. No plan limit was
  enforced before and none is now; the group storage quota does not depend
  on plans. The server's operator can show plans again (see Operators)
  ([#1389](https://github.com/denisvmedia/inventario/issues/1389)).
- **Add item** uploads the item's files to its draft before it creates the item,
  and the dialog stays open while it runs; leaving the page during the uploads
  stops them after the file in progress, and no item is created. Each file shows
  whether it is uploading, uploaded or failed, and the button counts the
  uploads. A failed upload creates no item: the dialog names the file and keeps
  the form, and **Retry** uploads only the files still missing before it creates
  the item. On a saved draft, a failed **Add item** leaves the draft's values as
  they were saved. Each staged file has an editable title, the file name without
  its extension by default. **Discard** in the Add item dialog, and closing the
  batch intake dialog before its batch is queued, move the files they uploaded
  to the Trash instead of leaving them as standalone files; **Discard** keeps a
  draft the user saved since
  ([#1626](https://github.com/denisvmedia/inventario/issues/1626)).

- On a phone (a screen narrower than 640px) the dialogs that hold a longer
  form fill the screen: adding and editing an item, locations, areas, custom
  fields, sellers, changing the group currency, adding items from files,
  uploading files, attaching an existing file, restoring a backup, lending
  and loans, sending for service, maintenance schedules, reminders, API
  tokens and feedback. The title and the close button stay at the top, the
  buttons stay at the bottom, and the form scrolls between them.
  Confirmations and short dialogs stay small cards. Wider screens keep the
  centered dialog, which now scrolls when it is taller than the screen, as on
  a phone held sideways
  ([#1627](https://github.com/denisvmedia/inventario/issues/1627)).

- An item's cover photo is one of its photos: an image attached to the item
  in another section of its Files tab can no longer be set as the cover, and
  a cover set that way before shows the first photo instead. The No photo
  filter counts the same photos
  ([#2794](https://github.com/denisvmedia/inventario/issues/2794)).

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
- Warranty and maintenance reminders send one email for the closest lead time
  reached, the way item reminders do. If the server was down, or a lead time
  was added after it had passed, the email for that closest lead time still
  goes out, late. Farther lead times that had already passed are not sent,
  and a longer lead time is not sent after a shorter one. Raising a lead time
  therefore does not email every item that is already closer than it. The
  `threshold` label of `inventario_warranty_reminders_sent_total` and
  `inventario_maintenance_reminders_sent_total` is `custom` for a lead time
  other than the defaults
  ([#2805](https://github.com/denisvmedia/inventario/issues/2805)).
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

- `GET /commodities` and `GET /search?type=commodities` read one shared set of
  filter parameters, including the new `tags`, `tag_mode`, `tags_exclude` and
  `no_tags`. An unknown parameter, an unreadable value or a contradictory
  combination is answered with 400 and the code `query.invalid_parameter`
  instead of being ignored, and the `operator` parameter of `/search` is gone
  (`tag_mode` replaces it). The search page's link to all matching items now
  includes inactive items, as the search does
  ([#2800](https://github.com/denisvmedia/inventario/issues/2800)).

### Fixed

- A file added with tags in the Add item dialog keeps its title. The dialog
  set the tags with `PUT /files/{id}`, which replaces the title as well, and
  sent no title, so the file had none
  ([#1626](https://github.com/denisvmedia/inventario/issues/1626)).

- A file title may have 255 characters in any language. The server counted
  the limit in UTF-8 bytes, so a Czech or Russian title of more than about
  127 letters was refused with 422
  ([#1626](https://github.com/denisvmedia/inventario/issues/1626)).

- Settings saved on the server show up in the app again. The settings
  endpoint sent its keys capitalized (`NotificationsWarrantyExpiry`) while the
  app reads camelCase (`notificationsWarrantyExpiry`), so after a reload the
  notification toggles showed their defaults, and the saved language and
  default items view were not applied
  ([#2805](https://github.com/denisvmedia/inventario/issues/2805)).
- A warranty, maintenance or loan reminder is no longer dropped when a
  recipient's account cannot be read for a moment, for example during a
  database restart. The reminder is retried on the next sweep instead of
  going out without that recipient, or, for a loan, being marked as sent
  with no email
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

### Operators

- `inventario jobs failed list|retry|discard` lists, retries and discards
  the background jobs that failed for good, across every tenant, and writes
  each retry and discard to the audit log. Two metrics come with it:
  `inventario_jobs_dead_lettered_total{type}`, counted by every process that
  runs workers, and the `inventario_jobs_dead_letter_depth{type}` gauge of
  jobs nobody retried or discarded, collected by the housekeeping worker
  group (or `run all`) every `--business-metrics-interval`. The upgrade runs
  one more database migration: it adds `discarded_at` to `exports`,
  `restore_operations`, `thumbnail_generation_jobs`, `intake_sources` and
  `currency_migrations`, `attempt_count` to `exports`, and an index on the
  failed intake sources. During a rolling upgrade, pods that still run the
  previous version can fail reads of those tables until they are replaced,
  as with the `hidden_sections` column below. See the admin runbook §10
  ([#1307](https://github.com/denisvmedia/inventario/issues/1307)).
- `--enable-plans` / `INVENTARIO_RUN_ENABLE_PLANS=true` (Helm value
  `features.plans`, Compose `ENABLE_PLANS` in `.env`) shows the subscription
  plan surfaces that are now hidden by default. It is off by default, and
  `GET /feature-flags` reports it as `plans`. With it on,
  `GET /g/{slug}/plan` reports the tenant's plan again
  ([#1389](https://github.com/denisvmedia/inventario/issues/1389)).
- The upgrade runs two more database migrations. One adds the
  `hidden_sections` column to `location_groups`. During a rolling upgrade,
  pods that still run the previous version fail every read of a group until
  they are replaced, because they read all columns of the table and do not
  know the new one. Expect group pages and group API calls served by those
  pods to fail with 500 until the rollout finishes
  ([#1607](https://github.com/denisvmedia/inventario/issues/1607), the class
  of problem tracked in
  [#2873](https://github.com/denisvmedia/inventario/issues/2873)).
  The other adds the `personal_data_exports` table for personal data exports.
  The archives are stored with the uploads under `t/<tenant>/personal-exports/`;
  the export worker deletes each one 7 days after it is finished
  ([#2177](https://github.com/denisvmedia/inventario/issues/2177)).
- Sign-in with an OpenID Connect provider you run yourself, such as Authelia,
  Keycloak or Authentik. Set `OAUTH_OIDC_ISSUER_URL`, `OAUTH_OIDC_CLIENT_ID`,
  `OAUTH_OIDC_CLIENT_SECRET` and, for the button, `OAUTH_OIDC_DISPLAY_NAME`;
  the endpoints come from the issuer's discovery document.
  `OAUTH_OIDC_BACKCHANNEL_URL` covers a server that cannot reach the issuer's
  public address. The Helm chart gets `oauth.*` values and `secrets.oauth*`
  keys for all three providers. Setup is described on the new "Sign-in
  providers" page of the user guide
  ([#2789](https://github.com/denisvmedia/inventario/pull/2789)).
- The upgrade runs one more database migration. It writes the descriptions of
  the row-level security policies and of the RLS helper functions into the
  database as comments (`COMMENT ON POLICY`, `COMMENT ON FUNCTION`). The models
  always declared them, but earlier migrations never wrote them. It changes no
  table, index or data. It comes with the move to Ptah 0.36, which compares
  these comments too
  ([#2849](https://github.com/denisvmedia/inventario/pull/2849)).
- The Helm chart's Grafana dashboard shows its "Scrape target up" panel on
  Kubernetes. The panel asked for `up{job="inventario"}`, a job name only the
  compose stack uses. Under the Prometheus Operator the job is named after the
  Service (for example `inventario-inventario`), so the panel stayed empty. It
  now shows every job that reports Inventario's metrics.
- The Helm chart's install notes point to the right place for the admin's
  email and password when `secrets.existingSecret` is set: the
  `SETUP_ADMIN_EMAIL` and `SETUP_ADMIN_PASSWORD` keys of that Secret. Before,
  they printed `setupJob.initData.adminEmail`, which that Secret's email
  overrides, and pointed to `setupJob.initData.adminPassword`, which an
  existing Secret does not use.
- `inventario users update --tenant` moves a user only when they belong to no
  group and own or authored nothing in their tenant, typically an account
  created in the wrong tenant. Otherwise it refuses, lists what blocks the
  move (for example "member of 2 groups, 14 items, 3 files") and changes
  nothing; `--dry-run` runs the same check. A move signs the user out
  everywhere: their sessions and API tokens are deleted. Their password,
  second factor, linked sign-in accounts, language and other settings and
  their login history move with them, and their profile photo is copied into
  the new tenant's storage, which the new `--upload-location` flag names. A
  user record written from a copy read before a move is refused instead of
  putting the old tenant back; the back-office block and unblock endpoints
  answer it with 409 and the code `user.tenant_changed`
  ([#2179](https://github.com/denisvmedia/inventario/issues/2179)).
- `inventario db migrate data`, and with it the Helm chart's init-data job,
  never moves a user between tenants. When the admin email exists only in
  another tenant, the run warns, names that tenant and creates no admin. It
  fails instead when the default tenant did not exist before the run, as on
  a database restored under another tenant id
  ([#2179](https://github.com/denisvmedia/inventario/issues/2179)).
- New profile photos are stored under their tenant's prefix,
  `t/<tenant>/avatars/<user>/...`, instead of `avatars/<user>/...`. Photos
  stored under `avatars/` keep working. A bucket lifecycle rule that matches
  `avatars/` needs the new prefix too
  ([#2179](https://github.com/denisvmedia/inventario/issues/2179)).
- Every API response carries an `X-Request-ID` header, and the log lines the
  server writes while handling that request carry the same value in a
  `request_id` field (a top-level key with `INVENTARIO_LOG_FORMAT=json`), so
  the id of a failed request finds its lines. The access log prints the same
  id, and `audit_logs.request_id` stores it. An `X-Request-ID` sent by a proxy
  or a client is kept when it is at most 128 characters of letters, digits and
  `-_.:/+=`; any other value, and a missing header, gets a generated UUID.
  Before, the server returned no header, printed any client value in the
  access log as sent, and generated ids in chi's
  `<hostname>/<random>-000001` form, which named the pod. CORS exposes the
  header to the frontend. The conventions behind the field are in the new
  [logging guideline](https://github.com/denisvmedia/inventario/blob/master/devdocs/logging.md)
  ([#521](https://github.com/denisvmedia/inventario/issues/521)).

## [0.2.2] - 2026-11-23

### Operators

- `setupJob.bootstrap.superuserSecret` in the Helm chart: a Secret of your own
  with the superuser DSN for the database bootstrap, which only the setup Job
  reads ([#2783](https://github.com/denisvmedia/inventario/pull/2783)).
- A `SETUP_ADMIN_EMAIL` key in the chart's `secrets.existingSecret`, which wins
  over `setupJob.initData.adminEmail`, so the first admin's real address need
  not be in the values ([#2785](https://github.com/denisvmedia/inventario/pull/2785)).
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

- Privacy Policy and Terms of Service pages, linked from the registration
  consent checkbox ([#2148](https://github.com/denisvmedia/inventario/issues/2148)).
- End-to-end coverage of the onboarding loop through the browser: register,
  verify by email, sign in, reset the password, sign in again
  ([#2114](https://github.com/denisvmedia/inventario/issues/2114)).

### Fixed

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
- Client-facing error bodies no longer carry stack traces
  ([#2178](https://github.com/denisvmedia/inventario/issues/2178)).
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

### Changed

- The admin, back-office and impersonation end-to-end suite runs again. It had
  been skipped entirely since the back-office auth plane landed
  ([#2100](https://github.com/denisvmedia/inventario/issues/2100)).

### Security

- The image is built with Go 1.27.4, and `golang.org/x/net`,
  `go-jose`, `klauspost/compress` and `andybalholm/brotli` are updated. The
  shipped binary had a man-in-the-middle flaw in `crypto/x509` and
  denial-of-service flaws in the HTML parser and in JOSE token parsing.

### Operators

- `extraVolumes` and `extraVolumeMounts` in the Helm chart, for example to
  mount the database server's CA for `sslmode=verify-full`. The volumes go into
  every pod of the chart except the demo ones, and the mounts into every
  container that runs Inventario or reaches the database, the `migrate` init
  container and the backup included
  ([#2774](https://github.com/denisvmedia/inventario/pull/2774)).
- `setupJob.migrateInInitContainer` makes the in-pod migration ordering
  available without ArgoCD, so a plain Helm or Helmfile install no longer has
  to migrate before the rollout starts
  ([#2086](https://github.com/denisvmedia/inventario/issues/2086)).
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
- Bootstrap now grants the app, worker and admin roles access to tables created
  by a migration login named anything other than `inventario_migrator`. With a
  custom name, every migrated table came out unreadable to the application
  ([#2520](https://github.com/denisvmedia/inventario/issues/2520)).
- `/metrics` no longer listens on the public port, where the default ingress
  rule exposed installation-wide gauges to the Internet
  ([#2244](https://github.com/denisvmedia/inventario/issues/2244)).
- The chart no longer mounts a Kubernetes API token into application pods that
  never use one ([#2245](https://github.com/denisvmedia/inventario/issues/2245)).
- A default `helm install` resolves an image again: the chart's `appVersion`
  named a tag that was never published
  ([#2035](https://github.com/denisvmedia/inventario/issues/2035),
  [#2036](https://github.com/denisvmedia/inventario/issues/2036)).
- `inventario version` prints to stdout
  ([#2455](https://github.com/denisvmedia/inventario/pull/2455)).

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

# Edge Cases

Every UI surface that's not the happy path. Senior-design discipline lives here — it's where products feel finished or feel rough.

## Navigation edge cases

### 404 — page not found

Per `11-page-layouts-and-flows.md`. Full-page empty state with illustration and "Take me home" CTA.

### 403 — permission denied

Page content replaced with:
```
[icon: lock]

You don't have access to this.

If you think this is a mistake, ask your workspace
admin to grant you access.

[Go back]   [Switch workspace]
```

### Stale link to deleted entity

User opens `/things/abc123` but the thing was deleted:
```
[icon: empty drawer]

This thing isn't here anymore.

It may have been deleted, or you may be in the
wrong workspace.

[See all things]
```

If user has audit-log permission, "When was it deleted?" link reveals the audit log entry.

### Wrong workspace

User navigates to `/g/foo/things/abc123` but thing belongs to a different workspace:
```
[icon: signpost]

This is in another workspace.

Switch to "Personal" to view it?

[Switch workspace]   [Go back]
```

## Empty data edge cases

### Brand-new account, never used

Per onboarding flow (`11-page-layouts-and-flows.md`). User lands on welcome screen, picks a use case, adds first thing. Dashboard shows "Recently added: 1 thing — your first one." with a celebratory tone (not over the top).

### Account with no items in some categories

Dashboard shows partial stats. Empty widgets show their own empty state (per `15-form-and-data-ux.md`).

```
Value over time
[empty chart with hint: Add prices to start tracking value over time.]
```

Each widget knows what data it needs and what to say if it's missing.

### Filter / search returns nothing

Per `11-page-layouts-and-flows.md`. "No things match \"foo\"." with "Try different terms" + "Clear filters" CTAs.

## Long-content edge cases

### Very long item name

Names can be arbitrarily long ("Vintage Mid-Century Walnut Sideboard with Brass Hardware and Sliding Doors").

Rules:
- Lists / cards: truncate to single line with ellipsis, full name on hover (tooltip) or in detail page
- Detail page hero: wrap to 2 lines, then truncate with ellipsis; full name accessible via "..." menu → "Copy name"
- Forms: max 200 characters server-side; UI shows character count when ≥150

### Very long description / notes

Server-side limit: 5000 chars. UI:
- Collapsed by default in detail page (show first ~200 chars + "Read more")
- Expand inline; clicking again collapses
- Print view shows full text

### Many tags

Users may apply many tags ("kitchen, white-goods, lg, dishwasher, brand-name, year-2024, ...").

Rules:
- Card view: show first 3 tags, then "+N more" chip
- Detail view: show all, wrapping
- Filter dropdown: search-as-you-type within the chip selector

### Many files attached

A commodity may have 50 photos.

Rules:
- File gallery: paginate or virtualize after 24 visible
- Detail view: section heading shows count "Images (52)"
- File viewer: thumbnail strip scrolls, current always centered

## Long async operations

### Slow upload

User uploads a 500MB video. Per `10-file-and-media.md`:
- Per-file progress bar
- Time estimate shown if upload >30s
- User can navigate away — upload continues in background, surfaced via persistent indicator
- Cancel option always available

### Slow backup

Server-side job. UI:
- "Generating backup… this may take a minute."
- Progress percentage if known
- Toast on completion with download link
- Notification center entry for later access

### Slow sync (future PWA)

If offline mode added:
- Pending mutations queued
- Sync indicator in shell
- On reconnect, sync runs; user sees a small banner: "Syncing 5 changes…" → "Up to date."

## Concurrency edge cases

### Simultaneous edits

Per `15-form-and-data-ux.md`. Server returns 409, UI offers conflict resolution.

### Stale lists during edit

User opens "Things" list, navigates to a thing, deletes it, returns to list. List should reflect deletion. Solution: invalidate query cache on mutation; refetch on focus.

### Tab open in two places

User opens app in two tabs, edits in one. The other tab still shows old data until refocused (then refetches). For shared workspaces, also invalidate via WebSocket or polling — but v1 acceptable to require a refresh.

## Network edge cases

### Offline

- Detect via `navigator.onLine` + heartbeat
- Show banner: "You're offline. Changes won't be saved until you reconnect."
- Disable mutations (or queue them, v2)
- Cached read-only data still browsable

### Slow connection

- Skeletons hold longer
- Show "Still loading…" copy after 5s
- Retry option after 30s timeout

### API errors

Server returns 500:
- Show inline error in affected widget ("Couldn't load things. [Retry]")
- Don't replace whole page unless the error is page-fatal

## Authentication edge cases

### Session expired during use

- Detect via 401 response
- Save current form state to sessionStorage
- Redirect to login with "Your session expired. Sign in to continue."
- After login, restore form state and offer to re-submit

### Wrong workspace selected

User has access to multiple workspaces; current workspace doesn't have the entity they're trying to view. Per "Wrong workspace" above.

### Invitation expired / revoked

User clicks invite link in email after expiration:
```
[icon: hourglass]

This invitation has expired.

Ask the person who invited you to send a new one.

[Got it]
```

## Data integrity edge cases

### Currency change

User changes their primary currency in profile. All values in display reformat instantly (per `13-formatting-and-i18n.md` — show in user's primary currency). Mention in toast: "Showing values in EUR now. Original prices stay in their original currency."

### Time zone change

Travel — user's browser TZ changes. All relative timestamps reflect new TZ. No UI fanfare; this is silent.

### Locale change

Language switches → entire UI re-translates instantly. Date / number formatting updates. Form drafts preserved.

### Workspace deleted while user is in it

User's workspace deleted by admin while user has a tab open. Next API call returns 410 Gone:
```
[icon: door-closed]

This workspace was deleted.

[Switch to another workspace]
```

## File edge cases

### Corrupt file

User uploads a file that fails server-side validation (truncated, malformed):
```
[icon: warning]

That file looks damaged.

We couldn't read it. Try uploading a fresh copy.

[Choose another file]
```

### Unsupported file type

Per `10-file-and-media.md`. Replace preview with type icon + "Preview not available — Download to view."

### File too large

Pre-flight client check; if exceeded:
```
That file is too large.

The maximum is 50 MB. This one is 142 MB.

[Choose a smaller file]
```

### Storage quota reached

Banner across app + upload disabled:
```
You've used all your storage.

Free up space by removing old files, or upgrade
your plan to add more.

[Manage files]   [Upgrade]
```

### File missing on server (orphan)

Database has a file record but the binary is gone (storage corruption, manual cleanup):
- File card shows "File missing" overlay with "Remove this record" action
- Admin alert in audit log
- Backup process flags missing files in summary

## Form edge cases

### User clicks Save twice

- After first click, button enters loading state (per `08-interaction-states.md`)
- Disabled during load, ignores second click
- Toast on completion

### User navigates away with unsaved changes

Confirmation dialog: "You have unsaved changes. Discard them?"

### User refreshes mid-form

For long forms, optionally save to sessionStorage on every change. On refresh, restore with "Restored from local — your draft is back. [Save now] [Discard]"

### Browser autofill clobbers fields

Native autofill can fill fields the user didn't intend. UI:
- Don't actively fight autofill (it's accessibility-positive)
- Provide clear field labels so users notice mismatches
- Critical fields (insurance values) require explicit user typing — disable autocomplete

## Search edge cases

### Empty query

Show recent searches + popular shortcuts ("All things", "Recently added", "Drafts"). Don't run a query with empty terms.

### Query with only special characters

Sanitize and handle gracefully. "@@@@" returns no results with helpful copy.

### Query that matches huge number of items

Cap displayed results at 100, show "Showing 100 of 4,328 results — refine your search." Provide refine-by suggestions.

### Search service unavailable

Fall back to client-side filtering on currently-loaded data, with a banner: "Server search is unavailable. Showing local matches only."

## Accessibility edge cases

### User uses keyboard exclusively

All flows tested for keyboard-only completion. No mouse-required interactions.

### User uses screen reader

All async updates announced via live regions. No visual-only state changes.

### User has reduced motion

Static states must be readable; animations replaced with instant transitions.

### User has very large default font

Font-relative sizing means UI scales. No fixed pixel widths. Tested at 200% browser zoom.

## Print edge cases

### Print button clicked on empty inventory

Skip print; show toast: "Add at least one thing before printing."

### Print on mobile

Mobile browsers don't all support `window.print()` reliably. Provide "Save as PDF" alternative.

## Multi-user edge cases

(Mostly future work, but design-system-aware now.)

### Concurrent editor leaves stale lock

If using soft locks: locks expire after 5 minutes idle. Other user can claim with confirmation.

### User removed from workspace mid-session

Next API call returns 403 → redirect to "You no longer have access to this workspace."

### Orphan invitations

Invites that haven't been accepted in 30 days expire. Sender can resend.

## What ships in sprint 0

These edge cases are mostly **components and copy** to handle. Foundation work that ships in sprint 0:

1. Build error-page templates: 404, 403, 500, "deleted entity", "wrong workspace"
2. Build offline banner + connection-state indicator
3. Build "session expired" redirect-with-state-preservation flow
4. Wire up all destructive-action confirmation patterns per `15-form-and-data-ux.md`
5. Build "unsaved changes" navigation guard

Sprint 1+:
- Storage quota system
- Concurrent-edit conflict resolver UI
- Audit-log integration with deleted-entity pages
- File-missing handling
- Search service fallback

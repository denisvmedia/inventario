# Notifications & Trust

How Inventario surfaces alerts, expirations, security signals, and handles user trust.

## Notification taxonomy

Five severity levels, each with consistent visual + tonal treatment:

| Severity | Examples | Visual |
| --- | --- | --- |
| **info** (default) | "New backup available", "Tip: tag this item" | `--info` icon (Phosphor Info, fill weight) |
| **success** | "Backup complete", "Saved" | `--success` (CheckCircle, fill) |
| **warning** | "Warranty expires in 14 days", "Storage 90% full" | `--warning` (Warning, fill) |
| **destructive** | "Backup failed", "Connection lost" | `--destructive` (XCircle, fill) |
| **action-needed** | "5 items need a price for insurance", "1 unrecognized login" | accent (CircleDashed, regular) |

## Where notifications appear

### Toast (transient)

Per `09-component-patterns.md`. Used for action confirmations and async outcomes. Disappears automatically (per severity timing).

### Inline alert (persistent, contextual)

Inside a page or section, when relevant context. Examples:
- Top of dashboard: "3 warranties expiring this month"
- Top of file viewer: "This file is shared with 2 collaborators"
- Inside form: "This action cannot be undone"

Persistent until acknowledged or context changes. Dismissible if not critical.

### Banner (sticky, app-level)

Pinned to the top of the app shell. Used for:
- Maintenance windows ("Scheduled maintenance at 3 AM tomorrow")
- Storage warnings ("Storage 95% full — clean up or upgrade")
- Trial expiration / payment failures (if monetized)
- Security incidents ("Password change required")

Banners have a close (X) where dismissible; some are critical and require user action before continuing.

### Notifications center (drawer)

Triggered by a bell icon in the top-right (next to user avatar). Drawer lists:
- Unread notifications (badge count on bell)
- All notifications, grouped by date
- Filter by severity
- Mark all as read
- Per-item: title, description, time, action (if applicable)

This is **not** a toast history — notifications here are server-pushed events about user data: warranty expirations, backup completions, storage alerts.

## Specific notification types in Inventario

### Warranty expirations

Triggered server-side when:
- 30 days before expiration → info notification
- 14 days before → warning + email (if user opts in)
- 1 day before → warning + email
- After expiration → archived but visible in notifications center

Display in notifications center:
```
[icon] Your dishwasher's warranty expires in 14 days.
       LG WD-15TD · purchased Apr 2024
       [View thing] [Snooze]
```

### Storage warnings

- 80%: info banner
- 95%: warning banner
- 100%: destructive banner (uploads disabled)

### Backup completion

- Success: toast + entry in notifications center with download link
- Failure: destructive toast (no auto-dismiss) + entry in notifications center with retry

### Security events

- New login from unrecognized device: action-needed in notifications center + email
- Password changed: success toast + email
- Failed login attempts: warning email (no in-app — user wasn't there)
- Data export requested: success notification when ready

### Insurance reminders (opt-in)

- Annual reminder: "It's been a year since you last reviewed your inventory for insurance. Want to print a summary?"
- Action: link to print/export

## Email templates

Inventario sends transactional emails:
- Welcome / verification
- Password reset
- Warranty reminder
- Backup ready
- Login alert
- Weekly digest (opt-in)

### Email design rules

- Plain text first; HTML version with palette tokens applied via inline CSS
- Subject lines: factual, not marketing ("Your warranty expires in 14 days" not "Don't miss this!")
- One CTA button per email
- No images required (text alternative always works)
- From: `Inventario <hello@inventario.app>` — friendly but not a person's name
- Footer: physical address (if business), unsubscribe link (for digests, not transactional)

### Tone in email

Same as in-app: plain, respectful, practical. Keep it under 100 words for transactional, under 200 for digests.

### Example: warranty reminder

> **Subject:** Your dishwasher's warranty expires in 14 days
>
> Hi Denis,
>
> A quick reminder: the warranty on your **LG WD-15TD dishwasher** expires on **May 10, 2026** — 14 days from now.
>
> If something's been off, this is a good moment to make a service call.
>
> [View in Inventario]
>
> — Inventario

## Trust signals

Inventario stores valuables and personal info. Trust signals matter.

### On the marketing/login page

- "Self-hosted or hosted by us — your choice."
- "Encrypted at rest with industry-standard AES-256."
- "Open source. Auditable. Yours to fork."
- "Backups you can download anytime."

### Inside the app

- Profile / Security tab clearly shows:
  - Account email + verification status
  - Last login + device
  - Active sessions (with revoke option)
  - Two-factor auth status
  - Connected services (none, in v1)
  - Data export option
  - Account deletion option

### After deletion

When user deletes a thing/file/account:
- Confirmation explicitly states what's deleted and from where
- For account deletion: "Your data will be permanently removed within 30 days. Some logs may be retained for security audits as required by law."
- No "shadow storage" — when we say deleted, we mean it. Build the audit log to demonstrate this.

## Sensitive data handling

### Insurance values, addresses, identification numbers

These are sensitive. UI treatment:
- Don't display in lists by default (visible only on detail page)
- Mask on screenshots / dev mode (build a `<MaskedValue>` primitive)
- Easy export but no "share publicly" affordance
- Audit log entries when viewed (for shared workspaces)

### Photos of receipts

May contain: address, full name, card numbers (if printed).
- Show user a hint at upload: "Your receipt may show personal info — that's fine, but be aware before sharing."
- No sharing via public link by default; require explicit opt-in per file.

## Permission model

For multi-user workspaces (small business use case):

| Role | Permissions |
| --- | --- |
| Owner | Everything, including delete workspace, manage billing |
| Admin | Everything except delete workspace, manage billing |
| Editor | Add/edit/delete things, files, places. Can't change settings or invite users. |
| Viewer | Read-only |

Permission denied UI: "You don't have access to this." with a link to "Request access" (notifies admins).

## Audit log

For workspaces with multiple users:
- Every mutation logged (who, what, when, from where)
- User can view their own activity in profile
- Admin can view workspace-wide activity
- Logs are append-only; can be exported

Personal use: audit log still exists but mostly invisible — surfaced only on security-sensitive events.

## Status indicators

App shell shows:
- Connection state (small dot near user avatar): green (online), amber (reconnecting), red (offline)
- Sync state: spinner if pending mutations queued (offline, future feature)
- Notification count badge on bell

## Quiet hours

User can set "quiet hours" for email notifications (no warranty reminders 22:00–08:00). Defaults to off.

## Rate limits and abuse

- Login attempts: 5 per 15 min per IP, 10 per hour per email
- File upload: rate-limited per user (configurable)
- API calls: per-tenant rate limit
- All limits surfaced in UI when hit ("Too many attempts. Try again in 5 minutes.")

## What ships in sprint 0

1. Build NotificationCallout primitive (per `09-component-patterns.md` Notification component)
2. Wire in toast notifications for save / delete / upload outcomes
3. Add warranty-expiration server-side event + UI surfacing in dashboard
4. Banner primitive for storage-warning and maintenance messages
5. Email template baseline (welcome, password reset, warranty reminder)

Sprint 1+:
- Notifications center drawer
- Audit log viewer
- Connection-state indicator
- Quiet hours setting

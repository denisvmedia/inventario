# Notifications & Trust

How the app talks back to the user — toasts, inline messages,
confirmations, audit cues, and the trust signals that make the
inventory feel like *theirs*.

## Toast hierarchy

Sonner is the toast layer. Wrapped in `useAppToast()`
(`src/hooks/useAppToast.ts`):

```ts
const toast = useAppToast()
toast.success(t("commodities:toast.created"))
toast.error(t("auth:login.errorGeneric"))
toast.warning(t("settings:warning.unsaved"))
toast.info(t("common:toast.copied"))
```

Severity contract:

| Severity | Auto-dismiss | Color | When |
| --- | --- | --- | --- |
| `info` | 4s | foreground | Neutral feedback ("Copied", "Refreshed") |
| `success` | 4s | `--status-active` (green) | Mutation succeeded ("Item added") |
| `warning` | 6s | `--status-expiring` (amber) | Heads-up ("Saving offline; will retry") |
| `error` | manual | `--destructive` (red) | Mutation failed; user must dismiss |

Errors require manual dismiss because they often pair with retry CTAs
or recovery info that auto-dismiss would erase.

## When to toast vs. when to inline

| Surface | Inline | Toast |
| --- | --- | --- |
| Form-level error | ✅ — banner above the form | ❌ |
| Field-level error | ✅ — below the field | ❌ |
| Successful mutation (single item) | — | ✅ ("Item added") |
| Bulk mutation success | — | ✅ ("3 items deleted") |
| Mutation failure (not field-specific) | — | ✅ (with retry) |
| Auto-save success | ✅ — "Saved" indicator near the field | ❌ |
| Auto-save failure | — | ✅ (with retry + rollback) |
| Page-level data couldn't load | ✅ — "Couldn't load. Retry." inside the data area | ❌ |
| App-level error (route blew up) | The 500 page | ❌ |
| Long task started | — | ✅ ("Export started") |
| Long task completed | — | ✅ ("Export ready") |

A toast is the right surface when the action is **transient**, the
result is **deferred** from the user's current focus, or the action
**affects state outside the current view**.

## Toast count

- One toast per action. Bulk actions emit one grouped toast ("3 files
  uploaded"), not three individual ones.
- Stacking limit: 4. Older toasts collapse; the user sees the most
  recent.
- Don't toast per-row in a loop. If 50 rows fail, emit one toast
  saying "12 of 50 failed; retry?".

## Toast actions

A toast can carry one CTA — a "Retry" or "Undo":

```ts
toast.success(t("commodities:toast.deleted"), {
  action: { label: t("common:actions.undo"), onClick: handleUndo },
})
```

- "Undo" is preferred over "Are you sure?" for reversible deletes.
  Sonner gives a 5-second window — long enough to recover, short
  enough not to nag.
- "Retry" rebuilds the same mutation on the same payload.
- Don't put two CTAs on a toast. The toast is small; don't compete
  with itself.

## Confirmation

`<AlertDialog>` for destructive flows. See [09-component-patterns.md](09-component-patterns.md),
[12-tone-of-voice-and-copy.md](12-tone-of-voice-and-copy.md) ("Destructive confirmations").

Confirmations are NOT toasts; toasts are post-hoc, confirmations are
pre-hoc.

## Audit cues

When the user wants to know *what changed* on an item:

- **Status history card** on the commodity detail (already in production).
- **"Edited 2h ago" footer** on detail pages — uses `formatRelative`
  per [13-formatting-and-i18n.md](13-formatting-and-i18n.md).
- **Member badges on shared changes** in multi-user groups (future).

The product positioning ([00-positioning.md](00-positioning.md)) commits to honesty —
the user owns the data; they get visibility into what's changed.

## Trust signals

The product positioning lists *data ownership* as a primary value
([00-positioning.md](00-positioning.md)). Surfaces that reinforce trust:

| Surface | Signal |
| --- | --- |
| Auth pages | No "Sign in with Google" buttons. Plain email + password. (Add OAuth later via the `auth/OAuthRow` component when it lands; current is a stub.) |
| Settings | "Export everything" is one click. (`exports` feature.) |
| Settings | "Delete account" is one click + confirm. The user can leave at any time. |
| Profile | "Last sign-in 2h ago, from <region>" surfaces session metadata. |
| File upload | Shows the upload progress + size budget. No "trust me, it's working". |
| Backup / restore | The full export ZIP is downloadable raw — the user can read what it contains. |

What we *don't* do:

- Marketing modals for "did you know we have …?".
- "Rate this app" prompts.
- Cookie banners (the app uses no third-party tracking).
- Email-collection for newsletter.

## Sensitive data

- **Never log PII** (email, name, address) to the browser console —
  see [../coding-standards.md](../coding-standards.md) ("Console policy").
- **Don't show secrets** (access tokens, refresh tokens, CSRF tokens)
  in any UI surface, including dev tools' visible state.
- **Mask credit card numbers** in any future surface that surfaces
  them — the BE doesn't return more than last-4 today; UI masks
  anything stricter.
- **No passwords in clear** — the password field uses
  `frontend/src/components/auth/PasswordInput.tsx` which has a
  show/hide toggle. The "show" reveals the user's own input only.
- **No session-expiry warnings** with countdowns — see
  [14-accessibility.md](14-accessibility.md) (no timed actions).

## Email and external

The app sends emails for:

- Email verification (`auth:verifyEmail`).
- Password reset (`auth:forgotPassword`).
- Group invitations (`groups:invite`).

The UI doesn't compose email content (BE owns the templates), but it
does:

- Tell the user the email was sent ("Check your inbox at user@…").
- Time-stamp the send so re-requests show the elapsed time
  ("Re-send in 30s").
- Surface "I didn't get it; resend" as a secondary action.

## Rate limiting

The BE rate-limits auth + global. The UI surfaces:

- 429 → toast saying "Too many attempts. Try again in <30s>." Use
  `Retry-After` from the response if present.
- Subsequent 429s extend the time without re-toasting.

## Notifications (future)

Push / email notifications for warranty expiry are server-side. UI:

- Settings toggles per channel (email / push) per category.
- A list of recent notifications under `/notifications` (future).

Until those land, the dashboard's "Warranty expiring" widget is the
notification surface.

## Hard rules

1. **One toast per action.** Group bulk; cap at 4 stacked.
2. **Errors don't auto-dismiss.**
3. **Toasts have ≤ 1 CTA.**
4. **Confirm destructive** before, toast post-hoc.
5. **Never log PII to the console.**
6. **No marketing modals.**
7. **No timed warnings.**

## Anti-patterns

- A `vi-mocked` toast assertion that fires on every test ("Login
  successful!"). The login flow shows no toast — see auth pages.
- A success toast on a destructive action ("Item deleted! 🎉"). Use a
  neutral past-tense ("Item deleted") with an Undo.
- A modal that says "We use cookies. Accept?" Inventario uses no
  cookies for tracking — no banner.
- A "Tell us what you think" survey popup. Out of voice.
- A status indicator that shifts focus when a toast appears.
- A toast carrying a "Learn more" link that opens a 2000-word docs
  page. Toasts are momentary.

## Cross-refs

- Voice / copy: [12-tone-of-voice-and-copy.md](12-tone-of-voice-and-copy.md).
- Confirmation dialogs: [09-component-patterns.md](09-component-patterns.md).
- Inline error UX: [15-form-and-data-ux.md](15-form-and-data-ux.md).
- Audit / history (item status): `frontend/src/pages/commodities/`.
- Sonner setup: `frontend/src/components/ui/sonner.tsx`.
- Toast wrapper: `frontend/src/hooks/useAppToast.ts`.

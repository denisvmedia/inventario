# Tone of Voice & Copy

The product's microcopy contract. **Committed.** The voice was set in
[00-positioning.md](00-positioning.md); this doc translates that voice into concrete
strings.

## Anchor

Inventario sounds **considered, honest, quiet, specific, forgiving.**
A representative line, in voice:

> You don't have any items in this area yet. When you do, they'll
> show up here.

What that line gets right:

- **Considered** — the tense is "yet"; it acknowledges the user is
  setting up.
- **Honest** — it doesn't promise auto-population.
- **Quiet** — no exclamation marks, no "hooray".
- **Specific** — "items in this area", not "stuff" or "data".
- **Forgiving** — implicit invitation, no scolding.

What it avoids:

- "Welcome to your empty area! Start adding items."
- "🎉 Your area is empty!"
- "Items: 0"

## Length budgets

| Surface | Target | Cap |
| --- | --- | --- |
| Button label | 1–2 words | 3 |
| Page title | 1–3 words | 5 |
| Section heading | 2–4 words | 6 |
| Empty-state title | 3–5 words | 7 |
| Empty-state body | 1 sentence, ≤ 14 words | 2 sentences |
| Toast | 1 short clause | 1 sentence |
| Tooltip | 1 short clause | 1 sentence |
| Form-field label | 1–2 words | 3 |
| Helper text under a field | 1 short clause | 1 sentence |
| Confirmation dialog body | 1–2 sentences | 3 |

When you need more than the cap, the design is wrong, not the copy
— the user shouldn't be reading paragraphs in a UI surface.

## Action verbs

**Imperative, present tense, second-person implicit.** No "Please" /
"Kindly" / "Could you".

| Action | Label | Not |
| --- | --- | --- |
| Save edits | "Save" | "Save changes", "Submit", "Update" |
| Cancel | "Cancel" | "Close", "Dismiss", "Never mind" |
| Delete | "Delete" | "Remove", "Trash", "Discard" (delete is destructive; remove is reversible) |
| Add new | "Add item", "Add location", … | "Create", "New", "+" alone |
| Confirm a destructive choice | "Yes, delete" / "Yes, leave group" | "OK", "Confirm" |
| Sign in | "Sign in" | "Log in", "Login" |
| Sign out | "Sign out" | "Log out", "Logout" |
| Sign up / register | "Create account" | "Sign up", "Register" |

The verb pair (`Sign in` / `Sign out`) is more common in modern English
than `Log in` / `Log out` and reads more natural. Use them
consistently.

## Naming

| Concept | Use | Don't use |
| --- | --- | --- |
| The thing the user owns | "Item" (UI) / "Commodity" (DB / API) | "Asset", "Property", "Object" |
| The container | "Location" (UI) / "Location" (DB) | "Place", "Site", "Room" |
| The sub-container | "Area" | "Zone", "Section" |
| The user's group | "Group" / "Household" (in plural-user copy) | "Tenant", "Workspace", "Team" |
| Files | "File" (singular), "Files" (plural) | "Attachment", "Document" (those are subcategories) |
| Tags | "Tag" | "Label", "Category" |
| Backup | "Export" / "Backup" — the export *of* the database is a "backup"; the act of producing it is "Export" | — |

The DB-side `commodity` name predates the rewrite and is permanent in
the API; the UI says "item" everywhere. Translation: BE strings are
internal vocabulary, FE strings are user-facing vocabulary.

PR #1362 proposed renaming "Commodity → Thing" / "Location → Place" /
"Export → Backup" in the UI. That proposal is partially adopted — UI
already uses "Item" and "Backup" / "Export" (depending on context).
"Place" wasn't worth the disruption; "Location" stays.

## Tone per surface

### Auth pages

Calm and copy-light. The product positioning ([00-positioning.md](00-positioning.md))
commits to no marketing voice on public surfaces.

- Login title: `Sign in to Inventario`
- Login subtitle: *(none)*
- Forgot-password page title: `Reset your password`
- Forgot-password body: `We'll send you a link to set a new password.`
- Register title: `Create your account`
- Register body: *(none)*

Don't write "Welcome back!" or "Glad to see you!" — the user knows.

### Empty states

Pattern: **"You don't have X yet. When you do, they'll Y."**

- No items: `You don't have any items yet. Add your first one to get started.`
- No locations: `You don't have any locations yet. A location is the top-level container — your home, your office, a vehicle.`
- No tags: `You don't have any tags yet. Tags help you label items across locations.`
- No files on a commodity: `No files attached. Drop one here, or click Browse.`

The pattern leaves room for context-specific phrasing — the no-files
copy is shorter because the user is mid-flow, not landing on the page.

### Loading

Don't write "Loading…" copy — render a skeleton instead (see
[05-motion.md](05-motion.md)). The one place "Loading…" is allowed: a button label
during in-flight submit, but only in addition to the original label
(via the spinner pattern in [08-interaction-states.md](08-interaction-states.md)).

### Errors

Always answer: **what happened, why (if known), what now**.

Patterns:

| Surface | Template |
| --- | --- |
| Field-level | `<rule violated>` (e.g. "Email is required") |
| Form-level | `<what failed>. <what to do>.` (e.g. "Invalid email or password. Try again or reset your password.") |
| Page-level data | `Couldn't load <thing>. <Retry>.` |
| Toast (transient) | Single sentence. Optional retry inside the toast. |
| Catastrophic (500) | `Something went wrong. We've been notified.` |

Don't:

- Apologize repeatedly. One apology per surface.
- Blame the user ("You typed the wrong password"). State the rule
  ("Invalid email or password").
- Write "Oops!" or "Whoops!". The voice is quiet, not cheerful.

### Success

Toasts after a successful mutation:

- `Item added`
- `Location updated`
- `2 files uploaded`
- `Group invitation sent`

No exclamation marks. No "Successfully" prefix. The verb is past
tense; the subject is implicit (the thing the user just acted on).

### Destructive confirmations

`useConfirm()` body — always answer "what's happening, what gets
deleted, can you undo?":

- Title: `Delete this item?`
- Body: `This will remove "Cordless drill" and its files. This can't
  be undone.`
- Confirm button: `Yes, delete`
- Cancel button: `Cancel`

Don't:

- "Are you sure?" alone — answer the implicit follow-up.
- "OK / Cancel" — generic. Use the action verb on the confirm button.

### Multi-tenancy / group

When the user belongs to multiple groups:

- Group switcher label: just the group name.
- "Switching" toast: none. The page reloads — the new context is
  obvious.
- Group invite toast: `You've joined "<group>".`

When the user has no group:

- No-group page title: `You're not in a group yet`
- Body: `Create one to start your inventory, or wait for an invite.`

## i18n

Every string is a translation key. See [13-formatting-and-i18n.md](13-formatting-and-i18n.md).

Key style: `namespace:dot.path`, lower-camelCase segments. The English
bundle is the source of truth; cs/ru fall back to en for missing keys
(per [../i18n.md](../i18n.md)).

Translators read context from the key path itself — keep keys
descriptive (`commodities:list.empty.title` over `commodities:e1`).

## Punctuation

- **Periods** at the end of full sentences. Not after standalone
  phrases / titles / button labels.
- **Em-dashes** (`—`, U+2014) for asides; never `--` in source. Word
  it out instead if your editor doesn't make `—` easy.
- **No exclamation marks.** Anywhere. The product positioning bans
  enthusiasm.
- **Quoted item names** use straight double quotes (`"`), not
  curly. ("Cordless drill", not "Cordless drill".)
- **Ellipsis** (`…`, U+2026) — not `...`. Used sparingly: "Loading…"
  inside a button, never as a sentence cue.

## Numbers in copy

- One-digit counts: spell out only in copy ("one item"). Use digits
  in titles / metrics.
- Plurals via i18next — `count: N` pluralizes via `_one` / `_other`.
  See [../i18n.md](../i18n.md).

## Hard rules

1. **No exclamation marks** outside literal exception cases (none
   identified yet).
2. **Imperative verbs**, present tense.
3. **Specific over generic** — "Add item" beats "Add", "Couldn't
   load items" beats "Error".
4. **Sentence-case headings**, not Title Case. Page titles like
   "Items" or "Forgot password" — capitalize the first word and
   proper nouns only.
5. **No "Please".** It reads obsequious in product UI.
6. **No "we're sorry to inform you".** Two words: "Couldn't load."

## Anti-patterns

- "Welcome back, <name>!" on the dashboard. The user knows.
- "Boom! 🎉 Item added." Use "Item added".
- "Hmm, something went wrong." Use "Couldn't save. Try again."
- "Click here to learn more." Linkify the noun, not the verb.
- A confirmation dialog with body "Are you sure?". Answer the
  question.

## Cross-refs

- Voice anchor: [00-positioning.md](00-positioning.md).
- Plurals / numbers / dates: [13-formatting-and-i18n.md](13-formatting-and-i18n.md).
- Toast hierarchy: [16-notifications-and-trust.md](16-notifications-and-trust.md).
- The i18n key contract: [../i18n.md](../i18n.md).

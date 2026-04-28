# Tone of Voice & Copy

The product's personality is half visual, half written. This document is the writing brief.

## Voice in one paragraph

Inventario sounds like a thoughtful, slightly understated friend who keeps your records straight. It speaks plainly, doesn't perform enthusiasm, and respects the user's time. It is occasionally warm — when something matters or when reassurance helps — but never cute. It treats your possessions as **yours**, with the dignity that implies.

## Tone characteristics

| Trait | Expression |
| --- | --- |
| **Plain** | Simple words. Short sentences. No jargon. |
| **Quiet** | One exclamation mark per app at most. No emoji. No marketing voice. |
| **Practical** | What this is, what to do next. No flourish. |
| **Warm but reserved** | Friendly without being your buddy. |
| **Trustworthy** | Honest about what we know and don't know. No fake numbers, no fake urgency. |
| **Owner-respectful** | "Your things" not "the entries". User's possessions matter. |

## Vocabulary

### Naming the entities

| Current | Recommended | Why |
| --- | --- | --- |
| Commodity | Thing (UI) / Item (formal contexts) | "Commodity" is finance/legalese, alien to a personal tool |
| Location | Place | Plainer, friendlier |
| Area | Area | Keep — it's already plain |
| Export | Backup (when about full data) / Export (when about format conversion) | "Backup" matches user mental model |
| File | File | Keep |
| Tag | Tag | Keep |
| Tenant / Group | Workspace | "Group" is too generic; "Workspace" is industry-standard for this concept |

### Words to use
- own, keep, remember, store, save, find, add, organize, place, thing, file, backup, photo, document, manual, receipt, warranty, expire, value, price, where, when, who

### Words to avoid
- entity, record, entry, item record, asset, data, system, database, repository, manage, configure, parameter, attribute (in user-facing strings — internal code can use these freely)
- delightful, awesome, amazing, simply, easy, just (filler)
- "Successfully" / "Failed to" prefixes (state the outcome directly)
- All-caps headers (we're not Mailchimp)

## Sentence patterns

### Do
- "Your dishwasher's warranty expires in 14 days."
- "Saved."
- "Couldn't open that file. Try downloading instead."
- "Add a photo so you remember what it looks like."

### Don't
- "Successfully saved your record! 🎉"
- "Oops! We had a tiny problem 😅"
- "Please be advised that the operation has been completed successfully."
- "Click here to add a new commodity entry to the database."

## Microcopy templates

### Buttons

| Action | Label |
| --- | --- |
| Primary save | Save / Save changes |
| Cancel | Cancel |
| Delete | Delete |
| Confirm destructive | Delete it / Remove it / Archive it |
| Add | Add a thing / Add a place / Add a file (specific noun preferred) |
| Generic continue | Continue |
| Try again after error | Try again |
| Dismiss | Got it |
| Skip | Skip |
| Close | Close |

Avoid: "Submit", "OK", "Confirm" without a noun, "Yes" / "No" alone.

### Form labels

| Concept | Label |
| --- | --- |
| Name | Name |
| Description | Description (optional) |
| Tags | Tags |
| Where it lives | Where it lives (not "Location ID") |
| Purchase date | When you bought it |
| Original price | What it cost |
| Current value | What it's worth now (if known) |
| Serial number | Serial number |
| Warranty until | Warranty until |
| Notes | Notes — anything you want to remember |

### Helper text

- Below a tag field: "Tags help you find things later. Try 'kitchen', 'gift', 'electronics'."
- Below price: "We use this for your insurance summary."
- Below warranty: "We'll remind you before it expires."
- Below photo upload: "A photo or two makes it much easier to remember."

### Error messages

Pattern: `[what didn't happen]. [why or what to do.]`

| Scenario | Message |
| --- | --- |
| Network failure | Couldn't reach the server. Check your connection and try again. |
| Validation: required | This one is required. |
| Validation: too long | Up to 100 characters, please. |
| Validation: format (email) | Doesn't look like an email address. |
| Validation: format (date) | Use a real date, like Apr 12, 2026. |
| Conflict (someone else changed) | Someone else updated this while you were editing. Reload to see the latest. |
| Permissions | You don't have access to this. |
| Not found | We couldn't find that. It may have been deleted. |
| Quota exceeded | You've used all your storage. Free up space or upgrade to add more. |
| Server error | Something on our end broke. Try again in a moment. |

### Success / confirmation

Direct, past tense, no exclamation:

| Action | Message |
| --- | --- |
| Saved | Saved. |
| Created | Added. |
| Deleted | Deleted. (Show "Undo" for 5s when reversible.) |
| Uploaded | Uploaded. |
| Downloaded | Downloaded. |
| Copied to clipboard | Copied. |
| Backup complete | Backup ready. [Download] |

### Empty states

Per surface, in `11-page-layouts-and-flows.md`. Pattern:
- Title: matter-of-fact, present tense ("Nothing here yet.")
- Description: explanation + invitation, max 2 sentences
- CTA: specific noun ("Add a thing", not "Get started")

### Confirmation dialogs

Pattern: `[Action] "[name]"?`

- Title: "Delete \"Camping Equipment\"?" — the name in quotes makes it concrete
- Body: explicit consequences ("This will remove the item, its 5 images, and 3 manuals. This cannot be undone.")
- Buttons: action-as-verb ("Delete it") not generic ("Confirm")

### Toast copy by action

| Action | Toast |
| --- | --- |
| Created thing | Added "[name]". |
| Edited thing | Saved. |
| Deleted thing | Deleted "[name]". · Undo |
| Uploaded files | Uploaded 3 files. |
| Failed to save | Couldn't save. Try again. |
| Network back online | Back online. |

Always quote the name when one item; pluralize cleanly when many ("Deleted 5 things").

### Date / time language (in copy, not formatting)

| Time delta | Display |
| --- | --- |
| < 1 min | Just now |
| 1–59 min | 12 minutes ago |
| 1–23 hr | 3 hours ago |
| Yesterday | Yesterday at 4:12 PM |
| 2–6 days | Tuesday at 4:12 PM |
| Same calendar year | Apr 12 at 4:12 PM |
| Older | Apr 12, 2024 |

(Formatting per `13-formatting-and-i18n.md`.)

## Onboarding language

The first 5 minutes are the most important text in the product. Per `11-page-layouts-and-flows.md`:

### Welcome screen

> **A quiet place for the things you own.**
> Receipts, warranties, where you bought it — all in one place, easy to find when you need them.
>
> What's the first thing you'd like to remember?

- Hero feels personal but doesn't promise miracles
- Categories framed as user goals, not feature lists

### First thing prompt

> **Let's add your first thing.**
> Anything works — your washing machine, a guitar, the toolbox on the shelf.

Examples in placeholders show range: "Dishwasher, sofa, your guitar". This signals the product handles diversity.

### Tour slides

Each slide: 1 sentence + 1 supporting clause. Total reading time per slide: <8 seconds.

## Reassurance moments

Inventario keeps records about *valuable* things (insurance, warranties). Reassurance copy belongs in:

- After backup completes: "Your records are safe, with you and a copy ready to download."
- On the security/profile page: "Your data is encrypted at rest and in transit."
- After deletion confirmation: "[name] is gone. We don't keep deleted items in shadow storage."

These build trust without lecturing.

## Brand voice no-go list

| ❌ Don't write | ✅ Write |
| --- | --- |
| "Successfully saved!" | "Saved." |
| "Oops, something went wrong 😅" | "Couldn't save that." |
| "Awesome! 🎉 Your dishwasher has been added!" | "Added." |
| "Easy! Just click here." | "Click to add one." |
| "Are you sure you want to delete this commodity?" | "Delete \"Dishwasher\"?" |
| "We will remind you" | "We'll remind you" |
| "It seems that the action could not be performed at this time" | "Couldn't do that. Try again?" |
| "Please be advised" | (delete entirely) |
| "Pro tip:" | (delete entirely) |
| "Welcome to the inventory management system!" | "Welcome back, [name]." |

## Internationalization

Translation considerations baked into the source copy:

- **Avoid puns and idioms** in user-facing strings. They don't translate.
- **Avoid embedded variables in mid-sentence** when possible:
  - ❌ "You added \{name\} on \{date\}."
  - ✅ "Added on \{date\}: \{name\}." (cleaner translation, names always last)
- **Avoid count-dependent grammar**: not "1 thing" / "2 things" hard-coded — use ICU MessageFormat (per `13-formatting-and-i18n.md`)
- **Plan for 30% expansion** — Russian is longer, German is longer, French is longer. Don't lock UI widths to English copy.

## Accessibility-aware copy

- Always provide a non-icon-dependent label ("Edit", not just a pencil icon)
- Error messages in `<output role="alert">` — announced to screen readers
- Loading states announce status: `aria-busy="true"`
- Status pill text is the source of truth — color is supplementary

## Style guide enforcement

- Lint user-facing strings: forbid emojis, "successfully", "failed to", "please", "click here"
- Spelling/grammar via Vale or LanguageTool in CI for `i18n/en.json`
- Reviewer checklist: tone-of-voice line in PR template

## Authorship

This brief is the canonical source for tone. When in doubt, re-read the "Voice in one paragraph" section above. New copy gets reviewed against:

1. Is it plain enough?
2. Is it specific (named) where it can be?
3. Does it tell the user what to do next?
4. Does it sound like a friend, not a corporation?
5. Would I be embarrassed reading this aloud?

If any answer is weak, rewrite.

import { screen, waitFor, waitForElementToBeRemoved, within } from "@testing-library/react"
import type userEvent from "@testing-library/user-event"

// Test helpers for Radix UI primitives in JSDOM. Radix's listbox/menu
// surfaces render into a portal once a pointer/keyboard interaction
// opens the trigger — `user.selectOptions()` from
// `@testing-library/user-event` doesn't apply (there is no
// `HTMLSelectElement`), and naive `user.click(trigger)` racing into a
// stale listbox left over from a sibling Select is the most common
// flake source. Centralise the open/pick/close sequence here so
// every test reads as "drive Select X to value Y" rather than
// re-deriving the dance per case.

type User = ReturnType<typeof userEvent.setup>

interface PickOptions {
  // Match the option text. Use a RegExp to anchor against the
  // accessible name; Radix `SelectItem` renders `<svg aria-hidden>`
  // glyphs alongside the label, but those are excluded from the
  // accessible name so a plain `/^Furniture$/i` matches.
  optionLabel: RegExp
}

// pickRadixSelect drives a shadcn/Radix `Select` to a specific option
// by accessible name. Use it instead of `user.selectOptions()` — the
// Radix trigger renders a `role="combobox"` button, not an
// `HTMLSelectElement`, so `selectOptions` silently no-ops.
//
// The helper:
//   1. Resolves the trigger by accessible name (the `aria-label` on
//      `SelectTrigger` or the `<label htmlFor>` associated with its
//      `id`).
//   2. Asserts the trigger isn't `disabled` — Radix Select disabled
//      triggers eat both pointer and Enter; a silent no-op there is
//      the most common reason a follow-up `findByRole("listbox")`
//      returns a sibling listbox from earlier in the test.
//   3. Opens the listbox with a real click (Radix listens for
//      `onPointerDown` + `onClick`; `userEvent.click` issues both,
//      which is what production users do).
//   4. Anchors on the trigger's own `aria-expanded="true"` /
//      `data-state="open"` to confirm THIS Select opened — without
//      the anchor a stale listbox from a sibling Select left mid-
//      cleanup would be returned by `findByRole("listbox")` and the
//      pick would silently land on the wrong field.
//   5. Picks the option inside the freshly-mounted listbox.
//   6. Waits for the listbox to unmount so the next pick is
//      unambiguous — Radix portals SelectContent and unmounts it on
//      close, so a stale listbox is impossible once this resolves.
//
// Example:
//   await pickRadixSelect(user, /^Type$/i, { optionLabel: /^Furniture$/i })
//   await pickRadixSelect(user, /^Area$/i, { optionLabel: /^Garage$/i })
export async function pickRadixSelect(
  user: User,
  triggerLabel: RegExp | string,
  { optionLabel }: PickOptions
): Promise<void> {
  const trigger = await screen.findByRole("combobox", { name: triggerLabel })
  if (trigger.hasAttribute("disabled") || trigger.getAttribute("aria-disabled") === "true") {
    throw new Error(
      `pickRadixSelect: trigger ${describe(triggerLabel)} is disabled — pick its dependency Select first`
    )
  }
  await user.click(trigger)
  // Anchor on THIS trigger's open state before querying the listbox
  // — disambiguates from a sibling Select's listbox that may still
  // be mid-unmount on the same tick.
  await waitFor(() => {
    if (trigger.getAttribute("aria-expanded") !== "true") {
      throw new Error(`pickRadixSelect: trigger ${describe(triggerLabel)} did not open`)
    }
  })
  const listbox = await screen.findByRole("listbox")
  const option = await within(listbox).findByRole("option", { name: optionLabel })
  await user.click(option)
  await waitForElementToBeRemoved(listbox).catch(async () => {
    // Radix occasionally keeps the listbox node attached for a tick
    // after close; fall back to a state-driven wait so flakes from
    // the unmount race don't surface.
    await waitFor(() => {
      const open = document.querySelector('[role="listbox"][data-state="open"]')
      if (open) throw new Error("listbox still open")
    })
  })
}

function describe(label: RegExp | string): string {
  return label instanceof RegExp ? label.toString() : JSON.stringify(label)
}

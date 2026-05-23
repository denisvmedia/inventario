import { expect, Page } from "@playwright/test";
import { TestRecorder } from "../../utils/test-recorder.js";

// React port of the Vue-era commodities helper. Post cutover #1423:
//
// - Trigger lives at `[data-testid="commodities-add-button"]` on the
//   /commodities list page (not on the area detail page — area detail is a
//   ComingSoon stub today).
// - The form is a multi-step Dialog (`[data-testid="commodity-form-dialog"]`)
//   with a leading inert "AI" step (PR #1621 / #1540) followed by five
//   real steps: basics → purchase → warranty → extras → files. Step
//   navigation goes through `[data-testid="commodity-form-next"]`;
//   submit is `[data-testid="commodity-form-submit"]`.
// - Type / area / status inputs are Radix Select primitives (#1621
//   migration away from native `<select>` so the dialog can render
//   them inside Sheet portals without browser-default styling
//   collisions). `selectByPartialOptionText` drives them by clicking
//   the trigger, then clicking the portalled option.
// - Currency is a CurrencyCombobox (Popover + cmdk Command, #1621).
//   `pickCurrency` opens it and clicks the `[data-currency-code]`
//   item that matches the ISO code.
// - Tags / extra-serials / part-numbers / urls are ChipInputs — type +
//   Enter to add a chip; chips render as `[data-testid="<id>-chip"]`.

export interface TestCommodity {
  name: string;
  shortName: string;
  /** Either the enum value ("electronics") or the localized label
   *  prefix ("Electronics") — the helper resolves to the matching
   *  <option> via partial text match. */
  type: string;
  count: number;
  originalPrice: number;
  /** ISO-4217 code, e.g. "USD". The form input is a 3-char text. */
  originalPriceCurrency: string;
  /** Optional explicit converted-price override. Defaults to
   *  `originalPrice` when not provided — the schema's
   *  `superRefine` block requires a non-empty value for non-draft
   *  commodities, but tests rarely care about FX nuance and just
   *  need *something* to satisfy the gate. */
  convertedOriginalPrice?: number;
  /** Optional explicit current-price override. Same default as
   *  `convertedOriginalPrice`. */
  currentPrice?: number;
  /** ISO date "YYYY-MM-DD". */
  purchaseDate: string;
  serialNumber?: string;
  extraSerialNumbers?: string[];
  partNumbers?: string[];
  tags?: string[];
  urls?: string[];
  /** Area to bind the commodity to. The dialog defaults `area_id` to
   *  the empty string (see `CommodityFormDialog.buildDefaults`) and
   *  the Area select stays disabled until a Location is picked, so the
   *  Basics → Purchase step transition fails validation when this is
   *  omitted. Practically required for any helper call that walks the
   *  full wizard; left optional for the rare API-only path that might
   *  add a commodity-creation surface without Area binding later. */
  areaName?: string;
  /** Optional location to bind the commodity to. PR #1621 split the
   *  Basics step into a Location picker + an Area picker gated on
   *  the chosen location — the Area select is `disabled` until a
   *  location is picked. When omitted, `fillBasicsStep` auto-picks
   *  the first option from the Location combobox, which keeps every
   *  test that only ever created one location working without
   *  per-test-suite churn. */
  locationName?: string;
  /** Optional warranty expiry date (YYYY-MM-DD). Skipping leaves the
   *  field blank, which the helper treats as "no warranty tracked"
   *  (no live status pill, no reminder rows). */
  warrantyExpiresAt?: string;
  /** Optional free-form warranty notes. */
  warrantyNotes?: string;
  [key: string]: unknown;
}

async function selectByPartialOptionText(
  page: Page,
  selectId: string,
  fragment: string,
) {
  // Radix Select migration (#1621): Type / Area / Status are no
  // longer native `<select>` elements — they're Radix triggers
  // (a `<button role="combobox">` carrying our testid) that open
  // a portalled listbox on click. Items inside the listbox carry
  // `role="option"`. Drive them by clicking the trigger to open,
  // then clicking the option whose visible text contains
  // `fragment` (matches the same partial-text contract the native
  // version had — type labels still carry emoji prefixes like
  // "💻 Electronics", `hasText` matches anywhere in the option).
  // The Radix `<SelectTrigger>` keeps the same `id=...` attribute the
  // old native `<select>` carried — see e.g. SelectTrigger id="commodity-type"
  // in `CommodityFormDialog.tsx`. That keeps `<FieldLabel htmlFor>` working
  // and lets this helper stay agnostic of the migration.
  const trigger = page.locator(`#${selectId}`);
  await trigger.click();
  // Radix renders the listbox in a portal under `<body>`, not as a
  // child of the trigger — query globally and wait for it to mount.
  // There is only ever one open Radix listbox at a time in the
  // dialog flow, so the page-level `role="listbox"` lookup is safe.
  const listbox = page.getByRole("listbox");
  await listbox.waitFor({ state: "visible", timeout: 5000 });
  const option = listbox.getByRole("option", {
    name: new RegExp(fragment, "i"),
  });
  await option.first().click();
  // Radix dismisses the portal on select; wait for the listbox to
  // detach so the next action doesn't race the close animation.
  await listbox.waitFor({ state: "detached", timeout: 5000 });
}

async function fillChip(page: Page, testId: string, value: string) {
  const input = page.locator(`[data-testid="${testId}-input"]`);
  await input.fill(value);
  await input.press("Enter");
}

async function fillChips(page: Page, testId: string, values: string[]) {
  for (const v of values) {
    await fillChip(page, testId, v);
  }
}

async function clearChips(page: Page, testId: string) {
  // Each chip's X button has aria-label="remove <value>". We click them
  // back-to-front via the count to avoid index shift after each removal.
  while (true) {
    const chip = page.locator(`[data-testid="${testId}-chip"]`).first();
    if ((await chip.count()) === 0) return;
    await chip.locator("button").click();
  }
}

async function gotoNext(page: Page) {
  await page.click('[data-testid="commodity-form-next"]');
}

// waitForStep blocks until the named step's container has mounted —
// each step renders `<div data-testid="commodity-form-${step}-step">`
// only while it is the current step (see CommodityFormDialog.tsx
// `step === "..." ? <...Step .../> : null`). The form's nextStep()
// silently returns when step-level `trigger()` fails, so without an
// explicit step-marker wait the next field-action would otherwise
// hang for the full Playwright test timeout. With it, we fail fast
// at the actual transition that didn't happen.
async function waitForStep(
  page: Page,
  step: "basics" | "purchase" | "warranty" | "extras" | "files",
) {
  await page.waitForSelector(`[data-testid="commodity-form-${step}-step"]`, {
    state: "visible",
    timeout: 10000,
  });
}

async function fillBasicsStep(page: Page, c: TestCommodity) {
  await page.fill("#commodity-name", c.name);
  await page.fill("#commodity-short-name", c.shortName);
  await page.fill("#commodity-count", String(c.count));
  if (c.type) {
    await selectByPartialOptionText(page, "commodity-type", c.type);
  }
  // PR #1621 split Location/Area: the Area select is `disabled` until
  // a Location is chosen, so picking an area without first picking a
  // location stalls the helper on a permanently-not-enabled trigger.
  // When the caller supplied `locationName`, drive the Location
  // combobox by name. Otherwise auto-pick the first available option
  // — every existing test only ever creates one location for its own
  // fixture, so the first option IS the location they want.
  if (c.areaName) {
    if (c.locationName) {
      await selectByPartialOptionText(
        page,
        "commodity-location",
        c.locationName,
      );
    } else {
      await pickFirstSelectOption(page, "commodity-location");
    }
    await selectByPartialOptionText(page, "commodity-area", c.areaName);
  }
  // PR #1621 moved Product URLs onto the Basics step (a UrlList of
  // single-input rows, NOT a ChipInput). Each row gets
  // `data-testid="commodity-urls-row-N"`. The first row is always
  // visible (phantom) even when values is empty; clicking the "+ Add"
  // button promotes it and appends another empty row.
  if (c.urls && c.urls.length) {
    for (let idx = 0; idx < c.urls.length; idx++) {
      if (idx > 0) {
        await page.click('[data-testid="commodity-urls-add"]');
      }
      await page.fill(`[data-testid="commodity-urls-row-${idx}"]`, c.urls[idx]);
    }
  }
}

async function pickFirstSelectOption(page: Page, selectId: string) {
  // Same Radix open-then-click flow as selectByPartialOptionText, but
  // we pick whatever option happens to be first in the listbox. Used
  // when the test doesn't care about the specific value (e.g. the
  // Location field on a single-location fixture).
  const trigger = page.locator(`#${selectId}`);
  await trigger.click();
  const listbox = page.getByRole("listbox");
  await listbox.waitFor({ state: "visible", timeout: 5000 });
  await listbox.getByRole("option").first().click();
  await listbox.waitFor({ state: "detached", timeout: 5000 });
}

async function fillPurchaseStep(page: Page, c: TestCommodity) {
  if (c.purchaseDate)
    await page.fill("#commodity-purchase-date", c.purchaseDate);

  // PR #1621 made `#commodity-currency` a CurrencyCombobox (Popover +
  // cmdk), not a text input — drive it via the searchable list when
  // the test asks for a currency. Always set this BEFORE the prices
  // so the form's `isForeignCurrency` flag is in its final state by
  // the time we look for the converted-price field below.
  if (c.originalPriceCurrency) {
    await pickCurrency(page, "commodity-currency", c.originalPriceCurrency);
  }

  if (c.originalPrice !== undefined) {
    await page.fill("#commodity-original-price", String(c.originalPrice));
    // PR #1621: when `original_price_currency === group_currency` the
    // Purchase step now hides `#commodity-converted-price` entirely
    // (mock AddItemDialog L1198-L1233 — same-currency drops the field
    // because the BE rule already forces it to 0). The form's
    // `toRequest` mirror writes 0 on save, so the helper has nothing
    // to do in that branch. The foreign-currency variant still renders
    // the input and we honor the `convertedOriginalPrice` override.
    if ((await page.locator("#commodity-converted-price").count()) > 0) {
      const converted = c.convertedOriginalPrice ?? 0;
      await page.fill("#commodity-converted-price", String(converted));
    }
    const current = c.currentPrice ?? c.originalPrice;
    await page.fill("#commodity-current-price", String(current));
  }

  if (c.serialNumber !== undefined) {
    await page.fill("#commodity-serial", c.serialNumber);
  }
}

async function pickCurrency(page: Page, triggerId: string, code: string) {
  // CurrencyCombobox renders a Popover whose trigger button carries
  // the requested `id`. Each option has `data-currency-code="XXX"`,
  // so we can match the ISO code directly without text-matching the
  // localised currency name. Skip-when-already-selected keeps the
  // helper idempotent for tests that re-use a fixture whose default
  // currency already matches the bootstrap group currency.
  const trigger = page.locator(`#${triggerId}`);
  const currentLabel =
    (await trigger.textContent())?.trim().toUpperCase() ?? "";
  if (currentLabel.startsWith(code.toUpperCase())) return;
  await trigger.click();
  const option = page.locator(`[data-currency-code="${code.toUpperCase()}"]`);
  await option.first().click({ timeout: 5000 });
  // cmdk dismisses the popover on select; wait for the option to
  // detach so the next field-action doesn't race the close.
  await option.first().waitFor({ state: "detached", timeout: 5000 });
}

async function fillWarrantyStep(page: Page, c: TestCommodity) {
  // Both warranty inputs are optional. The form's superRefine block
  // doesn't gate them, so omitting both is fine — the dialog just
  // saves the commodity without a tracked warranty.
  if (c.warrantyExpiresAt !== undefined) {
    await page.fill("#commodity-warranty-expires-at", c.warrantyExpiresAt);
  }
  if (c.warrantyNotes !== undefined) {
    await page.fill("#commodity-warranty-notes", c.warrantyNotes);
  }
}

async function fillExtrasStep(
  page: Page,
  c: TestCommodity,
  replaceArrays = false,
) {
  // PR #1621 moved URLs from Extras to Basics step; the test still
  // carries `urls`, but the Extras step itself has none. Skip the
  // `commodity-urls` chip path entirely from this helper.
  // (Drop the URL fill here — the field doesn't exist on this step
  // anymore, so the call would hang on a missing locator.)
  if (replaceArrays) {
    if (c.tags !== undefined) await clearChips(page, "commodity-tags");
    if (c.extraSerialNumbers !== undefined)
      await clearChips(page, "commodity-extra-serials");
    if (c.partNumbers !== undefined)
      await clearChips(page, "commodity-part-numbers");
  }
  // Reveal toggles: extra serials + part numbers are hidden behind a
  // disclosure button by default (#1621). Click the toggle BEFORE
  // calling fillChips, otherwise the chip input doesn't exist in the
  // DOM and we'd hit a 2-minute locator timeout.
  if (c.tags && c.tags.length) {
    await fillChips(page, "commodity-tags", c.tags);
    // The TagsInput popover (autocomplete dropdown) stays open after
    // commit and can overlap the reveal buttons below it. Press
    // Escape to close the dropdown before moving on.
    await page.keyboard.press("Escape");
  }
  if (c.extraSerialNumbers && c.extraSerialNumbers.length) {
    await revealAndFillChips(
      page,
      "commodity-extra-serials",
      c.extraSerialNumbers,
    );
  }
  if (c.partNumbers && c.partNumbers.length) {
    await revealAndFillChips(page, "commodity-part-numbers", c.partNumbers);
  }
}

// revealAndFillChips clicks the "Add part numbers" / "Add extra serial
// numbers" disclosure toggle (data-testid="${id}-reveal") if it's
// present, then fills the chip input. If the toggle is missing — edit
// mode auto-reveals when the field already has values — we just go
// straight to the chip input.
async function revealAndFillChips(
  page: Page,
  testId: string,
  values: string[],
) {
  const reveal = page.locator(`[data-testid="${testId}-reveal"]`);
  if ((await reveal.count()) > 0) {
    await reveal.click({ timeout: 2000 });
  }
  await fillChips(page, testId, values);
}

export async function createCommodity(
  page: Page,
  recorder: TestRecorder,
  testCommodity: TestCommodity,
): Promise<string> {
  await recorder.takeScreenshot("commodities-create-01-before-create");

  await page.click('[data-testid="commodities-add-button"]');
  await page.waitForSelector('[data-testid="commodity-form-dialog"]');

  // Step 0: AI scan (#1720 / PR #1835). Create mode opens on the
  // AI scan surface before Basics. The AI step owns its own footer
  // (no `commodity-form-next` button) — "Fill manually" carries the
  // distinct testid `commodity-form-ai-fill-manually` and advances
  // to Basics without a scan. Edit mode skips this step entirely so
  // `editCommodity` doesn't need this hop. Wait on the AI-step
  // marker before clicking to make sure we're not racing the
  // dialog's open animation.
  await page.waitForSelector('[data-testid="commodity-form-ai-step"]', {
    state: "visible",
    timeout: 5000,
  });
  await page.click('[data-testid="commodity-form-ai-fill-manually"]');

  // Step 1: Basics. Wait for the step marker before filling so a
  // skipped/blocked transition (e.g. a step-level `trigger()` returned
  // false and `nextStep` early-returned) fails fast with a clear
  // marker-missing error instead of a 2-minute field-locator timeout.
  await waitForStep(page, "basics");
  await fillBasicsStep(page, testCommodity);
  await recorder.takeScreenshot("commodity-create-02-basics");
  await gotoNext(page);

  // Step 2: Purchase.
  await waitForStep(page, "purchase");
  await fillPurchaseStep(page, testCommodity);
  await recorder.takeScreenshot("commodity-create-03-purchase");
  await gotoNext(page);

  // Step 3: Warranty (#1367) — optional, skip when no warranty fields
  // were passed.
  await waitForStep(page, "warranty");
  await fillWarrantyStep(page, testCommodity);
  await recorder.takeScreenshot("commodity-create-04-warranty");
  await gotoNext(page);

  // Step 4: Extras (chip inputs).
  await waitForStep(page, "extras");
  await fillExtrasStep(page, testCommodity);
  await recorder.takeScreenshot("commodity-create-04-extras");
  await gotoNext(page);

  // Step 5: Files (ComingSoon stub) → Submit.
  //
  // WebKit-specific quirk: pressing Enter inside a ChipInput field on
  // the previous (Extras) step sometimes leaks past React's
  // `e.preventDefault()` and submits the form before our `gotoNext()`
  // above advances to step 5. By the time we reach this line the
  // dialog is already gone and the page is on the detail URL.
  //
  // Detect that case BEFORE waiting on the step-5 marker — the marker
  // only renders inside FilesStep, so the wait would time out forever
  // once the dialog has unmounted. Both the early-submit branch and
  // the normal branch fall through to the same `waitForURL` below.
  const detailUrlRe = /\/commodities\/[0-9a-fA-F-]{36}/;
  const earlySubmitted = detailUrlRe.test(new URL(page.url()).pathname);
  if (!earlySubmitted) {
    // Race the FilesStep marker against URL navigation: if Enter
    // leaks during this very wait window (rare but observed), the
    // URL flips before the marker ever renders. waitForURL resolves
    // first in that case and we drop straight into the requestSubmit
    // branch below, which sees the form is already gone.
    await Promise.race([
      page.waitForSelector('[data-testid="commodity-form-files-step"]', {
        state: "visible",
        timeout: 10000,
      }),
      page.waitForURL(detailUrlRe, { timeout: 10000 }),
    ]);

    const stillOnDialog = !detailUrlRe.test(new URL(page.url()).pathname);
    if (stillOnDialog) {
      // Click the submit button. PR #1621 made the form's onSubmit
      // an unconditional preventDefault (to block Enter-triggered
      // implicit submits during multi-step navigation), so
      // `form.requestSubmit()` is a no-op — only an explicit click
      // on the submit button routes through RHF's `handleSubmit`.
      await page.click('[data-testid="commodity-form-submit"]');
    }
    await page.waitForURL(detailUrlRe, { timeout: 30000 });
  }
  await page.waitForSelector('[data-testid="page-commodity-detail"]');
  await page.waitForLoadState("networkidle");
  await recorder.takeScreenshot("commodity-create-05-created");

  return page.url();
}

export async function verifyCommodityDetails(
  page: Page,
  testCommodity: TestCommodity,
) {
  const detail = page.locator('[data-testid="page-commodity-detail"]');
  await detail.waitFor({ state: "visible", timeout: 10000 });

  await expect(detail.locator("h1")).toContainText(testCommodity.name);
  await expect(
    detail.locator('[data-testid="commodity-detail-short-name"]'),
  ).toContainText(testCommodity.shortName);

  // The Details Card hosts count, prices, serial, tags, urls, extras, etc.
  // Without granular per-row testids the highest-signal check is "the
  // value's text appears anywhere inside the card."
  const details = detail.locator('[data-testid="commodity-detail-details"]');
  await details.waitFor({ state: "visible", timeout: 10000 });
  await expect(details).toContainText(String(testCommodity.count));
  await expect(details).toContainText(String(testCommodity.originalPrice));

  if (testCommodity.serialNumber) {
    await expect(details).toContainText(testCommodity.serialNumber);
  }
  if (
    testCommodity.extraSerialNumbers &&
    testCommodity.extraSerialNumbers.length
  ) {
    for (const s of testCommodity.extraSerialNumbers) {
      await expect(details).toContainText(s);
    }
  }
  if (testCommodity.partNumbers && testCommodity.partNumbers.length) {
    for (const p of testCommodity.partNumbers) {
      await expect(details).toContainText(p);
    }
  }
  if (testCommodity.tags && testCommodity.tags.length) {
    for (const t of testCommodity.tags) {
      await expect(details).toContainText(t);
    }
  }
  if (testCommodity.urls && testCommodity.urls.length) {
    for (const u of testCommodity.urls) {
      await expect(details).toContainText(u);
    }
  }
}

export async function editCommodity(
  page: Page,
  recorder: TestRecorder,
  updatedCommodity: TestCommodity,
  buttonSelector?: string | boolean,
) {
  if (buttonSelector !== false) {
    if (typeof buttonSelector === "string" && buttonSelector.length > 0) {
      await page.click(buttonSelector);
    } else {
      await page.click('[data-testid="commodity-detail-edit"]');
    }
  }

  await page.waitForSelector('[data-testid="commodity-form-dialog"]');
  await recorder.takeScreenshot("commodity-edit-01-edit-form");

  // Edit mode skips the AI step entirely (it's create-only), so the
  // dialog opens directly on Basics. Wait on the marker anyway —
  // future-proofs against any timing race with the open animation.
  await waitForStep(page, "basics");
  await fillBasicsStep(page, updatedCommodity);
  await gotoNext(page);

  // Step 2: Purchase.
  await waitForStep(page, "purchase");
  await fillPurchaseStep(page, updatedCommodity);
  await gotoNext(page);

  // Step 3: Warranty stub.
  await waitForStep(page, "warranty");
  await gotoNext(page);

  // Step 4: Extras — replace existing chips with the updated values.
  await waitForStep(page, "extras");
  await fillExtrasStep(page, updatedCommodity, /*replaceArrays*/ true);
  await recorder.takeScreenshot("commodity-edit-02-edit-form-filled");
  await gotoNext(page);

  // Step 5: Files stub → Submit. The form's `onSubmit` unconditionally
  // calls preventDefault (PR #1621 — blocks Enter-triggered implicit
  // submits during multi-step navigation), so the actual submission
  // path is `onClick={() => handleSubmit(submit)()}` on the submit
  // button. `form.requestSubmit()` is a no-op there.
  await page.waitForSelector('[data-testid="commodity-form-files-step"]', {
    state: "visible",
    timeout: 5000,
  });
  await page.click('[data-testid="commodity-form-submit"]');
  // Wait for the form dialog to actually leave the DOM before the caller
  // continues. The cached detail query revalidates against the new row
  // only after the submit-then-close chain (mutation → setOpen(false) →
  // Radix transition) settles. Webkit-on-macOS schedules that chain
  // slower than chromium/firefox; a stale `commodity-form` in the DOM
  // when the next assertion runs is exactly the symptom #1591 logged
  // ("commodity-form not in DOM at edit-submit time" on a follow-up
  // action) and the same shape as the #1705 webkit hang.
  await page
    .locator('[data-testid="commodity-form-dialog"]')
    .waitFor({ state: "hidden", timeout: 30000 });
  // Stay on the detail page (no navigate after edit; the form just
  // closes the dialog and revalidates the cached detail query).
  await expect(page).toHaveURL(/\/commodities\/[0-9a-fA-F-]{36}(\?.*)?$/);
  await page.waitForLoadState("networkidle");
  await expect(page.locator("h1")).toContainText(updatedCommodity.name, {
    timeout: 10000,
  });
  await recorder.takeScreenshot("commodity-edit-03-after-edit");
}

export const BACK_TO_COMMODITIES = "commodities";
export const BACK_TO_AREAS = "areas";
export type BackTo = typeof BACK_TO_COMMODITIES | typeof BACK_TO_AREAS;

export async function deleteCommodity(
  page: Page,
  recorder: TestRecorder,
  commodityName: string,
  backTo: BackTo,
) {
  await page.click('[data-testid="commodity-detail-delete"]');
  await page
    .locator('[data-testid="confirm-dialog"]')
    .waitFor({ state: "visible", timeout: 5000 });
  await page.click('[data-testid="confirm-accept"]');
  await recorder.takeScreenshot("commodity-delete-01-on-delete-confirm");

  await page
    .locator('[data-testid="confirm-dialog"]')
    .waitFor({ state: "hidden", timeout: 5000 });

  if (backTo === BACK_TO_COMMODITIES) {
    await expect(page).toHaveURL(/\/commodities(?:\?.*)?$/, { timeout: 10000 });
    await recorder.takeScreenshot("commodity-delete-02-after-delete");
  } else if (backTo === BACK_TO_AREAS) {
    // Post-cutover the area detail page is a ComingSoon stub — the
    // commodity-delete redirect lands on /commodities anyway. Tests
    // that pass BACK_TO_AREAS get the same destination as
    // BACK_TO_COMMODITIES today; the assertion stays loose to avoid
    // false negatives once #1448 (quick-attach + area-scoped detail)
    // ships.
    await expect(page).toHaveURL(/\/(commodities|areas)/, { timeout: 10000 });
    await recorder.takeScreenshot("commodity-delete-01-after-delete");
  }

  // Verify the commodity card is gone from the list.
  const card = page.locator(
    `[data-testid="commodity-card"]:has-text("${commodityName}")`,
  );
  await expect(card).toHaveCount(0, { timeout: 15000 });
}

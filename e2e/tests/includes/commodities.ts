import { expect, Page } from "@playwright/test";
import { TestRecorder } from "../../utils/test-recorder.js";

// React port of the Vue-era commodities helper. Post cutover #1423:
//
// - Trigger lives at `[data-testid="commodities-add-button"]` on the
//   /commodities list page (not on the area detail page — area detail is a
//   ComingSoon stub today).
// - The form is a multi-step Dialog (`[data-testid="commodity-form-dialog"]`)
//   with five steps: basics → purchase → warranty → extras → files.
//   Step navigation goes through `[data-testid="commodity-form-next"]`;
//   submit is `[data-testid="commodity-form-submit"]`.
// - Type / area inputs are NATIVE `<select>` elements, not PrimeVue popups.
// - Currency is a plain text input (3-char ISO code).
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
    /** Optional area to bind the commodity to. The form defaults to the
     *  first area in the dropdown when omitted, which is fine for tests
     *  that only ever create one area. */
    areaName?: string;
    [key: string]: unknown;
}

async function selectByPartialOptionText(page: Page, selectId: string, fragment: string) {
    // Native <select> + selectOption({ label: ... }) requires an exact label
    // match, but our type options carry an emoji icon prefix
    // (`💻 Electronics`). Match the option element by partial text and
    // forward its value to selectOption.
    const value = await page
        .locator(`#${selectId} option`, { hasText: fragment })
        .first()
        .getAttribute('value');
    if (!value) {
        throw new Error(
            `selectByPartialOptionText: no <option> matching "${fragment}" inside #${selectId}`,
        );
    }
    await page.selectOption(`#${selectId}`, value);
}

async function fillChip(page: Page, testId: string, value: string) {
    const input = page.locator(`[data-testid="${testId}-input"]`);
    await input.fill(value);
    await input.press('Enter');
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
        await chip.locator('button').click();
    }
}

async function gotoNext(page: Page) {
    await page.click('[data-testid="commodity-form-next"]');
}

async function fillBasicsStep(page: Page, c: TestCommodity) {
    await page.fill('#commodity-name', c.name);
    await page.fill('#commodity-short-name', c.shortName);
    await page.fill('#commodity-count', String(c.count));
    if (c.type) {
        await selectByPartialOptionText(page, 'commodity-type', c.type);
    }
    if (c.areaName) {
        await selectByPartialOptionText(page, 'commodity-area', c.areaName);
    }
}

async function fillPurchaseStep(page: Page, c: TestCommodity) {
    if (c.purchaseDate) await page.fill('#commodity-purchase-date', c.purchaseDate);
    if (c.originalPrice !== undefined) {
        await page.fill('#commodity-original-price', String(c.originalPrice));
        // The form's `superRefine` block in `commoditySchema` requires
        // both `converted_original_price` and `current_price` to be
        // non-empty for non-draft commodities — leaving them blank
        // fails the step's `trigger()` silently and keeps the dialog
        // pinned on Purchase. The BE adds a second rule on top:
        // `converted_original_price` MUST be zero when
        // `original_price_currency` matches the group's main currency
        // (the seed dataset uses CZK as the group main currency, and
        // tests typically use CZK too — so the default has to be `"0"`
        // rather than the original price, which would 422). Tests that
        // explicitly need a non-zero converted value pass it via
        // `convertedOriginalPrice`.
        const converted = c.convertedOriginalPrice ?? 0;
        const current = c.currentPrice ?? c.originalPrice;
        await page.fill('#commodity-converted-price', String(converted));
        await page.fill('#commodity-current-price', String(current));
    }
    if (c.originalPriceCurrency) {
        await page.fill('#commodity-currency', c.originalPriceCurrency);
    }
    if (c.serialNumber !== undefined) {
        await page.fill('#commodity-serial', c.serialNumber);
    }
}

async function fillExtrasStep(page: Page, c: TestCommodity, replaceArrays = false) {
    if (replaceArrays) {
        if (c.tags !== undefined) await clearChips(page, 'commodity-tags');
        if (c.extraSerialNumbers !== undefined) await clearChips(page, 'commodity-extra-serials');
        if (c.partNumbers !== undefined) await clearChips(page, 'commodity-part-numbers');
        if (c.urls !== undefined) await clearChips(page, 'commodity-urls');
    }
    if (c.tags && c.tags.length) await fillChips(page, 'commodity-tags', c.tags);
    if (c.extraSerialNumbers && c.extraSerialNumbers.length) {
        await fillChips(page, 'commodity-extra-serials', c.extraSerialNumbers);
    }
    if (c.partNumbers && c.partNumbers.length) {
        await fillChips(page, 'commodity-part-numbers', c.partNumbers);
    }
    if (c.urls && c.urls.length) await fillChips(page, 'commodity-urls', c.urls);
}

export async function createCommodity(
    page: Page,
    recorder: TestRecorder,
    testCommodity: TestCommodity,
): Promise<string> {
    await recorder.takeScreenshot('commodities-create-01-before-create');

    await page.click('[data-testid="commodities-add-button"]');
    await page.waitForSelector('[data-testid="commodity-form-dialog"]');

    // Step 1: Basics.
    await fillBasicsStep(page, testCommodity);
    await recorder.takeScreenshot('commodity-create-02-basics');
    await gotoNext(page);

    // Step 2: Purchase.
    await fillPurchaseStep(page, testCommodity);
    await recorder.takeScreenshot('commodity-create-03-purchase');
    await gotoNext(page);

    // Step 3: Warranty (ComingSoon stub).
    await gotoNext(page);

    // Step 4: Extras (chip inputs).
    await fillExtrasStep(page, testCommodity);
    await recorder.takeScreenshot('commodity-create-04-extras');
    await gotoNext(page);

    // Step 5: Files (ComingSoon stub) → Submit. Wait for the FilesStep
    // marker first so we don't race the dialog's re-render — without this,
    // Playwright's auto-await sees the submit button mid-mount and retries
    // the click as the element transitions out of the previous step's
    // layout ("element is not stable" / "element was detached from the
    // DOM, retrying"). The marker only renders inside FilesStep, so its
    // presence is a positive signal that step 5 has committed.
    await page.waitForSelector('[data-testid="commodity-form-files-step"]', {
        state: 'visible',
        timeout: 5000,
    });
    // Imperatively trigger the form's `submit` event from the page
    // context. Playwright's `click` (and even `dispatchEvent`) gets
    // tangled in actionability auto-retry once the dialog starts
    // unmounting itself in the same React commit as the submission;
    // calling `requestSubmit()` on the form node side-steps the whole
    // locator pipeline and runs react-hook-form's validate→submit
    // chain via the same path a real user click takes.
    //
    // WebKit-specific quirk: pressing Enter inside a ChipInput field
    // (Extras step) sometimes leaks past the React `e.preventDefault()`
    // handler and submits the form ahead of schedule. By the time we
    // reach this line the dialog is already gone and the page is on
    // the detail URL. Treat that as success rather than throwing — if
    // the page already navigated, the create succeeded; otherwise
    // dispatch the submit and wait for the URL transition.
    const alreadyOnDetail = /\/commodities\/[0-9a-fA-F-]{36}/.test(new URL(page.url()).pathname);
    if (!alreadyOnDetail) {
        const formStillMounted = await page.evaluate(() => {
            const form = document.getElementById('commodity-form') as HTMLFormElement | null;
            if (!form) return false;
            form.requestSubmit();
            return true;
        });
        if (!formStillMounted) {
            // Dialog unmounted between our last gotoNext and this line —
            // something already submitted the form. Wait for the URL
            // settle below; if the create really hadn't fired, that
            // wait will time out with a clean error.
        }
        await page.waitForURL(/\/commodities\/[0-9a-fA-F-]{36}/, { timeout: 30000 });
    }
    await page.waitForSelector('[data-testid="page-commodity-detail"]');
    await page.waitForLoadState('networkidle');
    await recorder.takeScreenshot('commodity-create-05-created');

    return page.url();
}

export async function verifyCommodityDetails(page: Page, testCommodity: TestCommodity) {
    const detail = page.locator('[data-testid="page-commodity-detail"]');
    await detail.waitFor({ state: 'visible', timeout: 10000 });

    await expect(detail.locator('h1')).toContainText(testCommodity.name);
    await expect(detail.locator('[data-testid="commodity-detail-short-name"]')).toContainText(
        testCommodity.shortName,
    );

    // The Details Card hosts count, prices, serial, tags, urls, extras, etc.
    // Without granular per-row testids the highest-signal check is "the
    // value's text appears anywhere inside the card."
    const details = detail.locator('[data-testid="commodity-detail-details"]');
    await details.waitFor({ state: 'visible', timeout: 10000 });
    await expect(details).toContainText(String(testCommodity.count));
    await expect(details).toContainText(String(testCommodity.originalPrice));

    if (testCommodity.serialNumber) {
        await expect(details).toContainText(testCommodity.serialNumber);
    }
    if (testCommodity.extraSerialNumbers && testCommodity.extraSerialNumbers.length) {
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
        if (typeof buttonSelector === 'string' && buttonSelector.length > 0) {
            await page.click(buttonSelector);
        } else {
            await page.click('[data-testid="commodity-detail-edit"]');
        }
    }

    await page.waitForSelector('[data-testid="commodity-form-dialog"]');
    await recorder.takeScreenshot('commodity-edit-01-edit-form');

    // Step 1: Basics.
    await fillBasicsStep(page, updatedCommodity);
    await gotoNext(page);

    // Step 2: Purchase.
    await fillPurchaseStep(page, updatedCommodity);
    await gotoNext(page);

    // Step 3: Warranty stub.
    await gotoNext(page);

    // Step 4: Extras — replace existing chips with the updated values.
    await fillExtrasStep(page, updatedCommodity, /*replaceArrays*/ true);
    await recorder.takeScreenshot('commodity-edit-02-edit-form-filled');
    await gotoNext(page);

    // Step 5: Files stub → Submit. Same imperative trick as
    // createCommodity: dispatch the form's `submit` event directly so
    // we sidestep Playwright's actionability auto-retry while the
    // dialog unmounts in the same React commit as the mutation. We
    // settle on the rendered h1 instead of waiting for the PUT — the
    // listener-attach race that bit createCommodity bites here too.
    await page.waitForSelector('[data-testid="commodity-form-files-step"]', {
        state: 'visible',
        timeout: 5000,
    });
    await page.evaluate(() => {
        const form = document.getElementById('commodity-form') as HTMLFormElement | null;
        if (!form) throw new Error('commodity-form not in DOM at edit-submit time');
        form.requestSubmit();
    });
    // Stay on the detail page (no navigate after edit; the form just
    // closes the dialog and revalidates the cached detail query).
    await expect(page).toHaveURL(/\/commodities\/[0-9a-fA-F-]{36}(\?.*)?$/);
    await page.waitForLoadState('networkidle');
    await expect(page.locator('h1')).toContainText(updatedCommodity.name, { timeout: 10000 });
    await recorder.takeScreenshot('commodity-edit-03-after-edit');
}

export const BACK_TO_COMMODITIES = 'commodities';
export const BACK_TO_AREAS = 'areas';
export type BackTo = typeof BACK_TO_COMMODITIES | typeof BACK_TO_AREAS;

export async function deleteCommodity(
    page: Page,
    recorder: TestRecorder,
    commodityName: string,
    backTo: BackTo,
) {
    await page.click('[data-testid="commodity-detail-delete"]');
    await page.locator('[data-testid="confirm-dialog"]').waitFor({ state: 'visible', timeout: 5000 });
    await page.click('[data-testid="confirm-accept"]');
    await recorder.takeScreenshot('commodity-delete-01-on-delete-confirm');

    await page.locator('[data-testid="confirm-dialog"]').waitFor({ state: 'hidden', timeout: 5000 });

    if (backTo === BACK_TO_COMMODITIES) {
        await expect(page).toHaveURL(/\/commodities(?:\?.*)?$/, { timeout: 10000 });
        await recorder.takeScreenshot('commodity-delete-02-after-delete');
    } else if (backTo === BACK_TO_AREAS) {
        // Post-cutover the area detail page is a ComingSoon stub — the
        // commodity-delete redirect lands on /commodities anyway. Tests
        // that pass BACK_TO_AREAS get the same destination as
        // BACK_TO_COMMODITIES today; the assertion stays loose to avoid
        // false negatives once #1448 (quick-attach + area-scoped detail)
        // ships.
        await expect(page).toHaveURL(/\/(commodities|areas)/, { timeout: 10000 });
        await recorder.takeScreenshot('commodity-delete-01-after-delete');
    }

    // Verify the commodity card is gone from the list.
    const card = page.locator(
        `[data-testid="commodity-card"]:has-text("${commodityName}")`,
    );
    await expect(card).toHaveCount(0, { timeout: 15000 });
}

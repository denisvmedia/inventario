import { Page } from "@playwright/test";
import { checkSettingsRequired, navigateAndCheckSettings } from "./settings-check.js";
import { ensureAuthenticated } from "./auth.js";
import { TestRecorder } from "../../utils/test-recorder.js";

export const TO_HOME = 'home';
export const TO_LOCATIONS = 'locations';
export const TO_COMMODITIES = 'commodities';
export const TO_AREA_COMMODITIES = 'area-commodities';
export const TO_SYSTEM = 'system';
export const TO_EXPORTS = 'exports';

export type TypeTo =
    | typeof TO_HOME
    | typeof TO_LOCATIONS
    | typeof TO_COMMODITIES
    | typeof TO_AREA_COMMODITIES
    | typeof TO_SYSTEM
    | typeof TO_EXPORTS;

export const FROM_HOME = 'home';
export const FROM_LOCATIONS = 'locations';
export const FROM_LOCATIONS_AREA = 'locations-area';
export const FROM_COMMODITIES = 'commodities';
export const FROM_SYSTEM = 'system';

export type TypeFrom =
    | typeof FROM_HOME
    | typeof FROM_LOCATIONS
    | typeof FROM_LOCATIONS_AREA
    | typeof FROM_COMMODITIES
    | typeof FROM_SYSTEM;

// Post-cutover (#1423) navigation helpers. The Vue app exposed
// breadcrumbs and area-detail entries that the React app drops in favour
// of a flatter routing model: /commodities is the canonical list page,
// area detail is currently a ComingSoon stub. Navigation helpers
// therefore shed FROM-context detours that were Vue-specific UX scaffolding.
export async function navigateTo(
    page: Page,
    recorder: TestRecorder,
    to: TypeTo,
    from?: TypeFrom,
    source?: string,
) {
    switch (to) {
        case TO_HOME:
            await navigateAndCheckSettings(page, '/', recorder);
            break;
        case TO_LOCATIONS:
            // The Vue app routed back via a breadcrumb (`Back to Locations`)
            // when coming from a commodity. The React shell lacks that
            // breadcrumb; the canonical /locations route is reachable via
            // direct navigation regardless of where we came from.
            await ensureAuthenticated(page, recorder);
            await checkSettingsRequired(page);
            await navigateAndCheckSettings(page, '/locations', recorder);
            break;
        case TO_COMMODITIES:
            await navigateAndCheckSettings(page, '/commodities', recorder);
            break;
        case TO_AREA_COMMODITIES:
            // Vue routed via the area card into the area detail page where
            // commodities lived. Post #1423 the area detail page is a
            // ComingSoon stub and commodity creation is a top-level flow
            // that asks for the area on the form. We therefore land on
            // /commodities — the caller is expected to pass the area name
            // explicitly to createCommodity().
            void from;
            void source;
            await navigateAndCheckSettings(page, '/commodities', recorder);
            break;
        case TO_SYSTEM:
            await navigateAndCheckSettings(page, '/system', recorder);
            break;
        case TO_EXPORTS:
            await navigateAndCheckSettings(page, '/exports', recorder);
            break;
    }
}

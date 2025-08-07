import {Page} from "@playwright/test";
import {checkSettingsRequired, navigateAndCheckSettings} from "./settings-check";

export const TO_HOME = 'home';
export const TO_LOCATIONS = 'locations';
export const TO_COMMODITIES = 'commodities';
export const TO_AREA_COMMODITIES = 'area-commodities';
export const TO_SYSTEM = 'system';
export const TO_EXPORTS = 'exports';

export type TypeTo = typeof TO_HOME | typeof TO_LOCATIONS | typeof TO_COMMODITIES | typeof TO_AREA_COMMODITIES | typeof TO_SYSTEM | typeof TO_EXPORTS;

export const FROM_HOME = 'home';
export const FROM_LOCATIONS = 'locations';
export const FROM_LOCATIONS_AREA = 'locations-area';
export const FROM_COMMODITIES = 'commodities';
export const FROM_SYSTEM = 'system';

export type TypeFrom = typeof FROM_HOME | typeof FROM_LOCATIONS | typeof FROM_LOCATIONS_AREA | typeof FROM_COMMODITIES | typeof FROM_SYSTEM;

export async function navigateTo(page: Page, recorder: any, to : TypeTo, from?: TypeFrom, source?: string) {
    switch (to) {
        case TO_HOME:
            await navigateAndCheckSettings(page, '/')
            break;
        case TO_LOCATIONS:
            switch (from) {
                case FROM_COMMODITIES:
                    await checkSettingsRequired(page);
                    // Navigate back to the location detail page
                    await page.click(`.breadcrumb-link:has-text("Back to Locations")`);
                    break;
                default:
                    await navigateAndCheckSettings(page, '/locations')
            }
            break;
        case TO_COMMODITIES:
            await navigateAndCheckSettings(page, '/commodities')
            break;
        case TO_AREA_COMMODITIES:
            switch (from) {
                case FROM_LOCATIONS_AREA:
                    await checkSettingsRequired(page);
                    // source is the area name
                    await page.click(`.area-card:has-text("${source}")`);
                    break;
                default:
                    throw new Error('Not supported');
            }
            break;
        case TO_SYSTEM:
            await navigateAndCheckSettings(page, '/system')
            break;
        case TO_EXPORTS:
            await navigateAndCheckSettings(page, '/exports')
            break;
    }
}

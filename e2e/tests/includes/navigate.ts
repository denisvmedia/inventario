import {Page} from "@playwright/test";
import {createCommodity} from "./commodities.js";

export const TO_HOME = 'home';
export const TO_LOCATIONS = 'locations';
export const TO_COMMODITIES = 'commodities';
export const TO_AREA_COMMODITIES = 'area-commodities';
export const TO_SETTINGS = 'settings';

export type TypeTo = typeof TO_HOME | typeof TO_LOCATIONS | typeof TO_COMMODITIES | typeof TO_AREA_COMMODITIES | typeof TO_SETTINGS;

export const FROM_HOME = 'home';
export const FROM_LOCATIONS = 'locations';
export const FROM_LOCATIONS_AREA = 'locations-area';
export const FROM_COMMODITIES = 'commodities';
export const FROM_SETTINGS = 'settings';

export type TypeFrom = typeof FROM_HOME | typeof FROM_LOCATIONS | typeof FROM_LOCATIONS_AREA | typeof FROM_COMMODITIES | typeof FROM_SETTINGS;

export async function navitateTo(page: Page, to : TypeTo, from?: TypeFrom, source?: string) {
    switch (to) {
        case TO_HOME:
            await page.goto('/');
            break;
        case TO_LOCATIONS:
            switch (from) {
                case FROM_COMMODITIES:
                    // Navigate back to the location detail page
                    await page.click(`.breadcrumb-link:has-text("Back to Locations")`);
                    break;
                default:
                    await page.goto('/locations');
            }
            break;
        case TO_COMMODITIES:
            await page.goto('/commodities');
            break;
        case TO_AREA_COMMODITIES:
            switch (from) {
                case FROM_LOCATIONS_AREA:
                    // source is the area name
                    await page.click(`.area-card:has-text("${source}")`);
                    break;
                default:
                    throw new Error('Not supported');
            }
            break;
        case TO_SETTINGS:
            await page.goto('/settings');
            break;
    }
}

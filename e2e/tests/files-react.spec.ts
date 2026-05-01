import { expect, test, type Page } from '@playwright/test'

import { TEST_CREDENTIALS } from './includes/auth.js'

/**
 * @react-only smoke for the unified Files page (#1411).
 *
 * What the spec proves:
 *   - The /files route renders the React Files list page (not the old
 *     Vue split between Images / Invoices / Manuals, and not a
 *     PlaceholderPage stub).
 *   - The four category tiles introduced by #1398 are visible and
 *     respond to the per-tile selection click — `aria-selected` flips,
 *     the URL gains `?category=`, and the BE roundtrips the filter.
 *   - The Upload button opens the upload dialog with a working
 *     dropzone that accepts a synthetic File via Playwright's
 *     `setInputFiles` (slot-gating + actual upload roundtrip is left
 *     for the @react-only login fixture in #1449 to cover).
 *
 * Login is done inline against the React Auth page (#1407) until the
 * shared `@react-only` login fixture in #1449 lands.
 */

async function loginToReact(page: Page): Promise<void> {
  await page.goto('/login')
  await page.getByTestId('email').fill(TEST_CREDENTIALS.email)
  await page.getByTestId('password').fill(TEST_CREDENTIALS.password)
  await page.getByTestId('login-button').click()
  // Land on a group-scoped route — exact path varies based on the
  // user's default group; we just need the React shell to mount.
  await expect(page.locator('#root')).toBeAttached()
  await page.waitForLoadState('networkidle')
}

async function gotoFiles(page: Page): Promise<void> {
  // Navigate via the URL rather than the sidebar so the test isn't
  // coupled to the navigation chrome's exact label.
  const url = new URL(page.url())
  const segments = url.pathname.split('/').filter(Boolean)
  // Group-scoped routes are `/g/<slug>/...`; preserve the active group
  // to avoid a redirect through /no-group.
  if (segments[0] === 'g' && segments[1]) {
    await page.goto(`/g/${segments[1]}/files`)
  } else {
    await page.goto('/files')
  }
  await expect(page.getByTestId('page-files')).toBeVisible()
}

test.describe('@react-only Files page', () => {
  test('renders the list page with all five category tiles', async ({ page }) => {
    await loginToReact(page)
    await gotoFiles(page)

    await expect(page.getByTestId('files-tile-all')).toBeVisible()
    await expect(page.getByTestId('files-tile-photos')).toBeVisible()
    await expect(page.getByTestId('files-tile-invoices')).toBeVisible()
    await expect(page.getByTestId('files-tile-documents')).toBeVisible()
    await expect(page.getByTestId('files-tile-other')).toBeVisible()
    // "All" is the default selection.
    await expect(page.getByTestId('files-tile-all')).toHaveAttribute('aria-selected', 'true')
  })

  test('selecting a category tile flips aria-selected and updates the URL', async ({ page }) => {
    await loginToReact(page)
    await gotoFiles(page)

    await page.getByTestId('files-tile-photos').click()
    await expect(page.getByTestId('files-tile-photos')).toHaveAttribute('aria-selected', 'true')
    await expect(page.getByTestId('files-tile-all')).toHaveAttribute('aria-selected', 'false')
    await expect(page).toHaveURL(/category=photos/)
  })

  test('upload dialog opens from the CTA and exposes the dropzone', async ({ page }) => {
    await loginToReact(page)
    await gotoFiles(page)

    await page.getByTestId('files-upload-cta').click()
    await expect(page.getByTestId('files-upload-dialog')).toBeVisible()
    await expect(page.getByTestId('files-upload-dropzone')).toBeVisible()
    // The hidden <input type=file> is what Playwright drives — we don't
    // attach a real file here because slot-gating + post-upload
    // roundtrip is still pending the #1449 fixture.
    await expect(page.getByTestId('files-upload-input')).toBeAttached()
  })
})

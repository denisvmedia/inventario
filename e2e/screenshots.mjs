// Standalone Playwright script — login and screenshot the React
// frontend (epic #1397). Every authenticated page lives under
// /g/:groupSlug/* once a group is active.
//
// Usage:
//   1. Build the frontend so the embed package compiles:
//        make build-frontend
//   2. Build the binary with the with_frontend tag:
//        cd go/cmd/inventario && go build -tags with_frontend -o ../../../bin/inventario .
//   3. Run the binary:
//        ./bin/inventario run --db-dsn=memory:// \
//          --no-auth-rate-limit --no-global-rate-limit
//   4. Seed the DB:
//        curl -X POST http://localhost:3333/api/v1/seed
//   5. Run this script:
//        BASE_URL=http://localhost:3333 OUT=tmp-screenshots \
//          node e2e/screenshots.mjs

import { chromium } from "playwright"
import { mkdirSync } from "fs"
import { join } from "path"

const BASE_URL = process.env.BASE_URL || "http://localhost:3333"
const OUT = process.env.OUT || "tmp-screenshots-react"
const EMAIL = process.env.EMAIL || "admin@test-org.com"
const PASSWORD = process.env.PASSWORD || "testpassword123"

mkdirSync(OUT, { recursive: true })

const browser = await chromium.launch({ headless: true })
const context = await browser.newContext({ viewport: { width: 1440, height: 900 } })
const page = await context.newPage()

page.on("pageerror", (err) => console.error("PAGE ERROR:", err.message))
page.on("console", (msg) => {
  if (msg.type() === "error") console.error("CONSOLE ERR:", msg.text())
})

async function settle() {
  try {
    await page.waitForLoadState("networkidle", { timeout: 5000 })
  } catch {
    /* fine — some pages keep streaming a fetch */
  }
  await page.waitForTimeout(500)
}

async function login() {
  console.log(`-> login as ${EMAIL}`)
  await page.goto(`${BASE_URL}/login`, { waitUntil: "domcontentloaded" })
  await page.waitForSelector('input[type="email"]', { timeout: 15000 })
  await page.fill('input[type="email"]', EMAIL)
  await page.fill('input[type="password"]', PASSWORD)
  const respPromise = page.waitForResponse((r) => r.url().includes("/api/v1/auth/login"), {
    timeout: 20000,
  })
  await page.click('button[type="submit"]')
  const resp = await respPromise
  if (resp.status() !== 200) {
    const body = await resp.text().catch(() => "")
    throw new Error(`Login failed ${resp.status()}: ${body.slice(0, 300)}`)
  }
  try {
    await page.waitForURL((url) => !url.toString().includes("/login"), { timeout: 15000 })
  } catch (err) {
    const currentURL = page.url()
    if (currentURL.includes("/login")) {
      throw new Error(`Login succeeded but UI did not leave /login (current URL: ${currentURL})`)
    }
    throw err
  }
  await settle()
  console.log(`   post-login url: ${page.url()}`)
}

async function shoot(name, full = true) {
  const file = join(OUT, `${name}.png`)
  await page.screenshot({ path: file, fullPage: full })
  console.log(`   saved ${file}`)
}

// 1×1 transparent PNG used as the upload payload for the #1448
// quick-attach screenshots. Small + valid so the BE accepts it; the
// thumbnail in the panel renders as a near-empty card, which is the
// point — we want the panel state, not the file content itself.
const TINY_PNG = Buffer.from(
  "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNkYAAAAAYAAjCB0C8AAAAASUVORK5CYII=",
  "base64",
)

// Capture the four #1448 quick-attach surfaces on a given entity
// detail page (works for both commodity and location). Order:
//
//   <prefix>a  open Attach dialog (linkedEntity title)
//   <prefix>b  drop overlay synthesised via dispatchEvent
//   <prefix>c  dialog after files are queued (Step 1 → the list row)
//   <prefix>d  panel after a successful upload (file card visible)
//
// `pageTestId` is the wrapper's data-testid (page-commodity-detail or
// page-location-detail). `prefix` is the file-name prefix (e.g. "18c"
// for commodity, "13b" for location). `entityName` is what we expect
// to see in the dialog title (e.g. "Camping Equipment", "Home").
async function captureQuickAttach(pageTestId, prefix, entityName) {
  const attachBtn = page.locator('[data-testid="entity-files-panel-attach"]')
  if ((await attachBtn.count()) === 0) {
    console.warn(`   ${prefix}: attach button missing on ${pageTestId}; skipping`)
    return
  }

  // Step a — dialog opened from the button. Should display
  // "Attach files to {name}" in the title via linkedEntity.name.
  console.log(`-> ${prefix}a: ${entityName} attach dialog`)
  await attachBtn.click()
  await page.waitForSelector('[data-testid="files-upload-dialog"]')
  await settle()
  await shoot(`${prefix}a-attach-dialog`, false)
  await page.keyboard.press("Escape")
  await settle(200)

  // Step b — drop overlay. We hand-roll a DataTransfer in the page
  // context and pass the JSHandle so the synthesised DragEvent
  // carries `types: ["Files"]` (the hook gates on that).
  console.log(`-> ${prefix}b: ${entityName} drop overlay`)
  const dt = await page.evaluateHandle(() => {
    const t = new DataTransfer()
    t.items.add(new File(["x"], "evidence.png", { type: "image/png" }))
    return t
  })
  await page.dispatchEvent(`[data-testid="${pageTestId}"]`, "dragenter", {
    dataTransfer: dt,
  })
  try {
    await page.waitForSelector('[data-testid="entity-drop-overlay"]', { timeout: 3000 })
    await settle(200)
    await shoot(`${prefix}b-drop-overlay`, false)
  } catch {
    console.warn(`   ${prefix}b: overlay did not appear; skipping`)
  }
  // Clear the drag state. dragleave decrements the counter; we
  // only entered once so a single leave returns to "no drag".
  await page.dispatchEvent(`[data-testid="${pageTestId}"]`, "dragleave", {
    dataTransfer: dt,
  })
  await settle(200)

  // Step c — dialog with files queued. We open the dialog again and
  // feed a file through the hidden input (faster + more reliable
  // than synthesising a real drop, which jsdom and headless Chromium
  // both treat differently from a user gesture).
  console.log(`-> ${prefix}c: ${entityName} dialog with file queued`)
  await attachBtn.click()
  await page.waitForSelector('[data-testid="files-upload-dropzone"]')
  await page.setInputFiles('[data-testid="files-upload-input"]', {
    name: "evidence.png",
    mimeType: "image/png",
    buffer: TINY_PNG,
  })
  try {
    await page.waitForSelector('[data-testid="files-upload-list"]', { timeout: 3000 })
    await settle(200)
    await shoot(`${prefix}c-dialog-queued`, false)
  } catch {
    console.warn(`   ${prefix}c: file did not queue; skipping`)
    await page.keyboard.press("Escape")
    return
  }

  // Step d — run the full upload flow and capture the panel after
  // the dialog closes. Sequence: Next → metadata → Upload → wait
  // done → Close. Real multipart POST hits /api/v1/.../uploads/file
  // and the linkage PUT hits /api/v1/.../files/{id}; the file ends
  // up in the binary's uploadLocation (default ./bin/uploads/).
  console.log(`-> ${prefix}d: ${entityName} panel after upload`)
  try {
    await page.click('[data-testid="files-upload-next"]')
    await page.waitForSelector('[data-testid="files-upload-metadata-list"]', { timeout: 3000 })
    await page.click('[data-testid="files-upload-start"]')
    await page.waitForSelector(
      '[data-testid^="files-upload-progress-item-"][data-status="done"]',
      { timeout: 15000 },
    )
    const closeBtn = page.locator('[data-testid="files-upload-close"]')
    await closeBtn.waitFor({ timeout: 5000 })
    await closeBtn.click()
    // Wait for the panel to refetch (invalidate.all() fires after
    // the batch finishes) and surface the new file card.
    await page.waitForSelector('[data-testid="entity-files-panel-grid"]', { timeout: 8000 })
    await settle(400)
    await shoot(`${prefix}d-panel-after-upload`)
  } catch (err) {
    console.warn(`   ${prefix}d: upload flow failed (${err.message}); skipping`)
    // Try to dismiss any lingering dialog so the rest of the run
    // doesn't get stuck.
    await page.keyboard.press("Escape").catch(() => {})
    await settle(200)
  }
}

// Resolve the active group slug from the URL after login. The router
// redirects "/" to "/g/<first-slug>/" so we can read the slug straight
// off the post-login URL — but if the user lands on "/no-group" we
// won't have a slug at all.
function slugFromUrl() {
  const m = page.url().match(/\/g\/([^/]+)/)
  return m ? m[1] : null
}

try {
  // Public pages — no auth required.
  console.log("-> /login (unauthenticated)")
  await page.goto(`${BASE_URL}/login`, { waitUntil: "domcontentloaded" })
  await settle()
  await shoot("01-login")

  console.log("-> /register")
  await page.goto(`${BASE_URL}/register`, { waitUntil: "domcontentloaded" })
  await settle()
  await shoot("02-register")

  console.log("-> /forgot-password")
  await page.goto(`${BASE_URL}/forgot-password`, { waitUntil: "domcontentloaded" })
  await settle()
  await shoot("03-forgot-password")

  console.log("-> /some-nonexistent-route (NotFound)")
  await page.goto(`${BASE_URL}/some-nonexistent-route`, { waitUntil: "domcontentloaded" })
  await settle()
  await shoot("04-not-found")

  await login()

  const slug = slugFromUrl()
  if (!slug) {
    console.error(
      "✖ no group slug in URL after login — seeded data may be missing or the server redirected to /no-group",
    )
    process.exitCode = 1
  } else {
    console.log(`-> active group slug: ${slug}`)
    const groupPages = [
      { name: "10-dashboard", path: `/g/${slug}/` },
      { name: "11-locations", path: `/g/${slug}/locations` },
      { name: "12-locations-new", path: `/g/${slug}/locations/new` },
    ]
    for (const p of groupPages) {
      console.log(`-> ${p.path}`)
      await page.goto(`${BASE_URL}${p.path}`, { waitUntil: "domcontentloaded", timeout: 20000 })
      await settle()
      if (page.url().includes("/login")) {
        console.warn(`   redirected to login on ${p.path}`)
      }
      await shoot(p.name)
    }

    // Drill into the first location's detail page if the seed has any.
    console.log(`-> /g/${slug}/locations (probe first location)`)
    await page.goto(`${BASE_URL}/g/${slug}/locations`, { waitUntil: "domcontentloaded" })
    await settle()
    const firstCard = page.locator('[data-testid="location-card"]').first()
    if ((await firstCard.count()) > 0) {
      const link = firstCard.locator("a").first()
      const href = await link.getAttribute("href")
      if (href) {
        console.log(`-> location detail ${href}`)
        await page.goto(`${BASE_URL}${href}`, { waitUntil: "domcontentloaded" })
        await settle()
        await shoot("13-location-detail")

        // #1448 quick-attach surfaces on the location detail page.
        // Use the visible heading text as the entityName (we don't
        // need to read it back — the dialog title we'll capture
        // displays the same string the user sees).
        const locationName =
          (await page.locator("h1").first().textContent())?.trim() ?? "this location"
        await captureQuickAttach("page-location-detail", "13q", locationName)
        // captureQuickAttach navigates within the same page; the
        // post-upload state leaves the panel showing the new file
        // card, so the 14-area-detail probe below still works (it
        // only navigates to a different URL).

        // Drill again into the first area on the detail page if any.
        const areaRow = page.locator('[data-testid="location-detail-area"] a').first()
        if ((await areaRow.count()) > 0) {
          const areaHref = await areaRow.getAttribute("href")
          if (areaHref) {
            console.log(`-> area detail ${areaHref}`)
            await page.goto(`${BASE_URL}${areaHref}`, { waitUntil: "domcontentloaded" })
            await settle()
            await shoot("14-area-detail")
          }
        }
      }
    }
  }

  // Commodities (#1410) — list + sheet preview + add dialog +
  // detail + print.
  const slugForCommodities = slugFromUrl() ?? slug
  if (slugForCommodities) {
    console.log(`-> /g/${slugForCommodities}/commodities`)
    await page.goto(`${BASE_URL}/g/${slugForCommodities}/commodities`, {
      waitUntil: "domcontentloaded",
    })
    await settle()
    await shoot("15-commodities-list")

    // Sheet preview — bare-click on the first card.
    const firstCommodity = page.locator('[data-testid="commodity-card"] a').first()
    if ((await firstCommodity.count()) > 0) {
      await firstCommodity.click()
      try {
        await page.waitForSelector('[data-testid="commodity-preview-sheet"]', {
          timeout: 5000,
        })
        await settle()
        await shoot("16-commodities-preview-sheet")
      } catch {
        console.warn("   sheet preview did not open in time, skipping shot")
      }
      // Close sheet via Escape.
      await page.keyboard.press("Escape")
      await settle()
    }

    // Add Item dialog — first step.
    const addBtn = page.locator('[data-testid="commodities-add-button"]').first()
    if ((await addBtn.count()) > 0) {
      await addBtn.click()
      try {
        await page.waitForSelector('[aria-label="Form steps"]', { timeout: 5000 })
        await settle()
        await shoot("17-commodities-add-dialog-step1")
      } catch {
        console.warn("   add dialog did not open in time, skipping shot")
      }
      await page.keyboard.press("Escape")
      await settle()
    }

    // Detail page — drill into the first commodity via cmd-click
    // (Sheet preview default would block us). Use direct navigation.
    if ((await firstCommodity.count()) > 0) {
      const href = await firstCommodity.getAttribute("href")
      if (href) {
        console.log(`-> commodity detail ${href}`)
        await page.goto(`${BASE_URL}${href}`, { waitUntil: "domcontentloaded" })
        await settle()
        await shoot("18-commodity-detail")

        // Files tab (#1411 AC #4): clicking the tab swaps the
        // bottom-half of the detail page from the Details card to the
        // EntityFilesPanel. Captured separately from "18" because the
        // default tab is Details and the panel never appears in that
        // shot. Skip-only when the tab control didn't render (e.g.
        // commodity-detail layout regression) so the run keeps going.
        const filesTab = page.locator('[data-testid="commodity-detail-tab-files"]')
        if ((await filesTab.count()) > 0) {
          await filesTab.click()
          try {
            await page.waitForSelector('[data-testid="entity-files-panel"]', {
              timeout: 5000,
            })
            await settle()
            await shoot("18b-commodity-detail-files")
          } catch {
            console.warn("   commodity Files tab did not surface the panel in time")
          }

          // #1448 quick-attach surfaces on the commodity detail page.
          // The Attach button + dropzone live on the Files tab, so
          // run this after the tab click. The commodity heading text
          // doubles as the entityName — same string surfaces in the
          // dialog title via linkedEntity.name.
          const commodityName =
            (await page.locator("h1").first().textContent())?.trim() ?? "this item"
          await captureQuickAttach("page-commodity-detail", "18q", commodityName)
        }

        // Print page.
        await page.goto(`${BASE_URL}${href}/print`, { waitUntil: "domcontentloaded" })
        await settle()
        await shoot("19-commodity-print")
      }
    }
  }

  // Profile + settings — independent of /g/:slug/, captured regardless of
  // whether a group slug was found.
  console.log("-> /profile")
  await page.goto(`${BASE_URL}/profile`, { waitUntil: "domcontentloaded" })
  await settle()
  await shoot("20-profile")

  console.log("-> /settings")
  await page.goto(`${BASE_URL}/settings`, { waitUntil: "domcontentloaded" })
  await settle()
  await shoot("21-settings")
} catch (err) {
  console.error("Screenshot run failed:", err)
  process.exitCode = 1
} finally {
  await browser.close()
}

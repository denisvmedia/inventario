// Standalone Playwright script — login and screenshot the *new* React
// frontend (epic #1397). Follows the legacy Vue screenshot flow, but
// hits the React route layout: every authenticated page lives under
// /g/:groupSlug/* once a group is active.
//
// Usage:
//   1. Build both frontends so the embed packages compile:
//        make build-frontend && make build-frontend-react
//   2. Build the binary with the with_frontend tag:
//        cd go/cmd/inventario && go build -tags with_frontend -o ../../../bin/inventario .
//   3. Run the binary with the React bundle selected:
//        ./bin/inventario run --frontend-bundle=new --db-dsn=memory:// \
//          --no-auth-rate-limit --no-global-rate-limit
//   4. Seed the DB:
//        curl -X POST http://localhost:3333/api/v1/seed
//   5. Run this script:
//        BASE_URL=http://localhost:3333 OUT=tmp-screenshots-react \
//          node e2e/screenshots-react.mjs

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

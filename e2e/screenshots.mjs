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
//        node e2e/screenshots.mjs
//
// Outputs land under .research/screenshots/<git-branch>/ by default —
// .research/ is in the maintainer's global gitignore so screenshots stay
// local-only per the screenshot-review skill. Override with OUT=… or
// pin a stable folder name with LABEL=…. To publish a captured run for
// review (issue comment, design audit, mock-vs-real comparison), use
// `e2e/push-screenshots.sh <label>` — it pushes the LABEL folder to a
// new `assets/screenshots-<label>` branch and prints raw URLs.

import { chromium } from "playwright"
import { mkdirSync } from "fs"
import { join } from "path"
import { execSync } from "child_process"

// Derive a stable, persistent OUT folder so multiple runs on the same
// branch overwrite the previous capture instead of scattering shots
// across timestamped tmp dirs. LABEL env var wins (use it for design-
// audit runs or PR-pinned labels); otherwise the current git branch is
// slug-ified; if both fail (detached HEAD, no git) we fall back to a
// stable "latest" name so something always lands on disk.
function defaultLabel() {
  if (process.env.LABEL) return process.env.LABEL
  try {
    const branch = execSync("git rev-parse --abbrev-ref HEAD", {
      encoding: "utf8",
      stdio: ["ignore", "pipe", "ignore"],
    }).trim()
    if (branch && branch !== "HEAD") {
      return branch.replace(/[^a-z0-9._-]+/gi, "-")
    }
  } catch {}
  return "latest"
}

const BASE_URL = process.env.BASE_URL || "http://localhost:3333"
const OUT = process.env.OUT || join(".research", "screenshots", defaultLabel())
const EMAIL = process.env.EMAIL || "admin@test-org.com"
// Seeded admin password — the BE bumped to a complexity-meeting value in
// #849 / #1577 ("TestPassword123"). The legacy lowercase string is kept
// here as a usable env override only.
const PASSWORD = process.env.PASSWORD || "TestPassword123"

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

// Wait for the zxcvbn-ts dynamic chunks to load + the meter to settle on
// its post-zxcvbn score. The heuristic shows up first; we want the real
// score in the screenshot. ~800 ms hard wait covers the cold first-call
// import path (~200–400 ms) plus the score recompute. Subsequent calls
// re-use the cached loader and resolve in microtasks.
async function waitForMeterScore(testId) {
  await page.waitForTimeout(800)
  await settle()
  await page.waitForSelector(`[data-testid="${testId}"]`, { timeout: 5000 })
}

// Capture three states of the password strength meter on a public auth
// surface (no login required). `surfacePath` is the URL; `prefill` is
// an optional async fn that pre-types whatever the page needs to show
// the meter (e.g. name + email on /register so they feed into zxcvbn
// userInputs and the form looks lived-in). `passwordSelector` and
// `meterTestId` come from the surface's `data-testid` wiring.
async function captureMeterStatesPublic({
  prefix,
  surfacePath,
  prefill,
  passwordSelector,
  meterTestId,
}) {
  console.log(`-> ${surfacePath} (meter empty)`)
  await page.goto(`${BASE_URL}${surfacePath}`, { waitUntil: "domcontentloaded" })
  await settle()
  if (prefill) await prefill()
  await shoot(`${prefix}-empty`)

  console.log(`-> ${surfacePath} (meter weak: "password")`)
  await page.fill(passwordSelector, "password")
  await waitForMeterScore(meterTestId)
  await shoot(`${prefix}-weak`)

  console.log(`-> ${surfacePath} (meter strong: high-entropy passphrase)`)
  await page.fill(passwordSelector, "ZebraNectar7Tundra!Ocean3Quiver")
  await waitForMeterScore(meterTestId)
  await shoot(`${prefix}-strong`)
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

  // Register page + password-strength meter (#1381). Three states each
  // for register / reset-password / profile-edit; keeps the empty-state
  // shot as the canonical "what does the page look like" reference.
  await captureMeterStatesPublic({
    prefix: "02-register",
    surfacePath: "/register",
    prefill: async () => {
      // Pre-fill name + email so the meter can use them as zxcvbn user-
      // inputs (a password derived from the user's own data scores low).
      await page.fill('input[data-testid="name"]', "Alex Johnson")
      await page.fill('input[data-testid="email"]', "alex@example.com")
      await settle(200)
    },
    passwordSelector: 'input[data-testid="password"]',
    meterTestId: "register-password-strength",
  })

  console.log("-> /forgot-password")
  await page.goto(`${BASE_URL}/forgot-password`, { waitUntil: "domcontentloaded" })
  await settle()
  await shoot("03-forgot-password")

  // Reset-password requires a token. With memory:// we don't have one
  // pre-seeded, but the page renders the form whenever ?token=… is
  // present and only validates on submit, so a placeholder is enough to
  // surface the form + meter. The "missing-token" branch is implicitly
  // covered when /reset-password is hit without ?token=.
  await captureMeterStatesPublic({
    prefix: "03b-reset-password",
    surfacePath: "/reset-password?token=screenshot-only-not-a-real-token",
    passwordSelector: 'input[data-testid="password"]',
    meterTestId: "reset-password-strength",
  })

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

  // Profile-edit page + password-strength meter (#1381). Different from
  // the public surfaces because the meter lives inside a collapsible
  // "Change password" panel and the new-password field has a sibling
  // current-password field; we toggle the panel + fill current-password
  // first so the page looks lived-in.
  try {
    console.log("-> /profile/edit (meter empty)")
    await page.goto(`${BASE_URL}/profile/edit`, { waitUntil: "domcontentloaded" })
    await settle()
    await page.click('[data-testid="password-toggle"]')
    await page.waitForSelector('[data-testid="change-password-form"]', { timeout: 5000 })
    await page.fill('[data-testid="current-password"]', "old-password-here")
    await settle(200)
    await shoot("20b-profile-edit-empty")

    console.log('-> /profile/edit (meter weak: "password")')
    await page.fill('[data-testid="new-password"]', "password")
    await waitForMeterScore("change-password-strength")
    await shoot("20c-profile-edit-weak")

    console.log("-> /profile/edit (meter strong: high-entropy passphrase)")
    await page.fill('[data-testid="new-password"]', "ZebraNectar7Tundra!Ocean3Quiver")
    await waitForMeterScore("change-password-strength")
    await shoot("20d-profile-edit-strong")
  } catch (err) {
    console.warn(`   profile-edit meter capture failed (${err.message}); skipping 20b-d`)
  }

  console.log("-> /settings")
  await page.goto(`${BASE_URL}/settings`, { waitUntil: "domcontentloaded" })
  await settle()
  await shoot("21-settings")

  // Global Files page (#1411): list + 5 category tiles + Upload
  // dialog three-step flow + file detail sheet + file edit page +
  // category-filtered list. After the entity captures above this
  // page already has a couple of files in it (one attached to a
  // commodity, one to a location), so the empty-state shot doesn't
  // apply here — we lean on the entity-detail panel shots
  // (13-location-detail, 18b-commodity-detail-files) for the empty
  // surface and skip a 30-files-empty shot.
  // Fall back to the captured `slug` from after-login: by this
  // point in the run we've navigated to /settings (no group prefix
  // in the URL), so slugFromUrl() returns null on its own.
  const slugForFiles = slugFromUrl() ?? slug
  if (slugForFiles) {
    console.log(`-> /g/${slugForFiles}/files`)
    await page.goto(`${BASE_URL}/g/${slugForFiles}/files`, {
      waitUntil: "domcontentloaded",
    })
    await settle()
    await shoot("30-files-list")

    // Step 31 — open the global Upload dialog (no linked entity, so
    // the title reads "Upload files" instead of "Attach files to …").
    const uploadCta = page.locator('[data-testid="files-upload-cta"]').first()
    if ((await uploadCta.count()) > 0) {
      try {
        await uploadCta.click()
        await page.waitForSelector('[data-testid="files-upload-dialog"]', {
          timeout: 5000,
        })
        await settle()
        await shoot("31-files-upload-step1", false)

        // Feed a file through the hidden input, advance to step 2.
        await page.setInputFiles('[data-testid="files-upload-input"]', {
          name: "screenshot-fixture.png",
          mimeType: "image/png",
          buffer: TINY_PNG,
        })
        await page.waitForSelector('[data-testid="files-upload-list"]', { timeout: 3000 })
        await page.click('[data-testid="files-upload-next"]')
        await page.waitForSelector('[data-testid="files-upload-metadata-list"]', {
          timeout: 3000,
        })
        await settle(200)
        await shoot("32-files-upload-step2-metadata", false)

        // Step 3 — kick off the upload, capture mid/post-progress.
        await page.click('[data-testid="files-upload-start"]')
        // Wait for the progress bar to render so the shot has its
        // bar visible; one-file uploads finish near-instantly so by
        // the time we screenshot the bar is full and "Close" enabled.
        await page.waitForSelector('[data-testid="files-upload-progress"]', {
          timeout: 3000,
        })
        await page.waitForSelector(
          '[data-testid^="files-upload-progress-item-"][data-status="done"]',
          { timeout: 15000 },
        )
        await settle(200)
        await shoot("33-files-upload-step3-progress", false)

        // Close the dialog → list refetches → 34 captures the new
        // tile counts + grid card for the just-uploaded file.
        await page.click('[data-testid="files-upload-close"]')
        await page.waitForSelector('[data-testid="files-grid"]', { timeout: 5000 })
        await settle(400)
        await shoot("34-files-populated")
      } catch (err) {
        console.warn(`   files upload flow failed (${err.message}); skipping 31-34`)
        await page.keyboard.press("Escape").catch(() => {})
        await settle(200)
      }
    } else {
      console.warn("   files-upload-cta missing; skipping 31-34")
    }

    // Step 35 — click the first file card to open the detail sheet
    // (deep-links to /files/:id and renders FileDetailSheet).
    const firstFileCard = page.locator('[data-testid^="file-card-open-"]').first()
    if ((await firstFileCard.count()) > 0) {
      try {
        await firstFileCard.click()
        await page.waitForSelector('[data-testid="file-detail-sheet"]', { timeout: 5000 })
        await settle(300)
        await shoot("35-file-detail-sheet")

        // Step 36 — Edit metadata navigates to /files/:id/edit.
        const editBtn = page.locator('[data-testid="file-detail-edit"]')
        if ((await editBtn.count()) > 0) {
          await editBtn.click()
          // FileEditPage renders the same h1 "Edit file" + form;
          // wait for the page to swap before screenshotting.
          await page.waitForURL((u) => /\/files\/[^/]+\/edit$/.test(u.toString()), {
            timeout: 5000,
          })
          await settle(300)
          await shoot("36-file-edit")
        }
      } catch (err) {
        console.warn(`   file detail/edit flow failed (${err.message}); skipping 35-36`)
      }
    }

    // Step 37 — invoices-filtered list (likely empty since the
    // fixture file landed under "photos"; the empty-filtered card
    // is itself a meaningful state worth capturing).
    try {
      await page.goto(`${BASE_URL}/g/${slugForFiles}/files?category=invoices`, {
        waitUntil: "domcontentloaded",
      })
      await settle()
      await shoot("37-files-filtered-invoices")
    } catch (err) {
      console.warn(`   filtered-invoices shot failed (${err.message}); skipping`)
    }
  }

  // Tags page (#1412): empty state + create dialog + populated list +
  // edit dialog + both delete-confirm variants (not-in-use vs in-use
  // with force-delete). The in-use case requires a real BE side-effect
  // (attach the tag to a commodity) — done inside page.evaluate so the
  // fetch picks up the React app's auth token from localStorage.
  //
  // Slug naming: the seed + prior runs may already have a `kitchen`
  // slug auto-created from commodity tags arrays, so the BE returns a
  // non-zero usage count even though we just created the tag fresh.
  // To capture the not-in-use confirm dialog we use a per-run unique
  // slug (`scratch-<ts>`) that the seed never references; for the
  // in-use shot we deliberately reuse `kitchen` so the dialog renders
  // a realistic count + plural copy.
  const slugForTags = slugFromUrl() ?? slug
  if (slugForTags) {
    const scratchLabel = `Scratch ${Date.now()}`
    const scratchSlug = scratchLabel.toLowerCase().replace(/[^a-z0-9]+/g, "-")

    console.log(`-> /g/${slugForTags}/tags`)
    await page.goto(`${BASE_URL}/g/${slugForTags}/tags`, { waitUntil: "domcontentloaded" })
    await settle()
    await shoot("40-tags-list")

    // Step 41 — open the create dialog. Type a label so the slug
    // auto-derives + picks a color so the preview pill renders.
    try {
      await page.click('[data-testid="tags-create-button"]')
      await page.waitForSelector('[data-testid="tag-form-dialog"]', { timeout: 5000 })
      await page.fill('[data-testid="tag-form-label"]', scratchLabel)
      await page.click('[data-testid="tag-form-color-amber"]')
      await settle(200)
      await shoot("41-tags-create-dialog", false)
      await page.click('[data-testid="tag-form-submit"]')
      await page.waitForSelector('[data-testid="tag-form-dialog"]', {
        state: "detached",
        timeout: 5000,
      })
      await page.waitForSelector(`[data-testid="tag-row-${scratchSlug}"]`, { timeout: 5000 })
      await settle()
      await shoot("42-tags-list-populated")
    } catch (err) {
      console.warn(`   tags create flow failed (${err.message}); skipping 41-42`)
      await page.keyboard.press("Escape").catch(() => {})
      await settle(200)
    }

    // Step 43 — edit dialog with prefilled values (the slug auto-
    // derive is suppressed in edit mode, so this shot exercises that
    // path explicitly).
    try {
      const editBtn = page.locator(`[data-testid="tag-row-${scratchSlug}-edit"]`)
      if ((await editBtn.count()) > 0) {
        await editBtn.click()
        await page.waitForSelector('[data-testid="tag-form-dialog"]', { timeout: 5000 })
        await settle(200)
        await shoot("43-tags-edit-dialog", false)
        await page.click('[data-testid="tag-form-cancel"]')
        await page.waitForSelector('[data-testid="tag-form-dialog"]', {
          state: "detached",
          timeout: 5000,
        })
        await settle(200)
      }
    } catch (err) {
      console.warn(`   tags edit dialog failed (${err.message}); skipping 43`)
      await page.keyboard.press("Escape").catch(() => {})
      await settle(200)
    }

    // Step 44 — delete-confirm for a NOT-in-use tag. The scratch tag
    // we just created has zero usage, so this surfaces the simple
    // "Delete tag?" / "This cannot be undone." dialog. We cancel so
    // the row survives for step 45.
    try {
      const deleteBtn = page.locator(`[data-testid="tag-row-${scratchSlug}-delete"]`)
      if ((await deleteBtn.count()) > 0) {
        await deleteBtn.click()
        await page.waitForSelector('[data-testid="confirm-dialog"]', { timeout: 5000 })
        await settle(200)
        await shoot("44-tags-delete-confirm", false)
        await page.click('[data-testid="confirm-cancel"]')
        await page.waitForSelector('[data-testid="confirm-dialog"]', {
          state: "detached",
          timeout: 5000,
        })
        await settle(200)
      }
    } catch (err) {
      console.warn(`   tags delete-confirm failed (${err.message}); skipping 44`)
      await page.keyboard.press("Escape").catch(() => {})
      await settle(200)
    }

    // Step 45 — in-use delete-confirm with the "Force delete" button.
    // Attach the scratch slug to the first commodity via direct fetch
    // (re-using the React app's stored auth + CSRF tokens), reload,
    // then click delete on the row to trigger the in-use confirm.
    try {
      const attachResult = await page.evaluate(
        async ({ slug, tagSlug }) => {
          const token = localStorage.getItem("inventario_token")
          const csrf = sessionStorage.getItem("inventario_csrf_token")
          if (!token || !csrf) return { ok: false, reason: "no auth tokens in storage" }
          const headers = {
            "Content-Type": "application/vnd.api+json",
            Accept: "application/vnd.api+json",
            Authorization: `Bearer ${token}`,
            "X-CSRF-Token": csrf,
          }
          const listResp = await fetch(`/api/v1/g/${slug}/commodities`, { headers })
          if (!listResp.ok) return { ok: false, reason: `list ${listResp.status}` }
          const listBody = await listResp.json()
          const first = listBody?.data?.[0]
          if (!first?.id) return { ok: false, reason: "no commodity to attach to" }
          const detailResp = await fetch(`/api/v1/g/${slug}/commodities/${first.id}`, {
            headers,
          })
          if (!detailResp.ok) return { ok: false, reason: `detail ${detailResp.status}` }
          const detail = await detailResp.json()
          const attrs = detail?.data?.attributes ?? {}
          const tags = Array.from(new Set([...(attrs.tags ?? []), tagSlug]))
          const putResp = await fetch(`/api/v1/g/${slug}/commodities/${first.id}`, {
            method: "PUT",
            headers,
            body: JSON.stringify({
              data: {
                id: first.id,
                type: "commodities",
                attributes: { ...attrs, tags },
              },
            }),
          })
          if (!putResp.ok) return { ok: false, reason: `put ${putResp.status}` }
          return { ok: true }
        },
        { slug: slugForTags, tagSlug: scratchSlug },
      )
      if (!attachResult.ok) {
        console.warn(
          `   could not attach scratch slug to a commodity (${attachResult.reason}); 45 will mirror 44`,
        )
      } else {
        // Reload so the page re-fetches usage and the row reflects "1 item".
        await page.reload()
        await page.waitForSelector(`[data-testid="tag-row-${scratchSlug}"]`, { timeout: 5000 })
        await settle(300)
      }

      const deleteBtn = page.locator(`[data-testid="tag-row-${scratchSlug}-delete"]`)
      if ((await deleteBtn.count()) > 0) {
        await deleteBtn.click()
        await page.waitForSelector('[data-testid="confirm-dialog"]', { timeout: 5000 })
        await settle(200)
        await shoot("45-tags-delete-in-use-confirm", false)
        await page.click('[data-testid="confirm-cancel"]')
        await page.waitForSelector('[data-testid="confirm-dialog"]', {
          state: "detached",
          timeout: 5000,
        })
        await settle(200)
      }
    } catch (err) {
      console.warn(`   tags in-use confirm failed (${err.message}); skipping 45`)
      await page.keyboard.press("Escape").catch(() => {})
      await settle(200)
    }
  }

  // ---- Currency migration (#1553) ---------------------------------
  // Captures the four-step wizard + the host /groups/:id/settings page
  // after the migrate CTA + history list landed. Skipped when the
  // FEATURE_CURRENCY_MIGRATION flag is off (the CTA disabled state is
  // covered by the legacy settings shot above).
  try {
    // The post-login session is held in localStorage (Bearer) +
    // sessionStorage (CSRF); cookies alone aren't enough for the
    // group-list endpoint to authorize. Read both back so the
    // page.request call picks up the same token the FE uses.
    const sessionToken = await page.evaluate(
      () => localStorage.getItem("inventario_token") || "",
    )
    const groupsResp = await page.request
      .get(`${BASE_URL}/api/v1/groups`, {
        headers: {
          Accept: "application/vnd.api+json",
          Authorization: sessionToken ? `Bearer ${sessionToken}` : "",
        },
      })
      .catch(() => null)
    const groupsBody = groupsResp ? await groupsResp.json().catch(() => null) : null
    const groupId = groupsBody?.data?.[0]?.id
    if (!groupId) {
      console.warn("   no admin group available; skipping 50-55 currency-migration shots")
    } else {
      console.log("-> 50: group settings (with migrate CTA + migrations history)")
      await page.goto(`${BASE_URL}/groups/${groupId}/settings`, {
        waitUntil: "domcontentloaded",
      })
      await page.waitForSelector('[data-testid="group-settings-page"]', { timeout: 10000 })
      await settle()
      await shoot("50-group-settings")

      const migrateBtn = page.locator('[data-testid="migrate-currency-open"]')
      if ((await migrateBtn.count()) > 0 && (await migrateBtn.isEnabled())) {
        console.log("-> 51: wizard step 1 (target currency)")
        await migrateBtn.click()
        await page.waitForSelector('[data-testid="migrate-currency-dialog"]', {
          state: "visible",
          timeout: 5000,
        })
        await settle(300)
        await shoot("51-migrate-currency-step1", false)

        // Step 1 → 2 needs a target currency picked. The combobox lives
        // inside the dialog; pick EUR via the data-currency-code marker.
        try {
          await page.click('[data-testid="migrate-currency-dialog"] [role="combobox"]')
          await page.waitForSelector('[data-currency-code="EUR"]', { timeout: 5000 })
          await page.click('[data-currency-code="EUR"]')
          await settle(200)
          await page.click('[data-testid="wizard-next"]')
          await page.waitForSelector('[data-testid="wizard-rate-input"]', { timeout: 5000 })
          await settle(200)
          console.log("-> 52: wizard step 2 (rate input)")
          await shoot("52-migrate-currency-step2", false)

          await page.fill('[data-testid="wizard-rate-input"]', "0.9")
          await settle(200)
          await page.click('[data-testid="wizard-preview"]')
          await page.waitForSelector('[data-testid="wizard-total-before"]', { timeout: 10000 })
          await settle(300)
          console.log("-> 53: wizard step 3 (preview totals)")
          await shoot("53-migrate-currency-step3-preview", false)

          await page.click('[data-testid="wizard-confirm"]')
          await page.waitForSelector('[data-testid="wizard-confirm-input"]', { timeout: 5000 })
          await settle(200)
          console.log("-> 54: wizard step 4 (type-to-confirm)")
          await shoot("54-migrate-currency-step4-confirm", false)

          // Cancel out — we don't want the screenshot pass to actually
          // start a migration that locks the demo group for 10 minutes.
          await page.click('[data-testid="wizard-cancel"]')
          await page.waitForSelector('[data-testid="migrate-currency-dialog"]', {
            state: "detached",
            timeout: 5000,
          })
          await settle(200)
        } catch (innerErr) {
          console.warn(`   wizard advance failed (${innerErr.message}); leaving dialog as-is`)
          await page.keyboard.press("Escape").catch(() => {})
          await settle(200)
        }
      } else {
        console.warn("   migrate CTA not present (FEATURE_CURRENCY_MIGRATION off?); skipping 51-54")
      }
    }
  } catch (err) {
    console.warn(`   currency-migration shots failed (${err.message}); skipping 50-54`)
    await page.keyboard.press("Escape").catch(() => {})
    await settle(200)
  }
} catch (err) {
  console.error("Screenshot run failed:", err)
  process.exitCode = 1
} finally {
  await browser.close()
}

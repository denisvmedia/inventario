// One-off screenshot script for #1381 / PR #1583 — captures the password
// strength meter at three states (empty / weak / strong) on each of the
// three surfaces it integrates with: register, reset-password, profile-edit.
//
// Usage:
//   1. Make sure a built binary is running with --no-*-rate-limit and the
//      DB has been seeded (see e2e/screenshots.mjs header for the full
//      build incantation). The default targets :3334 to avoid clashing
//      with whatever's on :3333 in this worktree.
//   2. node e2e/screenshots-1381.mjs

import { chromium } from "playwright"
import { mkdirSync } from "fs"
import { join, dirname } from "path"
import { fileURLToPath } from "url"

const BASE_URL = process.env.BASE_URL || "http://localhost:3334"
const __filename = fileURLToPath(import.meta.url)
const OUT = process.env.OUT || join(dirname(__filename), "..", ".research", "screenshots", "pr-1583")
const EMAIL = process.env.EMAIL || "admin@test-org.com"
const PASSWORD = process.env.PASSWORD || "testpassword123"

mkdirSync(OUT, { recursive: true })

const browser = await chromium.launch({ headless: true })
const context = await browser.newContext({ viewport: { width: 720, height: 900 } })
const page = await context.newPage()

page.on("pageerror", (err) => console.error("PAGE ERROR:", err.message))
page.on("console", (msg) => {
  if (msg.type() === "error") console.error("CONSOLE ERR:", msg.text())
})

async function settle(ms = 600) {
  try {
    await page.waitForLoadState("networkidle", { timeout: 5000 })
  } catch {}
  await page.waitForTimeout(ms)
}

async function shoot(name) {
  const file = join(OUT, `${name}.png`)
  await page.screenshot({ path: file, fullPage: true })
  console.log(`   saved ${file}`)
}

// Wait for zxcvbn-ts dynamic chunks to load + the meter to settle on its
// post-zxcvbn score. Heuristic shows up first; we want the real one in
// the screenshot. Two settle ticks usually does it on a warm page; the
// first non-empty password triggers the lazy load (~200-400 ms cold).
async function waitForRealScore(testId) {
  // Give the dynamic import a hard second on the cold first-call path.
  await page.waitForTimeout(800)
  // Then settle network so the chunks are definitely flushed.
  await settle(200)
  // Sanity check that the meter root rendered at all.
  await page.waitForSelector(`[data-testid="${testId}"]`, { timeout: 5000 })
}

async function captureRegister() {
  console.log("-> /register (empty)")
  await page.goto(`${BASE_URL}/register`, { waitUntil: "domcontentloaded" })
  await settle()
  // Pre-fill name + email so the meter can use them as zxcvbn userInputs
  // and so the form looks lived-in. Leaves the password field empty.
  await page.fill('input[data-testid="name"]', "Alex Johnson")
  await page.fill('input[data-testid="email"]', "alex@example.com")
  await settle(200)
  await shoot("01-register-empty")

  console.log("-> /register (weak: \"password\")")
  await page.fill('input[data-testid="password"]', "password")
  await waitForRealScore("register-password-strength")
  await shoot("02-register-weak")

  console.log("-> /register (strong: high-entropy passphrase)")
  await page.fill('input[data-testid="password"]', "ZebraNectar7Tundra!Ocean3Quiver")
  await waitForRealScore("register-password-strength")
  await shoot("03-register-strong")
}

async function captureReset() {
  // The reset page requires a token. With memory:// we don't have one
  // pre-seeded, but the page renders the form anyway when ?token=… is
  // present and only validates it on submit, so a fake token is enough
  // to surface the form + meter. The "missing-token" branch is captured
  // separately by the default screenshots.mjs run.
  const fakeToken = "screenshot-only-not-a-real-token"
  console.log("-> /reset-password (empty)")
  await page.goto(`${BASE_URL}/reset-password?token=${fakeToken}`, {
    waitUntil: "domcontentloaded",
  })
  await settle()
  await shoot("04-reset-empty")

  console.log("-> /reset-password (weak: \"password\")")
  await page.fill('input[data-testid="password"]', "password")
  await waitForRealScore("reset-password-strength")
  await shoot("05-reset-weak")

  console.log("-> /reset-password (strong: high-entropy passphrase)")
  await page.fill('input[data-testid="password"]', "ZebraNectar7Tundra!Ocean3Quiver")
  await waitForRealScore("reset-password-strength")
  await shoot("06-reset-strong")
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
  await page.waitForURL((u) => !u.toString().includes("/login"), { timeout: 15000 })
  await settle()
}

async function captureProfileEdit() {
  console.log("-> /profile/edit (open password panel, empty)")
  await page.goto(`${BASE_URL}/profile/edit`, { waitUntil: "domcontentloaded" })
  await settle()
  await page.click('[data-testid="password-toggle"]')
  await page.waitForSelector('[data-testid="change-password-form"]')
  // Fill the current-password so the form looks realistic; the meter is
  // gated on `newPassword` so this doesn't activate it yet.
  await page.fill('[data-testid="current-password"]', "old-password-here")
  await settle(200)
  await shoot("07-profile-empty")

  console.log("-> /profile/edit (weak: \"password\")")
  await page.fill('[data-testid="new-password"]', "password")
  await waitForRealScore("change-password-strength")
  await shoot("08-profile-weak")

  console.log("-> /profile/edit (strong: high-entropy passphrase)")
  await page.fill('[data-testid="new-password"]', "ZebraNectar7Tundra!Ocean3Quiver")
  await waitForRealScore("change-password-strength")
  await shoot("09-profile-strong")
}

try {
  await captureRegister()
  await captureReset()
  await login()
  await captureProfileEdit()
} catch (err) {
  console.error("Screenshot run failed:", err)
  process.exitCode = 1
} finally {
  await browser.close()
}

#!/usr/bin/env node
// CI guard: ensures every t("...") in code has a key in src/i18n/locales/en/*
// (the canonical source). Runs `i18next-cli extract --ci`, which exits
// non-zero when any catalog file would have been updated. We then run
// Prettier + a final `git diff --exit-code` to catch the rare case where
// the CLI rewrote the file with a different formatting (different
// indentation, key sort order, etc.) that the CLI itself didn't flag.
//
// Local dev: run `npm run i18n:extract` to apply the diff and write a
// scaffold key into en/<ns>.json that you then fill in by hand.

import { spawnSync } from "node:child_process"
import process from "node:process"
import { fileURLToPath } from "node:url"
import path from "node:path"

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)
const repoRoot = path.resolve(__dirname, "..")

function run(cmd, args, options = {}) {
  const result = spawnSync(cmd, args, {
    stdio: "inherit",
    cwd: repoRoot,
    shell: process.platform === "win32",
    ...options,
  })
  if (result.status !== 0) {
    process.exit(result.status ?? 1)
  }
}

function captureGitDiff(targetPath) {
  const result = spawnSync("git", ["diff", "--", targetPath], {
    cwd: repoRoot,
    encoding: "utf8",
  })
  if (result.status !== 0) {
    console.error(result.stderr || "git diff failed")
    process.exit(result.status ?? 1)
  }
  return result.stdout
}

console.log("[i18n] running i18next-cli extract --ci…")
run("npx", ["--no-install", "i18next-cli", "extract", "--ci"])

console.log("[i18n] formatting catalogs with Prettier…")
run("npx", [
  "--no-install",
  "prettier",
  "--write",
  "--log-level",
  "error",
  "src/i18n/locales/**/*.json",
])

console.log("[i18n] checking for residual drift in src/i18n/locales/")
const diff = captureGitDiff("src/i18n/locales")
if (diff.trim()) {
  console.error(
    "\n[i18n] catalogs are out of sync with the keys used in source.\n" +
      "Run `npm run i18n:extract` locally, fill in the empty values in " +
      "src/i18n/locales/en/*.json, and commit the result.\n"
  )
  console.error(diff)
  process.exit(1)
}

console.log("[i18n] catalogs in sync ✓")

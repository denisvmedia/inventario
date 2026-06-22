#!/usr/bin/env node
// gen-versions.mjs — generate the docs version index for the gh-pages root.
//
// Given a directory (the gh-pages checkout containing one folder per published
// version), this:
//   (a) lists immediate subdirs that look like a version: `edge` or
//       `vX.Y` / `vX.Y.Z` (semver-ish, optional patch),
//   (b) computes the default = highest stable semver tag, else `edge`,
//   (c) writes `<dir>/versions.json` and `<dir>/index.html`
//       (meta-refresh + JS redirect to `<PAGES_PREFIX>/<default>/`).
//
// It is idempotent: running it repeatedly on the same tree yields byte-identical
// output. Node built-ins only — no dependencies.
//
// Usage:
//   node scripts/gen-versions.mjs <gh-pages-dir>
//   node scripts/gen-versions.mjs --selftest   # runs the in-process tests
//
// versions.json shape:
//   { "default": "edge", "versions": [ { "slug": "edge", "label": "edge" }, ... ] }
//
// Folder/URL mapping (must match astro.config.mjs `base`):
//   gh-pages/<ver>/  ->  https://denisvmedia.github.io/inventario/<ver>/
// so the redirect target and versions.json both live at the gh-pages ROOT,
// served from https://denisvmedia.github.io/inventario/.

import { existsSync, mkdtempSync, mkdirSync, readdirSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';

// The public path prefix the site is served under (the GitHub Pages project
// sub-path). Versions live as siblings beneath it. Keep in lockstep with
// astro.config.mjs `base` (which is `${PAGES_PREFIX}/<DOCS_VERSION>/`).
export const PAGES_PREFIX = '/inventario';

const EDGE = 'edge';
// vX.Y or vX.Y.Z (patch optional). No pre-release/build metadata — releases
// here are plain tags.
const VERSION_RE = /^v(\d+)\.(\d+)(?:\.(\d+))?$/;

/** Is `name` a recognised version folder? */
export function isVersionFolder(name) {
  return name === EDGE || VERSION_RE.test(name);
}

/** Parse a `vX.Y[.Z]` tag into a comparable [major, minor, patch] tuple, or null. */
export function parseSemver(name) {
  const m = VERSION_RE.exec(name);
  if (!m) return null;
  return [Number(m[1]), Number(m[2]), m[3] === undefined ? 0 : Number(m[3])];
}

/** Compare two semver tuples; returns >0 if a>b, <0 if a<b, 0 if equal. */
function cmpSemver(a, b) {
  for (let i = 0; i < 3; i++) {
    if (a[i] !== b[i]) return a[i] - b[i];
  }
  return 0;
}

/**
 * Compute the default version: highest stable semver tag among `slugs`, or
 * `edge` if there are no tags. Returns `edge` even when `edge` itself is
 * absent (callers always publish edge first), so the redirect never dangles
 * worse than the existing fallback.
 */
export function computeDefault(slugs) {
  let best = null;
  for (const slug of slugs) {
    const sv = parseSemver(slug);
    if (sv && (best === null || cmpSemver(sv, best.sv) > 0)) {
      best = { slug, sv };
    }
  }
  return best ? best.slug : EDGE;
}

/**
 * Build the versions.json object for a set of folder slugs.
 * Ordering: `edge` first (if present), then semver tags newest-first.
 */
export function buildIndex(slugs) {
  const tags = slugs
    .filter((s) => parseSemver(s))
    .sort((a, b) => cmpSemver(parseSemver(b), parseSemver(a)));
  const ordered = [];
  if (slugs.includes(EDGE)) ordered.push(EDGE);
  ordered.push(...tags);
  return {
    default: computeDefault(slugs),
    versions: ordered.map((slug) => ({ slug, label: slug })),
  };
}

/** Minimal HTML that redirects to the default version. */
export function renderRedirectHtml(defaultSlug) {
  const target = `${PAGES_PREFIX}/${defaultSlug}/`;
  return `<!doctype html>
<html lang="en">
  <head>
    <meta charset="utf-8" />
    <meta http-equiv="refresh" content="0; url=${target}" />
    <link rel="canonical" href="${target}" />
    <title>Inventario documentation</title>
    <script>location.replace(${JSON.stringify(target)});</script>
  </head>
  <body>
    <p>Redirecting to the <a href="${target}">Inventario documentation</a>&hellip;</p>
  </body>
</html>
`;
}

/** Stable, pretty JSON with a trailing newline (idempotent on re-run). */
function renderVersionsJson(index) {
  return JSON.stringify(index, null, 2) + '\n';
}

/**
 * Generate versions.json + index.html into `dir` from its version subfolders.
 * Returns the computed index for logging/testing.
 */
export function generate(dir) {
  const entries = readdirSync(dir, { withFileTypes: true });
  const slugs = entries
    .filter((e) => e.isDirectory() && isVersionFolder(e.name))
    .map((e) => e.name);
  const index = buildIndex(slugs);
  writeFileSync(join(dir, 'versions.json'), renderVersionsJson(index));
  writeFileSync(join(dir, 'index.html'), renderRedirectHtml(index.default));
  return index;
}

// --- self-test --------------------------------------------------------------
function selftest() {
  const assert = (cond, msg) => {
    if (!cond) {
      console.error(`FAIL: ${msg}`);
      process.exitCode = 1;
      throw new Error(msg);
    }
  };

  // Pure-function checks.
  assert(isVersionFolder('edge'), 'edge is a version folder');
  assert(isVersionFolder('v1.0.0'), 'v1.0.0 is a version folder');
  assert(isVersionFolder('v1.2'), 'v1.2 (no patch) is a version folder');
  assert(!isVersionFolder('latest'), 'latest is not a version folder');
  assert(!isVersionFolder('_astro'), '_astro is not a version folder');
  assert(!isVersionFolder('v1'), 'v1 (no minor) is not a version folder');

  assert(computeDefault(['edge']) === 'edge', 'no tags -> default edge');
  assert(
    computeDefault(['edge', 'v1.0.0', 'v1.2.0']) === 'v1.2.0',
    'highest minor wins',
  );
  assert(
    computeDefault(['edge', 'v1.2.0', 'v1.2.10', 'v1.2.2']) === 'v1.2.10',
    'numeric (not lexical) patch comparison',
  );
  assert(
    computeDefault(['v2.0.0', 'v10.0.0', 'edge']) === 'v10.0.0',
    'numeric major comparison',
  );

  const idx = buildIndex(['v1.0.0', 'edge', 'v1.2.0']);
  assert(idx.default === 'v1.2.0', 'sample default v1.2.0');
  assert(idx.versions[0].slug === 'edge', 'edge listed first');
  assert(idx.versions[1].slug === 'v1.2.0', 'newest tag second');
  assert(idx.versions[2].slug === 'v1.0.0', 'older tag last');

  // Idempotent filesystem round-trip on a sample {edge, v1.0.0, v1.2.0}.
  const tmp = mkdtempSync(join(tmpdir(), 'gen-versions-'));
  try {
    for (const v of ['edge', 'v1.0.0', 'v1.2.0', '_astro', 'assets']) {
      mkdirSync(join(tmp, v));
    }
    const first = generate(tmp);
    assert(first.default === 'v1.2.0', 'fs sample default v1.2.0');
    const json1 = readFileSync(join(tmp, 'versions.json'), 'utf8');
    const html1 = readFileSync(join(tmp, 'index.html'), 'utf8');
    assert(html1.includes('/inventario/v1.2.0/'), 'redirect points at default');
    assert(!json1.includes('_astro') && !json1.includes('assets'), 'non-version dirs ignored');
    // Re-run must be byte-identical (idempotent).
    generate(tmp);
    const json2 = readFileSync(join(tmp, 'versions.json'), 'utf8');
    const html2 = readFileSync(join(tmp, 'index.html'), 'utf8');
    assert(json1 === json2 && html1 === html2, 'idempotent on re-run');

    console.log('versions.json for {edge, v1.0.0, v1.2.0}:');
    console.log(json1);
    console.log('index.html redirect target: /inventario/v1.2.0/');
  } finally {
    rmSync(tmp, { recursive: true, force: true });
  }

  if (!process.exitCode) console.log('gen-versions.mjs --selftest: OK');
}

// --- CLI --------------------------------------------------------------------
function main() {
  const arg = process.argv[2];
  if (arg === '--selftest') {
    selftest();
    return;
  }
  if (!arg) {
    console.error('usage: node scripts/gen-versions.mjs <gh-pages-dir> | --selftest');
    process.exitCode = 2;
    return;
  }
  if (!existsSync(arg)) {
    console.error(`error: directory not found: ${arg}`);
    process.exitCode = 2;
    return;
  }
  const index = generate(arg);
  console.log(`wrote ${join(arg, 'versions.json')} (default=${index.default}, versions=${index.versions.map((v) => v.slug).join(', ')})`);
}

// Run only when invoked directly (not when imported for tests).
import { fileURLToPath } from 'node:url';
if (process.argv[1] && fileURLToPath(import.meta.url) === process.argv[1]) {
  main();
}

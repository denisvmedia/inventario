// Assemble the static API reference into public/api/ before Astro builds.
//
// The OpenAPI spec is generated from the Go source by `make swagger` and lives
// at go/docs/swagger.yaml. Copying it in at build time rather than committing a
// second copy means the published reference is always the spec this checkout
// carries — which, because CI builds the site once per version tag, gives a
// per-version API reference for free.
//
// Redoc is vendored from node_modules rather than loaded from a CDN. The repo
// pins its GitHub Actions to commit SHAs for supply-chain reasons; pulling an
// unpinned script from a CDN into its own documentation would undo that, and
// it would also leave the page blank for anyone reading offline.
import { copyFile, mkdir, access } from "node:fs/promises";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const here = dirname(fileURLToPath(import.meta.url));
const repoRoot = resolve(here, "../../..");
const outDir = resolve(here, "../public/api");

const spec = resolve(repoRoot, "go/docs/swagger.yaml");
const redoc = resolve(here, "../node_modules/redoc/bundles/redoc.standalone.js");

async function requireFile(path, hint) {
  try {
    await access(path);
  } catch {
    console.error(`build-api-reference: missing ${path}\n  ${hint}`);
    process.exit(1);
  }
}

await requireFile(spec, "Run `make swagger` from the repo root to generate it.");
await requireFile(redoc, "Run `npm install` in docs/site.");

await mkdir(outDir, { recursive: true });
await copyFile(spec, resolve(outDir, "swagger.yaml"));
await copyFile(redoc, resolve(outDir, "redoc.standalone.js"));

console.log("build-api-reference: public/api/ assembled");

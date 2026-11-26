import fs from "node:fs"
import type { Plugin } from "vite"

import {
  describeBuild,
  parseChangelog,
  selectReleases,
  whatsNewMeta,
} from "../src/lib/changelog.ts"

// Build-time source of the What's new page (#1386). Reads the repository's
// CHANGELOG.md, keeps the releases the build contains (see
// `selectReleases`), and serves them as two virtual modules:
//
//   virtual:whats-new/meta      { build, latestVersion, latestUserVersion }
//                               for the unread dot,
//                               small enough for the entry chunk
//   virtual:whats-new/releases  the parsed releases, imported only by the
//                               lazy What's new page
//
// The Unreleased section never reaches the bundle. The build fails when the
// file cannot be read: a bundle without its release notes is a packaging
// mistake (the Dockerfile has to copy the file in), not an empty page.

const META_ID = "virtual:whats-new/meta"
const RELEASES_ID = "virtual:whats-new/releases"

// Relative links in the changelog (CONTRIBUTING.md, release notes files)
// point into the repository.
const REPOSITORY_BLOB_URL = "https://github.com/denisvmedia/inventario/blob/master/"

export interface WhatsNewPluginOptions {
  // Absolute path of CHANGELOG.md.
  changelogPath: string
  // The version the build is stamped with (INVENTARIO_VERSION), the same
  // string `__APP_VERSION__` carries.
  version: string
}

export function whatsNewPlugin({ changelogPath, version }: WhatsNewPluginOptions): Plugin {
  const resolved = new Map([
    [META_ID, `\0${META_ID}`],
    [RELEASES_ID, `\0${RELEASES_ID}`],
  ])

  const read = () => {
    let markdown: string
    try {
      markdown = fs.readFileSync(changelogPath, "utf8")
    } catch (err) {
      throw new Error(
        `whats-new: cannot read ${changelogPath}, which the What's new page is built from: ${String(err)}`,
        { cause: err }
      )
    }
    const build = describeBuild(version)
    const releases = selectReleases(
      parseChangelog(markdown, { linkBase: REPOSITORY_BLOB_URL }),
      build
    )
    return { build, releases }
  }

  return {
    name: "inventario-whats-new",
    resolveId(id) {
      return resolved.get(id)
    },
    load(id) {
      if (id !== resolved.get(META_ID) && id !== resolved.get(RELEASES_ID)) return null
      // Rebuild the module when the file changes under `vite` dev.
      this.addWatchFile(changelogPath)
      const { build, releases } = read()
      if (id === resolved.get(META_ID)) {
        return `export default ${JSON.stringify(whatsNewMeta(build, releases))}`
      }
      return `export default ${JSON.stringify(releases)}`
    },
  }
}

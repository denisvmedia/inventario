// The virtual modules `vite-plugins/whats-new.ts` serves (#1386).

declare module "virtual:whats-new/meta" {
  import type { WhatsNewMeta } from "@/lib/changelog"

  const meta: WhatsNewMeta
  export default meta
}

declare module "virtual:whats-new/releases" {
  import type { ChangelogRelease } from "@/lib/changelog"

  const releases: ChangelogRelease[]
  export default releases
}

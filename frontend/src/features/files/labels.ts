import { useTranslation } from "react-i18next"

import type { FileCategoryTile } from "./constants"

// i18next-cli's static-key extractor can't see template-literal keys
// like `t(\`files:${tile.descriptionI18nKey}\`)`, so the catalogue
// drifts every CI run. These helpers shape every dynamic key as an
// explicit `t()` call inside a switch, mirroring the pre-existing
// `useCategoryLabel` pattern. Keep new files-page i18n lookups going
// through these hooks (or add another switch like them) to keep the
// extractor happy.

export function useCategoryLabel(): (key: FileCategoryTile) => string {
  const { t } = useTranslation()
  return (key) => {
    switch (key) {
      case "all":
        return t("files:categoryAll", { defaultValue: "All" })
      case "images":
        return t("files:categoryImages", { defaultValue: "Images" })
      case "documents":
        return t("files:categoryDocuments", { defaultValue: "Documents" })
      case "other":
        return t("files:categoryOther", { defaultValue: "Other" })
    }
  }
}

export function useCategoryDescription(): (key: FileCategoryTile) => string {
  const { t } = useTranslation()
  return (key) => {
    switch (key) {
      case "all":
        return t("files:descriptionAll", {
          defaultValue: "Every file attached to your inventory",
        })
      case "images":
        return t("files:descriptionImages", {
          defaultValue: "Item photos — shown on cards and in galleries",
        })
      case "documents":
        return t("files:descriptionDocuments", {
          defaultValue: "Receipts, manuals, warranties, certificates",
        })
      case "other":
        return t("files:descriptionOther", {
          defaultValue: "Backups and miscellaneous files",
        })
    }
  }
}

// Curated tag-pill ids. Mirrors FILE_TAG_PILLS in constants — kept in
// sync by hand because the static-extractor needs literal keys to find
// in the source.
export type FileTagPillId = "invoice" | "warranty" | "manual" | "photo" | "certificate" | "backup"

export function useTagPillLabel(): (id: FileTagPillId) => string {
  const { t } = useTranslation()
  return (id) => {
    switch (id) {
      case "invoice":
        return t("files:tagInvoice", { defaultValue: "Invoice" })
      case "warranty":
        return t("files:tagWarranty", { defaultValue: "Warranty" })
      case "manual":
        return t("files:tagManual", { defaultValue: "Manual" })
      case "photo":
        return t("files:tagPhoto", { defaultValue: "Photo" })
      case "certificate":
        return t("files:tagCertificate", { defaultValue: "Certificate" })
      case "backup":
        return t("files:tagBackup", { defaultValue: "Backup" })
    }
  }
}

// useLinkTypeLabel names the kind of entity a file is attached to.
export function useLinkTypeLabel(): (entityType: string | undefined) => string {
  const { t } = useTranslation()
  return (entityType) => {
    switch (entityType) {
      case "commodity":
        return t("files:links.type.commodity", { defaultValue: "Item" })
      case "location":
        return t("files:links.type.location", { defaultValue: "Location" })
      case "area":
        return t("files:links.type.area", { defaultValue: "Area" })
      case "draft":
        return t("files:links.type.draft", { defaultValue: "Draft" })
      case "intake":
        return t("files:links.type.intake", { defaultValue: "Added from files" })
      default:
        return entityType ?? ""
    }
  }
}

// useLinkName is the name a file's link shows: the entity's name, or its
// kind when the name is unknown. A draft reads "Draft: <name>", so it is not
// taken for an item with the same name. The server names an open draft only;
// a link to a draft being discarded is left out of the file.
export function useLinkName(): (link: { entity_type?: string; entity_name?: string }) => string {
  const { t } = useTranslation()
  const typeLabel = useLinkTypeLabel()
  return (link) => {
    if (link.entity_type === "draft" && link.entity_name) {
      return t("files:links.draftName", {
        name: link.entity_name,
        defaultValue: "Draft: {{name}}",
      })
    }
    return link.entity_name || typeLabel(link.entity_type)
  }
}

// useLinkSummary is the short "attached to" text of a file card or row: the
// name of the first entity (its kind when the name is unknown), plus how
// many more there are. Empty for a standalone file.
export function useLinkSummary(): (
  links: { entity_type?: string; entity_name?: string }[]
) => string {
  const { t } = useTranslation()
  const linkName = useLinkName()
  return (links) => {
    const [head] = links
    if (!head) return ""
    const first = linkName(head)
    if (links.length === 1) return first
    return t("files:links.summary", {
      first,
      count: links.length - 1,
      defaultValue: "{{first}} +{{count}}",
    })
  }
}

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
      case "invoices":
        return t("files:categoryInvoices", { defaultValue: "Invoices" })
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
      case "invoices":
        return t("files:descriptionInvoices", {
          defaultValue: "Purchase receipts for insurance and reports",
        })
      case "documents":
        return t("files:descriptionDocuments", {
          defaultValue: "Manuals, warranties, certificates",
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

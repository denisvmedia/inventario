import type { TFunction } from "i18next"

// The label of a job type. Spelled out per type so the i18n extractor sees
// every key; an unknown type from a newer BE shows as it is.
export function failedJobTypeLabel(t: TFunction, type: string | undefined): string {
  switch (type) {
    case "export":
      return t("common:failedJobs.types.export")
    case "import":
      return t("common:failedJobs.types.import")
    case "restore":
      return t("common:failedJobs.types.restore")
    case "thumbnail":
      return t("common:failedJobs.types.thumbnail")
    case "intake":
      return t("common:failedJobs.types.intake")
    case "currency-migration":
      return t("common:failedJobs.types.currencyMigration")
    default:
      return type ?? "—"
  }
}

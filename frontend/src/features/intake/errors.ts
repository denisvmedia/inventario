import { useTranslation } from "react-i18next"

// useIntakeSourceErrorText says why a file could not be scanned, from the
// error code the worker stored on its source.
export function useIntakeSourceErrorText(): (code: string) => string {
  const { t } = useTranslation()
  return (code) => {
    switch (code) {
      case "no_files":
        return t("commodities:intake.sourceError.noFiles")
      case "file_unreadable":
        return t("commodities:intake.sourceError.fileUnreadable")
      case "unsupported_file":
        return t("commodities:intake.sourceError.unsupportedFile")
      case "file_too_large":
        return t("commodities:intake.sourceError.fileTooLarge")
      case "too_many_files":
        return t("commodities:intake.sourceError.tooManyFiles")
      case "no_details":
        return t("commodities:intake.sourceError.noDetails")
      case "author_unavailable":
        return t("commodities:intake.sourceError.authorUnavailable")
      case "provider_disabled":
      case "provider_misconfigured":
        return t("commodities:intake.sourceError.providerDisabled")
      case "provider_timeout":
        return t("commodities:intake.sourceError.providerTimeout")
      default:
        return t("commodities:intake.sourceError.providerError")
    }
  }
}

// useIntakeDraftErrorText says why the draft of a kept product could not be
// made.
export function useIntakeDraftErrorText(): (code: string) => string {
  const { t } = useTranslation()
  return (code) => {
    switch (code) {
      case "draft_discarded":
        return t("commodities:intake.draftError.discarded")
      case "invalid_values":
        return t("commodities:intake.draftError.invalidValues")
      default:
        return t("commodities:intake.draftError.other")
    }
  }
}

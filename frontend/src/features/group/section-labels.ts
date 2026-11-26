import type { useTranslation } from "react-i18next"

import type { GroupSection } from "./api"

type TFn = ReturnType<typeof useTranslation>["t"]

// Literal keys per section, so the i18n extractor can check each one.
export function sectionLabel(t: TFn, section: GroupSection): string {
  switch (section) {
    case "warranties":
      return t("groups:settings.hiddenSections.warranties.label")
    case "maintenance":
      return t("groups:settings.hiddenSections.maintenance.label")
    case "loans":
      return t("groups:settings.hiddenSections.loans.label")
    case "upcoming":
      return t("groups:settings.hiddenSections.upcoming.label")
  }
}

export function sectionDescription(t: TFn, section: GroupSection): string {
  switch (section) {
    case "warranties":
      return t("groups:settings.hiddenSections.warranties.description")
    case "maintenance":
      return t("groups:settings.hiddenSections.maintenance.description")
    case "loans":
      return t("groups:settings.hiddenSections.loans.description")
    case "upcoming":
      return t("groups:settings.hiddenSections.upcoming.description")
  }
}

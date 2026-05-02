import { useTranslation } from "react-i18next"

// useNavLabel resolves the visible label for a sidebar / command-palette
// nav entry from its full namespace-qualified `labelKey` (e.g.
// "common:nav.dashboard").
//
// We call `t()` with explicit string literals here — one per case — so
// i18next-cli's AST extractor sees real keys and never invents a literal
// "${labelKey}" entry from a variable call. Passing `t` as a parameter
// from the call site or building a template literal both trip the same
// extractor heuristic; the explicit switch is the only shape the parser
// reads cleanly. preservePatterns alone can't prevent the addition.
//
// Centralised here so AppSidebar and CommandPalette stay in sync.
export function useNavLabel(labelKey: string): string {
  const { t } = useTranslation()
  switch (labelKey) {
    case "common:nav.dashboard":
      return t("common:nav.dashboard")
    case "common:nav.locations":
      return t("common:nav.locations")
    case "common:nav.items":
      return t("common:nav.items")
    case "common:nav.warranties":
      return t("common:nav.warranties")
    case "common:nav.tags":
      return t("common:nav.tags")
    case "common:nav.files":
      return t("common:nav.files")
    case "common:nav.members":
      return t("common:nav.members")
    case "common:nav.backup":
      return t("common:nav.backup")
    case "common:nav.system":
      return t("common:nav.system")
    case "common:nav.profile":
      return t("common:nav.profile")
    default:
      return labelKey
  }
}

import { useTranslation } from "react-i18next"
import { PencilLine } from "lucide-react"

import { ANON_DRAFT_KEY } from "@/components/items/AnonymousCommodityDialog"
import { readDraft } from "@/features/commodities/draft"
import { cn } from "@/lib/utils"

// Search param the auth-page pills set to bounce a logged-out visitor back to
// the landing page and reopen their drafted item. The landing page reads it
// to auto-open the dialog; kept here so reader and writers can't drift.
export const RESUME_FIRST_ITEM_PARAM = "addFirstItem"

// anonDraftHasContent reports whether the anonymous first-item draft holds
// something worth resuming. The create dialog auto-saves defaults the moment
// it opens (count 1, currency, the draft toggle), so "a key exists" isn't
// enough — we require one of the user-entered identity fields to be non-empty
// before offering the "continue" affordance.
export function anonDraftHasContent(): boolean {
  const draft = readDraft(ANON_DRAFT_KEY)
  if (!draft) return false
  const filled = (v: unknown) => typeof v === "string" && v.trim() !== ""
  return filled(draft.name) || filled(draft.short_name) || filled(draft.type)
}

interface ResumeFirstItemPillProps {
  onResume: () => void
}

// ResumeFirstItemPill is the fixed bottom-right "Continue your item"
// affordance for the anonymous first-item flow (#1988). It appears in two
// places so a logged-out visitor can always get back to what they entered:
//   - the landing page — resume a draft you closed without finishing;
//   - the Login / Register pages — jump back to your item after the
//     hand-off (the draft is stashed but not yet replayed).
// Presentational only: the caller decides WHEN to render it (draft present)
// and what `onResume` does (open the dialog locally, or route back to it).
export function ResumeFirstItemPill({ onResume }: ResumeFirstItemPillProps) {
  const { t } = useTranslation()
  return (
    <button
      type="button"
      onClick={onResume}
      data-testid="resume-first-item-pill"
      className={cn(
        "fixed bottom-5 right-5 z-40 flex items-center gap-2 rounded-full border border-border",
        "bg-card px-4 py-2.5 text-sm font-medium shadow-lg transition-colors",
        "hover:border-primary/40 hover:bg-muted/30",
        "focus-visible:outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50"
      )}
    >
      <PencilLine className="size-4 text-primary" aria-hidden="true" />
      {t("landing:draft.resume")}
    </button>
  )
}

import { useNavigate } from "react-router-dom"

import { CommodityFormDialog } from "@/components/items/CommodityFormDialog"
import { writeDraft } from "@/features/commodities/draft"
import type { CreateCommodityRequest } from "@/features/commodities/api"
import { savePendingFirstItem } from "@/features/auth/firstItemHandoff"
import { inferDefaultCurrency } from "@/lib/currency-default"

// Fixed localStorage draft key for the anonymous flow. Shared by the
// dialog's auto-save, the explicit writeDraft on submit, and the
// post-login FirstItemResolver replay — they MUST agree on one key so
// the resolver can find the stashed values + IndexedDB pending files.
export const ANON_DRAFT_KEY = "commodity-draft:anon:create"

// Where the resolver replays the stash. The hand-off sends the visitor to
// /register first (the anonymous fill is new-user onboarding); the marker
// survives the register → verify-email → sign-in round-trip, and the LOGIN
// page is what finally bounces here once peekPendingFirstItem() is set.
const WELCOME_PATH = "/welcome"

interface AnonymousCommodityDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  // Mirrors the `public_scan` deployment flag. When false the wrapped
  // form skips the AI photo-scan step and opens directly on manual
  // entry — the public scan endpoint is unmounted (404) in that posture,
  // so offering the scan UI would only dead-end. The rest of the
  // hand-off (stash draft → login → replay) is unaffected. Defaults to
  // false so a caller that forgets to pass it degrades safely to manual.
  aiScanEnabled?: boolean
}

// AnonymousCommodityDialog wraps the create-mode CommodityFormDialog for
// the unauthenticated landing-page "add your first item" CTA (#1988).
//
// It renders the same multi-step form (the public AI scan step via
// `anonymous` is included only when `aiScanEnabled` — i.e. the
// `public_scan` deployment flag — is on; otherwise the form opens
// straight on manual entry), but its `onSubmit` is a PURE HAND-OFF, not
// a POST: there is no group, no auth, and nothing to persist to the BE
// yet. On submit
// we
//   1. write the validated values into the fixed anonymous draft key
//      (belt-and-suspenders over the dialog's own rAF-debounced
//      auto-save — guarantees the final keystroke is captured), then
//   2. set the pending-first-item marker (draftKey + inferred currency),
//      then
//   3. redirect to /register?redirect=/welcome — the anonymous fill is
//      framed as new-user onboarding, so it hands off to account creation.
// After the visitor creates an account, verifies their email, and signs
// in, FirstItemResolver reads the marker, auto-creates a group seeded with
// the item's own currency (when they have none), POSTs the stashed
// commodity into it, uploads its IndexedDB pending files, and clears
// everything. No entered data is ever lost.
//
// `defaultCurrency` is inferred from the browser locale because there's
// no group to read one from; `areas`/`locations` are empty (the create
// dialog no longer asks for a location — #1987 — so the anonymous user
// never needs one).
export function AnonymousCommodityDialog({
  open,
  onOpenChange,
  aiScanEnabled = false,
}: AnonymousCommodityDialogProps) {
  const navigate = useNavigate()
  const defaultCurrency = inferDefaultCurrency()

  // onSubmit receives the already-validated, BE-shaped request. We don't
  // POST it — the form-shaped draft is already in localStorage via the
  // dialog's auto-save, so we just persist the request's currency into the
  // pending-first-item marker (so the auto-created "Main" group, when the
  // user has zero groups, is seeded coherently) and redirect to register.
  // The dialog's anonymous branch bails before any upload/clear/navigate,
  // leaving the redirect to us.
  async function handleSubmit(values: CreateCommodityRequest) {
    // The dialog already mirrors form state to localStorage under
    // ANON_DRAFT_KEY (auto-save effect, gated on draftKey + create mode).
    // Re-write the BE-shaped request defensively so a final-keystroke
    // race can never strand un-persisted data. We store the request shape
    // because the resolver reads it back through readDraft → toRequest;
    // toRequest is idempotent over an already-request-shaped object only
    // for the fields it touches, so we keep the draft in form-shape by
    // letting the dialog's own auto-save own the canonical write and only
    // top it up here with whatever the dialog last serialized.
    //
    // NOTE: the dialog's auto-save writes the *form input* shape (strings
    // for numerics, urls as string[]). We must NOT overwrite that with the
    // BE-shaped `values` (numbers, urls as string), or readDraft would hand
    // toRequest the wrong types. So we leave the auto-saved form-shaped
    // draft in place and only ensure SOMETHING is written when the dialog
    // somehow hasn't (storage disabled mid-session is the only path).
    if (!hasDraft(ANON_DRAFT_KEY)) {
      // Last-ditch: persist a minimal form-shaped projection so the
      // resolver has at least the user's entered values. Keys mirror the
      // CommodityFormInput shape the dialog auto-saves.
      writeDraft(ANON_DRAFT_KEY, {
        name: values.name,
        short_name: values.short_name,
        type: values.type,
        status: values.status,
        count: values.count !== undefined ? String(values.count) : "1",
        original_price: values.original_price !== undefined ? String(values.original_price) : "",
        original_price_currency: values.original_price_currency,
        converted_original_price:
          values.converted_original_price !== undefined
            ? String(values.converted_original_price)
            : "",
        current_price: values.current_price !== undefined ? String(values.current_price) : "",
        serial_number: values.serial_number,
        purchase_date: values.purchase_date ?? "",
        comments: values.comments,
        draft: values.draft,
        warranty_expires_at: values.warranty_expires_at ?? "",
        warranty_notes: values.warranty_notes,
      })
    }
    savePendingFirstItem({
      draftKey: ANON_DRAFT_KEY,
      currency: values.original_price_currency || defaultCurrency,
      savedAt: Date.now(),
    })
    navigate(`/register?redirect=${encodeURIComponent(WELCOME_PATH)}`)
  }

  // "Save as draft" from the dismiss-confirm. Unlike an authenticated user,
  // an anonymous visitor has nowhere to persist a draft except their own
  // account — so saving a (possibly partial) draft hands off to register
  // just like a full submit: the dialog has already written the form-shaped
  // draft under ANON_DRAFT_KEY, so we only set the pending-first-item marker
  // and route to register. Currency is the locale default (the resolver re-derives
  // it from the real group on replay, so a partial draft with no price is
  // fine). Without this the user would land back on the bare landing page
  // with no obvious way to actually keep what they typed.
  function handleSaveDraft() {
    savePendingFirstItem({
      draftKey: ANON_DRAFT_KEY,
      currency: defaultCurrency,
      savedAt: Date.now(),
    })
    navigate(`/register?redirect=${encodeURIComponent(WELCOME_PATH)}`)
  }

  return (
    <CommodityFormDialog
      open={open}
      onOpenChange={onOpenChange}
      mode="create"
      anonymous
      enableAiScan={aiScanEnabled}
      // Start as a draft so the first-time visitor only needs name +
      // short name + type; everything else (price, date, …) is optional.
      defaultDraft
      areas={[]}
      locations={[]}
      defaultCurrency={defaultCurrency}
      onSubmit={handleSubmit}
      onSaveDraft={handleSaveDraft}
      draftKey={ANON_DRAFT_KEY}
    />
  )
}

function hasDraft(key: string): boolean {
  if (typeof window === "undefined") return false
  try {
    return window.localStorage.getItem(key) !== null
  } catch {
    return false
  }
}

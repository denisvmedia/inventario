import { useState } from "react"
import { ArrowRight, Mail, X } from "lucide-react"
import { useNavigate } from "react-router-dom"
import { useTranslation } from "react-i18next"

import { Button } from "@/components/ui/button"

interface InviteBannerProps {
  // Number of pending invites for the active user. Driven by data
  // (#1413 wires this once the invites query lands); 0 hides the banner.
  count: number
  // Where to send the user when they click "View". `null` (the default)
  // hides the action button — every caller should pass an explicit target
  // since the right destination depends on the surrounding context (the
  // members page on the active group, the user profile, an invites
  // listing). #1413 will set this from the call site once the invites
  // surface lands.
  viewHref?: string | null
}

// Sticky banner just under the top bar that surfaces pending invites. The
// user can dismiss in-session (state-only — comes back on next mount); a
// persistent dismiss is tracked separately in #1413 once the invite list
// lands.
export function InviteBanner({ count, viewHref = null }: InviteBannerProps) {
  const [dismissed, setDismissed] = useState(false)
  const navigate = useNavigate()
  const { t } = useTranslation()

  if (dismissed || count <= 0) return null

  return (
    <div
      className="flex items-center gap-3 border-b border-border bg-primary/5 px-4 py-2.5"
      role="status"
      data-testid="invite-banner"
    >
      <div className="flex size-6 shrink-0 items-center justify-center rounded-full bg-primary/15">
        <Mail className="size-3.5 text-primary" />
      </div>
      <p className="flex-1 text-sm text-foreground">{t("common:shell.inviteBanner", { count })}</p>
      {viewHref ? (
        <Button
          variant="ghost"
          size="sm"
          className="gap-1.5 h-7 text-xs shrink-0"
          onClick={() => navigate(viewHref)}
        >
          {t("common:shell.inviteBannerView")}
          <ArrowRight className="size-3" />
        </Button>
      ) : null}
      <button
        type="button"
        onClick={() => setDismissed(true)}
        className="text-muted-foreground hover:text-foreground transition-colors shrink-0"
        aria-label={t("common:shell.inviteBannerDismiss")}
      >
        <X className="size-4" />
      </button>
    </div>
  )
}

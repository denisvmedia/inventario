import { useState } from "react"
import { Mail, X, ArrowRight } from "lucide-react"
import { Button } from "@/components/ui/button"

interface InviteBannerProps {
  count: number
  onViewInvites: () => void
}

export function InviteBanner({ count, onViewInvites }: InviteBannerProps) {
  const [dismissed, setDismissed] = useState(false)
  if (dismissed || count === 0) return null

  return (
    <div className="flex items-center gap-3 border-b border-border bg-primary/5 px-4 py-2.5">
      <div className="flex size-6 shrink-0 items-center justify-center rounded-full bg-primary/15">
        <Mail className="size-3.5 text-primary" />
      </div>
      <p className="flex-1 text-sm text-foreground">
        You have{" "}
        <span className="font-semibold">
          {count} pending invite{count !== 1 ? "s" : ""}
        </span>
        {" "}to inventory groups.
      </p>
      <Button
        variant="ghost"
        size="sm"
        className="gap-1.5 h-7 text-xs shrink-0"
        onClick={onViewInvites}
      >
        View
        <ArrowRight className="size-3" />
      </Button>
      <button
        onClick={() => setDismissed(true)}
        className="text-muted-foreground hover:text-foreground transition-colors shrink-0"
        aria-label="Dismiss"
      >
        <X className="size-4" />
      </button>
    </div>
  )
}

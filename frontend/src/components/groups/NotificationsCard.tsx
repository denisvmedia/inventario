import { useTranslation } from "react-i18next"

import { Alert, AlertDescription } from "@/components/ui/alert"
import { Skeleton } from "@/components/ui/skeleton"
import { Switch } from "@/components/ui/switch"
import {
  useGroupNotifications,
  useUpdateGroupNotifications,
} from "@/features/notifications-group/hooks"
import { useAppToast } from "@/hooks/useAppToast"
import { cn } from "@/lib/utils"

// NotificationsCard renders the per-group notification card on
// GroupSettings (issue #1648). Two divide-y switch rows; each row
// optimistically flips the local state on click and PATCHes the BE.
// On error the cache rolls back and a toast surfaces the failure —
// the user can retry from the same Switch without reloading.
//
// Resolution semantics: the BE returns the EFFECTIVE state of each
// toggle (per-group override → user-global → in-code default), so
// the UI just renders what the API hands back. The act of flipping
// the switch writes a per-group override row, which then wins over
// the user-global pref on subsequent reads.
interface NotificationsCardProps {
  groupSlug: string | null
  className?: string
}

export function NotificationsCard({ groupSlug, className }: NotificationsCardProps) {
  const { t } = useTranslation()
  const toast = useAppToast()
  const query = useGroupNotifications(groupSlug)
  const mutation = useUpdateGroupNotifications(groupSlug)

  // Order matters: isError before !data, otherwise the failed-but-no-
  // data state would loop on the skeleton forever (same guard pattern
  // as PlanCard — see review #r3229682994 on #1656).
  if (query.isError) {
    return (
      <Alert variant="destructive" className={className} data-testid="notifications-card-error">
        <AlertDescription>{t("groups:settings.notifications.errorGeneric")}</AlertDescription>
      </Alert>
    )
  }
  if (!query.data) {
    return <NotificationsCardSkeleton className={className} />
  }

  const data = query.data
  const onToggle = (key: "warranty_expiring_alerts" | "weekly_digest", value: boolean) => {
    mutation.mutate(
      { [key]: value },
      {
        onError: () => {
          toast.error(t("groups:settings.notifications.saveError"))
        },
      }
    )
  }

  return (
    <div
      className={cn("rounded-xl border border-border bg-card p-6 space-y-4", className)}
      data-testid="notifications-card"
    >
      <div>
        <h2 className="text-base font-semibold">{t("groups:settings.notifications.title")}</h2>
        <p className="text-sm text-muted-foreground mt-0.5">
          {t("groups:settings.notifications.subtitle")}
        </p>
      </div>

      <div className="divide-y divide-border">
        <ToggleRow
          label={t("groups:settings.notifications.toggles.warrantyExpiringAlerts.label")}
          description={t(
            "groups:settings.notifications.toggles.warrantyExpiringAlerts.description"
          )}
          checked={!!data.warranty_expiring_alerts}
          disabled={mutation.isPending}
          onChange={(v) => onToggle("warranty_expiring_alerts", v)}
          testId="notifications-toggle-warranty"
        />
        <ToggleRow
          label={t("groups:settings.notifications.toggles.weeklyDigest.label")}
          description={t("groups:settings.notifications.toggles.weeklyDigest.description")}
          checked={!!data.weekly_digest}
          disabled={mutation.isPending}
          onChange={(v) => onToggle("weekly_digest", v)}
          testId="notifications-toggle-weekly-digest"
        />
      </div>
    </div>
  )
}

interface ToggleRowProps {
  label: string
  description: string
  checked: boolean
  disabled?: boolean
  onChange: (value: boolean) => void
  testId: string
}

function ToggleRow({ label, description, checked, disabled, onChange, testId }: ToggleRowProps) {
  return (
    <div className="flex items-center justify-between gap-4 py-3.5">
      <div className="min-w-0">
        <p className="text-sm font-medium">{label}</p>
        <p className="text-xs text-muted-foreground mt-0.5">{description}</p>
      </div>
      <Switch
        checked={checked}
        onCheckedChange={onChange}
        disabled={disabled}
        data-testid={testId}
        aria-label={label}
      />
    </div>
  )
}

function NotificationsCardSkeleton({ className }: { className?: string }) {
  return (
    <div
      className={cn("rounded-xl border border-border bg-card p-6 space-y-4", className)}
      data-testid="notifications-card-skeleton"
    >
      <div className="space-y-2">
        <Skeleton className="h-4 w-32" />
        <Skeleton className="h-3 w-64" />
      </div>
      <div className="space-y-3">
        <Skeleton className="h-10 w-full" />
        <Skeleton className="h-10 w-full" />
      </div>
    </div>
  )
}

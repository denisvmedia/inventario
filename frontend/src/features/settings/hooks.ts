import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"

import { useCurrentGroup } from "@/features/group/GroupContext"

import { type SettingsObject, getSettings, patchSetting } from "./api"
import { settingsKeys } from "./keys"

interface QueryOptions {
  enabled?: boolean
}

// useUserSettings fetches the per-user SettingsObject. Read-only; for
// writes use usePatchSetting which mutates one field at a time so the
// UI can autosave on Switch flip / Select change without batching.
//
// The query is keyed on the active group slug because the underlying
// `/settings` URL rewrites under `/g/{slug}/settings`. The row data is
// the same across groups (per-tenant + per-user), but keying on the
// slug keeps the cache honest if the rewrite ever changes shape.
export function useUserSettings({ enabled = true }: QueryOptions = {}) {
  const { currentGroup } = useCurrentGroup()
  const slug = currentGroup?.slug ?? ""
  return useQuery<SettingsObject>({
    queryKey: settingsKeys.preferences(slug),
    queryFn: ({ signal }) => getSettings(signal),
    enabled: enabled && !!slug,
    placeholderData: (prev) => prev,
  })
}

interface PatchVars {
  field: string
  value: unknown
}

// usePatchSetting is the canonical write path. Optimistically updates
// the cached SettingsObject on toggle / change so the Switch doesn't
// flicker, then snaps back to the BE-returned value on success. On
// error the snapshot is restored so the UI reflects the durable state.
export function usePatchSetting() {
  const qc = useQueryClient()
  const { currentGroup } = useCurrentGroup()
  const slug = currentGroup?.slug ?? ""
  return useMutation<SettingsObject, Error, PatchVars, { previous: SettingsObject | undefined }>({
    mutationFn: ({ field, value }) => patchSetting(field, value),
    onMutate: async ({ field, value }) => {
      await qc.cancelQueries({ queryKey: settingsKeys.preferences(slug) })
      const previous = qc.getQueryData<SettingsObject>(settingsKeys.preferences(slug))
      if (previous) {
        const optimistic = applyOptimisticPatch(previous, field, value)
        qc.setQueryData<SettingsObject>(settingsKeys.preferences(slug), optimistic)
      }
      return { previous }
    },
    onError: (_err, _vars, ctx) => {
      if (ctx?.previous) {
        qc.setQueryData<SettingsObject>(settingsKeys.preferences(slug), ctx.previous)
      }
    },
    onSuccess: (data) => {
      qc.setQueryData<SettingsObject>(settingsKeys.preferences(slug), data)
    },
  })
}

// applyOptimisticPatch maps a snake_case setting path (e.g.
// `notifications.warranty_expiry`) to the matching camelCase field on
// SettingsObject so the cached entry updates without a round-trip.
// Unknown paths are ignored — the BE will 400 anyway and the rollback
// snapshot will restore the cache.
function applyOptimisticPatch(
  current: SettingsObject,
  field: string,
  value: unknown
): SettingsObject {
  const next = { ...current }
  switch (field) {
    case "notifications.warranty_expiry":
      next.notificationsWarrantyExpiry = value as boolean
      break
    case "notifications.maintenance_reminder":
      next.notificationsMaintenanceReminder = value as boolean
      break
    case "notifications.weekly_digest":
      next.notificationsWeeklyDigest = value as boolean
      break
    case "notifications.price_drop":
      next.notificationsPriceDrop = value as boolean
      break
    case "notifications.loan_reminder":
      next.notificationsLoanReminder = value as boolean
      break
    case "notifications.channel.email":
      next.notificationsChannelEmail = value as boolean
      break
    case "notifications.channel.push":
      next.notificationsChannelPush = value as boolean
      break
    case "appearance.default_items_view":
      next.appearanceDefaultItemsView = value as string
      break
    case "appearance.preferred_display_currency":
      next.appearancePreferredDisplayCurrency = value as string
      break
    case "appearance.number_format_locale":
      next.appearanceNumberFormatLocale = value as string
      break
    case "appearance.language":
      next.appearanceLanguage = value as string
      break
  }
  return next
}

import { useState, type ReactNode } from "react"
import { useTranslation } from "react-i18next"
import { Link, useNavigate } from "react-router-dom"
import {
  ArrowRight,
  Bell,
  Building2,
  Check,
  ChevronRight,
  CircleHelp,
  LogOut,
  Monitor,
  Moon,
  Palette,
  Plus,
  Shield,
  Sun,
  Trash2,
  User,
} from "lucide-react"

import { ComingSoonBanner } from "@/components/coming-soon"
import { useSessionsList } from "@/features/sessions/hooks"
import { CurrencyCombobox } from "@/components/CurrencyCombobox"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Separator } from "@/components/ui/separator"
import { Switch } from "@/components/ui/switch"
import { useAuth } from "@/features/auth/AuthContext"
import { useLogout, useUpdateProfile } from "@/features/auth/hooks"
import { useCurrentGroup } from "@/features/group/GroupContext"
import {
  SETTING_APPEARANCE_DEFAULT_ITEMS_VIEW,
  SETTING_APPEARANCE_PREFERRED_DISPLAY_CURRENCY,
  SETTING_NOTIFICATIONS_CHANNEL_EMAIL,
  SETTING_NOTIFICATIONS_CHANNEL_PUSH,
  SETTING_NOTIFICATIONS_MAINTENANCE_REMINDER,
  SETTING_NOTIFICATIONS_PRICE_DROP,
  SETTING_NOTIFICATIONS_WARRANTY_EXPIRY,
  SETTING_NOTIFICATIONS_WEEKLY_DIGEST,
  type SettingsObject,
} from "@/features/settings/api"
import { usePatchSetting, useUserSettings } from "@/features/settings/hooks"
import { useAppToast } from "@/hooks/useAppToast"
import { useConfirm } from "@/hooks/useConfirm"
import { useDensity, DENSITIES, type Density } from "@/hooks/useDensity"
import { useTheme } from "@/components/theme-provider"
import { i18next, SUPPORTED_LANGUAGES, type SupportedLanguage } from "@/i18n"
import { withGroupQuery } from "@/lib/group-aware-url"
import { formatDate } from "@/lib/intl"
import { parseServerError } from "@/lib/server-error"
import { cn } from "@/lib/utils"
import { RouteTitle } from "@/components/routing/RouteTitle"
import { APP_VERSION, shortAppVersion } from "@/lib/app-version"

type SectionId = "account" | "appearance" | "notifications" | "privacy" | "help"

interface SectionMeta {
  id: SectionId
  // Lucide icon for the nav rail.
  icon: typeof User
}

const SECTIONS: SectionMeta[] = [
  { id: "account", icon: User },
  { id: "appearance", icon: Palette },
  { id: "notifications", icon: Bell },
  { id: "privacy", icon: Shield },
  { id: "help", icon: CircleHelp },
]

// SettingsPage — preferences hub. Two-pane layout:
//   - left rail: section nav + sign-out shortcut
//   - right pane: the selected section's content
//
// Theme / density / locale persist to localStorage via the existing
// providers + i18next detection cache. Notifications + default-view +
// preferred-currency persist via the `/settings` endpoint
// (models.SettingsObject) keyed by (tenant_id, user_id) — see
// features/settings/api.ts. The two persistence layers are
// intentionally split: chrome-state that the UI needs synchronously
// at boot stays local; preferences that affect server-side behaviour
// (e.g. whether to send a warranty reminder email) round-trip.
export function SettingsPage() {
  const { t } = useTranslation()
  const [active, setActive] = useState<SectionId>("appearance")

  return (
    <>
      <RouteTitle title={t("settings:title")} />
      <div className="mx-auto flex w-full max-w-4xl flex-col gap-8" data-testid="settings-page">
        <header className="space-y-1">
          <h1 className="scroll-m-20 text-3xl font-semibold tracking-tight">
            {t("settings:title")}
          </h1>
          <p className="text-sm text-muted-foreground">{t("settings:subtitle")}</p>
        </header>

        <div className="flex flex-col gap-6 md:flex-row">
          <SettingsNav active={active} onSelect={setActive} />
          <div className="min-w-0 flex-1">
            {active === "account" ? <AccountSection /> : null}
            {active === "appearance" ? <AppearanceSection /> : null}
            {active === "notifications" ? <NotificationsSection /> : null}
            {active === "privacy" ? <PrivacySection /> : null}
            {active === "help" ? <HelpSection /> : null}
          </div>
        </div>
      </div>
    </>
  )
}

function SettingsNav({
  active,
  onSelect,
}: {
  active: SectionId
  onSelect: (id: SectionId) => void
}) {
  const { t } = useTranslation()
  const logoutMutation = useLogout()
  const navigate = useNavigate()

  async function handleSignOut() {
    try {
      await logoutMutation.mutateAsync()
    } finally {
      navigate("/login")
    }
  }

  return (
    <aside className="md:w-48 md:shrink-0">
      <nav className="space-y-0.5" aria-label={t("settings:title")}>
        {SECTIONS.map(({ id, icon: Icon }) => (
          <button
            key={id}
            type="button"
            onClick={() => onSelect(id)}
            data-testid={`settings-nav-${id}`}
            data-active={active === id ? "true" : undefined}
            className={cn(
              "flex w-full items-center gap-2.5 rounded-md px-3 py-2 text-sm transition-colors",
              active === id
                ? "bg-accent text-accent-foreground font-medium"
                : "text-muted-foreground hover:bg-muted hover:text-foreground"
            )}
            aria-current={active === id ? "page" : undefined}
          >
            <Icon className="size-4 shrink-0" aria-hidden="true" />
            {t(`settings:sections.${id}`)}
            {active === id ? (
              <ChevronRight className="ml-auto size-3.5" aria-hidden="true" />
            ) : null}
          </button>
        ))}
        <Separator className="my-2" />
        <button
          type="button"
          onClick={handleSignOut}
          disabled={logoutMutation.isPending}
          data-testid="settings-sign-out"
          className="flex w-full items-center gap-2.5 rounded-md px-3 py-2 text-sm text-destructive hover:bg-destructive/10 transition-colors disabled:opacity-50"
        >
          <LogOut className="size-4 shrink-0" aria-hidden="true" />
          {t("settings:signOut")}
        </button>
      </nav>
    </aside>
  )
}

function SectionTitle({ children }: { children: ReactNode }) {
  return <h2 className="mb-4 text-base font-semibold">{children}</h2>
}

function SettingRow({
  label,
  description,
  children,
}: {
  label: string
  description?: string
  children: ReactNode
}) {
  return (
    <div className="flex items-start justify-between gap-4 py-3.5">
      <div className="min-w-0">
        <p className="text-sm font-medium">{label}</p>
        {description ? (
          <p className="mt-0.5 text-xs leading-relaxed text-muted-foreground">{description}</p>
        ) : null}
      </div>
      <div className="shrink-0">{children}</div>
    </div>
  )
}

function AccountSection() {
  const { t } = useTranslation()
  const { user } = useAuth()
  const { groups, currentGroup, isLoading } = useCurrentGroup()
  const confirm = useConfirm()
  const memberSince = user?.created_at
    ? formatDate(user.created_at, { style: "long" })
    : t("settings:account.memberSinceUnknown")

  // Account deletion isn't backend-supported yet; render a danger-zone
  // panel that opens a confirm dialog explaining the limitation rather
  // than fake a destructive action that does nothing.
  async function handleDeleteAccount() {
    await confirm({
      title: t("settings:account.dangerZone.deleteUnavailableTitle"),
      description: t("settings:account.dangerZone.deleteUnavailableDescription"),
      confirmLabel: t("settings:account.dangerZone.deleteUnavailableConfirm"),
      cancelLabel: t("settings:profile.edit.cancel"),
      destructive: false,
    })
  }

  // Wait for the membership list to load before deciding which surface to
  // render — a `groups === undefined` state would briefly show the empty-
  // state CTA to a user who actually has groups, and vice versa.
  const groupsReady = !isLoading && Array.isArray(groups)
  const hasMemberships = groupsReady && groups.length > 0

  return (
    <div className="space-y-6" data-testid="section-account">
      <SectionTitle>{t("settings:account.title")}</SectionTitle>

      <div className="rounded-xl border border-border bg-card p-4">
        <div className="flex items-center gap-4">
          <div
            className="flex size-12 items-center justify-center rounded-full bg-primary text-base font-semibold text-primary-foreground"
            aria-hidden="true"
          >
            {(user?.name?.[0] ?? user?.email?.[0] ?? "?").toUpperCase()}
          </div>
          <div className="min-w-0 flex-1">
            <p className="truncate font-semibold">{user?.name ?? "—"}</p>
            <p className="truncate text-sm text-muted-foreground">{user?.email ?? "—"}</p>
            <Badge variant="secondary" className="mt-1 text-xs">
              {t("settings:profile.planFree")}
            </Badge>
          </div>
          <Button asChild variant="outline" size="sm">
            <Link
              to={withGroupQuery("/profile/edit", currentGroup?.slug)}
              data-testid="settings-edit-profile"
            >
              {t("settings:account.editProfile")}
            </Link>
          </Button>
        </div>
      </div>

      {groupsReady && !hasMemberships ? <NoGroupCta /> : null}

      <Separator />

      <div className="divide-y divide-border">
        <SettingRow label={t("settings:account.displayName")}>
          <span className="text-sm text-muted-foreground">{user?.name ?? "—"}</span>
        </SettingRow>
        <SettingRow label={t("settings:account.email")}>
          <span className="text-sm text-muted-foreground">{user?.email ?? "—"}</span>
        </SettingRow>
        {hasMemberships ? <DefaultGroupSelectorRow /> : null}
        <SettingRow label={t("settings:account.memberSince")}>
          <span className="text-sm text-muted-foreground">{memberSince}</span>
        </SettingRow>
        <SettingRow
          label={t("settings:account.changePassword")}
          description={t("settings:account.changePasswordDescription")}
        >
          <Button asChild variant="outline" size="sm">
            <Link
              to={withGroupQuery("/profile/edit", currentGroup?.slug)}
              data-testid="settings-change-password"
            >
              {t("settings:profile.password.title")}
            </Link>
          </Button>
        </SettingRow>
      </div>

      <div className="rounded-xl border border-destructive/30 bg-destructive/5 p-4 space-y-2">
        <p className="text-sm font-semibold text-destructive">
          {t("settings:account.dangerZone.title")}
        </p>
        <p className="text-xs text-muted-foreground">
          {t("settings:account.dangerZone.description")}
        </p>
        <Button
          variant="outline"
          size="sm"
          className="text-destructive border-destructive/40 hover:bg-destructive/10 gap-1.5"
          onClick={handleDeleteAccount}
          data-testid="delete-account-button"
        >
          <Trash2 className="size-3.5" aria-hidden="true" />
          {t("settings:account.dangerZone.deleteAccount")}
        </Button>
      </div>
    </div>
  )
}

// DefaultGroupSelectorRow renders the inline default-group <select>. Persists
// every change via PUT /auth/me; on success the auth/group queries invalidate
// so RootRedirect / sidebar pick up the new preference. Read-only mode falls
// back to the user's saved default when groups exist but the auth layer is
// still warming up.
function DefaultGroupSelectorRow() {
  const { t } = useTranslation()
  const { user } = useAuth()
  const { groups } = useCurrentGroup()
  const updateMutation = useUpdateProfile()
  const toast = useAppToast()
  const [error, setError] = useState<string | null>(null)

  const userId = user?.id
  const userName = user?.name ?? ""
  const currentDefault = user?.default_group_id ?? ""
  // The selector value is the user's default if it still matches a current
  // membership; otherwise it falls back to the first available group so the
  // browser doesn't show a phantom "—" item, but we never auto-PATCH on that
  // fallback (the change must come from a real user click).
  const value =
    currentDefault && groups?.some((g) => g.id === currentDefault)
      ? currentDefault
      : (groups?.[0]?.id ?? "")

  async function handleChange(event: React.ChangeEvent<HTMLSelectElement>) {
    const next = event.target.value
    if (!next || next === currentDefault || !userId) return
    setError(null)
    try {
      await updateMutation.mutateAsync({
        name: userName,
        default_group_id: next,
      })
      toast.success(t("settings:profile.defaultGroupSaved"))
    } catch (err) {
      setError(parseServerError(err, t("settings:profile.edit.errorGeneric")))
    }
  }

  return (
    <SettingRow
      label={t("settings:profile.defaultGroup")}
      description={t("settings:profile.defaultGroupHelp")}
    >
      <div className="flex flex-col items-end gap-1">
        <select
          value={value}
          onChange={handleChange}
          disabled={updateMutation.isPending}
          aria-label={t("settings:profile.defaultGroup")}
          data-testid="settings-default-group-select"
          className="h-8 rounded-md border border-input bg-background px-2.5 text-sm shadow-xs focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50 disabled:cursor-not-allowed disabled:opacity-50"
        >
          {groups?.map((g) => (
            <option key={g.id} value={g.id}>
              {g.name}
            </option>
          ))}
        </select>
        {error ? (
          <p className="text-[11px] text-destructive" data-testid="settings-default-group-error">
            {error}
          </p>
        ) : null}
      </div>
    </SettingRow>
  )
}

// NoGroupCta is the empty-state for users with zero memberships. The CTA links
// to /no-group so the same onboarding flow used at first login also serves the
// "I leaved my last group" repair case. Pending-invite surfacing is deferred
// per #1592 open-questions.
function NoGroupCta() {
  const { t } = useTranslation()

  return (
    <div
      className="rounded-xl border border-border bg-card p-5 space-y-3"
      data-testid="settings-no-groups-cta"
    >
      <div className="flex items-center gap-3">
        <div className="flex size-10 items-center justify-center rounded-lg bg-primary/10 shrink-0">
          <Building2 className="size-5 text-primary" aria-hidden="true" />
        </div>
        <div className="min-w-0">
          <p className="text-sm font-semibold">{t("settings:profile.noGroupsTitle")}</p>
          <p className="mt-0.5 text-xs text-muted-foreground leading-relaxed">
            {t("settings:profile.noGroupsHelp")}
          </p>
        </div>
      </div>
      <Button asChild className="w-full gap-2" data-testid="settings-no-groups-cta-button">
        <Link to="/no-group">
          <Plus className="size-4" aria-hidden="true" />
          {t("settings:profile.noGroupsCta")}
        </Link>
      </Button>
    </div>
  )
}

function AppearanceSection() {
  const { t, i18n } = useTranslation()
  const { theme, setTheme } = useTheme()
  const { density, setDensity } = useDensity()
  const toast = useAppToast()
  const settingsQuery = useUserSettings()
  const patchMutation = usePatchSetting()

  // Mock-spec order: Light, Dark, System (was System, Light, Dark — see
  // bonus item in design-audit #1536). The cards still render as a
  // simple radiogroup so screen readers see the same shape.
  const THEMES = [
    { id: "light" as const, icon: Sun },
    { id: "dark" as const, icon: Moon },
    { id: "system" as const, icon: Monitor },
  ]

  const currentLanguage = (i18n.resolvedLanguage ?? "en") as SupportedLanguage
  const settings = settingsQuery.data
  const defaultItemsView = settings?.appearanceDefaultItemsView ?? "grid"
  const preferredCurrency = settings?.appearancePreferredDisplayCurrency ?? ""

  const onChangeRemote = (field: string, value: unknown) => {
    patchMutation.mutate(
      { field, value },
      {
        onError: () => toast.error(t("settings:notifications.errors.saveFailed")),
      }
    )
  }

  return (
    <div className="space-y-6" data-testid="section-appearance">
      <SectionTitle>{t("settings:appearance.title")}</SectionTitle>

      <div>
        <p className="mb-3 text-sm font-medium">{t("settings:appearance.themeLabel")}</p>
        <div className="grid grid-cols-3 gap-3" role="radiogroup">
          {THEMES.map(({ id, icon: Icon }) => (
            <button
              key={id}
              type="button"
              role="radio"
              aria-checked={theme === id}
              onClick={() => setTheme(id)}
              data-testid={`theme-${id}`}
              className={cn(
                "flex flex-col items-center gap-2 rounded-xl border-2 p-4 transition-all",
                theme === id
                  ? "border-primary bg-primary/5"
                  : "border-border hover:border-muted-foreground/40"
              )}
            >
              <Icon
                className={cn("size-5", theme === id ? "text-primary" : "text-muted-foreground")}
                aria-hidden="true"
              />
              <span
                className={cn(
                  "text-xs font-medium",
                  theme === id ? "text-primary" : "text-muted-foreground"
                )}
              >
                {t(`settings:appearance.themeOptions.${id}`)}
              </span>
              {theme === id ? (
                <span className="flex size-4 items-center justify-center rounded-full bg-primary">
                  <Check className="size-2.5 text-primary-foreground" aria-hidden="true" />
                </span>
              ) : null}
            </button>
          ))}
        </div>
      </div>

      <Separator />

      <div className="divide-y divide-border">
        <SettingRow
          label={t("settings:appearance.densityLabel")}
          description={t("settings:appearance.densityHelp")}
        >
          <select
            value={density}
            onChange={(e) => setDensity(e.target.value as Density)}
            data-testid="density-select"
            aria-label={t("settings:appearance.densityLabel")}
            className="h-8 rounded-md border border-input bg-background px-2.5 text-sm shadow-xs focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50"
          >
            {DENSITIES.map((d) => (
              <option key={d} value={d}>
                {t(`common:shell.density${d.charAt(0).toUpperCase()}${d.slice(1)}`)}
              </option>
            ))}
          </select>
        </SettingRow>

        <SettingRow
          label={t("settings:appearance.localeLabel")}
          description={t("settings:appearance.localeHelp")}
        >
          <select
            value={currentLanguage}
            onChange={(e) => {
              const next = e.target.value as SupportedLanguage
              // i18next-browser-languagedetector caches to localStorage at
              // key "inventario-language" — calling changeLanguage triggers
              // that cache write, so reload picks the new locale up.
              void i18next.changeLanguage(next)
            }}
            data-testid="locale-select"
            aria-label={t("settings:appearance.localeLabel")}
            className="h-8 rounded-md border border-input bg-background px-2.5 text-sm shadow-xs focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50"
          >
            {SUPPORTED_LANGUAGES.map((lng) => (
              <option key={lng} value={lng}>
                {t(`settings:appearance.localeOptions.${lng}`)}
              </option>
            ))}
          </select>
        </SettingRow>

        <SettingRow
          label={t("settings:appearance.defaultViewLabel")}
          description={t("settings:appearance.defaultViewHelp")}
        >
          <select
            value={defaultItemsView}
            onChange={(e) => onChangeRemote(SETTING_APPEARANCE_DEFAULT_ITEMS_VIEW, e.target.value)}
            disabled={!settings}
            data-testid="default-view-select"
            aria-label={t("settings:appearance.defaultViewLabel")}
            className="h-8 rounded-md border border-input bg-background px-2.5 text-sm shadow-xs focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50"
          >
            <option value="grid">{t("settings:appearance.defaultViewOptions.grid")}</option>
            <option value="list">{t("settings:appearance.defaultViewOptions.list")}</option>
          </select>
        </SettingRow>

        <SettingRow
          label={t("settings:appearance.preferredCurrencyLabel")}
          description={t("settings:appearance.preferredCurrencyHelp")}
        >
          <div className="w-48" data-testid="preferred-currency-row">
            <CurrencyCombobox
              value={preferredCurrency}
              onChange={(next) =>
                onChangeRemote(SETTING_APPEARANCE_PREFERRED_DISPLAY_CURRENCY, next)
              }
              disabled={!settings}
              variant="compact"
            />
          </div>
        </SettingRow>
      </div>
    </div>
  )
}

// Defaults for each toggle when the BE has no row yet — mirrors the
// `categoryDefaults` / `channelDefaults` maps in
// go/services/notifications/preferences.go. Kept in lockstep with the BE
// so the FE shows the same checked-state on first load as the BE would
// apply on the first send.
const NOTIFICATION_FIELD_DEFAULTS: Record<string, boolean> = {
  [SETTING_NOTIFICATIONS_WARRANTY_EXPIRY]: true,
  [SETTING_NOTIFICATIONS_MAINTENANCE_REMINDER]: true,
  [SETTING_NOTIFICATIONS_WEEKLY_DIGEST]: true,
  [SETTING_NOTIFICATIONS_PRICE_DROP]: true,
  [SETTING_NOTIFICATIONS_CHANNEL_EMAIL]: true,
  [SETTING_NOTIFICATIONS_CHANNEL_PUSH]: false,
}

interface NotificationRow {
  field: string
  // Settings model key — used to read the current value from the
  // SettingsObject the hook returns.
  read: (s: SettingsObject) => boolean | undefined
  labelKey: string
  descriptionKey: string
  testId: string
}

const NOTIFICATION_GROUPS: Array<{ titleKey: string; rows: NotificationRow[] }> = [
  {
    titleKey: "settings:notifications.groups.reminders",
    rows: [
      {
        field: SETTING_NOTIFICATIONS_WARRANTY_EXPIRY,
        read: (s) => s.notificationsWarrantyExpiry,
        labelKey: "settings:notifications.rows.warrantyExpiry",
        descriptionKey: "settings:notifications.rows.warrantyExpiryDescription",
        testId: "notification-row-warranty-expiry",
      },
      {
        field: SETTING_NOTIFICATIONS_MAINTENANCE_REMINDER,
        read: (s) => s.notificationsMaintenanceReminder,
        labelKey: "settings:notifications.rows.maintenanceReminder",
        descriptionKey: "settings:notifications.rows.maintenanceReminderDescription",
        testId: "notification-row-maintenance-reminder",
      },
    ],
  },
  {
    titleKey: "settings:notifications.groups.updates",
    rows: [
      {
        field: SETTING_NOTIFICATIONS_WEEKLY_DIGEST,
        read: (s) => s.notificationsWeeklyDigest,
        labelKey: "settings:notifications.rows.weeklyDigest",
        descriptionKey: "settings:notifications.rows.weeklyDigestDescription",
        testId: "notification-row-weekly-digest",
      },
      {
        field: SETTING_NOTIFICATIONS_PRICE_DROP,
        read: (s) => s.notificationsPriceDrop,
        labelKey: "settings:notifications.rows.priceDrop",
        descriptionKey: "settings:notifications.rows.priceDropDescription",
        testId: "notification-row-price-drop",
      },
    ],
  },
  {
    titleKey: "settings:notifications.groups.channels",
    rows: [
      {
        field: SETTING_NOTIFICATIONS_CHANNEL_EMAIL,
        read: (s) => s.notificationsChannelEmail,
        labelKey: "settings:notifications.rows.channelEmail",
        descriptionKey: "settings:notifications.rows.channelEmailDescription",
        testId: "notification-row-channel-email",
      },
      {
        field: SETTING_NOTIFICATIONS_CHANNEL_PUSH,
        read: (s) => s.notificationsChannelPush,
        labelKey: "settings:notifications.rows.channelPush",
        descriptionKey: "settings:notifications.rows.channelPushDescription",
        testId: "notification-row-channel-push",
      },
    ],
  },
]

function NotificationsSection() {
  const { t } = useTranslation()
  const toast = useAppToast()
  const settingsQuery = useUserSettings()
  const patchMutation = usePatchSetting()

  const settings = settingsQuery.data

  function onToggle(field: string, next: boolean) {
    patchMutation.mutate(
      { field, value: next },
      {
        onError: () => toast.error(t("settings:notifications.errors.saveFailed")),
      }
    )
  }

  if (settingsQuery.isError) {
    return (
      <div className="space-y-4" data-testid="section-notifications">
        <SectionTitle>{t("settings:notifications.title")}</SectionTitle>
        <p
          className="rounded-md border border-destructive/40 bg-destructive/5 p-4 text-sm text-destructive"
          role="alert"
          data-testid="notifications-load-error"
        >
          {t("settings:notifications.errors.loadFailed")}
        </p>
      </div>
    )
  }

  return (
    <div className="space-y-6" data-testid="section-notifications">
      <SectionTitle>{t("settings:notifications.title")}</SectionTitle>
      {NOTIFICATION_GROUPS.map((group) => (
        <div key={group.titleKey} className="space-y-2">
          <h3 className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">
            {t(group.titleKey)}
          </h3>
          <div className="rounded-xl border border-border divide-y divide-border">
            {group.rows.map((row) => {
              const checked = settings
                ? (row.read(settings) ?? NOTIFICATION_FIELD_DEFAULTS[row.field])
                : NOTIFICATION_FIELD_DEFAULTS[row.field]
              return (
                <div
                  key={row.field}
                  className="flex items-start justify-between gap-4 p-4"
                  data-testid={row.testId}
                >
                  <div className="min-w-0">
                    <p className="text-sm font-medium">{t(row.labelKey)}</p>
                    <p className="mt-0.5 text-xs text-muted-foreground leading-relaxed">
                      {t(row.descriptionKey)}
                    </p>
                  </div>
                  <Switch
                    checked={checked}
                    onCheckedChange={(next) => onToggle(row.field, next)}
                    disabled={!settings}
                    aria-label={t(row.labelKey)}
                  />
                </div>
              )
            })}
          </div>
        </div>
      ))}
    </div>
  )
}

function PrivacySection() {
  const { t } = useTranslation()
  // Sessions count drives the badge on the "Active sessions" row. Loading +
  // error are intentionally silent — the row keeps the link affordance even
  // when the count can't be fetched; the page itself renders the real state.
  const sessionsQuery = useSessionsList()
  const sessionCount = sessionsQuery.data?.sessions?.length ?? 0

  return (
    <div className="space-y-4" data-testid="section-privacy">
      <SectionTitle>{t("settings:privacy.title")}</SectionTitle>
      <div className="rounded-xl border border-border divide-y divide-border">
        {/* Two-factor stays a ComingSoonBanner until #1380 ships. We keep it
            in the same divided card as the live rows so the layout matches
            the design mock at design-mocks/src/views/SettingsView.tsx. */}
        <PrivacyRow
          label={t("settings:privacy.rows.twoFactor")}
          description={t("settings:privacy.rows.twoFactorDescription")}
          badge={t("settings:privacy.rows.twoFactorBadge")}
          badgeVariant="outline"
          to={null}
          testId="privacy-row-twoFactor"
        />
        <PrivacyRow
          label={t("settings:privacy.rows.activeSessions")}
          description={t("settings:privacy.rows.activeSessionsDescription")}
          badge={sessionCount > 0 ? t("settings:privacy.rows.activeSessionsBadge", { count: sessionCount }) : null}
          badgeVariant="secondary"
          to="/profile/sessions"
          testId="privacy-row-activeSessions"
        />
        <PrivacyRow
          label={t("settings:privacy.rows.loginHistory")}
          description={t("settings:privacy.rows.loginHistoryDescription")}
          badge={null}
          badgeVariant="secondary"
          to="/profile/login-history"
          testId="privacy-row-loginHistory"
        />
      </div>
      {/* Connected accounts stays a ComingSoonBanner per #1644 acceptance. */}
      <ComingSoonBanner surface="connectedAccounts" />
    </div>
  )
}

interface PrivacyRowProps {
  label: string
  description: string
  badge: string | null
  badgeVariant: "outline" | "secondary"
  to: string | null
  testId: string
}

function PrivacyRow({ label, description, badge, badgeVariant, to, testId }: PrivacyRowProps) {
  const inner = (
    <>
      <div className="text-left">
        <p className="text-sm font-medium">{label}</p>
        <p className="text-xs text-muted-foreground mt-0.5">{description}</p>
      </div>
      <div className="flex items-center gap-2">
        {badge ? (
          <Badge variant={badgeVariant} className="text-xs">
            {badge}
          </Badge>
        ) : null}
        {to ? (
          <ChevronRight className="size-4 text-muted-foreground" aria-hidden="true" />
        ) : null}
      </div>
    </>
  )
  if (!to) {
    return (
      <div
        className="flex items-center justify-between p-4"
        data-testid={testId}
        aria-disabled="true"
      >
        {inner}
      </div>
    )
  }
  return (
    <Link
      to={to}
      data-testid={testId}
      className="flex items-center justify-between p-4 text-left transition-colors hover:bg-muted/50"
    >
      {inner}
    </Link>
  )
}

function HelpSection() {
  const { t } = useTranslation()

  // Five rows: docs (#1384), shortcuts (#1385), what's new (#1386 — with
  // a marketing v{Major.Minor} badge per design-audit #1536), send
  // feedback (#1387 — still a ComingSoon stub), contact support
  // (mailto fallback while a real ticketing surface is scoped). Real
  // destinations behind each route are ComingSoonPage already; this
  // section mostly acts as a discovery aid.
  type HelpRowKey = "documentation" | "shortcuts" | "whatsNew" | "feedback" | "contactSupport"
  const rows: Array<{ key: HelpRowKey; href: string | null }> = [
    { key: "documentation", href: "/help" },
    { key: "shortcuts", href: "/help/shortcuts" },
    { key: "whatsNew", href: "/whats-new" },
    { key: "feedback", href: null },
    { key: "contactSupport", href: "mailto:support@inventario.app" },
  ]

  const versionShort = shortAppVersion(APP_VERSION)

  return (
    <div className="space-y-4" data-testid="section-help">
      <SectionTitle>{t("settings:help.title")}</SectionTitle>
      <div className="rounded-xl border border-border divide-y divide-border">
        {rows.map(({ key, href }) => {
          if (!href) {
            return (
              <div key={key} className="p-4">
                <ComingSoonBanner surface="sendFeedback" />
              </div>
            )
          }
          // Two-arm union: external (mailto) or in-app route. Both use the
          // same chrome — chevron-right + label + description. The
          // version Badge only renders on the "whatsNew" row.
          const labelKey = key
          const isExternal = href.startsWith("mailto:") || href.startsWith("http")
          const RowInner = (
            <>
              <div className="flex items-center gap-2">
                <p className="text-sm font-medium">{t(`settings:help.rows.${labelKey}`)}</p>
                {labelKey === "whatsNew" ? (
                  <Badge variant="secondary" data-testid="help-row-whatsNew-badge">
                    {t("settings:help.rows.whatsNewBadge", { version: versionShort })}
                  </Badge>
                ) : null}
              </div>
              <p className="mt-0.5 text-xs text-muted-foreground">
                {t(`settings:help.rows.${labelKey}Description`)}
              </p>
            </>
          )
          if (isExternal) {
            return (
              <a
                key={key}
                href={href}
                data-testid={`help-row-${key}`}
                className="flex items-center justify-between p-4 text-left transition-colors hover:bg-muted/50"
              >
                <div>{RowInner}</div>
                <ArrowRight className="size-4 text-muted-foreground" aria-hidden="true" />
              </a>
            )
          }
          return (
            <Link
              key={key}
              to={href}
              data-testid={`help-row-${key}`}
              className="flex items-center justify-between p-4 text-left transition-colors hover:bg-muted/50"
            >
              <div>{RowInner}</div>
              <ArrowRight className="size-4 text-muted-foreground" aria-hidden="true" />
            </Link>
          )
        })}
      </div>
      <p className="text-center text-xs text-muted-foreground">{t("settings:help.version")}</p>
    </div>
  )
}

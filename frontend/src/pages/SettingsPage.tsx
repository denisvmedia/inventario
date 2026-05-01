import { useState, type ReactNode } from "react"
import { useTranslation } from "react-i18next"
import { Link, useNavigate } from "react-router-dom"
import {
  ArrowRight,
  Bell,
  Check,
  ChevronRight,
  CircleHelp,
  Database,
  Download,
  LogOut,
  Monitor,
  Moon,
  Palette,
  Shield,
  Sun,
  Trash2,
  User,
} from "lucide-react"

import { ComingSoonBanner } from "@/components/coming-soon"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Separator } from "@/components/ui/separator"
import { useAuth } from "@/features/auth/AuthContext"
import { useLogout } from "@/features/auth/hooks"
import { useCurrentGroup } from "@/features/group/GroupContext"
import { useConfirm } from "@/hooks/useConfirm"
import { useDensity, DENSITIES, type Density } from "@/hooks/useDensity"
import { useTheme } from "@/components/theme-provider"
import { i18next, SUPPORTED_LANGUAGES, type SupportedLanguage } from "@/i18n"
import { formatDate } from "@/lib/intl"
import { cn } from "@/lib/utils"
import { RouteTitle } from "@/components/routing/RouteTitle"

type SectionId = "account" | "appearance" | "notifications" | "privacy" | "data" | "help"

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
  { id: "data", icon: Database },
  { id: "help", icon: CircleHelp },
]

// SettingsPage — preferences hub. Two-pane layout:
//   - left rail: section nav + sign-out shortcut
//   - right pane: the selected section's content
//
// Theme / density / locale persist to localStorage via the existing
// providers and i18next detection cache; they don't round-trip to the
// backend on this page (system-wide /settings is a separate admin scope
// owned by the System view).
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
            {active === "data" ? <DataSection /> : null}
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
  const { groups, currentGroup } = useCurrentGroup()
  const memberSince = user?.created_at
    ? formatDate(user.created_at, { style: "long" })
    : t("settings:account.memberSinceUnknown")

  // Show the group the user is currently looking at (URL slug) — falls
  // back to "no default" if Settings is reached from a non-group route.
  const groupLabel =
    currentGroup?.name ??
    (user?.default_group_id
      ? (groups?.find((g) => g.id === user.default_group_id)?.name ?? "—")
      : t("settings:profile.noGroupSelection"))

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
            <Link to="/profile/edit" data-testid="settings-edit-profile">
              {t("settings:account.editProfile")}
            </Link>
          </Button>
        </div>
      </div>

      <Separator />

      <div className="divide-y divide-border">
        <SettingRow label={t("settings:account.displayName")}>
          <span className="text-sm text-muted-foreground">{user?.name ?? "—"}</span>
        </SettingRow>
        <SettingRow label={t("settings:account.email")}>
          <span className="text-sm text-muted-foreground">{user?.email ?? "—"}</span>
        </SettingRow>
        <SettingRow label={t("settings:profile.defaultGroup")}>
          <span className="text-sm text-muted-foreground">{groupLabel}</span>
        </SettingRow>
        <SettingRow label={t("settings:account.memberSince")}>
          <span className="text-sm text-muted-foreground">{memberSince}</span>
        </SettingRow>
        <SettingRow
          label={t("settings:account.changePassword")}
          description={t("settings:account.changePasswordDescription")}
        >
          <Button asChild variant="outline" size="sm">
            <Link to="/profile/edit" data-testid="settings-change-password">
              {t("settings:profile.password.title")}
            </Link>
          </Button>
        </SettingRow>
      </div>
    </div>
  )
}

function AppearanceSection() {
  const { t, i18n } = useTranslation()
  const { theme, setTheme } = useTheme()
  const { density, setDensity } = useDensity()

  const THEMES = [
    { id: "system" as const, icon: Monitor },
    { id: "light" as const, icon: Sun },
    { id: "dark" as const, icon: Moon },
  ]

  const currentLanguage = (i18n.resolvedLanguage ?? "en") as SupportedLanguage

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
      </div>
    </div>
  )
}

function NotificationsSection() {
  const { t } = useTranslation()
  return (
    <div className="space-y-4" data-testid="section-notifications">
      <SectionTitle>{t("settings:notifications.title")}</SectionTitle>
      <ComingSoonBanner surface="notificationPreferences" />
      <ComingSoonBanner surface="maintenanceReminders" />
    </div>
  )
}

function PrivacySection() {
  const { t } = useTranslation()
  return (
    <div className="space-y-4" data-testid="section-privacy">
      <SectionTitle>{t("settings:privacy.title")}</SectionTitle>
      <ComingSoonBanner surface="twoFactor" />
      <ComingSoonBanner surface="activeSessions" />
      <ComingSoonBanner surface="loginHistory" />
      <ComingSoonBanner surface="connectedAccounts" />
    </div>
  )
}

function DataSection() {
  const { t } = useTranslation()
  const { currentGroup } = useCurrentGroup()
  const confirm = useConfirm()

  // Account deletion isn't backend-supported yet; render a danger-zone
  // panel that opens a confirm dialog explaining the limitation rather
  // than fake a destructive action that does nothing.
  async function handleDeleteAccount() {
    await confirm({
      title: t("settings:data.deleteUnavailableTitle"),
      description: t("settings:data.deleteUnavailableDescription"),
      confirmLabel: t("settings:data.deleteUnavailableConfirm"),
      cancelLabel: t("settings:profile.edit.cancel"),
      destructive: false,
    })
  }

  return (
    <div className="space-y-6" data-testid="section-data">
      <SectionTitle>{t("settings:data.title")}</SectionTitle>

      <div className="rounded-xl border border-border bg-card p-4 space-y-3">
        <div>
          <p className="text-sm font-medium">{t("settings:data.exportTitle")}</p>
          <p className="mt-0.5 text-xs text-muted-foreground">
            {t("settings:data.exportDescription")}
          </p>
        </div>
        {currentGroup?.slug ? (
          <Button asChild size="sm" variant="outline" className="gap-1.5">
            <Link
              to={`/g/${encodeURIComponent(currentGroup.slug)}/exports`}
              data-testid="settings-open-exports"
            >
              <Download className="size-3.5" aria-hidden="true" />
              {t("settings:data.exportCta")}
            </Link>
          </Button>
        ) : (
          <p className="text-xs text-muted-foreground">{t("settings:profile.noGroupSet")}</p>
        )}
      </div>

      <ComingSoonBanner surface="storageQuota" />

      <div className="rounded-xl border border-destructive/30 bg-destructive/5 p-4 space-y-2">
        <p className="text-sm font-semibold text-destructive">
          {t("settings:data.dangerZoneTitle")}
        </p>
        <p className="text-xs text-muted-foreground">{t("settings:data.dangerZoneDescription")}</p>
        <Button
          variant="outline"
          size="sm"
          className="text-destructive border-destructive/40 hover:bg-destructive/10 gap-1.5"
          onClick={handleDeleteAccount}
          data-testid="delete-account-button"
        >
          <Trash2 className="size-3.5" aria-hidden="true" />
          {t("settings:data.deleteAccount")}
        </Button>
      </div>
    </div>
  )
}

function HelpSection() {
  const { t } = useTranslation()

  // All four rows are stubs today: docs (#1384), shortcuts (#1385),
  // what's new (#1386), feedback (#1387). Real destinations behind each
  // route are ComingSoonPage already; this section mostly acts as a
  // discovery aid + a place for #1387 to grow into a real form later.
  type HelpRowKey = "documentation" | "shortcuts" | "whatsNew" | "feedback"
  const rows: Array<{ key: HelpRowKey; href: string | null }> = [
    { key: "documentation", href: "/help" },
    { key: "shortcuts", href: "/help/shortcuts" },
    { key: "whatsNew", href: "/whats-new" },
    { key: "feedback", href: null },
  ]

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
          const labelKey = key as "documentation" | "shortcuts" | "whatsNew"
          return (
            <Link
              key={key}
              to={href}
              data-testid={`help-row-${key}`}
              className="flex items-center justify-between p-4 text-left transition-colors hover:bg-muted/50"
            >
              <div>
                <p className="text-sm font-medium">{t(`settings:help.rows.${labelKey}`)}</p>
                <p className="mt-0.5 text-xs text-muted-foreground">
                  {t(`settings:help.rows.${labelKey}Description`)}
                </p>
              </div>
              <ArrowRight className="size-4 text-muted-foreground" aria-hidden="true" />
            </Link>
          )
        })}
      </div>
      <p className="text-center text-xs text-muted-foreground">{t("settings:help.version")}</p>
    </div>
  )
}

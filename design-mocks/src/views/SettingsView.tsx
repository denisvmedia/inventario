import { useState } from "react"
import { User, Bell, Shield, Palette, Database, Circle as HelpCircle, LogOut, ChevronRight, Moon, Sun, Monitor, Check, Trash2, Download } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Switch } from "@/components/ui/switch"
import { Separator } from "@/components/ui/separator"
import { Badge } from "@/components/ui/badge"
import { Slider } from "@/components/ui/slider"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { useTheme } from "@/components/theme-provider"
import { CurrencyCombobox } from "@/components/CurrencyCombobox"

type SettingsSection =
  | "account"
  | "appearance"
  | "notifications"
  | "privacy"
  | "data"
  | "help"

const SECTIONS = [
  { id: "account" as SettingsSection, label: "Account", icon: User },
  { id: "appearance" as SettingsSection, label: "Appearance", icon: Palette },
  { id: "notifications" as SettingsSection, label: "Notifications", icon: Bell },
  { id: "privacy" as SettingsSection, label: "Privacy & Security", icon: Shield },
  { id: "data" as SettingsSection, label: "Data & Storage", icon: Database },
  { id: "help" as SettingsSection, label: "Help & Support", icon: HelpCircle },
]

interface SettingsViewProps {
  onNavigate?: (view: string) => void
}

export function SettingsView({ onNavigate }: SettingsViewProps) {
  const [active, setActive] = useState<SettingsSection>("appearance")

  return (
    <div className="flex h-full min-h-0 flex-col">
      <div className="flex-1 overflow-y-auto">
        <div className="flex flex-col gap-8 p-6 max-w-4xl mx-auto w-full">
          <div>
            <h1 className="scroll-m-20 text-3xl font-semibold tracking-tight">Preferences</h1>
            <p className="mt-1 text-muted-foreground">Manage your account and preferences.</p>
          </div>

          <div className="flex gap-6">
            {/* Nav */}
            <aside className="w-48 shrink-0">
              <nav className="space-y-0.5">
                {SECTIONS.map((s) => (
                  <button
                    key={s.id}
                    onClick={() => setActive(s.id)}
                    className={`flex w-full items-center gap-2.5 rounded-md px-3 py-2 text-sm transition-colors ${
                      active === s.id
                        ? "bg-accent text-accent-foreground font-medium"
                        : "text-muted-foreground hover:bg-muted hover:text-foreground"
                    }`}
                  >
                    <s.icon className="size-4 shrink-0" />
                    {s.label}
                    {active === s.id && <ChevronRight className="ml-auto size-3.5" />}
                  </button>
                ))}

                <Separator className="my-2" />
                <button
                  onClick={() => onNavigate?.("auth")}
                  className="flex w-full items-center gap-2.5 rounded-md px-3 py-2 text-sm text-destructive hover:bg-destructive/10 transition-colors"
                >
                  <LogOut className="size-4 shrink-0" />
                  Sign out
                </button>
              </nav>
            </aside>

            {/* Content */}
            <div className="flex-1 min-w-0">
              {active === "account" && <AccountSection onNavigate={onNavigate} />}
              {active === "appearance" && <AppearanceSection />}
              {active === "notifications" && <NotificationsSection />}
              {active === "privacy" && <PrivacySection />}
              {active === "data" && <DataSection />}
              {active === "help" && <HelpSection />}
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}

function SectionTitle({ children }: { children: React.ReactNode }) {
  return (
    <h2 className="text-base font-semibold mb-4">{children}</h2>
  )
}

function SettingRow({
  label,
  description,
  children,
}: {
  label: string
  description?: string
  children: React.ReactNode
}) {
  return (
    <div className="flex items-start justify-between gap-4 py-3.5">
      <div className="min-w-0">
        <p className="text-sm font-medium">{label}</p>
        {description && <p className="text-xs text-muted-foreground mt-0.5 leading-relaxed">{description}</p>}
      </div>
      <div className="shrink-0">{children}</div>
    </div>
  )
}

function AccountSection({ onNavigate }: { onNavigate?: (v: string) => void }) {
  return (
    <div className="space-y-6">
      <div>
        <SectionTitle>Account</SectionTitle>
        {/* Profile card */}
        <div className="flex items-center gap-4 rounded-xl border border-border bg-card p-4 mb-4">
          <div className="flex size-14 items-center justify-center rounded-full bg-primary text-primary-foreground text-lg font-semibold shrink-0">
            AJ
          </div>
          <div className="flex-1 min-w-0">
            <p className="font-semibold truncate">Alex Johnson</p>
            <p className="text-sm text-muted-foreground truncate">alex@example.com</p>
            <Badge variant="secondary" className="mt-1 text-xs">Free plan</Badge>
          </div>
          <Button variant="outline" size="sm" onClick={() => onNavigate?.("profile")}>
            Edit profile
          </Button>
        </div>

        <Separator />
        <div className="divide-y divide-border">
          {[
            { label: "Display name", value: "Alex Johnson" },
            { label: "Email address", value: "alex@example.com" },
            { label: "Member since", value: "January 2024" },
          ].map((row) => (
            <SettingRow key={row.label} label={row.label}>
              <span className="text-sm text-muted-foreground">{row.value}</span>
            </SettingRow>
          ))}
        </div>
      </div>
    </div>
  )
}

function AppearanceSection() {
  const { theme, setTheme } = useTheme()
  const [currency, setCurrency] = useState("USD")
  const [fontSize, setFontSize] = useState([14])

  const THEMES = [
    { id: "light", label: "Light", icon: Sun },
    { id: "dark", label: "Dark", icon: Moon },
    { id: "system", label: "System", icon: Monitor },
  ] as const

  return (
    <div className="space-y-6">
      <div>
        <SectionTitle>Appearance</SectionTitle>
        <div className="space-y-6">
          <div>
            <p className="text-sm font-medium mb-3">Theme</p>
            <div className="grid grid-cols-3 gap-3">
              {THEMES.map((t) => (
                <button
                  key={t.id}
                  onClick={() => setTheme(t.id)}
                  className={`flex flex-col items-center gap-2 rounded-xl border-2 p-4 transition-all ${
                    theme === t.id
                      ? "border-primary bg-primary/5"
                      : "border-border hover:border-muted-foreground/40"
                  }`}
                >
                  <t.icon className={`size-5 ${theme === t.id ? "text-primary" : "text-muted-foreground"}`} />
                  <span className={`text-xs font-medium ${theme === t.id ? "text-primary" : "text-muted-foreground"}`}>
                    {t.label}
                  </span>
                  {theme === t.id && (
                    <span className="flex size-4 items-center justify-center rounded-full bg-primary">
                      <Check className="size-2.5 text-primary-foreground" />
                    </span>
                  )}
                </button>
              ))}
            </div>
          </div>

          <Separator />
          <div className="divide-y divide-border">
            <SettingRow label="Currency" description="Used for item values throughout the app">
              <CurrencyCombobox value={currency} onValueChange={setCurrency} variant="compact" />
            </SettingRow>
            <SettingRow label="Default view" description="Items list layout preference">
              <Select defaultValue="grid">
                <SelectTrigger className="w-24 h-8 text-sm">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="grid">Grid</SelectItem>
                  <SelectItem value="list">List</SelectItem>
                </SelectContent>
              </Select>
            </SettingRow>
          </div>

          <Separator />
          <div>
            <div className="flex items-center justify-between mb-3">
              <p className="text-sm font-medium">Interface density</p>
              <span className="text-sm text-muted-foreground">{fontSize[0]}px</span>
            </div>
            <Slider
              value={fontSize}
              onValueChange={setFontSize}
              min={12}
              max={18}
              step={1}
              className="w-full"
            />
            <div className="flex justify-between mt-1.5">
              <span className="text-xs text-muted-foreground">Compact</span>
              <span className="text-xs text-muted-foreground">Comfortable</span>
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}

function NotificationsSection() {
  const [settings, setSettings] = useState({
    warrantyExpiry: true,
    weeklyDigest: false,
    maintenanceReminders: true,
    priceAlerts: false,
    emailNotifs: true,
    pushNotifs: false,
  })

  const toggle = (key: keyof typeof settings) =>
    setSettings((s) => ({ ...s, [key]: !s[key] }))

  const GROUPS = [
    {
      title: "Reminders",
      items: [
        {
          key: "warrantyExpiry" as const,
          label: "Warranty expiry",
          description: "Get notified 60, 30, and 7 days before a warranty expires",
        },
        {
          key: "maintenanceReminders" as const,
          label: "Maintenance reminders",
          description: "Scheduled care alerts for items you've set up",
        },
      ],
    },
    {
      title: "Updates",
      items: [
        {
          key: "weeklyDigest" as const,
          label: "Weekly digest",
          description: "Summary of inventory changes and upcoming expirations",
        },
        {
          key: "priceAlerts" as const,
          label: "Price drop alerts",
          description: "Notify when tracked supply links drop in price",
        },
      ],
    },
    {
      title: "Channels",
      items: [
        {
          key: "emailNotifs" as const,
          label: "Email notifications",
          description: "Receive notifications at alex@example.com",
        },
        {
          key: "pushNotifs" as const,
          label: "Push notifications",
          description: "Browser and mobile push (requires permission)",
        },
      ],
    },
  ]

  return (
    <div>
      <SectionTitle>Notifications</SectionTitle>
      <div className="space-y-5">
        {GROUPS.map((group) => (
          <div key={group.title}>
            <p className="text-xs font-semibold uppercase tracking-wide text-muted-foreground mb-2">
              {group.title}
            </p>
            <div className="rounded-xl border border-border divide-y divide-border">
              {group.items.map((item) => (
                <div key={item.key} className="flex items-start justify-between gap-4 p-4">
                  <div>
                    <p className="text-sm font-medium">{item.label}</p>
                    <p className="text-xs text-muted-foreground mt-0.5 leading-relaxed">
                      {item.description}
                    </p>
                  </div>
                  <Switch
                    checked={settings[item.key]}
                    onCheckedChange={() => toggle(item.key)}
                    className="shrink-0 mt-0.5"
                  />
                </div>
              ))}
            </div>
          </div>
        ))}
      </div>
    </div>
  )
}

function PrivacySection() {
  return (
    <div>
      <SectionTitle>Privacy & Security</SectionTitle>
      <div className="space-y-4">
        <div className="rounded-xl border border-border divide-y divide-border">
          {[
            {
              label: "Two-factor authentication",
              description: "Secure your account with 2FA",
              badge: "Inactive",
              badgeVariant: "outline" as const,
            },
            {
              label: "Active sessions",
              description: "Manage devices logged into your account",
              badge: "2 active",
              badgeVariant: "secondary" as const,
            },
            {
              label: "Login history",
              description: "Review recent sign-in activity",
              badge: null,
              badgeVariant: "secondary" as const,
            },
          ].map((row) => (
            <button
              key={row.label}
              className="flex w-full items-center justify-between p-4 text-left hover:bg-muted/50 transition-colors"
            >
              <div>
                <p className="text-sm font-medium">{row.label}</p>
                <p className="text-xs text-muted-foreground mt-0.5">{row.description}</p>
              </div>
              <div className="flex items-center gap-2">
                {row.badge && (
                  <Badge variant={row.badgeVariant} className="text-xs">
                    {row.badge}
                  </Badge>
                )}
                <ChevronRight className="size-4 text-muted-foreground" />
              </div>
            </button>
          ))}
        </div>

        <div className="rounded-xl border border-destructive/30 bg-destructive/5 p-4 space-y-2">
          <p className="text-sm font-semibold text-destructive">Danger zone</p>
          <p className="text-xs text-muted-foreground">
            Once you delete your account, there is no going back. All your data will be permanently removed.
          </p>
          <Button variant="outline" size="sm" className="text-destructive border-destructive/40 hover:bg-destructive/10 mt-2">
            <Trash2 className="size-3.5 mr-1.5" />
            Delete account
          </Button>
        </div>
      </div>
    </div>
  )
}

function DataSection() {
  const [autoBackup, setAutoBackup] = useState(true)
  const [syncEnabled, setSyncEnabled] = useState(false)

  return (
    <div>
      <SectionTitle>Data & Storage</SectionTitle>
      <div className="space-y-5">
        <div className="rounded-xl border border-border p-4 space-y-3">
          <div className="flex items-center justify-between">
            <p className="text-sm font-medium">Storage used</p>
            <span className="text-sm text-muted-foreground">2.4 MB of 100 MB</span>
          </div>
          <div className="h-2 rounded-full bg-muted overflow-hidden">
            <div className="h-full w-[2.4%] rounded-full bg-primary" />
          </div>
          <p className="text-xs text-muted-foreground">97.6 MB available on free plan</p>
        </div>

        <div className="rounded-xl border border-border divide-y divide-border">
          <div className="flex items-center justify-between p-4">
            <div>
              <p className="text-sm font-medium">Automatic backup</p>
              <p className="text-xs text-muted-foreground mt-0.5">Back up your data daily</p>
            </div>
            <Switch checked={autoBackup} onCheckedChange={setAutoBackup} />
          </div>
          <div className="flex items-center justify-between p-4">
            <div>
              <p className="text-sm font-medium">Cross-device sync</p>
              <p className="text-xs text-muted-foreground mt-0.5">Requires Pro plan</p>
            </div>
            <Switch checked={syncEnabled} onCheckedChange={setSyncEnabled} disabled />
          </div>
        </div>

        <div className="flex gap-3">
          <Button variant="outline" size="sm" className="gap-1.5">
            <Download className="size-3.5" />
            Export data (JSON)
          </Button>
          <Button variant="outline" size="sm" className="gap-1.5">
            <Download className="size-3.5" />
            Export data (CSV)
          </Button>
        </div>
      </div>
    </div>
  )
}

function HelpSection() {
  return (
    <div>
      <SectionTitle>Help & Support</SectionTitle>
      <div className="space-y-4">
        <div className="rounded-xl border border-border divide-y divide-border">
          {[
            { label: "Documentation", description: "Browse guides and tutorials" },
            { label: "Keyboard shortcuts", description: "View all available shortcuts" },
            { label: "What's new", description: "See latest features and updates", badge: "v1.2" },
            { label: "Send feedback", description: "Help us improve Inventario" },
            { label: "Contact support", description: "Get help from the team" },
          ].map((row) => (
            <button
              key={row.label}
              className="flex w-full items-center justify-between p-4 text-left hover:bg-muted/50 transition-colors"
            >
              <div>
                <div className="flex items-center gap-2">
                  <p className="text-sm font-medium">{row.label}</p>
                  {row.badge && (
                    <Badge variant="secondary" className="text-[10px] h-4 px-1.5">
                      {row.badge}
                    </Badge>
                  )}
                </div>
                <p className="text-xs text-muted-foreground mt-0.5">{row.description}</p>
              </div>
              <ChevronRight className="size-4 text-muted-foreground" />
            </button>
          ))}
        </div>
        <p className="text-xs text-center text-muted-foreground">Inventario v1.2.0 · © 2026</p>
      </div>
    </div>
  )
}

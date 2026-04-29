import { useTranslation } from "react-i18next"
import { Link } from "react-router-dom"
import { Building2, Calendar, Mail, Pencil, Zap } from "lucide-react"

import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Separator } from "@/components/ui/separator"
import { ComingSoonBanner } from "@/components/coming-soon"
import { RouteTitle } from "@/components/routing/RouteTitle"
import { useAuth } from "@/features/auth/AuthContext"
import { useGroups } from "@/features/group/hooks"
import { formatDate } from "@/lib/intl"

// Initials for the avatar fallback. "Alex Johnson" → "AJ", "alex" → "A".
// We keep at most two letters and fall back to the email's local part if
// `name` isn't available, then to "?" so the badge never reads as empty.
function initialsFor(name?: string, email?: string): string {
  const source = name?.trim() || (email?.split("@")[0] ?? "")
  if (!source) return "?"
  const parts = source.split(/\s+/).filter(Boolean)
  const letters = (parts[0]?.[0] ?? "") + (parts[parts.length - 1]?.[0] ?? "")
  return (letters || source[0] || "?").toUpperCase().slice(0, 2)
}

export function ProfilePage() {
  const { t } = useTranslation()
  const { user } = useAuth()
  const { data: groups } = useGroups()

  const name = user?.name?.trim() || (user?.email?.split("@")[0] ?? "")
  const email = user?.email ?? t("settings:profile.noEmail")
  const memberSince = user?.created_at && formatDate(user.created_at, { style: "long" })
  const defaultGroup =
    user?.default_group_id && groups
      ? (groups.find((g) => g.id === user.default_group_id) ?? null)
      : null

  return (
    <>
      <RouteTitle title={t("settings:profile.title")} />
      <div className="mx-auto flex w-full max-w-3xl flex-col gap-6" data-testid="profile-page">
        {/* Identity card. Banner + avatar + name + plan badge. The cover
            stripe pattern is taken from the design mock; the avatar shows
            initials because uploads are tracked under #1382. */}
        <section className="rounded-2xl border border-border overflow-hidden">
          <div className="relative h-24 bg-primary overflow-hidden">
            <div
              aria-hidden="true"
              className="absolute inset-0 opacity-[0.07]"
              style={{
                backgroundImage:
                  "repeating-linear-gradient(-45deg, currentColor 0, currentColor 1px, transparent 1px, transparent 10px)",
              }}
            />
            <Button
              asChild
              variant="secondary"
              size="sm"
              className="absolute top-3 right-3 gap-1.5 bg-background/80 hover:bg-background/95 backdrop-blur-sm shadow-sm"
            >
              <Link to="/profile/edit" data-testid="profile-edit-link">
                <Pencil className="size-3.5" />
                {t("settings:profile.editProfile")}
              </Link>
            </Button>
          </div>

          <div className="px-5 pb-5">
            <div className="flex items-end justify-between -mt-9 mb-3">
              <div
                className="flex size-[72px] items-center justify-center rounded-2xl bg-card border-4 border-background text-xl font-bold text-primary shadow-md"
                aria-hidden="true"
              >
                {initialsFor(user?.name, user?.email)}
              </div>
              <div className="flex items-center gap-2 mb-1">
                <Badge variant="outline" className="text-xs font-medium">
                  {t("settings:profile.planFree")}
                </Badge>
                <Button asChild variant="outline" size="sm" className="gap-1.5 h-7 px-2.5 text-xs">
                  <Link to="/plans" data-testid="profile-upgrade-link">
                    <Zap className="size-3" />
                    {t("settings:profile.upgrade")}
                  </Link>
                </Button>
              </div>
            </div>

            <div className="space-y-0.5 mb-4">
              <h1 className="text-xl font-bold tracking-tight" data-testid="profile-name">
                {name}
              </h1>
              <p className="text-sm text-muted-foreground" data-testid="profile-email">
                {email}
              </p>
            </div>

            <div className="flex flex-wrap gap-x-4 gap-y-1.5">
              {memberSince ? (
                <div className="flex items-center gap-1.5 text-xs text-muted-foreground">
                  <Calendar className="size-3.5" aria-hidden="true" />
                  <span>{t("settings:profile.memberSince", { date: memberSince })}</span>
                </div>
              ) : null}
              <div className="flex items-center gap-1.5 text-xs text-muted-foreground">
                <Building2 className="size-3.5" aria-hidden="true" />
                <span data-testid="profile-default-group">
                  {defaultGroup?.name
                    ? `${t("settings:profile.defaultGroup")}: ${defaultGroup.name}`
                    : t("settings:profile.noGroupSet")}
                </span>
              </div>
              {email ? (
                <div className="flex items-center gap-1.5 text-xs text-muted-foreground">
                  <Mail className="size-3.5" aria-hidden="true" />
                  <span>{email}</span>
                </div>
              ) : null}
            </div>
          </div>
        </section>

        {/* Avatar upload tracker — visible per-section stub linked to #1382.
            The decorative initials avatar above is the placeholder; this
            banner is what tells the user uploads are coming. */}
        <ComingSoonBanner surface="profilePhoto" />

        <Separator />

        <div className="text-sm text-muted-foreground">
          <p>
            {t("settings:profile.subtitle")}{" "}
            <Link
              to="/settings"
              className="font-medium text-foreground hover:underline underline-offset-4"
            >
              {t("settings:title")}
            </Link>
            .
          </p>
        </div>
      </div>
    </>
  )
}

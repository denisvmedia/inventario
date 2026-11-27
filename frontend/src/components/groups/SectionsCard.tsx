import { useTranslation } from "react-i18next"

import { Switch } from "@/components/ui/switch"
import type { LocationGroup } from "@/features/group/api"
import { useSetHiddenSections } from "@/features/group/hooks"
import { sectionDescription, sectionLabel } from "@/features/group/section-labels"
import { GROUP_SECTIONS, type GroupSection } from "@/features/group/sections"
import { toastMutationError } from "@/lib/mutation-error-toast"
import { cn } from "@/lib/utils"

// SectionsCard lets a group admin hide sections the group does not use, for
// every member (#1607). One switch per section, on while the section shows.
// Hiding keeps the data: items that hold warranties, loans or maintenance
// still show them, and the switch brings the section back.
//
// Members without the admin role see the same rows with the switches
// disabled, so they can tell why a section is missing.
interface SectionsCardProps {
  group: LocationGroup
  isAdmin: boolean
  className?: string
}

export function SectionsCard({ group, isAdmin, className }: SectionsCardProps) {
  const { t } = useTranslation()
  const mutation = useSetHiddenSections()
  const groupId = group.id ?? ""

  // While a save is in flight the switches show what was asked for, so a
  // click reads as done without waiting for the round-trip.
  const hidden: readonly GroupSection[] =
    mutation.isPending && mutation.variables
      ? mutation.variables.sections
      : (group.hidden_sections ?? [])

  function onToggle(section: GroupSection, visible: boolean) {
    const next = GROUP_SECTIONS.filter((s) => (s === section ? !visible : hidden.includes(s)))
    mutation.mutate(
      { groupId, sections: next },
      { onError: (err) => toastMutationError(err, t("groups:settings.hiddenSections.saveError")) }
    )
  }

  return (
    <div
      className={cn("rounded-xl border border-border bg-card p-6 space-y-4", className)}
      data-testid="sections-card"
    >
      <div>
        <h2 className="text-base font-semibold">{t("groups:settings.hiddenSections.title")}</h2>
        <p className="text-sm text-muted-foreground mt-0.5">
          {t("groups:settings.hiddenSections.subtitle")}
        </p>
      </div>

      <div className="divide-y divide-border">
        {GROUP_SECTIONS.map((section) => (
          <SectionRow
            key={section}
            section={section}
            checked={!hidden.includes(section)}
            disabled={!isAdmin || !groupId || mutation.isPending}
            onChange={(visible) => onToggle(section, visible)}
          />
        ))}
      </div>

      {!isAdmin ? (
        <p className="text-xs text-muted-foreground" data-testid="sections-card-admin-only">
          {t("groups:settings.hiddenSections.adminOnly")}
        </p>
      ) : null}
    </div>
  )
}

interface SectionRowProps {
  section: GroupSection
  checked: boolean
  disabled: boolean
  onChange: (visible: boolean) => void
}

function SectionRow({ section, checked, disabled, onChange }: SectionRowProps) {
  const { t } = useTranslation()
  const label = sectionLabel(t, section)
  return (
    <div className="flex items-center justify-between gap-4 py-3.5">
      <div className="min-w-0">
        <p className="text-sm font-medium">{label}</p>
        <p className="text-xs text-muted-foreground mt-0.5">{sectionDescription(t, section)}</p>
      </div>
      <Switch
        checked={checked}
        onCheckedChange={onChange}
        disabled={disabled}
        data-testid={`sections-toggle-${section}`}
        aria-label={label}
      />
    </div>
  )
}

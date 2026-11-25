import { useState } from "react"
import { useTranslation } from "react-i18next"
import { Ban, Check, ChevronDown, Tag as TagIcon } from "lucide-react"

import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
  CommandSeparator,
} from "@/components/ui/command"
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover"
import type { TagMatchMode } from "@/features/commodities/query"
import type { ResolvedTag } from "@/features/tags/hooks"
import { cn } from "@/lib/utils"

import { TAG_DOT_TONE } from "./TagBadge"

export interface TagFilterValue {
  include: string[]
  exclude: string[]
  mode: TagMatchMode
  untagged: boolean
}

export interface TagFilterMenuProps {
  // Every tag the picker offers.
  tags: ResolvedTag[]
  value: TagFilterValue
  onChange: (next: TagFilterValue) => void
  // Per slug, how many items the tag would list; absent while loading or
  // for a picker without counts.
  counts?: Record<string, number>
  untaggedCount?: number
  // Which parts of the filter the picker offers. A picker without
  // exclusion toggles a tag on and off; one with it goes on, excluded,
  // off.
  allowExclude?: boolean
  allowMode?: boolean
  allowUntagged?: boolean
  // Called when the menu opens or closes, so counts can load on demand.
  onOpenChange?: (open: boolean) => void
  label: string
  testIdPrefix: string
}

type TagState = "include" | "exclude" | "off"

function stateOf(value: TagFilterValue, slug: string): TagState {
  if (value.include.includes(slug)) return "include"
  if (value.exclude.includes(slug)) return "exclude"
  return "off"
}

// A searchable tag picker in a toolbar: each tag with its color and, when
// given, its count; optional all/any switch, exclusion (click a selected
// tag again) and a "No tags" option. The parts it does not offer are left
// as they are in value.
export function TagFilterMenu({
  tags,
  value,
  onChange,
  counts,
  untaggedCount,
  allowExclude = false,
  allowMode = false,
  allowUntagged = false,
  onOpenChange,
  label,
  testIdPrefix,
}: TagFilterMenuProps) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  const active = value.include.length + value.exclude.length + (value.untagged ? 1 : 0)

  // Selected tags first, then by label. A selected slug with no tag row
  // (a hand-edited URL) is listed too, so it can be cleared.
  const known = new Set(tags.map((tag) => tag.slug))
  const orphans: ResolvedTag[] = [...value.include, ...value.exclude]
    .filter((slug) => !known.has(slug))
    .map((slug) => ({ slug, label: slug, color: "muted" }))
  const rank = (tag: ResolvedTag) => (stateOf(value, tag.slug) === "off" ? 1 : 0)
  const options = [...orphans, ...tags].sort(
    (a, b) => rank(a) - rank(b) || a.label.localeCompare(b.label)
  )

  function cycle(slug: string) {
    const state = stateOf(value, slug)
    const include = value.include.filter((s) => s !== slug)
    const exclude = value.exclude.filter((s) => s !== slug)
    if (state === "off") include.push(slug)
    else if (state === "include" && allowExclude) exclude.push(slug)
    onChange({ ...value, include, exclude, untagged: false })
  }

  function toggleUntagged() {
    onChange({ ...value, include: [], exclude: [], untagged: !value.untagged })
  }

  function handleOpenChange(next: boolean) {
    setOpen(next)
    onOpenChange?.(next)
  }

  return (
    <Popover open={open} onOpenChange={handleOpenChange}>
      <PopoverTrigger asChild>
        <Button
          variant={active > 0 ? "default" : "outline"}
          size="sm"
          className="gap-1.5"
          data-testid={testIdPrefix}
        >
          <TagIcon className="size-3.5" aria-hidden="true" />
          {label}
          {active > 0 ? (
            <Badge variant="secondary" className="ml-0.5 h-4 px-1 text-xs">
              {active}
            </Badge>
          ) : null}
          <ChevronDown className="size-3.5" aria-hidden="true" />
        </Button>
      </PopoverTrigger>
      <PopoverContent
        align="start"
        className="w-72 p-0"
        onWheel={(e) => e.stopPropagation()}
        data-testid={`${testIdPrefix}-menu`}
      >
        <Command>
          <CommandInput placeholder={t("tags:filter.search")} />
          {allowMode ? (
            <div className="flex items-center gap-1 border-b px-2 py-1.5 text-xs text-muted-foreground">
              <span className="mr-auto">{t("tags:filter.match")}</span>
              {(["all", "any"] as const).map((mode) => (
                <Button
                  key={mode}
                  type="button"
                  variant={value.mode === mode ? "secondary" : "ghost"}
                  size="sm"
                  className="h-6 px-2 text-xs"
                  aria-pressed={value.mode === mode}
                  onClick={() => onChange({ ...value, mode })}
                  data-testid={`${testIdPrefix}-mode-${mode}`}
                >
                  {t(mode === "all" ? "tags:filter.matchAll" : "tags:filter.matchAny")}
                </Button>
              ))}
            </div>
          ) : null}
          <CommandList className="max-h-72 overscroll-contain">
            <CommandEmpty>{t("tags:filter.empty")}</CommandEmpty>
            {allowUntagged ? (
              <>
                <CommandGroup>
                  <CommandItem
                    value={`__untagged__ ${t("tags:filter.noTags")}`}
                    onSelect={toggleUntagged}
                    data-testid={`${testIdPrefix}-untagged`}
                    data-state={value.untagged ? "include" : "off"}
                    className="gap-2"
                  >
                    <StateMark state={value.untagged ? "include" : "off"} />
                    <span className="flex-1 truncate italic">{t("tags:filter.noTags")}</span>
                    <Count value={untaggedCount} />
                  </CommandItem>
                </CommandGroup>
                <CommandSeparator />
              </>
            ) : null}
            <CommandGroup>
              {options.map((tag) => {
                const state = stateOf(value, tag.slug)
                return (
                  <CommandItem
                    key={tag.slug}
                    value={`${tag.label} ${tag.slug}`}
                    onSelect={() => cycle(tag.slug)}
                    data-testid={`${testIdPrefix}-option-${tag.slug}`}
                    data-state={state}
                    className="gap-2"
                  >
                    <StateMark state={state} />
                    <span
                      aria-hidden="true"
                      className={cn("size-2 shrink-0 rounded-full", TAG_DOT_TONE[tag.color])}
                    />
                    <span className={cn("flex-1 truncate", state === "exclude" && "line-through")}>
                      {tag.label}
                    </span>
                    {state === "exclude" ? (
                      <span className="sr-only">{t("tags:filter.excluded")}</span>
                    ) : null}
                    <Count value={counts ? (counts[tag.slug] ?? 0) : undefined} slug={tag.slug} />
                  </CommandItem>
                )
              })}
            </CommandGroup>
          </CommandList>
          {allowExclude || active > 0 ? (
            <div className="flex items-center gap-2 border-t px-2 py-1.5 text-[11px] text-muted-foreground">
              {allowExclude ? <span className="flex-1">{t("tags:filter.excludeHint")}</span> : null}
              {active > 0 ? (
                <Button
                  type="button"
                  variant="ghost"
                  size="sm"
                  className="ml-auto h-6 px-2 text-xs"
                  onClick={() => onChange({ ...value, include: [], exclude: [], untagged: false })}
                  data-testid={`${testIdPrefix}-clear`}
                >
                  {t("tags:filter.clear")}
                </Button>
              ) : null}
            </div>
          ) : null}
        </Command>
      </PopoverContent>
    </Popover>
  )
}

function StateMark({ state }: { state: TagState }) {
  return (
    <span
      aria-hidden="true"
      className={cn(
        "flex size-4 shrink-0 items-center justify-center rounded-sm border",
        state === "include" && "border-primary bg-primary text-primary-foreground",
        state === "exclude" && "border-destructive bg-destructive text-white"
      )}
    >
      {state === "include" ? <Check className="size-3" /> : null}
      {state === "exclude" ? <Ban className="size-3" /> : null}
    </span>
  )
}

function Count({ value, slug }: { value: number | undefined; slug?: string }) {
  if (value === undefined) return null
  return (
    <span
      className="ml-auto text-xs tabular-nums text-muted-foreground"
      data-testid={slug ? `tag-filter-count-${slug}` : undefined}
    >
      {value}
    </span>
  )
}

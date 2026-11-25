import { useState } from "react"
import { useTranslation } from "react-i18next"
import { Plus } from "lucide-react"

import { TagBadge } from "@/components/tags/TagBadge"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import type { TagColor, TagKind } from "@/features/tags/api"
import { useTagCatalog } from "@/features/tags/hooks"
import { cn } from "@/lib/utils"

export interface TagPaletteProps {
  kind: TagKind
  /** Selected tag slugs. */
  values: string[]
  onChange: (next: string[]) => void
  /** id of the new-tag input, for a label's `htmlFor`. */
  inputId: string
  /** False hides the new-tag input, for picking from existing tags only. */
  allowCreate?: boolean
  testId?: string
}

// The mock's tag palette (design-mocks ItemDetail.tsx, ItemEditForm): every
// tag of the group as a toggle button, the selected ones as removable pills
// under it. One click adds or removes a tag.
//
// The input under the pills adds a tag the group does not have yet; the
// server creates it on save. Text that matches an existing tag's label or
// slug selects that tag instead.
export function TagPalette({
  kind,
  values,
  onChange,
  inputId,
  allowCreate = true,
  testId,
}: TagPaletteProps) {
  const { t } = useTranslation()
  const catalog = useTagCatalog(kind).data ?? []
  const [draft, setDraft] = useState("")

  const bySlug = new Map(catalog.map((tag) => [tag.slug ?? "", tag]))
  const labelOf = (slug: string) => bySlug.get(slug)?.label || slug
  const colorOf = (slug: string): TagColor => bySlug.get(slug)?.color ?? "muted"

  function toggle(slug: string) {
    onChange(values.includes(slug) ? values.filter((v) => v !== slug) : [...values, slug])
  }

  function addDraft() {
    const text = draft.trim()
    if (!text) return
    const needle = text.toLowerCase()
    const existing = catalog.find(
      (tag) => tag.slug?.toLowerCase() === needle || tag.label?.toLowerCase() === needle
    )
    const value = existing?.slug || text
    if (!values.includes(value)) onChange([...values, value])
    setDraft("")
  }

  return (
    <div className="flex flex-col gap-1.5" data-testid={testId}>
      {catalog.length > 0 ? (
        <div className="flex flex-wrap gap-1.5">
          {catalog.map((tag) => {
            const slug = tag.slug ?? ""
            const active = values.includes(slug)
            return (
              <button
                key={slug}
                type="button"
                onClick={() => toggle(slug)}
                aria-pressed={active}
                className={cn(
                  "rounded-full border px-2.5 py-0.5 text-xs font-medium transition-colors",
                  active
                    ? "bg-primary text-primary-foreground border-primary"
                    : "border-border bg-background text-muted-foreground hover:border-primary/50"
                )}
                data-testid={testId ? `${testId}-option-${slug}` : undefined}
              >
                #{tag.label || slug}
              </button>
            )
          })}
        </div>
      ) : null}
      {values.length > 0 ? (
        <div className="mt-1 flex flex-wrap gap-1">
          {values.map((slug) => (
            <TagBadge
              key={slug}
              label={labelOf(slug)}
              color={colorOf(slug)}
              size="xs"
              onRemove={() => toggle(slug)}
              removeLabel={t("tags:palette.remove", { label: labelOf(slug) })}
              testId={testId ? `${testId}-selected-${slug}` : undefined}
            />
          ))}
        </div>
      ) : null}
      {allowCreate ? (
        <div className="mt-1 flex gap-1.5">
          <Input
            id={inputId}
            value={draft}
            onChange={(e) => setDraft(e.target.value)}
            onKeyDown={(e) => {
              if (e.key !== "Enter") return
              // Enter also ends an IME composition; adding here would turn a
              // half-composed word into a tag.
              if (e.nativeEvent.isComposing || e.nativeEvent.keyCode === 229) return
              // Enter adds the tag; it must not also submit the form around it.
              e.preventDefault()
              addDraft()
            }}
            placeholder={t("tags:palette.newTagPlaceholder")}
            aria-label={t("tags:palette.newTagLabel")}
            className="h-8 text-sm"
            data-testid={testId ? `${testId}-input` : undefined}
          />
          <Button
            type="button"
            variant="outline"
            size="sm"
            className="gap-1"
            onClick={addDraft}
            disabled={!draft.trim()}
            data-testid={testId ? `${testId}-add` : undefined}
          >
            <Plus className="size-3.5" aria-hidden="true" />
            {t("tags:palette.add")}
          </Button>
        </div>
      ) : null}
    </div>
  )
}

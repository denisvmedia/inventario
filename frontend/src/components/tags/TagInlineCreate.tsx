import { Check, Plus, X } from "lucide-react"
import { useEffect, useId, useRef, useState } from "react"
import { useTranslation } from "react-i18next"

import { TagColorPicker } from "@/components/tags/TagColorPicker"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { cn } from "@/lib/utils"

import type { TagColor } from "@/features/tags/api"
import { normaliseSlug } from "@/features/tags/schemas"

// Inline fast-path tag creator that sits at the bottom of the tag list,
// per `design-mocks/src/views/TagsView.tsx`. Captures label-only — the
// slug is auto-derived from the label via `normaliseSlug` (same rule
// the dialog uses while the slug field is unedited). The full
// label-AND-slug experience stays in `<TagFormDialog>`, opened by the
// page header's "Add tag" button or any row's edit affordance.
export interface TagInlineCreateProps {
  existingSlugs: ReadonlySet<string>
  // Resolves on success; rejecting bubbles back so this component can
  // leave the typed value in place for retry while the caller surfaces
  // the failure (toast). The caller is responsible for cache
  // invalidation; we don't read the freshly-created row from the
  // returned value.
  onCreate: (values: { label: string; slug: string; color: TagColor }) => Promise<void>
  isPending?: boolean
  className?: string
  testId?: string
}

const DEFAULT_COLOR: TagColor = "amber"

export function TagInlineCreate({
  existingSlugs,
  onCreate,
  isPending = false,
  className,
  testId,
}: TagInlineCreateProps) {
  const { t } = useTranslation(["tags"])
  const [open, setOpen] = useState(false)
  const [label, setLabel] = useState("")
  const [color, setColor] = useState<TagColor>(DEFAULT_COLOR)
  const [error, setError] = useState<string | null>(null)
  const inputRef = useRef<HTMLInputElement>(null)
  // Stable ids so the visually-hidden label, input, and error message
  // wire up for screen readers — axe flags the input as "unlabeled
  // form control" otherwise.
  const inputId = useId()
  const errorId = useId()

  // Focus the input when the row expands. Without this the user has to
  // click the input after toggling the dashed button — the mock auto-
  // focuses immediately so a fast typist can chain create → enter →
  // type-next-name without a mouse round-trip.
  useEffect(() => {
    if (open) inputRef.current?.focus()
  }, [open])

  function reset() {
    setLabel("")
    setColor(DEFAULT_COLOR)
    setError(null)
  }

  function close() {
    setOpen(false)
    reset()
  }

  async function submit() {
    const trimmed = label.trim()
    if (!trimmed) {
      setError("tags:validation.labelRequired")
      return
    }
    const slug = normaliseSlug(trimmed)
    if (!slug) {
      setError("tags:validation.slugInvalid")
      return
    }
    if (existingSlugs.has(slug)) {
      setError("tags:validation.duplicateSlug")
      return
    }
    try {
      await onCreate({ label: trimmed, slug, color })
      reset()
      // Stay expanded so the user can punch in another tag immediately.
      // The mock collapses; we leave open because the BE round-trip is
      // long enough that re-opening a fresh card feels stuttery.
      inputRef.current?.focus()
    } catch {
      // Caller is responsible for toast on failure — we keep the typed
      // value so the user can retry without re-typing.
    }
  }

  if (!open) {
    return (
      <Button
        type="button"
        variant="outline"
        size="sm"
        className={cn("gap-1.5 w-full justify-start border-dashed", className)}
        onClick={() => setOpen(true)}
        data-testid={testId ?? "tags-inline-create-toggle"}
      >
        <Plus aria-hidden="true" className="size-3.5" />
        {t("tags:inlineCreate.toggle")}
      </Button>
    )
  }

  return (
    <div
      className={cn(
        "flex items-start gap-2 rounded-lg border border-ring bg-card px-3 py-2",
        className
      )}
      data-testid={testId ?? "tags-inline-create"}
    >
      <div className="flex flex-col gap-2 flex-1 min-w-0">
        <Label htmlFor={inputId} className="sr-only">
          {t("tags:inlineCreate.label")}
        </Label>
        <Input
          id={inputId}
          ref={inputRef}
          value={label}
          onChange={(e) => {
            setLabel(e.target.value)
            if (error) setError(null)
          }}
          onKeyDown={(e) => {
            if (e.key === "Enter") {
              e.preventDefault()
              void submit()
            }
            if (e.key === "Escape") {
              e.preventDefault()
              close()
            }
          }}
          placeholder={t("tags:inlineCreate.placeholder")}
          className="h-7 text-sm"
          disabled={isPending}
          aria-invalid={!!error}
          aria-describedby={error ? errorId : undefined}
          data-testid="tags-inline-create-label"
        />
        <TagColorPicker
          value={color}
          onChange={setColor}
          disabled={isPending}
          testId="tags-inline-create-color"
        />
        {error ? (
          <p
            id={errorId}
            className="text-xs text-destructive"
            data-testid="tags-inline-create-error"
          >
            {t(error)}
          </p>
        ) : null}
      </div>
      <div className="flex gap-1 shrink-0">
        <Button
          type="button"
          size="icon"
          variant="ghost"
          className="size-7"
          onClick={() => void submit()}
          disabled={isPending || !label.trim()}
          aria-label={t("tags:inlineCreate.save")}
          data-testid="tags-inline-create-save"
        >
          <Check aria-hidden="true" className="size-3.5" />
        </Button>
        <Button
          type="button"
          size="icon"
          variant="ghost"
          className="size-7"
          onClick={close}
          disabled={isPending}
          aria-label={t("tags:inlineCreate.cancel")}
          data-testid="tags-inline-create-cancel"
        >
          <X aria-hidden="true" className="size-3.5" />
        </Button>
      </div>
    </div>
  )
}

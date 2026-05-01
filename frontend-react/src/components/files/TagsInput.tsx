import { Plus, X } from "lucide-react"
import { useState } from "react"

import { Badge } from "@/components/ui/badge"
import { Label } from "@/components/ui/label"

// Lightweight tag chip input — type, hit Enter / comma to commit;
// Backspace on empty draft pops the last chip. Used by the files
// metadata edit form. The same UX pattern as commodities ChipInput,
// kept independent here so we don't reach into a private dialog
// helper from a different feature.
export interface TagsInputProps {
  label?: string
  values: string[]
  onChange: (next: string[]) => void
  placeholder?: string
  testId?: string
}

export function TagsInput({ label, values, onChange, placeholder, testId }: TagsInputProps) {
  const [draft, setDraft] = useState("")

  function commit() {
    const trimmed = draft.trim()
    if (!trimmed) return
    if (values.includes(trimmed)) {
      setDraft("")
      return
    }
    onChange([...values, trimmed])
    setDraft("")
  }

  return (
    <div className="flex flex-col gap-1.5" data-testid={testId}>
      {label ? <Label>{label}</Label> : null}
      <div className="flex flex-wrap items-center gap-1.5 rounded-md border border-input px-2 py-1.5">
        {values.map((v) => (
          <Badge
            key={v}
            variant="secondary"
            className="h-5 gap-1 px-1.5 text-xs"
            data-testid={testId ? `${testId}-chip` : undefined}
          >
            {v}
            <button
              type="button"
              className="ml-0.5 inline-flex items-center"
              aria-label={`remove ${v}`}
              onClick={() => onChange(values.filter((x) => x !== v))}
            >
              <X className="size-3" aria-hidden="true" />
            </button>
          </Badge>
        ))}
        <input
          value={draft}
          placeholder={placeholder}
          onChange={(e) => setDraft(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter" || e.key === ",") {
              e.preventDefault()
              commit()
            } else if (e.key === "Backspace" && draft === "" && values.length > 0) {
              onChange(values.slice(0, -1))
            }
          }}
          onBlur={commit}
          className="min-w-24 flex-1 bg-transparent text-sm outline-none"
          data-testid={testId ? `${testId}-input` : undefined}
        />
        {draft.trim() ? (
          <button
            type="button"
            className="text-muted-foreground hover:text-foreground"
            aria-label="add"
            onClick={commit}
          >
            <Plus className="size-3.5" aria-hidden="true" />
          </button>
        ) : null}
      </div>
    </div>
  )
}

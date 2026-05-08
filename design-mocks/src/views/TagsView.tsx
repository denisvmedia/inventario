import { useState, useRef } from "react"
import { Tag, Plus, Pencil, Trash2, Hash, Package, Search, X, Check } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Separator } from "@/components/ui/separator"
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog"
import { cn } from "@/lib/utils"
import { MOCK_ITEMS } from "@/data/mock"

// ─── Tag color palette ────────────────────────────────────────

const TAG_COLORS = [
  { id: "amber",   dot: "bg-chart-1",        pill: "bg-chart-1/15 text-chart-1 border-chart-1/30" },
  { id: "green",   dot: "bg-status-active",   pill: "bg-status-active/15 text-status-active border-status-active/30" },
  { id: "blue",    dot: "bg-chart-3",         pill: "bg-chart-3/15 text-chart-3 border-chart-3/30" },
  { id: "orange",  dot: "bg-chart-4",         pill: "bg-chart-4/15 text-chart-4 border-chart-4/30" },
  { id: "red",     dot: "bg-chart-5",         pill: "bg-chart-5/15 text-chart-5 border-chart-5/30" },
  { id: "muted",   dot: "bg-muted-foreground",pill: "bg-muted text-muted-foreground border-border" },
]

interface TagDef {
  id: string
  label: string
  colorId: string
}

// Seed from items
const seedTags = (): TagDef[] => {
  const seen = new Set<string>()
  const result: TagDef[] = []
  const colors = TAG_COLORS.map((c) => c.id)
  MOCK_ITEMS.forEach((item) => {
    item.tags.forEach((t) => {
      if (!seen.has(t)) {
        seen.add(t)
        result.push({ id: t, label: t, colorId: colors[result.length % colors.length] })
      }
    })
  })
  return result
}

const INITIAL_TAGS = seedTags()

function itemCountForTag(tag: string) {
  return MOCK_ITEMS.filter((item) => item.tags.includes(tag)).length
}

// ─── Tag pill (display) ───────────────────────────────────────

function TagPill({ tag, onRemove }: { tag: TagDef; onRemove?: () => void }) {
  const color = TAG_COLORS.find((c) => c.id === tag.colorId) ?? TAG_COLORS[5]
  return (
    <span
      className={cn(
        "inline-flex items-center gap-1 rounded-full border px-2.5 py-0.5 text-xs font-medium select-none",
        color.pill
      )}
    >
      <Hash className="size-2.5 shrink-0" />
      {tag.label}
      {onRemove && (
        <button
          type="button"
          onClick={onRemove}
          className="ml-0.5 rounded-full opacity-60 hover:opacity-100 transition-opacity"
          aria-label="Remove tag"
        >
          <X className="size-2.5" />
        </button>
      )}
    </span>
  )
}

// ─── Color picker dot row ─────────────────────────────────────

function ColorPicker({ value, onChange }: { value: string; onChange: (id: string) => void }) {
  return (
    <div className="flex gap-1.5 flex-wrap">
      {TAG_COLORS.map((c) => (
        <button
          key={c.id}
          type="button"
          onClick={() => onChange(c.id)}
          className={cn(
            "size-5 rounded-full transition-all ring-offset-background",
            c.dot,
            value === c.id ? "ring-2 ring-ring ring-offset-2" : "opacity-60 hover:opacity-100"
          )}
          aria-label={c.id}
        />
      ))}
    </div>
  )
}

// ─── Inline edit row ──────────────────────────────────────────

function EditRow({ tag, onSave, onCancel }: { tag: TagDef; onSave: (t: TagDef) => void; onCancel: () => void }) {
  const [label, setLabel] = useState(tag.label)
  const [colorId, setColorId] = useState(tag.colorId)
  const inputRef = useRef<HTMLInputElement>(null)

  function submit() {
    const trimmed = label.trim().toLowerCase().replace(/\s+/g, "-")
    if (!trimmed) return
    onSave({ ...tag, label: trimmed, colorId })
  }

  return (
    <div className="flex items-center gap-2 rounded-lg border border-ring bg-card px-3 py-2">
      <div className="flex flex-col gap-2 flex-1 min-w-0">
        <Input
          ref={inputRef}
          value={label}
          onChange={(e) => setLabel(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter") submit()
            if (e.key === "Escape") onCancel()
          }}
          className="h-7 text-sm"
          autoFocus
        />
        <ColorPicker value={colorId} onChange={setColorId} />
      </div>
      <div className="flex gap-1 shrink-0">
        <Button size="icon" variant="ghost" className="size-7" onClick={submit} disabled={!label.trim()}>
          <Check className="size-3.5" />
        </Button>
        <Button size="icon" variant="ghost" className="size-7" onClick={onCancel}>
          <X className="size-3.5" />
        </Button>
      </div>
    </div>
  )
}

// ─── Create row ───────────────────────────────────────────────

function CreateRow({ existingLabels, onCreate }: { existingLabels: Set<string>; onCreate: (t: Omit<TagDef, "id">) => void }) {
  const [open, setOpen] = useState(false)
  const [label, setLabel] = useState("")
  const [colorId, setColorId] = useState(TAG_COLORS[0].id)

  function submit() {
    const trimmed = label.trim().toLowerCase().replace(/\s+/g, "-")
    if (!trimmed || existingLabels.has(trimmed)) return
    onCreate({ label: trimmed, colorId })
    setLabel("")
    setColorId(TAG_COLORS[0].id)
    setOpen(false)
  }

  if (!open) {
    return (
      <Button variant="outline" size="sm" className="gap-1.5 w-full justify-start border-dashed" onClick={() => setOpen(true)}>
        <Plus className="size-3.5" />
        New tag
      </Button>
    )
  }

  return (
    <div className="flex items-center gap-2 rounded-lg border border-ring bg-card px-3 py-2">
      <div className="flex flex-col gap-2 flex-1 min-w-0">
        <Input
          value={label}
          onChange={(e) => setLabel(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter") submit()
            if (e.key === "Escape") setOpen(false)
          }}
          placeholder="tag-name"
          className="h-7 text-sm"
          autoFocus
        />
        <ColorPicker value={colorId} onChange={setColorId} />
      </div>
      <div className="flex gap-1 shrink-0">
        <Button size="icon" variant="ghost" className="size-7" onClick={submit} disabled={!label.trim()}>
          <Check className="size-3.5" />
        </Button>
        <Button size="icon" variant="ghost" className="size-7" onClick={() => setOpen(false)}>
          <X className="size-3.5" />
        </Button>
      </div>
    </div>
  )
}

// ─── Main view ────────────────────────────────────────────────

export function TagsView() {
  const [tags, setTags] = useState<TagDef[]>(INITIAL_TAGS)
  const [search, setSearch] = useState("")
  const [editingId, setEditingId] = useState<string | null>(null)
  const [deleteId, setDeleteId] = useState<string | null>(null)

  const filtered = tags.filter((t) =>
    !search.trim() || t.label.toLowerCase().includes(search.toLowerCase())
  )

  const existingLabels = new Set(tags.map((t) => t.label))

  function addTag(partial: Omit<TagDef, "id">) {
    const id = partial.label.replace(/\s+/g, "-") + "-" + Date.now()
    setTags((prev) => [...prev, { ...partial, id }])
  }

  function saveTag(updated: TagDef) {
    setTags((prev) => prev.map((t) => (t.id === updated.id ? updated : t)))
    setEditingId(null)
  }

  function confirmDelete() {
    if (!deleteId) return
    setTags((prev) => prev.filter((t) => t.id !== deleteId))
    setDeleteId(null)
  }

  const totalTagged = MOCK_ITEMS.filter((item) => item.tags.length > 0).length

  return (
    <div className="flex flex-col gap-6 p-6 max-w-2xl mx-auto w-full">

      {/* Header */}
      <div className="flex items-start justify-between gap-4">
        <div>
          <h1 className="scroll-m-20 text-3xl font-semibold tracking-tight">Tags</h1>
          <p className="mt-1 text-muted-foreground">Organise your inventory with custom labels.</p>
        </div>
      </div>

      {/* Stats row */}
      <div className="grid grid-cols-3 gap-3">
        {[
          { label: "Total tags", value: tags.length, icon: Tag },
          { label: "Tagged items", value: totalTagged, icon: Package },
          { label: "Untagged items", value: MOCK_ITEMS.length - totalTagged, icon: Hash },
        ].map(({ label, value, icon: Icon }) => (
          <div key={label} className="rounded-xl border border-border bg-card px-4 py-3 flex items-center gap-3">
            <div className="flex size-8 items-center justify-center rounded-lg bg-muted shrink-0">
              <Icon className="size-4 text-muted-foreground" />
            </div>
            <div>
              <p className="text-xs text-muted-foreground">{label}</p>
              <p className="text-lg font-semibold leading-tight">{value}</p>
            </div>
          </div>
        ))}
      </div>

      {/* Search */}
      <div className="relative">
        <Search className="absolute left-2.5 top-1/2 size-3.5 -translate-y-1/2 text-muted-foreground" />
        <Input
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          placeholder="Filter tags…"
          className="pl-8"
        />
        {search && (
          <button
            type="button"
            onClick={() => setSearch("")}
            className="absolute right-2.5 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground transition-colors"
          >
            <X className="size-3.5" />
          </button>
        )}
      </div>

      {/* Tag list */}
      <div className="rounded-xl border border-border bg-card overflow-hidden">
        <div className="px-4 py-3 border-b border-border flex items-center justify-between">
          <p className="text-xs font-semibold uppercase tracking-widest text-muted-foreground">
            {search ? `${filtered.length} result${filtered.length !== 1 ? "s" : ""}` : `${tags.length} tag${tags.length !== 1 ? "s" : ""}`}
          </p>
        </div>

        {filtered.length === 0 ? (
          <div className="flex flex-col items-center justify-center gap-3 py-16">
            <Tag className="size-8 text-muted-foreground/30" />
            <p className="text-sm text-muted-foreground">
              {search ? "No tags match your search." : "No tags yet. Create your first tag below."}
            </p>
          </div>
        ) : (
          <ul className="divide-y divide-border">
            {filtered.map((tag) => {
              const count = itemCountForTag(tag.id)
              const color = TAG_COLORS.find((c) => c.id === tag.colorId) ?? TAG_COLORS[5]
              const isEditing = editingId === tag.id

              return (
                <li key={tag.id} className="px-4 py-3">
                  {isEditing ? (
                    <EditRow
                      tag={tag}
                      onSave={saveTag}
                      onCancel={() => setEditingId(null)}
                    />
                  ) : (
                    <div className="flex items-center gap-3 group">
                      <div className={cn("size-2.5 rounded-full shrink-0", color.dot)} />
                      <div className="flex-1 min-w-0">
                        <div className="flex items-center gap-2 min-w-0">
                          <span className="text-sm font-medium truncate"># {tag.label}</span>
                        </div>
                        <p className="text-xs text-muted-foreground mt-0.5">
                          {count === 0
                            ? "No items"
                            : `${count} item${count !== 1 ? "s" : ""}`}
                        </p>
                      </div>

                      {/* Preview pills of items using this tag */}
                      <div className="hidden sm:flex items-center gap-1 flex-wrap max-w-48">
                        {MOCK_ITEMS.filter((i) => i.tags.includes(tag.id))
                          .slice(0, 2)
                          .map((item) => (
                            <span
                              key={item.id}
                              className="inline-flex items-center gap-1 rounded-md bg-muted px-1.5 py-0.5 text-[10px] text-muted-foreground"
                            >
                              <Package className="size-2.5 shrink-0" />
                              <span className="truncate max-w-20">{item.shortName ?? item.name}</span>
                            </span>
                          ))}
                        {count > 2 && (
                          <span className="text-[10px] text-muted-foreground">+{count - 2}</span>
                        )}
                      </div>

                      <div className="flex gap-0.5 opacity-0 group-hover:opacity-100 transition-opacity shrink-0">
                        <Button
                          size="icon"
                          variant="ghost"
                          className="size-7"
                          onClick={() => setEditingId(tag.id)}
                        >
                          <Pencil className="size-3.5" />
                        </Button>
                        <Button
                          size="icon"
                          variant="ghost"
                          className="size-7 text-destructive hover:bg-destructive/10 hover:text-destructive"
                          onClick={() => setDeleteId(tag.id)}
                        >
                          <Trash2 className="size-3.5" />
                        </Button>
                      </div>
                    </div>
                  )}
                </li>
              )
            })}
          </ul>
        )}

        <Separator />
        <div className="px-4 py-3">
          <CreateRow existingLabels={existingLabels} onCreate={addTag} />
        </div>
      </div>

      {/* All tags preview */}
      {tags.length > 0 && (
        <div className="rounded-xl border border-border bg-card px-4 py-4 space-y-3">
          <p className="text-xs font-semibold uppercase tracking-widest text-muted-foreground">Preview</p>
          <div className="flex flex-wrap gap-1.5">
            {tags.map((tag) => (
              <TagPill key={tag.id} tag={tag} />
            ))}
          </div>
        </div>
      )}

      {/* Delete confirmation */}
      <AlertDialog open={!!deleteId} onOpenChange={(o) => !o && setDeleteId(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete tag</AlertDialogTitle>
            <AlertDialogDescription>
              {(() => {
                const tag = tags.find((t) => t.id === deleteId)
                const count = tag ? itemCountForTag(tag.id) : 0
                return count > 0
                  ? `This will remove "#${tag?.label}" from ${count} item${count !== 1 ? "s" : ""}. This action cannot be undone.`
                  : `Delete "#${tag?.label}"? This action cannot be undone.`
              })()}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction
              onClick={confirmDelete}
              className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
            >
              Delete
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}

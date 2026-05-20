import { useState } from "react"
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
  SheetDescription,
} from "@/components/ui/sheet"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Separator } from "@/components/ui/separator"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { Input } from "@/components/ui/input"
import { Textarea } from "@/components/ui/textarea"
import { Label } from "@/components/ui/label"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from "@/components/ui/dialog"
import { ShieldCheck, ShieldAlert, ShieldOff, Shield, ExternalLink, MapPin, Calendar, Hash, Tag, Pencil, Trash2, DollarSign, FileText, Image as ImageIcon, File, Upload, ArrowLeft, Paperclip, ChartBar as FileBarChart2, Package, CircleDot, TriangleAlert as AlertTriangle, Link as LinkIcon, Layers } from "lucide-react"
import { WarrantyBadge } from "@/components/WarrantyBadge"
import { FilePreviewDialog } from "@/components/FilePreviewDialog"
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
import {
  MOCK_ITEMS,
  MOCK_FILES,
  MOCK_TAGS,
  resolveTags,
  CATEGORIES,
  MOCK_LOCATIONS,
  MOCK_AREAS,
  CURRENCIES,
  warrantyStatus,
  areaLabel,
  type InventoryItem,
  type AttachedFile,
  type ItemCategory,
  type CommodityStatus,
  type FileCategory,
  WARRANTY_STATUS_CONFIG,
  COMMODITY_STATUS_CONFIG,
} from "@/data/mock"
import { TagPill } from "@/components/TagPill"
import { cn } from "@/lib/utils"

interface ItemDetailProps {
  itemId: string | null
  onClose: () => void
  onOpenInsuranceReport?: (itemId: string) => void
}

function formatCurrency(n: number | null, currency = "USD") {
  if (n === null) return "—"
  return new Intl.NumberFormat("en-US", { style: "currency", currency, maximumFractionDigits: 0 }).format(n)
}

function formatDate(d: string | null) {
  if (!d) return "—"
  return new Date(d).toLocaleDateString("en-US", { year: "numeric", month: "long", day: "numeric" })
}

function daysUntil(dateStr: string | null) {
  if (!dateStr) return null
  return Math.ceil((new Date(dateStr).getTime() - Date.now()) / (1000 * 60 * 60 * 24))
}

interface DetailRowProps {
  icon: React.ElementType
  label: string
  value: React.ReactNode
}

function DetailRow({ icon: Icon, label, value }: DetailRowProps) {
  return (
    <div className="flex items-start gap-3 py-2.5">
      <Icon className="mt-0.5 size-4 shrink-0 text-muted-foreground" />
      <div className="flex-1 min-w-0">
        <p className="text-xs text-muted-foreground mb-0.5">{label}</p>
        <div className="text-sm font-medium">{value}</div>
      </div>
    </div>
  )
}

const CATEGORY_ICONS: Record<string, string> = {
  appliance: "🏠", electronics: "💻", tool: "🔧",
  furniture: "🪑", vehicle: "🚗", other: "📦",
}

function fileIcon(mimeType: string) {
  if (mimeType === "application/pdf") return <FileText className="size-4 text-status-expired" />
  if (mimeType.startsWith("image/")) return <ImageIcon className="size-4 text-status-active" />
  return <File className="size-4 text-muted-foreground" />
}

// ─── Status transition dialog ─────────────────────────────────────────────────

interface StatusTransitionDialogProps {
  open: boolean
  targetStatus: CommodityStatus | null
  item: InventoryItem
  onClose: () => void
  onConfirm: (status: CommodityStatus, note: string, date: string, salePrice: string) => void
}

function StatusTransitionDialog({ open, targetStatus, item, onClose, onConfirm }: StatusTransitionDialogProps) {
  const [note, setNote] = useState("")
  const [date, setDate] = useState(new Date().toISOString().split("T")[0])
  const [salePrice, setSalePrice] = useState("")

  if (!targetStatus) return null
  const cfg = COMMODITY_STATUS_CONFIG[targetStatus]

  return (
    <Dialog open={open} onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="sm:max-w-sm">
        <DialogHeader>
          <DialogTitle>Mark as {cfg.label}</DialogTitle>
        </DialogHeader>
        <div className="space-y-3 py-1">
          <p className="text-sm text-muted-foreground">{cfg.description}</p>

          <div className="space-y-1.5">
            <Label htmlFor="status-date">Date</Label>
            <Input id="status-date" type="date" value={date} onChange={(e) => setDate(e.target.value)} />
          </div>

          {targetStatus === "sold" && (
            <div className="space-y-1.5">
              <Label htmlFor="sale-price">Sale Price (optional)</Label>
              <div className="relative">
                <span className="absolute left-3 top-1/2 -translate-y-1/2 text-sm text-muted-foreground">
                  {CURRENCIES.find((c) => c.code === item.purchaseCurrency)?.symbol ?? "$"}
                </span>
                <Input
                  id="sale-price"
                  type="number"
                  min="0"
                  className="pl-6"
                  value={salePrice}
                  onChange={(e) => setSalePrice(e.target.value)}
                />
              </div>
            </div>
          )}

          <div className="space-y-1.5">
            <Label htmlFor="status-note">Notes (optional)</Label>
            <Textarea
              id="status-note"
              rows={2}
              className="resize-none"
              placeholder={targetStatus === "sold" ? "Sold to…" : targetStatus === "lost" ? "Last seen at…" : "Details…"}
              value={note}
              onChange={(e) => setNote(e.target.value)}
            />
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={onClose}>Cancel</Button>
          <Button onClick={() => onConfirm(targetStatus, note, date, salePrice)}>
            Confirm
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

// ─── Files tab ───────────────────────────────────────────────────────────────

type FilesTabCategory = "all" | FileCategory

const FILE_TAB_SECTIONS: { id: FilesTabCategory; label: string; icon: React.ElementType; emptyHint: string }[] = [
  { id: "all",      label: "All",       icon: Paperclip,    emptyHint: "No files attached yet." },
  { id: "image",    label: "Photos",    icon: ImageIcon,    emptyHint: "No photos yet. Add item photos to use them on cards." },
  { id: "invoice",  label: "Invoices",  icon: DollarSign,   emptyHint: "No invoices yet. Add purchase receipts to surface them in reports." },
  { id: "document", label: "Documents", icon: FileText,     emptyHint: "No documents yet. Manuals, warranties, certificates." },
]

function ItemFilesTab({ item }: { item: InventoryItem }) {
  const [activeCategory, setActiveCategory] = useState<FilesTabCategory>("all")
  const [dragging, setDragging] = useState(false)
  const [previewFile, setPreviewFile] = useState<AttachedFile | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<AttachedFile | null>(null)
  const [deletedIds, setDeletedIds] = useState<Set<string>>(new Set())

  const allAttached = MOCK_FILES.filter((f) => f.attachedTo.type === "commodity" && f.attachedTo.id === item.id)
  const liveFiles = allAttached.filter((f) => !deletedIds.has(f.id))
  const visible = activeCategory === "all" ? liveFiles : liveFiles.filter((f) => f.category === activeCategory)

  const countByCategory = (cat: FileCategory) => liveFiles.filter((f) => f.category === cat).length

  function handleDelete(fileId: string) {
    setDeletedIds((prev) => new Set([...prev, fileId]))
    setDeleteTarget(null)
  }

  function previewLabel(mimeType: string) {
    if (mimeType.startsWith("image/")) return "View"
    if (mimeType === "application/pdf") return "Open"
    return "Download"
  }

  const photos = visible.filter((f) => f.category === "image")
  const nonPhotos = visible.filter((f) => f.category !== "image")
  const showGallery = activeCategory === "image" || (activeCategory === "all" && photos.length > 0)
  const activeSection = FILE_TAB_SECTIONS.find((s) => s.id === activeCategory)!

  return (
    <>
      <div className="flex flex-col gap-3">
        {/* Category switcher */}
        <div className="flex items-center gap-1 rounded-lg bg-muted/50 p-1">
          {FILE_TAB_SECTIONS.map((section) => {
            const count = section.id === "all" ? liveFiles.length : countByCategory(section.id as FileCategory)
            const Icon = section.icon
            return (
              <button
                key={section.id}
                onClick={() => setActiveCategory(section.id)}
                className={cn(
                  "flex flex-1 items-center justify-center gap-1.5 rounded-md px-2 py-1.5 text-xs font-medium transition-all",
                  activeCategory === section.id
                    ? "bg-background text-foreground shadow-sm"
                    : "text-muted-foreground hover:text-foreground"
                )}
              >
                <Icon className="size-3" />
                <span className="hidden sm:inline">{section.label}</span>
                {count > 0 && (
                  <span className={cn(
                    "flex size-4 items-center justify-center rounded-full text-[10px] font-semibold",
                    activeCategory === section.id ? "bg-muted text-foreground" : "bg-muted/60 text-muted-foreground"
                  )}>
                    {count}
                  </span>
                )}
              </button>
            )
          })}
        </div>

        {/* Upload zone */}
        <div
          className={cn(
            "flex items-center gap-3 rounded-lg border-2 border-dashed px-4 py-3 cursor-pointer transition-colors",
            dragging ? "border-primary bg-primary/5" : "border-border hover:border-primary/40 hover:bg-muted/30"
          )}
          onDragOver={(e) => { e.preventDefault(); setDragging(true) }}
          onDragLeave={() => setDragging(false)}
          onDrop={(e) => { e.preventDefault(); setDragging(false) }}
          onClick={() => document.getElementById(`item-detail-file-input-${activeCategory}`)?.click()}
        >
          <Upload className="size-4 text-muted-foreground shrink-0" />
          <p className="text-sm text-muted-foreground flex-1">
            Drop {activeCategory === "image" ? "photos" : activeCategory === "invoice" ? "invoices" : activeCategory === "document" ? "documents" : "files"} or{" "}
            <span className="font-medium text-foreground">browse</span>
          </p>
          <input
            id={`item-detail-file-input-${activeCategory}`}
            type="file"
            multiple
            accept={
              activeCategory === "image" ? "image/*" :
              activeCategory === "invoice" ? "application/pdf,image/*" :
              activeCategory === "document" ? ".pdf,.doc,.docx,application/pdf" :
              "*"
            }
            className="sr-only"
          />
        </div>

        {visible.length === 0 ? (
          <div className="flex flex-col items-center gap-2 rounded-lg border border-dashed border-border py-8 text-center">
            {(() => { const Icon = activeSection.icon; return <Icon className="size-7 text-muted-foreground/30" /> })()}
            <p className="text-sm text-muted-foreground max-w-xs leading-relaxed">{activeSection.emptyHint}</p>
          </div>
        ) : (
          <div className="flex flex-col gap-3">
            {/* Photo gallery */}
            {showGallery && photos.length > 0 && (
              <div className="grid grid-cols-3 gap-1.5">
                {photos.map((f) => (
                  <button
                    key={f.id}
                    className="group relative aspect-square rounded-lg overflow-hidden border border-border bg-muted focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                    onClick={() => setPreviewFile(f)}
                  >
                    {f.thumbnailUrl ? (
                      <img src={f.thumbnailUrl} alt={f.name} className="absolute inset-0 w-full h-full object-cover" />
                    ) : (
                      <div className="absolute inset-0 flex items-center justify-center">
                        <ImageIcon className="size-8 text-muted-foreground/30" />
                      </div>
                    )}
                    <div className="absolute inset-0 bg-black/0 group-hover:bg-black/30 transition-colors flex items-end p-1.5 opacity-0 group-hover:opacity-100">
                      <p className="text-[10px] text-white font-medium truncate leading-tight">{f.name}</p>
                    </div>
                    <button
                      type="button"
                      className="absolute top-1 right-1 flex size-5 items-center justify-center rounded-full bg-black/60 text-white opacity-0 group-hover:opacity-100 hover:bg-destructive transition-all"
                      onClick={(e) => { e.stopPropagation(); setDeleteTarget(f) }}
                    >
                      <Trash2 className="size-2.5" />
                    </button>
                  </button>
                ))}
              </div>
            )}

            {/* Non-photo list (or all-non-photo when in "all" view) */}
            {(activeCategory !== "image" && nonPhotos.length > 0) && (
              <ul className="flex flex-col gap-1.5">
                {nonPhotos.map((f) => {
                  const tagObjs = resolveTags(f.tags)
                  return (
                    <li key={f.id} className="flex items-center gap-3 rounded-lg border border-border bg-card px-3 py-2.5">
                      {fileIcon(f.mimeType)}
                      <div className="flex-1 min-w-0">
                        <div className="flex items-center gap-1.5">
                          <p className="text-sm font-medium truncate">{f.name}</p>
                          {activeCategory === "all" && (
                            <span className={cn(
                              "text-[10px] font-medium px-1.5 py-0.5 rounded-full shrink-0",
                              f.category === "invoice" ? "bg-chart-1/10 text-chart-1" : "bg-chart-3/10 text-chart-3"
                            )}>
                              {f.category === "invoice" ? "Invoice" : f.category === "document" ? "Doc" : f.category}
                            </span>
                          )}
                        </div>
                        <div className="flex items-center gap-2 mt-0.5">
                          <span className="text-xs text-muted-foreground">{f.size}</span>
                          {tagObjs.map((t) => (
                            <TagPill key={t.id} tag={t} size="xs" />
                          ))}
                        </div>
                      </div>
                      <div className="flex items-center gap-1 shrink-0">
                        <Button
                          variant="ghost"
                          size="sm"
                          className="h-7 px-2 text-xs gap-1"
                          onClick={() => setPreviewFile(f)}
                        >
                          {previewLabel(f.mimeType)}
                          <ExternalLink className="size-3" />
                        </Button>
                        <Button
                          variant="ghost"
                          size="icon"
                          className="size-7 text-muted-foreground hover:text-destructive hover:bg-destructive/10"
                          onClick={() => setDeleteTarget(f)}
                        >
                          <Trash2 className="size-3.5" />
                        </Button>
                      </div>
                    </li>
                  )
                })}
              </ul>
            )}
          </div>
        )}
      </div>

      <FilePreviewDialog
        file={previewFile}
        onClose={() => setPreviewFile(null)}
        onDelete={(id) => handleDelete(id)}
      />

      <AlertDialog open={!!deleteTarget} onOpenChange={(open) => !open && setDeleteTarget(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete file?</AlertDialogTitle>
            <AlertDialogDescription>
              <span className="font-medium text-foreground">{deleteTarget?.name}</span> will be permanently deleted. This cannot be undone.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction
              onClick={() => deleteTarget && handleDelete(deleteTarget.id)}
              className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
            >
              Delete
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  )
}

// ─── Edit form ───────────────────────────────────────────────────────────────

function F({ label, htmlFor, hint, children }: {
  label: string; htmlFor: string; hint?: string; children: React.ReactNode
}) {
  return (
    <div className="flex flex-col gap-1.5">
      <Label htmlFor={htmlFor} className="text-sm font-medium">{label}</Label>
      {children}
      {hint && <p className="text-xs text-muted-foreground">{hint}</p>}
    </div>
  )
}

function ItemEditForm({ item, onCancel }: { item: InventoryItem; onCancel: () => void }) {
  const [name, setName] = useState(item.name)
  const [brand, setBrand] = useState(item.brand)
  const [model, setModel] = useState(item.model)
  const [category, setCategory] = useState<ItemCategory>(item.category)
  const [serialNumber, setSerialNumber] = useState(item.serialNumber)

  const area = MOCK_AREAS.find((a) => a.id === item.areaId)
  const [selectedLocationId, setSelectedLocationId] = useState(area?.locationId ?? "")
  const [selectedAreaId, setSelectedAreaId] = useState(item.areaId)

  const [purchasedAt, setPurchasedAt] = useState(item.purchasedAt ?? "")
  const [purchasePrice, setPurchasePrice] = useState(item.purchasePrice?.toString() ?? "")
  const [currentValue, setCurrentValue] = useState(item.currentValue?.toString() ?? "")

  const [warrantyExpiry, setWarrantyExpiry] = useState(item.warranty.expiresAt ?? "")
  const [warrantyNotes, setWarrantyNotes] = useState(item.warranty.notes)

  const [notes, setNotes] = useState(item.notes)
  const [tags, setTags] = useState<string[]>(item.tags)

  const availableAreas = selectedLocationId
    ? MOCK_AREAS.filter((a) => a.locationId === selectedLocationId)
    : []

  const itemTagPalette = MOCK_TAGS.filter((t) => t.id.startsWith("i"))

  function toggleTag(id: string) {
    setTags((prev) => prev.includes(id) ? prev.filter((x) => x !== id) : [...prev, id])
  }

  return (
    <div className="flex flex-col gap-5 py-2">
      <div className="space-y-3">
        <p className="text-xs font-semibold uppercase tracking-widest text-muted-foreground">Identity</p>
        <F label="Item Name" htmlFor="edit-name">
          <Input id="edit-name" value={name} onChange={(e) => setName(e.target.value)} />
        </F>
        <div className="grid grid-cols-2 gap-3">
          <F label="Brand" htmlFor="edit-brand">
            <Input id="edit-brand" value={brand} onChange={(e) => setBrand(e.target.value)} />
          </F>
          <F label="Model" htmlFor="edit-model">
            <Input id="edit-model" value={model} onChange={(e) => setModel(e.target.value)} />
          </F>
        </div>
        <F label="Category" htmlFor="edit-category">
          <Select value={category} onValueChange={(v) => setCategory(v as ItemCategory)}>
            <SelectTrigger id="edit-category"><SelectValue /></SelectTrigger>
            <SelectContent>
              {CATEGORIES.map((c) => <SelectItem key={c.value} value={c.value}>{c.label}</SelectItem>)}
            </SelectContent>
          </Select>
        </F>
        <F label="Serial Number" htmlFor="edit-serial">
          <Input id="edit-serial" value={serialNumber} onChange={(e) => setSerialNumber(e.target.value)} className="font-mono text-sm" />
        </F>
      </div>

      <Separator />

      <div className="space-y-3">
        <p className="text-xs font-semibold uppercase tracking-widest text-muted-foreground">Location</p>
        <div className="grid grid-cols-2 gap-3">
          <F label="Location" htmlFor="edit-location">
            <Select value={selectedLocationId} onValueChange={(v) => { setSelectedLocationId(v); setSelectedAreaId("") }}>
              <SelectTrigger id="edit-location"><SelectValue placeholder="Select…" /></SelectTrigger>
              <SelectContent>
                {MOCK_LOCATIONS.map((l) => <SelectItem key={l.id} value={l.id}>{l.icon} {l.name}</SelectItem>)}
              </SelectContent>
            </Select>
          </F>
          <F label="Area" htmlFor="edit-area">
            <Select value={selectedAreaId} onValueChange={setSelectedAreaId} disabled={!selectedLocationId}>
              <SelectTrigger id="edit-area"><SelectValue placeholder="Select…" /></SelectTrigger>
              <SelectContent>
                {availableAreas.map((a) => <SelectItem key={a.id} value={a.id}>{a.icon} {a.name}</SelectItem>)}
              </SelectContent>
            </Select>
          </F>
        </div>
      </div>

      <Separator />

      <div className="space-y-3">
        <p className="text-xs font-semibold uppercase tracking-widest text-muted-foreground">Purchase</p>
        <F label="Purchase Date" htmlFor="edit-purchased">
          <Input id="edit-purchased" type="date" value={purchasedAt} onChange={(e) => setPurchasedAt(e.target.value)} />
        </F>
        <div className="grid grid-cols-2 gap-3">
          <F label="Purchase Price" htmlFor="edit-price">
            <div className="relative">
              <span className="absolute left-3 top-1/2 -translate-y-1/2 text-sm text-muted-foreground">$</span>
              <Input id="edit-price" type="number" min="0" className="pl-6" value={purchasePrice} onChange={(e) => setPurchasePrice(e.target.value)} />
            </div>
          </F>
          <F label="Current Value" htmlFor="edit-value">
            <div className="relative">
              <span className="absolute left-3 top-1/2 -translate-y-1/2 text-sm text-muted-foreground">$</span>
              <Input id="edit-value" type="number" min="0" className="pl-6" value={currentValue} onChange={(e) => setCurrentValue(e.target.value)} />
            </div>
          </F>
        </div>
      </div>

      <Separator />

      <div className="space-y-3">
        <p className="text-xs font-semibold uppercase tracking-widest text-muted-foreground">Warranty</p>
        <F label="Expiry Date" htmlFor="edit-warranty-exp" hint="Leave blank if no warranty">
          <Input id="edit-warranty-exp" type="date" value={warrantyExpiry} onChange={(e) => setWarrantyExpiry(e.target.value)} />
        </F>
        <F label="Warranty Notes" htmlFor="edit-warranty-notes">
          <Textarea id="edit-warranty-notes" rows={2} className="resize-none" value={warrantyNotes} onChange={(e) => setWarrantyNotes(e.target.value)} />
        </F>
      </div>

      <Separator />

      <div className="space-y-3">
        <p className="text-xs font-semibold uppercase tracking-widest text-muted-foreground">Notes & Tags</p>
        <F label="Notes" htmlFor="edit-notes">
          <Textarea id="edit-notes" rows={3} className="resize-none" value={notes} onChange={(e) => setNotes(e.target.value)} />
        </F>

        <div className="flex flex-col gap-1.5">
          <Label>Tags</Label>
          <div className="flex flex-wrap gap-1.5">
            {itemTagPalette.map((tag) => {
              const active = tags.includes(tag.id)
              return (
                <button
                  key={tag.id}
                  type="button"
                  onClick={() => toggleTag(tag.id)}
                  className={cn(
                    "rounded-full border px-2.5 py-0.5 text-xs font-medium transition-colors",
                    active
                      ? "bg-primary text-primary-foreground border-primary"
                      : "border-border bg-background text-muted-foreground hover:border-primary/50"
                  )}
                >
                  #{tag.label}
                </button>
              )
            })}
          </div>
          {tags.length > 0 && (
            <div className="flex flex-wrap gap-1 mt-1">
              {resolveTags(tags).map((tag) => (
                <TagPill key={tag.id} tag={tag} size="xs" onRemove={() => toggleTag(tag.id)} />
              ))}
            </div>
          )}
        </div>
      </div>

      <Separator />

      <div className="flex gap-2 pb-2">
        <Button variant="outline" className="flex-1" onClick={onCancel}>Cancel</Button>
        <Button className="flex-1" onClick={onCancel}>Save Changes</Button>
      </div>
    </div>
  )
}

// ─── Main sheet ───────────────────────────────────────────────────────────────

export function ItemDetail({ itemId, onClose, onOpenInsuranceReport }: ItemDetailProps) {
  const item = itemId ? MOCK_ITEMS.find((i) => i.id === itemId) ?? null : null

  return (
    <Sheet open={!!item} onOpenChange={(open) => !open && onClose()}>
      <SheetContent className="w-full sm:max-w-lg flex flex-col gap-0 overflow-y-auto p-0">
        {item && <ItemDetailContent item={item} onOpenInsuranceReport={onOpenInsuranceReport} />}
      </SheetContent>
    </Sheet>
  )
}

function ItemDetailContent({ item, onOpenInsuranceReport }: { item: InventoryItem; onOpenInsuranceReport?: (id: string) => void }) {
  const [editing, setEditing] = useState(false)
  const [deleteOpen, setDeleteOpen] = useState(false)
  const [transitionTarget, setTransitionTarget] = useState<CommodityStatus | null>(null)
  const [currentStatus, setCurrentStatus] = useState<CommodityStatus>(item.status)

  const status = warrantyStatus(item)
  const days = daysUntil(item.warranty.expiresAt)
  const config = WARRANTY_STATUS_CONFIG[status]
  const fileCount = MOCK_FILES.filter((f) => f.attachedTo.type === "commodity" && f.attachedTo.id === item.id).length
  const statusCfg = COMMODITY_STATUS_CONFIG[currentStatus]

  function handleStatusTransition(newStatus: CommodityStatus, _note: string, _date: string, _salePrice: string) {
    setCurrentStatus(newStatus)
    setTransitionTarget(null)
  }

  if (editing) {
    return (
      <div className="flex flex-col gap-0 px-5 pb-5">
        <div className="flex items-center gap-2 pt-5 pb-4 border-b border-border mb-5 sticky top-0 bg-background z-10">
          <Button variant="ghost" size="icon" className="size-8 -ml-1 shrink-0" onClick={() => setEditing(false)}>
            <ArrowLeft className="size-4" />
          </Button>
          <div className="min-w-0">
            <p className="text-sm font-semibold leading-tight">Editing</p>
            <p className="text-xs text-muted-foreground truncate">{item.name}</p>
          </div>
        </div>
        <ItemEditForm item={item} onCancel={() => setEditing(false)} />
      </div>
    )
  }

  return (
    <div className="flex flex-col gap-0 px-5 pb-5">
      {/* Header */}
      <SheetHeader className="pt-6 pb-4 px-0">
        <div className="flex items-start gap-3">
          <div className="flex size-12 shrink-0 items-center justify-center rounded-xl bg-muted text-2xl">
            {CATEGORY_ICONS[item.category]}
          </div>
          <div className="flex-1 min-w-0 pr-6">
            <SheetTitle className="text-lg leading-tight">{item.name}</SheetTitle>
            {item.shortName && item.shortName !== item.name && (
              <p className="text-xs font-mono text-muted-foreground mt-0.5">{item.shortName}</p>
            )}
            <SheetDescription className="mt-0.5">
              {item.brand} · {item.model}
            </SheetDescription>
          </div>
        </div>

        {/* Status badges row */}
        <div className="flex flex-wrap items-center gap-2 pt-1">
          {/* Commodity status */}
          <span className={cn(
            "inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-xs font-medium border",
            statusCfg.bg, statusCfg.color, "border-current/20"
          )}>
            <CircleDot className="size-3" />
            {statusCfg.label}
          </span>

          {/* Draft badge */}
          {item.draft && (
            <Badge variant="outline" className="gap-1 text-xs text-muted-foreground border-dashed">
              Draft
            </Badge>
          )}

          {/* Warranty badge */}
          <WarrantyBadge item={item} />
          {days !== null && days > 0 && (
            <span className="text-xs text-muted-foreground">{days} days remaining</span>
          )}
        </div>
      </SheetHeader>

      {/* Action buttons */}
      <div className="flex gap-2 pb-4 flex-wrap">
        <Button variant="outline" size="sm" className="flex-1 gap-1.5" onClick={() => setEditing(true)}>
          <Pencil className="size-3.5" />
          Edit
        </Button>
        <Button
          variant="outline"
          size="sm"
          className="gap-1.5"
          onClick={() => onOpenInsuranceReport?.(item.id)}
        >
          <FileBarChart2 className="size-3.5" />
          Insurance Report
        </Button>
        <Button
          variant="outline"
          size="sm"
          className="text-destructive hover:bg-destructive/10"
          onClick={() => setDeleteOpen(true)}
        >
          <Trash2 className="size-3.5" />
        </Button>
      </div>

      {/* Status transition actions (only for in_use items) */}
      {currentStatus === "in_use" && (
        <div className="mb-4 rounded-xl border border-border bg-muted/30 p-3 space-y-2">
          <p className="text-xs font-semibold uppercase tracking-widest text-muted-foreground">Change Status</p>
          <div className="flex flex-wrap gap-1.5">
            {(["sold", "lost", "disposed", "written_off"] as CommodityStatus[]).map((s) => {
              const c = COMMODITY_STATUS_CONFIG[s]
              return (
                <Button
                  key={s}
                  variant="outline"
                  size="sm"
                  className={cn("gap-1.5 text-xs h-7", c.color)}
                  onClick={() => setTransitionTarget(s)}
                >
                  {c.label}
                </Button>
              )
            })}
          </div>
        </div>
      )}

      {/* If item has a terminal status, show status info card */}
      {currentStatus !== "in_use" && (
        <div className={cn("mb-4 rounded-xl border p-3 space-y-1", statusCfg.bg, "border-current/15")}>
          <div className="flex items-center gap-1.5">
            <AlertTriangle className={cn("size-3.5", statusCfg.color)} />
            <p className={cn("text-xs font-semibold", statusCfg.color)}>{statusCfg.label}</p>
          </div>
          {item.statusDate && (
            <p className="text-xs text-muted-foreground">Date: {formatDate(item.statusDate)}</p>
          )}
          {item.statusNote && (
            <p className="text-xs text-muted-foreground">{item.statusNote}</p>
          )}
          {item.salePrice != null && (
            <p className="text-xs text-muted-foreground">
              Sale price: {formatCurrency(item.salePrice, item.purchaseCurrency)}
            </p>
          )}
          <Button
            variant="ghost"
            size="sm"
            className="h-6 text-xs px-1 mt-1"
            onClick={() => setTransitionTarget("in_use")}
          >
            Revert to In Use
          </Button>
        </div>
      )}

      {/* Tabs */}
      <Tabs defaultValue="details">
        <TabsList variant="line" className="w-full justify-start">
          <TabsTrigger value="details">Details</TabsTrigger>
          <TabsTrigger value="warranty">Warranty</TabsTrigger>
          <TabsTrigger value="files" className="gap-1.5">
            <Paperclip className="size-3.5" />
            Files
            {fileCount > 0 && (
              <span className="flex size-4 items-center justify-center rounded-full bg-muted text-[10px] font-medium">
                {fileCount}
              </span>
            )}
          </TabsTrigger>
          <TabsTrigger value="supplies">Supplies</TabsTrigger>
        </TabsList>

        <TabsContent value="details" className="mt-4 space-y-0">
          <DetailRow icon={MapPin} label="Location" value={areaLabel(item.areaId)} />
          <Separator />
          {item.count > 1 && (
            <>
              <DetailRow icon={Layers} label="Quantity" value={item.count} />
              <Separator />
            </>
          )}
          <DetailRow icon={Calendar} label="Purchase Date" value={formatDate(item.purchasedAt)} />
          <Separator />
          <DetailRow
            icon={DollarSign}
            label="Purchase Price"
            value={item.purchasePrice != null
              ? `${formatCurrency(item.purchasePrice, item.purchaseCurrency)} ${item.purchaseCurrency !== "USD" ? `(${item.purchaseCurrency})` : ""}`
              : "—"
            }
          />
          <Separator />
          <DetailRow icon={DollarSign} label="Current Value" value={formatCurrency(item.currentValue)} />
          <Separator />
          <DetailRow
            icon={Hash}
            label="Serial Number"
            value={item.serialNumber ? <span className="font-mono text-xs">{item.serialNumber}</span> : "—"}
          />
          {item.extraSerialNumbers.length > 0 && (
            <>
              <Separator />
              <DetailRow
                icon={Hash}
                label="Additional Serials"
                value={
                  <div className="flex flex-wrap gap-1">
                    {item.extraSerialNumbers.map((s) => (
                      <span key={s} className="font-mono text-xs bg-muted px-1.5 py-0.5 rounded">{s}</span>
                    ))}
                  </div>
                }
              />
            </>
          )}
          {item.partNumbers.length > 0 && (
            <>
              <Separator />
              <DetailRow
                icon={Package}
                label="Part Numbers"
                value={
                  <div className="flex flex-wrap gap-1">
                    {item.partNumbers.map((p) => (
                      <span key={p} className="font-mono text-xs bg-muted px-1.5 py-0.5 rounded">{p}</span>
                    ))}
                  </div>
                }
              />
            </>
          )}
          {item.urls.length > 0 && (
            <>
              <Separator />
              <DetailRow
                icon={LinkIcon}
                label="Product URLs"
                value={
                  <ul className="space-y-0.5">
                    {item.urls.map((u, i) => (
                      <li key={i}>
                        <a
                          href={u.url}
                          target="_blank"
                          rel="noopener noreferrer"
                          className="text-xs text-primary hover:underline inline-flex items-center gap-1"
                        >
                          {u.label || u.url}
                          <ExternalLink className="size-3" />
                        </a>
                      </li>
                    ))}
                  </ul>
                }
              />
            </>
          )}
          {item.notes && (
            <>
              <Separator />
              <DetailRow icon={FileText} label="Notes" value={<span className="font-normal text-muted-foreground">{item.notes}</span>} />
            </>
          )}
          {item.tags.length > 0 && (
            <>
              <Separator />
              <DetailRow
                icon={Tag}
                label="Tags"
                value={
                  <div className="flex flex-wrap gap-1 mt-0.5">
                    {resolveTags(item.tags).map((tag) => (
                      <TagPill key={tag.id} tag={tag} size="xs" />
                    ))}
                  </div>
                }
              />
            </>
          )}
        </TabsContent>

        <TabsContent value="warranty" className="mt-4">
          <div className={`rounded-lg p-4 ${config.bg} border border-current/10 mb-4`}>
            <div className="flex items-center gap-2 mb-2">
              {status === "active" && <ShieldCheck className={`size-4 ${config.color}`} />}
              {status === "expiring" && <ShieldAlert className={`size-4 ${config.color}`} />}
              {status === "expired" && <ShieldOff className={`size-4 ${config.color}`} />}
              {status === "none" && <Shield className={`size-4 ${config.color}`} />}
              <span className={`text-sm font-semibold ${config.color}`}>{config.label}</span>
            </div>
            {item.warranty.expiresAt && (
              <p className="text-sm text-muted-foreground">
                {days !== null && days > 0
                  ? `Expires ${formatDate(item.warranty.expiresAt)} — ${days} days from now`
                  : `Expired ${formatDate(item.warranty.expiresAt)}`}
              </p>
            )}
            {status === "none" && (
              <p className="text-sm text-muted-foreground">No warranty information recorded.</p>
            )}
          </div>

          {item.warranty.notes && (
            <div className="rounded-lg border border-border p-3 text-sm text-muted-foreground mb-4">
              {item.warranty.notes}
            </div>
          )}

          <Button variant="outline" size="sm" className="gap-1.5">
            <FileText className="size-3.5" />
            Upload Receipt
          </Button>
        </TabsContent>

        <TabsContent value="files" className="mt-4">
          <ItemFilesTab item={item} />
        </TabsContent>

        <TabsContent value="supplies" className="mt-4">
          {item.supplyLinks.length === 0 ? (
            <div className="rounded-lg border border-dashed border-border py-10 text-center">
              <p className="text-sm text-muted-foreground">No supply links added yet.</p>
              <Button variant="outline" size="sm" className="mt-3">Add Link</Button>
            </div>
          ) : (
            <ul className="space-y-2">
              {item.supplyLinks.map((link, i) => (
                <li key={i}>
                  <a
                    href={link.url}
                    className="flex items-center justify-between rounded-lg border border-border px-4 py-3 transition-colors hover:bg-muted/50 group"
                  >
                    <span className="text-sm font-medium">{link.label}</span>
                    <ExternalLink className="size-3.5 text-muted-foreground group-hover:text-foreground transition-colors" />
                  </a>
                </li>
              ))}
              <li>
                <Button variant="outline" size="sm" className="w-full">Add Supply Link</Button>
              </li>
            </ul>
          )}
        </TabsContent>
      </Tabs>

      {/* Delete confirmation */}
      <AlertDialog open={deleteOpen} onOpenChange={setDeleteOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete item?</AlertDialogTitle>
            <AlertDialogDescription>
              <span className="font-medium text-foreground">{item.name}</span> will be permanently deleted along with all its files and data. This cannot be undone.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction className="bg-destructive text-destructive-foreground hover:bg-destructive/90">
              Delete
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      {/* Status transition dialog */}
      <StatusTransitionDialog
        open={!!transitionTarget}
        targetStatus={transitionTarget}
        item={item}
        onClose={() => setTransitionTarget(null)}
        onConfirm={handleStatusTransition}
      />
    </div>
  )
}

import React, { useState, useEffect, useCallback } from "react"
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from "@/components/ui/dialog"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Textarea } from "@/components/ui/textarea"
import { Label } from "@/components/ui/label"
import { Badge } from "@/components/ui/badge"
import { Separator } from "@/components/ui/separator"
import { Switch } from "@/components/ui/switch"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { Upload, X, FileText, Image, File, Plus, Image as ImageIcon, Receipt, BookOpen, Sparkles, Camera, ScanText, CircleCheck as CheckCircle2, CircleAlert as AlertCircle, ChevronDown, TriangleAlert, RefreshCw, ServerCrash } from "lucide-react"
import { CATEGORIES, MOCK_LOCATIONS, MOCK_AREAS, CURRENCIES, type ItemCategory, type FileCategory } from "@/data/mock"
import { cn, makeId } from "@/lib/utils"
import { CurrencyCombobox } from "@/components/CurrencyCombobox"

// ─── Brand / model data ───────────────────────────────────────

const BRAND_MODELS: Record<string, string[]> = {
  Apple: ["MacBook Pro 16\"", "MacBook Air M3", "iPhone 15 Pro", "iPad Pro 13\"", "AirPods Pro", "Apple Watch Ultra 2"],
  Samsung: ["Galaxy S24 Ultra", "QN65Q80C QLED TV", "Bespoke Refrigerator", "WW90T 9000 Washer", "Galaxy Tab S9"],
  Miele: ["WCI 870 Washer", "G 7600 Dishwasher", "KFN 29683 D Fridge", "H 7860 BP Oven", "S 8340 Vacuum"],
  Bosch: ["SMS6ZCW08E Dishwasher", "WGB256A40 Washer", "KGN86AIDR Fridge", "HBG675BS1 Oven", "GHO 18V-Li Planer"],
  Sony: ["WH-1000XM5 Headphones", "Bravia XR A95L TV", "PlayStation 5", "ZV-E10 Camera", "LinkBuds S"],
  LG: ["OLED C3 65\" TV", "WM4000HWA Washer", "LRMVS3006S Fridge", "InstaView Range", "Gram 16 Laptop"],
  Dyson: ["V15 Detect Vacuum", "V12 Slim Vacuum", "Airwrap Complete", "Hot+Cool Fan", "Cinetic Big Ball"],
  Philips: ["Sonicare 9900 Toothbrush", "Hue Bridge Starter Kit", "55OLED807 TV", "Airfryer XXL", "S9000 Shaver"],
  Nikon: ["Z8 Camera", "Z6 III Camera", "D850 DSLR", "AF-S 50mm f/1.8G Lens"],
  Canon: ["EOS R5 Camera", "EOS 5D Mark IV", "RF 24-70mm f/2.8 L Lens", "PIXMA G620 Printer"],
  Dell: ["XPS 15 9530", "Alienware m18 R2", "UltraSharp U2723QE Monitor", "Latitude 5540"],
  HP: ["Spectre x360 14", "Envy 17", "LaserJet Pro MFP", "DesignJet T250"],
  Lenovo: ["ThinkPad X1 Carbon Gen 12", "ThinkPad T14s", "IdeaPad 5 Pro", "Legion 7i"],
  Makita: ["DHP486 Drill Driver", "DGA516 Angle Grinder", "DHR243 SDS Drill", "DCS552 Multi-Cutter"],
  DeWalt: ["DCD999 Drill", "DCF850 Impact Driver", "DCS334 Jigsaw", "DCK300P3 3-Tool Combo"],
  IKEA: ["KALLAX Shelf", "BILLY Bookcase", "HEMNES Bed Frame", "EKTORP Sofa"],
  Siemens: ["iQ500 Washer", "SN678X26TE Dishwasher", "KG39NXIEP Fridge", "HB578A0S0 Oven"],
  Whirlpool: ["WTW8127LC Washer", "WRS325SDHZ Refrigerator", "WDT730PAHZ Dishwasher"],
  KitchenAid: ["Artisan Stand Mixer", "Sous Vide Precision Cooker", "5-Speed Hand Blender"],
  Nespresso: ["Vertuo Next", "Vertuo Pop", "Original Essenza Mini", "Lattissima Pro"],
}

const ALL_BRANDS = [
  "Apple", "Samsung", "Miele", "Bosch", "Sony", "LG", "Dyson", "Philips",
  "Nikon", "Canon", "Dell", "HP", "Lenovo", "Makita", "DeWalt", "IKEA",
  "Siemens", "Whirlpool", "KitchenAid", "Nespresso", "Microsoft", "Asus",
  "Acer", "MSI", "Logitech", "Razer", "Corsair", "Jabra", "Bose", "JBL",
  "AEG", "Electrolux", "Hoover", "Candy", "Smeg", "Fisher & Paykel",
  "Thermador", "Viking", "Subzero", "Moen", "Kohler", "Grohe", "Hansgrohe",
  "Husqvarna", "Black+Decker", "Ryobi", "Ridgid", "Milwaukee",
  "Xiaomi", "Huawei", "OnePlus", "Google", "Amazon", "Ring",
  "Nest", "Ecobee", "Lutron", "Sonos", "Harman Kardon",
]

// ─── Types ────────────────────────────────────────────────────

interface AttachedFileDraft {
  id: string
  name: string
  size: string
  mimeType: string
  category: FileCategory
}

// step -1 = AI photo offer, steps 0..4 = form
const FORM_STEPS = [
  { id: "basics", label: "Basics" },
  { id: "purchase", label: "Purchase" },
  { id: "warranty", label: "Warranty" },
  { id: "extras", label: "Extras" },
  { id: "files", label: "Files" },
]

interface AddItemDialogProps {
  open: boolean
  onClose: () => void
  defaultAreaId?: string
}

type AiPhase = "offer" | "scanning" | "review" | "skipped"

// ─── Validation types ─────────────────────────────────────────

type StepErrors = Record<string, string>

// Server error shape
interface ServerError {
  type: "network" | "conflict" | "validation" | "unknown"
  message: string
  fieldErrors?: Record<string, string>
}

// ─── Field error display ───────────────────────────────────────

function FieldError({ message }: { message: string }) {
  return (
    <p className="flex items-center gap-1.5 text-xs text-destructive" role="alert">
      <AlertCircle className="size-3 shrink-0" />
      {message}
    </p>
  )
}

// ─── Field wrapper with error support ─────────────────────────

function F({ label, htmlFor, hint, required, error, children }: {
  label: string
  htmlFor: string
  hint?: string
  required?: boolean
  error?: string
  children: React.ReactNode
}) {
  return (
    <div className="flex flex-col gap-1.5">
      <Label htmlFor={htmlFor} className="text-sm font-medium">
        {label}{required && <span className="text-destructive ml-0.5">*</span>}
      </Label>
      {children}
      {error ? (
        <FieldError message={error} />
      ) : hint ? (
        <p className="text-xs text-muted-foreground">{hint}</p>
      ) : null}
    </div>
  )
}

// ─── Server error banner ──────────────────────────────────────

function ServerErrorBanner({ error, onRetry, onDismiss }: {
  error: ServerError
  onRetry?: () => void
  onDismiss: () => void
}) {
  const icons: Record<ServerError["type"], React.ReactNode> = {
    network: <RefreshCw className="size-4 shrink-0" />,
    conflict: <AlertCircle className="size-4 shrink-0" />,
    validation: <TriangleAlert className="size-4 shrink-0" />,
    unknown: <ServerCrash className="size-4 shrink-0" />,
  }

  const titles: Record<ServerError["type"], string> = {
    network: "Connection error",
    conflict: "Duplicate item",
    validation: "Invalid data",
    unknown: "Server error",
  }

  return (
    <div className="rounded-lg border border-destructive/30 bg-destructive/8 px-3 py-3 flex gap-3">
      <span className="text-destructive mt-0.5 shrink-0">{icons[error.type]}</span>
      <div className="flex-1 min-w-0 space-y-1">
        <p className="text-sm font-semibold text-destructive">{titles[error.type]}</p>
        <p className="text-xs text-destructive/90 leading-relaxed">{error.message}</p>
        {error.fieldErrors && Object.keys(error.fieldErrors).length > 0 && (
          <ul className="mt-1.5 space-y-0.5">
            {Object.entries(error.fieldErrors).map(([field, msg]) => (
              <li key={field} className="text-xs text-destructive/80 flex items-start gap-1">
                <span className="text-destructive/50 shrink-0">·</span>
                <span><span className="font-medium capitalize">{field}</span>: {msg}</span>
              </li>
            ))}
          </ul>
        )}
      </div>
      <div className="flex flex-col items-end gap-1 shrink-0">
        {onRetry && (
          <button
            type="button"
            onClick={onRetry}
            className="text-xs font-medium text-destructive hover:text-destructive/80 transition-colors underline underline-offset-2"
          >
            Retry
          </button>
        )}
        <button
          type="button"
          onClick={onDismiss}
          className="text-muted-foreground hover:text-foreground transition-colors"
          aria-label="Dismiss error"
        >
          <X className="size-3.5" />
        </button>
      </div>
    </div>
  )
}

// ─── Validation logic ──────────────────────────────────────────

function validateBasics(
  name: string,
  shortName: string,
  count: string,
  _isDraft: boolean,
  touched: Set<string>
): StepErrors {
  const errors: StepErrors = {}

  if (touched.has("name") && !name.trim()) {
    errors.name = "Item name is required."
  }
  if (touched.has("shortName") && shortName.trim() && shortName.trim().length > 20) {
    errors.shortName = "Short name must be 20 characters or fewer."
  }
  if (touched.has("count")) {
    const n = Number(count)
    if (!count.trim() || isNaN(n) || n < 1 || !Number.isInteger(n)) {
      errors.count = "Quantity must be a whole number of at least 1."
    }
  }
  return errors
}

function validatePurchase(
  purchasedAt: string,
  purchasePrice: string,
  isDraft: boolean,
  touched: Set<string>
): StepErrors {
  const errors: StepErrors = {}

  if (!isDraft && touched.has("purchasedAt") && !purchasedAt) {
    errors.purchasedAt = "Purchase date is required for non-draft items."
  }
  if (touched.has("purchasePrice") && purchasePrice.trim()) {
    const n = Number(purchasePrice)
    if (isNaN(n) || n < 0) {
      errors.purchasePrice = "Price must be a positive number."
    }
  }
  if (touched.has("purchasedAt") && purchasedAt) {
    const d = new Date(purchasedAt)
    if (d > new Date()) {
      errors.purchasedAt = "Purchase date cannot be in the future."
    }
  }
  return errors
}

function validateWarranty(
  warrantyExpiry: string,
  touched: Set<string>
): StepErrors {
  const errors: StepErrors = {}
  if (touched.has("warrantyExpiry") && warrantyExpiry) {
    const d = new Date(warrantyExpiry)
    const today = new Date()
    today.setHours(0, 0, 0, 0)
    if (isNaN(d.getTime())) {
      errors.warrantyExpiry = "Invalid date format."
    }
  }
  return errors
}


// ─── Main dialog ───────────────────────────────────────────────

export function AddItemDialog({ open, onClose, defaultAreaId }: AddItemDialogProps) {
  const [step, setStep] = useState(-1)
  const [aiPhase, setAiPhase] = useState<AiPhase>("offer")
  const [aiPhotos, setAiPhotos] = useState<{ id: string; name: string; preview: string }[]>([])

  // Basics
  const [name, setName] = useState("")
  const [shortName, setShortName] = useState("")
  const [brand, setBrand] = useState("")
  const [model, setModel] = useState("")
  const [category, setCategory] = useState<ItemCategory | "">("")
  const [selectedLocationId, setSelectedLocationId] = useState("")
  const [selectedAreaId, setSelectedAreaId] = useState(defaultAreaId ?? "")
  const [serialNumber, setSerialNumber] = useState("")
  const [extraSerialNumbers, setExtraSerialNumbers] = useState<string[]>([])
  const [partNumbers, setPartNumbers] = useState<string[]>([])
  const [count, setCount] = useState("1")
  const [draft, setDraft] = useState(false)

  // Purchase
  const [purchasedAt, setPurchasedAt] = useState("")
  const [purchasePrice, setPurchasePrice] = useState("")
  const [purchaseCurrency, setPurchaseCurrency] = useState("USD")
  const [currentValue, setCurrentValue] = useState("")
  const [convertedPrice, setConvertedPrice] = useState("")

  // Warranty
  const [warrantyExpiry, setWarrantyExpiry] = useState("")
  const [warrantyNotes, setWarrantyNotes] = useState("")

  // Extras
  const [notes, setNotes] = useState("")
  const [tagInput, setTagInput] = useState("")
  const [tags, setTags] = useState<string[]>([])
  const [supplyLinks, setSupplyLinks] = useState<{ id: string; label: string; url: string }[]>([])
  const [urls, setUrls] = useState<{ id: string; label: string; url: string }[]>([])

  // Files
  const [photoFiles, setPhotoFiles] = useState<AttachedFileDraft[]>([])
  const [receiptFiles, setReceiptFiles] = useState<AttachedFileDraft[]>([])
  const [documentFiles, setDocumentFiles] = useState<AttachedFileDraft[]>([])

  // Validation state — tracks which fields the user has interacted with
  const [touchedFields, setTouchedFields] = useState<Set<string>>(new Set())

  // Saving state
  const [saving, setSaving] = useState(false)
  const [serverError, setServerError] = useState<ServerError | null>(null)

  // Derived validation
  const basicsErrors = validateBasics(name, shortName, count, draft, touchedFields)
  const purchaseErrors = validatePurchase(purchasedAt, purchasePrice, draft, touchedFields)
  const warrantyErrors = validateWarranty(warrantyExpiry, touchedFields)

  const isFormStep = step >= 0
  const isLast = step === FORM_STEPS.length - 1

  // Mark a field as touched
  const touch = useCallback((field: string) => {
    setTouchedFields((prev) => new Set([...prev, field]))
  }, [])

  useEffect(() => {
    if (open) {
      setStep(-1)
      setAiPhase("offer")
      setAiPhotos([])
      setTouchedFields(new Set())
      setServerError(null)
      setSaving(false)
      const areaId = defaultAreaId ?? ""
      setSelectedAreaId(areaId)
      if (areaId) {
        const area = MOCK_AREAS.find((a) => a.id === areaId)
        if (area) setSelectedLocationId(area.locationId)
      } else {
        setSelectedLocationId("")
      }
    }
  }, [open, defaultAreaId])

  const availableAreas = selectedLocationId
    ? MOCK_AREAS.filter((a) => a.locationId === selectedLocationId)
    : []

  function handleLocationChange(locId: string) {
    setSelectedLocationId(locId)
    setSelectedAreaId("")
  }

  function handleClose() {
    setStep(-1)
    setAiPhase("offer")
    setAiPhotos([])
    setTouchedFields(new Set())
    setServerError(null)
    setSaving(false)
    setName(""); setShortName(""); setBrand(""); setModel(""); setCategory(""); setSerialNumber("")
    setExtraSerialNumbers([]); setPartNumbers([]); setCount("1"); setDraft(false)
    setSelectedLocationId(""); setSelectedAreaId("")
    setPurchasedAt(""); setPurchasePrice(""); setPurchaseCurrency("USD"); setCurrentValue(""); setConvertedPrice("")
    setWarrantyExpiry(""); setWarrantyNotes("")
    setNotes(""); setTagInput(""); setTags([])
    setSupplyLinks([])
    setUrls([]); setPhotoFiles([]); setReceiptFiles([]); setDocumentFiles([])
    onClose()
  }

  function addTag(raw: string) {
    const newTags = raw.split(",").map((t) => t.trim().toLowerCase()).filter((t) => t && !tags.includes(t))
    if (newTags.length) setTags((prev) => [...prev, ...newTags])
    setTagInput("")
  }

  function addFilesToList(files: File[], setter: React.Dispatch<React.SetStateAction<AttachedFileDraft[]>>, category: FileCategory) {
    const drafts: AttachedFileDraft[] = files.map((f) => ({
      id: makeId(),
      name: f.name,
      size: formatFileSize(f.size),
      mimeType: f.type,
      category,
    }))
    setter((prev) => [...prev, ...drafts])
  }

  // Touch all current-step fields to show errors before advancing
  function touchCurrentStepFields() {
    if (step === 0) {
      setTouchedFields((prev) => new Set([...prev, "name", "shortName", "count"]))
    } else if (step === 1) {
      setTouchedFields((prev) => new Set([...prev, "purchasedAt", "purchasePrice"]))
    } else if (step === 2) {
      setTouchedFields((prev) => new Set([...prev, "warrantyExpiry"]))
    }
  }

  function handleContinue() {
    touchCurrentStepFields()
    // Re-validate after marking touched
    const errors = step === 0
      ? validateBasics(name, shortName, count, draft, new Set(["name", "shortName", "count"]))
      : step === 1
      ? validatePurchase(purchasedAt, purchasePrice, draft, new Set(["purchasedAt", "purchasePrice"]))
      : step === 2
      ? validateWarranty(warrantyExpiry, new Set(["warrantyExpiry"]))
      : {}
    const minMet = !(step === 0 && !draft && !name.trim())
    if (Object.keys(errors).length > 0 || !minMet) return
    setStep((s) => s + 1)
  }

  async function handleSave() {
    touchCurrentStepFields()
    setServerError(null)
    setSaving(true)

    // Simulate network request
    await new Promise((r) => setTimeout(r, 1400))

    // Simulate server-side errors for demo purposes:
    // If the item name matches "error test", show a conflict error
    // If the item name matches "network", show a network error
    // Otherwise, succeed
    const lowerName = name.trim().toLowerCase()

    if (lowerName.includes("network")) {
      setSaving(false)
      setServerError({
        type: "network",
        message: "Could not reach the server. Check your connection and try again.",
      })
      return
    }

    if (lowerName.includes("error test")) {
      setSaving(false)
      setServerError({
        type: "conflict",
        message: "An item with this serial number already exists in this location group.",
        fieldErrors: {
          "serial number": "SN-MEL-2022-00412 is already registered under Washing Machine.",
        },
      })
      return
    }

    if (lowerName.includes("server error")) {
      setSaving(false)
      setServerError({
        type: "validation",
        message: "The server rejected the submission. Please review the fields below.",
        fieldErrors: {
          name: "Contains disallowed characters.",
          "purchase price": "Value exceeds the maximum allowed amount of $1,000,000.",
        },
      })
      return
    }

    setSaving(false)
    handleClose()
  }

  function handleAiScan() {
    if (aiPhotos.length === 0) return
    setAiPhase("scanning")
    setTimeout(() => {
      if (!name) setName("Samsung 65\" QLED TV")
      if (!brand) setBrand("Samsung")
      if (!model) setModel("QN65Q80CAFXZA")
      if (!serialNumber) setSerialNumber("SN-SAM-2023-88421")
      setCategory("electronics" as ItemCategory)
      setAiPhase("review")
    }, 2200)
  }

  function handleAiSkip() {
    setAiPhase("skipped")
    setStep(0)
  }

  function handleAiConfirm() {
    setStep(0)
  }

  // Whether a step has errors that should be shown on the progress bar
  // (only after the user has visited and tried to advance from it)
  const stepHasVisibleError = (i: number) => {
    if (i > step) return false
    if (i === 0) return Object.keys(validateBasics(name, shortName, count, draft, touchedFields)).length > 0
    if (i === 1) return Object.keys(validatePurchase(purchasedAt, purchasePrice, draft, touchedFields)).length > 0
    if (i === 2) return Object.keys(validateWarranty(warrantyExpiry, touchedFields)).length > 0
    return false
  }

  return (
    <Dialog open={open} onOpenChange={(o) => !o && handleClose()}>
      <DialogContent className="sm:max-w-lg max-h-[90vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            {step === -1 && aiPhase !== "skipped" ? (
              <>
                <Sparkles className="size-4 text-amber-500" />
                Fill with AI
              </>
            ) : (
              "Add Item"
            )}
          </DialogTitle>
          <DialogDescription>
            {step === -1
              ? aiPhase === "scanning"
                ? "Analysing your photos…"
                : aiPhase === "review"
                ? "Review the extracted details below before continuing"
                : "Photograph the item and its label — AI will pre-fill the form for you"
              : `Step ${step + 1} of ${FORM_STEPS.length} — ${FORM_STEPS[step].label}`
            }
          </DialogDescription>
        </DialogHeader>

        {/* Progress bar — only shown during form steps */}
        {isFormStep && (
          <div className="flex gap-1.5">
            {FORM_STEPS.map((s, i) => (
              <button
                key={s.id}
                title={s.label}
                className={cn(
                  "h-1.5 flex-1 rounded-full transition-all",
                  i === step
                    ? stepHasVisibleError(i) ? "bg-destructive" : "bg-primary"
                    : i < step
                    ? stepHasVisibleError(i) ? "bg-destructive/60 cursor-pointer" : "bg-primary cursor-pointer"
                    : "bg-muted"
                )}
                onClick={() => i < step && setStep(i)}
              />
            ))}
          </div>
        )}

        {/* Draft toggle — only on form steps */}
        {isFormStep && (
          <div className="flex items-center gap-3 rounded-lg border border-border bg-muted/30 px-3 py-2.5">
            <Switch id="draft-mode" checked={draft} onCheckedChange={(v) => {
              setDraft(v)
              // Re-touch to re-validate against new draft state
              if (touchedFields.size > 0) {
                setTouchedFields((prev) => new Set(prev))
              }
            }} />
            <div className="flex-1 min-w-0">
              <Label htmlFor="draft-mode" className="text-sm font-medium cursor-pointer">Save as draft</Label>
              <p className="text-xs text-muted-foreground">Required fields are relaxed — finish later</p>
            </div>
          </div>
        )}

        <div className="min-h-52">
          {/* ─── AI step ──────────────────────────────────────── */}
          {step === -1 && (
            <AiPhotoStep
              phase={aiPhase}
              photos={aiPhotos}
              setPhotos={setAiPhotos}
              filledName={name || "Samsung 65\" QLED TV"}
              filledBrand={brand || "Samsung"}
              filledModel={model || "QN65Q80CAFXZA"}
              filledSerial={serialNumber || "SN-SAM-2023-88421"}
            />
          )}

          {/* ─── Form steps ───────────────────────────────────── */}
          {step === 0 && (
            <BasicsStep
              name={name} setName={setName}
              shortName={shortName} setShortName={setShortName}
              brand={brand} setBrand={setBrand}
              model={model} setModel={setModel}
              category={category} setCategory={setCategory}
              selectedLocationId={selectedLocationId} onLocationChange={handleLocationChange}
              selectedAreaId={selectedAreaId} setSelectedAreaId={setSelectedAreaId}
              availableAreas={availableAreas}
              serialNumber={serialNumber} setSerialNumber={setSerialNumber}
              extraSerialNumbers={extraSerialNumbers} setExtraSerialNumbers={setExtraSerialNumbers}
              partNumbers={partNumbers} setPartNumbers={setPartNumbers}
              count={count} setCount={setCount}
              errors={basicsErrors}
              touch={touch}
            />
          )}
          {step === 1 && (
            <PurchaseStep
              purchasedAt={purchasedAt} setPurchasedAt={setPurchasedAt}
              purchasePrice={purchasePrice} setPurchasePrice={setPurchasePrice}
              purchaseCurrency={purchaseCurrency} setPurchaseCurrency={setPurchaseCurrency}
              currentValue={currentValue} setCurrentValue={setCurrentValue}
              convertedPrice={convertedPrice} setConvertedPrice={setConvertedPrice}
              isDraft={draft}
              errors={purchaseErrors}
              touch={touch}
            />
          )}
          {step === 2 && (
            <WarrantyStep
              warrantyExpiry={warrantyExpiry} setWarrantyExpiry={setWarrantyExpiry}
              warrantyNotes={warrantyNotes} setWarrantyNotes={setWarrantyNotes}
              errors={warrantyErrors}
              touch={touch}
            />
          )}
          {step === 3 && (
            <ExtrasStep
              notes={notes} setNotes={setNotes}
              tagInput={tagInput} setTagInput={setTagInput}
              tags={tags} setTags={setTags}
              addTag={addTag}
              supplyLinks={supplyLinks} setSupplyLinks={setSupplyLinks}
              urls={urls} setUrls={setUrls}
            />
          )}
          {step === 4 && (
            <FilesStep
              photoFiles={photoFiles} setPhotoFiles={setPhotoFiles}
              addPhotoFiles={(fs: File[]) => addFilesToList(fs, setPhotoFiles, "image")}
              receiptFiles={receiptFiles} setReceiptFiles={setReceiptFiles}
              addReceiptFiles={(fs: File[]) => addFilesToList(fs, setReceiptFiles, "invoice")}
              documentFiles={documentFiles} setDocumentFiles={setDocumentFiles}
              addDocumentFiles={(fs: File[]) => addFilesToList(fs, setDocumentFiles, "document")}
            />
          )}
        </div>

        {/* Server error banner — shown above the footer */}
        {serverError && (
          <ServerErrorBanner
            error={serverError}
            onRetry={isLast ? handleSave : undefined}
            onDismiss={() => setServerError(null)}
          />
        )}

        <Separator />

        <DialogFooter className="gap-2">
          <Button variant="ghost" onClick={handleClose} className="mr-auto" disabled={saving}>Cancel</Button>

          {/* AI step footer */}
          {step === -1 && aiPhase === "offer" && (
            <>
              <Button variant="outline" onClick={handleAiSkip}>
                Fill manually
              </Button>
              <Button
                onClick={handleAiScan}
                disabled={aiPhotos.length === 0}
                className="gap-1.5"
              >
                <Sparkles className="size-3.5" />
                Scan photos
              </Button>
            </>
          )}
          {step === -1 && aiPhase === "scanning" && (
            <Button disabled className="gap-1.5">
              <Sparkles className="size-3.5 animate-pulse" />
              Scanning…
            </Button>
          )}
          {step === -1 && aiPhase === "review" && (
            <>
              <Button variant="outline" onClick={handleAiSkip}>
                Start over manually
              </Button>
              <Button onClick={handleAiConfirm} className="gap-1.5">
                <CheckCircle2 className="size-3.5" />
                Looks good, continue
              </Button>
            </>
          )}

          {/* Form steps footer */}
          {isFormStep && (
            <>
              {step > 0 && (
                <Button variant="outline" onClick={() => { setServerError(null); setStep((s) => s - 1) }} disabled={saving}>
                  Back
                </Button>
              )}
              {isLast ? (
                <Button onClick={handleSave} disabled={saving}>
                  {saving ? (
                    <>
                      <RefreshCw className="size-3.5 animate-spin" />
                      Saving…
                    </>
                  ) : (
                    draft ? "Save Draft" : "Save Item"
                  )}
                </Button>
              ) : (
                <Button onClick={handleContinue}>Continue</Button>
              )}
            </>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

// ─── AI step ───────────────────────────────────────────────────

function AiPhotoStep({ phase, photos, setPhotos, filledName, filledBrand, filledModel, filledSerial }: {
  phase: AiPhase
  photos: { id: string; name: string; preview: string }[]
  setPhotos: React.Dispatch<React.SetStateAction<{ id: string; name: string; preview: string }[]>>
  filledName: string
  filledBrand: string
  filledModel: string
  filledSerial: string
}) {
  function handlePhotoFiles(files: File[]) {
    const previews = files.map((f) => ({
      id: makeId(),
      name: f.name,
      preview: URL.createObjectURL(f),
    }))
    setPhotos((prev) => [...prev, ...previews])
  }

  if (phase === "scanning") {
    return (
      <div className="flex flex-col items-center justify-center gap-4 py-10 text-center">
        <div className="relative flex size-14 items-center justify-center rounded-2xl bg-amber-500/10">
          <Sparkles className="size-7 text-amber-500 animate-pulse" />
        </div>
        <div>
          <p className="text-sm font-medium">Analysing your photos…</p>
          <p className="text-xs text-muted-foreground mt-1">Extracting brand, model, and serial number</p>
        </div>
        <div className="w-full max-w-48 h-1.5 rounded-full bg-muted overflow-hidden">
          <div className="h-full w-2/3 rounded-full bg-amber-500 animate-pulse" />
        </div>
      </div>
    )
  }

  if (phase === "review") {
    return (
      <div className="flex flex-col gap-4 py-2">
        <div className="flex items-center gap-2 rounded-lg bg-status-active/8 border border-status-active/20 px-3 py-2.5">
          <CheckCircle2 className="size-4 text-status-active shrink-0" />
          <p className="text-sm text-status-active font-medium">AI extracted the following — please review</p>
        </div>
        <div className="grid grid-cols-2 gap-3">
          {[
            { label: "Name", value: filledName },
            { label: "Brand", value: filledBrand },
            { label: "Model", value: filledModel },
            { label: "Serial number", value: filledSerial },
          ].map(({ label, value }) => (
            <div key={label} className="rounded-lg border border-border bg-muted/30 px-3 py-2.5">
              <p className="text-[10px] font-medium uppercase tracking-wide text-muted-foreground mb-0.5">{label}</p>
              <p className="text-sm font-medium truncate">{value}</p>
            </div>
          ))}
        </div>
        <div className="flex items-start gap-2 rounded-lg bg-muted/40 px-3 py-2.5">
          <AlertCircle className="size-3.5 text-muted-foreground mt-0.5 shrink-0" />
          <p className="text-xs text-muted-foreground">You can edit any of these fields in the next steps.</p>
        </div>
      </div>
    )
  }

  // phase === "offer"
  return (
    <div className="flex flex-col gap-4 py-2">
      <div className="grid grid-cols-2 gap-3">
        <div className="flex flex-col gap-2 rounded-xl border border-border bg-muted/20 p-3">
          <div className="flex size-8 items-center justify-center rounded-lg bg-primary/10">
            <Camera className="size-4 text-primary" />
          </div>
          <p className="text-xs font-semibold">Full item photo</p>
          <p className="text-[11px] text-muted-foreground leading-relaxed">Photograph the whole item clearly so AI can identify it</p>
        </div>
        <div className="flex flex-col gap-2 rounded-xl border border-border bg-muted/20 p-3">
          <div className="flex size-8 items-center justify-center rounded-lg bg-primary/10">
            <ScanText className="size-4 text-primary" />
          </div>
          <p className="text-xs font-semibold">Label / rating plate</p>
          <p className="text-[11px] text-muted-foreground leading-relaxed">Photograph the information label with model & serial numbers</p>
        </div>
      </div>
      <div>
        <div
          className={cn(
            "flex flex-col items-center justify-center gap-2 rounded-xl border-2 border-dashed py-6 cursor-pointer transition-colors",
            photos.length > 0
              ? "border-primary/40 bg-primary/5"
              : "border-border hover:border-primary/40 hover:bg-muted/30"
          )}
          onClick={() => document.getElementById("ai-photo-input")?.click()}
          onDragOver={(e) => e.preventDefault()}
          onDrop={(e) => { e.preventDefault(); handlePhotoFiles(Array.from(e.dataTransfer.files)) }}
        >
          {photos.length > 0 ? (
            <div className="flex flex-wrap gap-2 px-3 justify-center">
              {photos.map((p) => (
                <div key={p.id} className="relative group">
                  <img src={p.preview} alt={p.name} className="size-14 rounded-lg object-cover border border-border" />
                  <button
                    type="button"
                    className="absolute -top-1.5 -right-1.5 flex size-4 items-center justify-center rounded-full bg-background border border-border text-muted-foreground hover:text-foreground shadow-sm"
                    onClick={(e) => { e.stopPropagation(); setPhotos((prev) => prev.filter((x) => x.id !== p.id)) }}
                  >
                    <X className="size-2.5" />
                  </button>
                </div>
              ))}
              <button
                type="button"
                className="flex size-14 items-center justify-center rounded-lg border-2 border-dashed border-border bg-muted/30 text-muted-foreground hover:border-primary/40 hover:text-foreground transition-colors"
                onClick={(e) => { e.stopPropagation(); document.getElementById("ai-photo-input")?.click() }}
              >
                <Plus className="size-4" />
              </button>
            </div>
          ) : (
            <>
              <div className="flex size-10 items-center justify-center rounded-xl bg-amber-500/10">
                <Sparkles className="size-5 text-amber-500" />
              </div>
              <p className="text-sm text-muted-foreground">Drop photos here or <span className="text-foreground font-medium">browse</span></p>
              <p className="text-xs text-muted-foreground">JPG, PNG, HEIC — up to 5 photos</p>
            </>
          )}
          <input
            id="ai-photo-input"
            type="file"
            multiple
            accept="image/*"
            className="sr-only"
            onChange={(e) => { handlePhotoFiles(Array.from(e.target.files ?? [])); e.target.value = "" }}
          />
        </div>
      </div>
      {photos.length === 0 && (
        <p className="text-center text-xs text-muted-foreground">
          Add at least one photo to enable AI scanning, or tap <span className="font-medium text-foreground">Fill manually</span> below.
        </p>
      )}
    </div>
  )
}

// ─── StringArrayField ──────────────────────────────────────────

function StringArrayField({
  label, placeholder, hint, values, onChange,
}: {
  label: string; placeholder: string; hint?: string
  values: string[]; onChange: (v: string[]) => void
}) {
  const [input, setInput] = useState("")
  function add() {
    const v = input.trim()
    if (v && !values.includes(v)) onChange([...values, v])
    setInput("")
  }
  return (
    <div className="flex flex-col gap-1.5">
      <Label className="text-sm font-medium">{label}</Label>
      <div className="flex gap-2">
        <Input
          placeholder={placeholder}
          value={input}
          onChange={(e) => setInput(e.target.value)}
          onKeyDown={(e) => { if (e.key === "Enter") { e.preventDefault(); add() } }}
          className="flex-1 font-mono text-sm"
        />
        <Button type="button" variant="outline" size="sm" onClick={add} disabled={!input.trim()}>
          <Plus className="size-3.5" />
        </Button>
      </div>
      {hint && <p className="text-xs text-muted-foreground">{hint}</p>}
      {values.length > 0 && (
        <div className="flex flex-wrap gap-1">
          {values.map((v) => (
            <Badge key={v} variant="secondary" className="gap-1 h-5 text-xs font-mono">
              {v}
              <button type="button" onClick={() => onChange(values.filter((x) => x !== v))}>
                <X className="size-3" />
              </button>
            </Badge>
          ))}
        </div>
      )}
    </div>
  )
}

// ─── BrandInput with autocomplete ─────────────────────────────

function BrandInput({ value, onChange, hasError }: { value: string; onChange: (v: string) => void; hasError?: boolean }) {
  const [open, setOpen] = useState(false)
  const inputRef = React.useRef<HTMLInputElement>(null)

  const suggestions = value.trim().length > 0
    ? ALL_BRANDS.filter((b) => b.toLowerCase().startsWith(value.toLowerCase()) && b.toLowerCase() !== value.toLowerCase()).slice(0, 6)
    : []

  return (
    <div className="relative">
      <Input
        ref={inputRef}
        id="item-brand"
        placeholder="e.g. Miele"
        value={value}
        autoComplete="off"
        aria-invalid={hasError}
        onChange={(e) => { onChange(e.target.value); setOpen(true) }}
        onFocus={() => setOpen(true)}
        onBlur={() => setTimeout(() => setOpen(false), 150)}
      />
      {open && suggestions.length > 0 && (
        <div className="absolute z-50 top-full mt-1 w-full rounded-lg border border-border bg-popover shadow-md overflow-hidden">
          {suggestions.map((b) => (
            <button
              key={b}
              type="button"
              onMouseDown={(e) => e.preventDefault()}
              onClick={() => { onChange(b); setOpen(false); inputRef.current?.blur() }}
              className="w-full px-3 py-2 text-left text-sm hover:bg-accent hover:text-accent-foreground transition-colors"
            >
              {b}
            </button>
          ))}
        </div>
      )}
    </div>
  )
}

// ─── ModelInput with autocomplete ─────────────────────────────

function ModelInput({ value, onChange, brand }: { value: string; onChange: (v: string) => void; brand: string }) {
  const [open, setOpen] = useState(false)
  const inputRef = React.useRef<HTMLInputElement>(null)

  const brandModels = BRAND_MODELS[brand] ?? []
  const suggestions = brandModels.filter((m) =>
    value.trim().length === 0
      ? true
      : m.toLowerCase().includes(value.toLowerCase())
  ).slice(0, 5)

  return (
    <div className="relative">
      <Input
        ref={inputRef}
        id="item-model"
        placeholder={brand ? `e.g. ${(BRAND_MODELS[brand]?.[0] ?? "Model").split(" ")[0]}…` : "e.g. WCI 870"}
        value={value}
        autoComplete="off"
        onChange={(e) => { onChange(e.target.value); setOpen(true) }}
        onFocus={() => setOpen(true)}
        onBlur={() => setTimeout(() => setOpen(false), 150)}
      />
      {open && suggestions.length > 0 && (
        <div className="absolute z-50 top-full mt-1 w-full rounded-lg border border-border bg-popover shadow-md overflow-hidden">
          {suggestions.map((m) => (
            <button
              key={m}
              type="button"
              onMouseDown={(e) => e.preventDefault()}
              onClick={() => { onChange(m); setOpen(false); inputRef.current?.blur() }}
              className="w-full px-3 py-2 text-left text-sm hover:bg-accent hover:text-accent-foreground transition-colors"
            >
              {m}
            </button>
          ))}
        </div>
      )}
    </div>
  )
}

// ─── BasicsStep ────────────────────────────────────────────────

function BasicsStep({ name, setName, shortName, setShortName, brand, setBrand, model, setModel,
  category, setCategory, selectedLocationId, onLocationChange, selectedAreaId, setSelectedAreaId,
  availableAreas, serialNumber, setSerialNumber, extraSerialNumbers, setExtraSerialNumbers,
  partNumbers, setPartNumbers, count, setCount,
  errors, touch,
}: any) {
  const [showExtraSerials, setShowExtraSerials] = useState(extraSerialNumbers.length > 0)
  const [showPartNumbers, setShowPartNumbers] = useState(partNumbers.length > 0)

  return (
    <div className="space-y-4 py-2">
      <F label="Item Name" htmlFor="item-name" required error={errors.name}>
        <Input
          id="item-name"
          placeholder="e.g. Washing Machine"
          value={name}
          onChange={(e) => setName(e.target.value)}
          onBlur={() => touch("name")}
          autoFocus
          aria-invalid={!!errors.name}
          className={cn(errors.name && "border-destructive focus-visible:ring-destructive/20")}
        />
      </F>

      <F label="Short Name" htmlFor="item-short" error={errors.shortName}
        hint={!errors.shortName ? "1–20 chars, used in compact lists and labels" : undefined}>
        <Input
          id="item-short"
          placeholder="e.g. Washer"
          maxLength={22}
          value={shortName}
          onChange={(e) => setShortName(e.target.value)}
          onBlur={() => touch("shortName")}
          aria-invalid={!!errors.shortName}
          className={cn("font-mono text-sm", errors.shortName && "border-destructive focus-visible:ring-destructive/20")}
        />
      </F>

      <div className="grid grid-cols-2 gap-3">
        <F label="Brand" htmlFor="item-brand">
          <BrandInput value={brand} onChange={setBrand} />
        </F>
        <F label="Model" htmlFor="item-model">
          <ModelInput value={model} onChange={setModel} brand={brand} />
        </F>
      </div>

      <div className="grid grid-cols-2 gap-3">
        <F label="Category" htmlFor="item-category">
          <Select value={category} onValueChange={(v) => setCategory(v as ItemCategory)}>
            <SelectTrigger id="item-category"><SelectValue placeholder="Select…" /></SelectTrigger>
            <SelectContent>
              {CATEGORIES.map((c) => <SelectItem key={c.value} value={c.value}>{c.label}</SelectItem>)}
            </SelectContent>
          </Select>
        </F>
        <F label="Quantity" htmlFor="item-count" error={errors.count}>
          <Input
            id="item-count"
            type="number"
            min="1"
            value={count}
            onChange={(e) => setCount(e.target.value)}
            onBlur={() => touch("count")}
            aria-invalid={!!errors.count}
            className={cn(errors.count && "border-destructive focus-visible:ring-destructive/20")}
          />
        </F>
      </div>

      <div className="grid grid-cols-2 gap-3">
        <F label="Location" htmlFor="item-location">
          <Select value={selectedLocationId} onValueChange={onLocationChange}>
            <SelectTrigger id="item-location"><SelectValue placeholder="Select…" /></SelectTrigger>
            <SelectContent>
              {MOCK_LOCATIONS.map((l) => <SelectItem key={l.id} value={l.id}>{l.icon} {l.name}</SelectItem>)}
            </SelectContent>
          </Select>
        </F>
        <F label="Area" htmlFor="item-area">
          <Select value={selectedAreaId} onValueChange={setSelectedAreaId} disabled={!selectedLocationId}>
            <SelectTrigger id="item-area"><SelectValue placeholder={selectedLocationId ? "Select…" : "Pick location first"} /></SelectTrigger>
            <SelectContent>
              {availableAreas.map((a: any) => <SelectItem key={a.id} value={a.id}>{a.icon} {a.name}</SelectItem>)}
            </SelectContent>
          </Select>
        </F>
      </div>

      <F label="Serial Number" htmlFor="item-serial" hint="Found on device label or packaging">
        <Input
          id="item-serial"
          placeholder="e.g. SN-MEL-2022-00412"
          className="font-mono text-sm"
          value={serialNumber}
          onChange={(e) => setSerialNumber(e.target.value)}
        />
      </F>

      {!showExtraSerials ? (
        <button
          type="button"
          onClick={() => setShowExtraSerials(true)}
          className="flex items-center gap-1 text-xs text-muted-foreground hover:text-foreground transition-colors"
        >
          <ChevronDown className="size-3.5" />
          This item has multiple serial numbers
        </button>
      ) : (
        <StringArrayField
          label="Additional Serial Numbers"
          placeholder="Enter serial, press Enter"
          hint="For multi-unit or component serials"
          values={extraSerialNumbers}
          onChange={setExtraSerialNumbers}
        />
      )}

      {!showPartNumbers ? (
        <button
          type="button"
          onClick={() => setShowPartNumbers(true)}
          className="flex items-center gap-1 text-xs text-muted-foreground hover:text-foreground transition-colors"
        >
          <ChevronDown className="size-3.5" />
          Add part numbers
        </button>
      ) : (
        <StringArrayField
          label="Part Numbers"
          placeholder="Enter part number, press Enter"
          hint="Manufacturer part reference codes"
          values={partNumbers}
          onChange={setPartNumbers}
        />
      )}
    </div>
  )
}

// ─── PurchaseStep ──────────────────────────────────────────────

const GROUP_CURRENCY = "USD"

function PurchaseStep({ purchasedAt, setPurchasedAt, purchasePrice, setPurchasePrice,
  purchaseCurrency, setPurchaseCurrency, currentValue, setCurrentValue,
  convertedPrice, setConvertedPrice, isDraft,
  errors, touch,
}: any) {
  const currencySymbol = CURRENCIES.find((c) => c.code === purchaseCurrency)?.symbol ?? "$"
  const isForeignCurrency = purchaseCurrency !== GROUP_CURRENCY

  return (
    <div className="space-y-4 py-2">
      <F label="Purchase Date" htmlFor="item-purchased-at"
        required={!isDraft}
        error={errors.purchasedAt}
        hint={!errors.purchasedAt && !isDraft ? "Required for non-draft items" : undefined}>
        <Input
          id="item-purchased-at"
          type="date"
          value={purchasedAt}
          onChange={(e) => setPurchasedAt(e.target.value)}
          onBlur={() => touch("purchasedAt")}
          aria-invalid={!!errors.purchasedAt}
          className={cn(errors.purchasedAt && "border-destructive focus-visible:ring-destructive/20")}
        />
      </F>

      <div className="flex flex-col gap-1.5">
        <Label className="text-sm font-medium">Purchase Price</Label>
        <div className="flex gap-2">
          <div className="relative flex-1">
            <span className="absolute left-3 top-1/2 -translate-y-1/2 text-sm text-muted-foreground select-none">{currencySymbol}</span>
            <Input
              type="number"
              min="0"
              placeholder="0"
              className={cn("pl-6", errors.purchasePrice && "border-destructive focus-visible:ring-destructive/20")}
              value={purchasePrice}
              onChange={(e) => setPurchasePrice(e.target.value)}
              onBlur={() => touch("purchasePrice")}
              aria-invalid={!!errors.purchasePrice}
            />
          </div>
          <CurrencyCombobox value={purchaseCurrency} onValueChange={setPurchaseCurrency} variant="compact" />
        </div>
        {errors.purchasePrice ? (
          <FieldError message={errors.purchasePrice} />
        ) : (
          <p className="text-xs text-muted-foreground">Original purchase currency</p>
        )}
      </div>

      {isForeignCurrency && (
        <div className="flex flex-col gap-3 rounded-lg border border-amber-200 bg-amber-50 dark:border-amber-900/60 dark:bg-amber-950/30 p-3">
          <p className="text-xs text-amber-800 dark:text-amber-300 leading-relaxed">
            <span className="font-semibold">Foreign currency detected.</span> The group uses {GROUP_CURRENCY}. Enter at least one of the fields below so the item value can be used in reports.
          </p>
          <F label={`Converted Purchase Price (${GROUP_CURRENCY})`} htmlFor="item-converted-price" hint="What you paid, expressed in the group currency">
            <div className="relative">
              <span className="absolute left-3 top-1/2 -translate-y-1/2 text-sm text-muted-foreground select-none">$</span>
              <Input id="item-converted-price" type="number" min="0" placeholder="0" className="pl-6 bg-background"
                value={convertedPrice} onChange={(e) => setConvertedPrice(e.target.value)} />
            </div>
          </F>
          <div className="flex items-center gap-2">
            <div className="h-px flex-1 bg-amber-200 dark:bg-amber-900/60" />
            <span className="text-[10px] font-medium text-amber-600 dark:text-amber-400 uppercase tracking-wide">or</span>
            <div className="h-px flex-1 bg-amber-200 dark:bg-amber-900/60" />
          </div>
          <F label={`Current Value (${GROUP_CURRENCY})`} htmlFor="item-value-foreign" hint="Current resale / insurance value in group currency">
            <div className="relative">
              <span className="absolute left-3 top-1/2 -translate-y-1/2 text-sm text-muted-foreground select-none">$</span>
              <Input id="item-value-foreign" type="number" min="0" placeholder="0" className="pl-6 bg-background"
                value={currentValue} onChange={(e) => setCurrentValue(e.target.value)} />
            </div>
          </F>
        </div>
      )}

      {!isForeignCurrency && (
        <F label="Current Value" htmlFor="item-value" hint="For insurance purposes — in group currency">
          <div className="relative">
            <span className="absolute left-3 top-1/2 -translate-y-1/2 text-sm text-muted-foreground select-none">$</span>
            <Input id="item-value" type="number" min="0" placeholder="0" className="pl-6"
              value={currentValue} onChange={(e) => setCurrentValue(e.target.value)} />
          </div>
        </F>
      )}
    </div>
  )
}

// ─── WarrantyStep ──────────────────────────────────────────────

function WarrantyStep({ warrantyExpiry, setWarrantyExpiry, warrantyNotes, setWarrantyNotes, errors, touch }: any) {
  return (
    <div className="space-y-4 py-2">
      <F label="Warranty Expiry Date" htmlFor="item-warranty-exp"
        error={errors.warrantyExpiry}
        hint={!errors.warrantyExpiry ? "Leave blank if no warranty" : undefined}>
        <Input
          id="item-warranty-exp"
          type="date"
          value={warrantyExpiry}
          onChange={(e) => setWarrantyExpiry(e.target.value)}
          onBlur={() => touch("warrantyExpiry")}
          aria-invalid={!!errors.warrantyExpiry}
          className={cn(errors.warrantyExpiry && "border-destructive focus-visible:ring-destructive/20")}
        />
      </F>
      <F label="Warranty Notes" htmlFor="item-warranty-notes">
        <Textarea
          id="item-warranty-notes"
          placeholder="Registration number, extended plan, contact…"
          rows={3}
          className="resize-none"
          value={warrantyNotes}
          onChange={(e) => setWarrantyNotes(e.target.value)}
        />
      </F>
    </div>
  )
}

// ─── ExtrasStep ────────────────────────────────────────────────

function ExtrasStep({ notes, setNotes, tagInput, setTagInput, tags, setTags, addTag,
  supplyLinks, setSupplyLinks, urls, setUrls,
}: any) {
  return (
    <div className="space-y-4 py-2">
      <F label="Notes" htmlFor="item-notes">
        <Textarea id="item-notes" placeholder="Maintenance tips, filter models, anything useful…"
          rows={3} className="resize-none" value={notes} onChange={(e) => setNotes(e.target.value)} />
      </F>

      <div className="flex flex-col gap-1.5">
        <Label>Tags</Label>
        <div className="flex gap-2">
          <Input
            placeholder="e.g. kitchen, samsung — comma to add"
            value={tagInput}
            onChange={(e) => setTagInput(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter" || e.key === ",") { e.preventDefault(); addTag(tagInput) }
            }}
          />
          <Button type="button" variant="outline" size="sm" onClick={() => addTag(tagInput)} disabled={!tagInput.trim()}>Add</Button>
        </div>
        {tags.length > 0 && (
          <div className="flex flex-wrap gap-1 mt-1">
            {tags.map((t: string) => (
              <Badge key={t} variant="secondary" className="gap-1 h-5 text-xs">
                {t}
                <button type="button" onClick={() => setTags((prev: string[]) => prev.filter((x) => x !== t))}>
                  <X className="size-3" />
                </button>
              </Badge>
            ))}
          </div>
        )}
      </div>

      <div className="flex flex-col gap-1.5">
        <Label className="flex items-center justify-between">
          Product URLs
          <button
            type="button"
            className="text-xs text-muted-foreground hover:text-foreground flex items-center gap-1"
            onClick={() => setUrls((prev: any[]) => [...prev, { id: makeId(), label: "", url: "" }])}
          >
            <Plus className="size-3" />Add
          </button>
        </Label>
        {urls.length === 0 && (
          <p className="text-xs text-muted-foreground">Product page, support, documentation links</p>
        )}
        <div className="flex flex-col gap-2">
          {urls.map((link: any) => (
            <div key={link.id} className="flex gap-2 items-center">
              <Input placeholder="Label" className="w-28 shrink-0 text-sm"
                value={link.label}
                onChange={(e) => setUrls((prev: any[]) => prev.map((l) => l.id === link.id ? { ...l, label: e.target.value } : l))} />
              <Input placeholder="https://…" className="flex-1 text-sm"
                value={link.url}
                onChange={(e) => setUrls((prev: any[]) => prev.map((l) => l.id === link.id ? { ...l, url: e.target.value } : l))} />
              <button type="button" onClick={() => setUrls((prev: any[]) => prev.filter((l) => l.id !== link.id))}
                className="text-muted-foreground hover:text-foreground shrink-0">
                <X className="size-4" />
              </button>
            </div>
          ))}
        </div>
      </div>

      <div className="flex flex-col gap-1.5">
        <Label className="flex items-center justify-between">
          Supply Links
          <button
            type="button"
            className="text-xs text-muted-foreground hover:text-foreground flex items-center gap-1"
            onClick={() => setSupplyLinks((prev: any[]) => [...prev, { id: makeId(), label: "", url: "" }])}
          >
            <Plus className="size-3" />Add
          </button>
        </Label>
        {supplyLinks.length === 0 && (
          <p className="text-xs text-muted-foreground">Where to re-buy consumables, filters, accessories</p>
        )}
        <div className="flex flex-col gap-2">
          {supplyLinks.map((link: any) => (
            <div key={link.id} className="flex gap-2 items-center">
              <Input placeholder="Label (e.g. Water Filter)" className="w-28 shrink-0 text-sm"
                value={link.label}
                onChange={(e) => setSupplyLinks((prev: any[]) => prev.map((l) => l.id === link.id ? { ...l, label: e.target.value } : l))} />
              <Input placeholder="https://…" className="flex-1 text-sm"
                value={link.url}
                onChange={(e) => setSupplyLinks((prev: any[]) => prev.map((l) => l.id === link.id ? { ...l, url: e.target.value } : l))} />
              <button type="button" onClick={() => setSupplyLinks((prev: any[]) => prev.filter((l) => l.id !== link.id))}
                className="text-muted-foreground hover:text-foreground shrink-0">
                <X className="size-4" />
              </button>
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}

// ─── FilesStep ─────────────────────────────────────────────────

function FilesStep({ photoFiles, setPhotoFiles, addPhotoFiles,
  receiptFiles, setReceiptFiles, addReceiptFiles,
  documentFiles, setDocumentFiles, addDocumentFiles,
}: any) {
  return (
    <div className="space-y-3 py-2">
      <p className="text-xs text-muted-foreground">Attach photos, purchase receipts, and documents. All fields are optional.</p>

      <div className="rounded-xl border border-border overflow-hidden">
        <div className="flex items-center gap-2.5 px-3 py-2.5 bg-muted/40">
          <div className="flex size-6 items-center justify-center rounded-md bg-status-active/15">
            <ImageIcon className="size-3.5 text-status-active" />
          </div>
          <div className="flex-1">
            <p className="text-xs font-semibold text-foreground">Photos</p>
            <p className="text-[10px] text-muted-foreground leading-tight">Item images — displayed on cards &amp; in gallery</p>
          </div>
          {photoFiles.length > 0 && (
            <span className="text-xs text-muted-foreground">{photoFiles.length} file{photoFiles.length !== 1 ? "s" : ""}</span>
          )}
        </div>
        <div className="p-3">
          <FileDropZone files={photoFiles} setFiles={setPhotoFiles} addFiles={addPhotoFiles}
            inputId="photo-file-input" accept="image/*" emptyHint="JPG, PNG, WebP, HEIC" />
        </div>
      </div>

      <div className="rounded-xl border border-border overflow-hidden">
        <div className="flex items-center gap-2.5 px-3 py-2.5 bg-muted/40">
          <div className="flex size-6 items-center justify-center rounded-md bg-chart-1/15">
            <Receipt className="size-3.5 text-chart-1" />
          </div>
          <div className="flex-1">
            <p className="text-xs font-semibold text-foreground">Receipts &amp; Invoices</p>
            <p className="text-[10px] text-muted-foreground leading-tight">Purchase receipts — surfaced in insurance &amp; price reports</p>
          </div>
          {receiptFiles.length > 0 && (
            <span className="text-xs text-muted-foreground">{receiptFiles.length} file{receiptFiles.length !== 1 ? "s" : ""}</span>
          )}
        </div>
        <div className="p-3">
          <FileDropZone files={receiptFiles} setFiles={setReceiptFiles} addFiles={addReceiptFiles}
            inputId="receipt-file-input" accept="application/pdf,image/*" emptyHint="PDF or image of the receipt" />
        </div>
      </div>

      <div className="rounded-xl border border-border overflow-hidden">
        <div className="flex items-center gap-2.5 px-3 py-2.5 bg-muted/40">
          <div className="flex size-6 items-center justify-center rounded-md bg-chart-3/15">
            <BookOpen className="size-3.5 text-chart-3" />
          </div>
          <div className="flex-1">
            <p className="text-xs font-semibold text-foreground">Documents</p>
            <p className="text-[10px] text-muted-foreground leading-tight">Manuals, warranties, certificates, guides</p>
          </div>
          {documentFiles.length > 0 && (
            <span className="text-xs text-muted-foreground">{documentFiles.length} file{documentFiles.length !== 1 ? "s" : ""}</span>
          )}
        </div>
        <div className="p-3">
          <FileDropZone files={documentFiles} setFiles={setDocumentFiles} addFiles={addDocumentFiles}
            inputId="document-file-input" accept=".pdf,.doc,.docx,application/pdf" emptyHint="PDF, Word, or plain text" />
        </div>
      </div>
    </div>
  )
}

// ─── FileDropZone ──────────────────────────────────────────────

function fileIcon(mimeType: string) {
  if (mimeType === "application/pdf") return <FileText className="size-4 text-status-expired" />
  if (mimeType.startsWith("image/")) return <Image className="size-4 text-status-active" />
  return <File className="size-4 text-muted-foreground" />
}

function FileDropZone({ files, setFiles, addFiles, inputId, accept = "*", emptyHint = "PDFs, images, documents" }: {
  files: AttachedFileDraft[]
  setFiles: React.Dispatch<React.SetStateAction<AttachedFileDraft[]>>
  addFiles: (fs: File[]) => void
  inputId: string
  accept?: string
  emptyHint?: string
}) {
  const [dragging, setDragging] = useState(false)
  return (
    <div className="flex flex-col gap-2">
      <div
        className={cn(
          "flex flex-col items-center justify-center gap-1.5 rounded-lg border-2 border-dashed py-4 cursor-pointer transition-colors",
          dragging ? "border-primary bg-primary/5" : "border-border hover:border-primary/40 hover:bg-muted/30"
        )}
        onDragOver={(e) => { e.preventDefault(); setDragging(true) }}
        onDragLeave={() => setDragging(false)}
        onDrop={(e) => { e.preventDefault(); setDragging(false); addFiles(Array.from(e.dataTransfer.files)) }}
        onClick={() => document.getElementById(inputId)?.click()}
      >
        <Upload className="size-4 text-muted-foreground" />
        <p className="text-sm text-muted-foreground">Drop here or <span className="text-foreground font-medium">browse</span></p>
        <p className="text-xs text-muted-foreground">{emptyHint}</p>
        <input id={inputId} type="file" multiple accept={accept} className="sr-only"
          onChange={(e) => { addFiles(Array.from(e.target.files ?? [])); e.target.value = "" }} />
      </div>
      {files.length > 0 && (
        <ul className="flex flex-col gap-1">
          {files.map((f) => (
            <li key={f.id} className="flex items-center gap-2 rounded-lg border border-border bg-card px-3 py-2">
              {fileIcon(f.mimeType)}
              <span className="flex-1 text-sm truncate min-w-0">{f.name}</span>
              <span className="text-xs text-muted-foreground shrink-0">{f.size}</span>
              <button type="button" onClick={() => setFiles((prev) => prev.filter((x) => x.id !== f.id))}
                className="text-muted-foreground hover:text-foreground shrink-0">
                <X className="size-3.5" />
              </button>
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}

function formatFileSize(bytes: number) {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
}

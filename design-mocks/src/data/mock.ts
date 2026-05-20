export type WarrantyStatus = "active" | "expiring" | "expired" | "none"
export type ItemCategory =
  | "appliance"
  | "electronics"
  | "tool"
  | "furniture"
  | "vehicle"
  | "other"

export type CommodityStatus = "in_use" | "sold" | "lost" | "disposed" | "written_off"

export type MemberRole = "viewer" | "user" | "admin" | "owner"

export interface Member {
  id: string
  name: string
  email: string
  role: MemberRole
  joinedAt: string
  avatarInitials: string
}

export interface LocationGroup {
  id: string
  name: string
  description: string
  members: Member[]
  createdAt: string
}

export interface Location {
  id: string
  groupId: string
  name: string
  icon: string
  description: string
}

export interface Area {
  id: string
  locationId: string
  name: string
  icon: string
}

// Unified tag entity — used for both inventory items and attached files.
// `color` is a Tailwind text-color class from the design token palette.
export interface Tag {
  id: string
  label: string
  color: string   // e.g. "text-chart-1"
  bg: string      // e.g. "bg-chart-1/15"
  border: string  // e.g. "border-chart-1/30"
}

/** @deprecated Use Tag instead */
export type FileTag = Tag

export type FileCategory = "image" | "invoice" | "document" | "other"

export const FILE_CATEGORY_CONFIG: Record<FileCategory, { label: string; plural: string; accept: string }> = {
  image:    { label: "Photo",    plural: "Photos",    accept: "image/*" },
  invoice:  { label: "Invoice",  plural: "Invoices",  accept: "application/pdf,image/*" },
  document: { label: "Document", plural: "Documents", accept: ".pdf,.doc,.docx,.txt,application/pdf" },
  other:    { label: "Other",    plural: "Other",     accept: "*" },
}

export interface AttachedFile {
  id: string
  name: string
  size: string
  uploadedAt: string
  attachedTo: { type: "location" | "commodity"; id: string; name: string }
  tags: string[]
  mimeType: string
  category: FileCategory
  thumbnailUrl?: string
}

export interface WarrantyInfo {
  expiresAt: string | null
  receiptUrl: string | null
  notes: string
}

export interface SupplyLink {
  label: string
  url: string
}

export interface InventoryItem {
  id: string
  name: string
  shortName?: string
  brand: string
  model: string
  category: ItemCategory
  status: CommodityStatus
  draft: boolean
  count: number
  areaId: string
  purchasedAt: string | null
  purchasePrice: number | null
  purchaseCurrency: string
  currentValue: number | null
  serialNumber: string
  extraSerialNumbers: string[]
  partNumbers: string[]
  urls: { label: string; url: string }[]
  warranty: WarrantyInfo
  supplyLinks: SupplyLink[]
  notes: string
  imageUrl: string | null
  tags: string[]
  statusNote?: string
  statusDate?: string
  salePrice?: number | null
}

// ─── time helpers ────────────────────────────────────────────
const today = new Date()
const daysFromNow = (d: number) => {
  const dt = new Date(today)
  dt.setDate(dt.getDate() + d)
  return dt.toISOString().split("T")[0]
}

export function warrantyStatus(item: InventoryItem): WarrantyStatus {
  if (!item.warranty.expiresAt) return "none"
  const exp = new Date(item.warranty.expiresAt)
  const diff = (exp.getTime() - today.getTime()) / (1000 * 60 * 60 * 24)
  if (diff < 0) return "expired"
  if (diff <= 60) return "expiring"
  return "active"
}

// ─── Location Groups ──────────────────────────────────────────
export const MOCK_GROUPS: LocationGroup[] = [
  {
    id: "g1",
    name: "Main Residence",
    description: "Primary home at 14 Oak Street",
    createdAt: "2023-01-15",
    members: [
      { id: "u1", name: "Alex Johnson", email: "alex@example.com", role: "owner", joinedAt: "2023-01-15", avatarInitials: "AJ" },
      { id: "u2", name: "Sam Carter", email: "sam@example.com", role: "user", joinedAt: "2023-03-20", avatarInitials: "SC" },
    ],
  },
  {
    id: "g2",
    name: "Country Cottage",
    description: "Weekend retreat in the countryside",
    createdAt: "2024-05-10",
    members: [
      { id: "u1", name: "Alex Johnson", email: "alex@example.com", role: "owner", joinedAt: "2024-05-10", avatarInitials: "AJ" },
    ],
  },
]

// ─── Locations ────────────────────────────────────────────────
export const MOCK_LOCATIONS: Location[] = [
  { id: "loc1", groupId: "g1", name: "Main House", icon: "🏠", description: "Primary dwelling" },
  { id: "loc2", groupId: "g1", name: "Garage", icon: "🚗", description: "Detached garage & workshop" },
  { id: "loc3", groupId: "g2", name: "Cottage", icon: "🌿", description: "Weekend house" },
]

// ─── Areas ───────────────────────────────────────────────────
export const MOCK_AREAS: Area[] = [
  { id: "a1", locationId: "loc1", name: "Kitchen", icon: "🍳" },
  { id: "a2", locationId: "loc1", name: "Living Room", icon: "🛋️" },
  { id: "a3", locationId: "loc1", name: "Home Office", icon: "💼" },
  { id: "a4", locationId: "loc1", name: "Laundry Room", icon: "🧺" },
  { id: "a5", locationId: "loc1", name: "Utility Closet", icon: "🪣" },
  { id: "a6", locationId: "loc2", name: "Workshop", icon: "🔧" },
  { id: "a7", locationId: "loc2", name: "Storage", icon: "📦" },
  { id: "a8", locationId: "loc3", name: "Living Area", icon: "🛋️" },
]

// ─── Commodities (was InventoryItem) ─────────────────────────
export const MOCK_ITEMS: InventoryItem[] = [
  {
    id: "1",
    name: "Washing Machine",
    shortName: "Washer",
    brand: "Miele",
    model: "WCI 870 WPS",
    category: "appliance",
    status: "in_use",
    draft: false,
    count: 1,
    areaId: "a4",
    purchasedAt: "2022-03-15",
    purchasePrice: 1299,
    purchaseCurrency: "USD",
    currentValue: 950,
    serialNumber: "SN-MEL-2022-00412",
    extraSerialNumbers: [],
    partNumbers: [],
    urls: [],
    warranty: { expiresAt: daysFromNow(40), receiptUrl: null, notes: "5-year extended warranty purchased at checkout." },
    supplyLinks: [{ label: "Descaler tablets", url: "#" }, { label: "Drum cleaner", url: "#" }],
    notes: "Uses Miele UltraTabs. Run maintenance cycle monthly.",
    imageUrl: null,
    tags: ["i1", "i2"],
  },
  {
    id: "2",
    name: 'MacBook Pro 16"',
    shortName: "MacBook",
    brand: "Apple",
    model: "MK183LL/A",
    category: "electronics",
    status: "in_use",
    draft: false,
    count: 1,
    areaId: "a3",
    purchasedAt: "2021-11-08",
    purchasePrice: 2499,
    purchaseCurrency: "USD",
    currentValue: 1400,
    serialNumber: "C02G9...",
    extraSerialNumbers: [],
    partNumbers: ["MK183LL/A"],
    urls: [{ label: "Apple Support", url: "#" }],
    warranty: { expiresAt: daysFromNow(-90), receiptUrl: null, notes: "AppleCare+ expired." },
    supplyLinks: [{ label: "USB-C charger 140W", url: "#" }, { label: "Screen cleaner kit", url: "#" }],
    notes: "M1 Pro chip. 16GB RAM / 1TB SSD.",
    imageUrl: null,
    tags: ["i3", "i4"],
  },
  {
    id: "3",
    name: "Refrigerator",
    shortName: "Fridge",
    brand: "Samsung",
    model: "RF23M8590SG",
    category: "appliance",
    status: "in_use",
    draft: false,
    count: 1,
    areaId: "a1",
    purchasedAt: "2020-06-20",
    purchasePrice: 1799,
    purchaseCurrency: "USD",
    currentValue: 1100,
    serialNumber: "05TZ3CFK800123",
    extraSerialNumbers: [],
    partNumbers: [],
    urls: [],
    warranty: { expiresAt: daysFromNow(420), receiptUrl: null, notes: "" },
    supplyLinks: [{ label: "Water filter DA29-00020B", url: "#" }, { label: "Ice maker assembly", url: "#" }],
    notes: "Change water filter every 6 months.",
    imageUrl: null,
    tags: ["i5", "i6"],
  },
  {
    id: "4",
    name: "Dyson V15 Detect",
    shortName: "Dyson V15",
    brand: "Dyson",
    model: "V15 Detect",
    category: "appliance",
    status: "in_use",
    draft: false,
    count: 1,
    areaId: "a5",
    purchasedAt: "2023-01-10",
    purchasePrice: 649,
    purchaseCurrency: "USD",
    currentValue: 500,
    serialNumber: "DYS-V15-00981",
    extraSerialNumbers: [],
    partNumbers: [],
    urls: [],
    warranty: { expiresAt: daysFromNow(310), receiptUrl: null, notes: "2-year Dyson warranty." },
    supplyLinks: [{ label: "HEPA filter", url: "#" }, { label: "Replacement battery", url: "#" }],
    notes: "Clean filter every 3 months.",
    imageUrl: null,
    tags: ["i7"],
  },
  {
    id: "5",
    name: "Sony WH-1000XM5",
    shortName: "Sony WH5",
    brand: "Sony",
    model: "WH-1000XM5",
    category: "electronics",
    status: "sold",
    draft: false,
    count: 1,
    areaId: "a3",
    purchasedAt: "2022-09-01",
    purchasePrice: 349,
    purchaseCurrency: "USD",
    currentValue: 220,
    serialNumber: "SNY-WH5-22-4419",
    extraSerialNumbers: [],
    partNumbers: [],
    urls: [],
    warranty: { expiresAt: daysFromNow(-300), receiptUrl: null, notes: "" },
    supplyLinks: [{ label: "Replacement ear cushions", url: "#" }, { label: "USB-C cable", url: "#" }],
    notes: "Best noise-cancelling on the market.",
    imageUrl: null,
    tags: ["i8", "i9"],
    statusNote: "Sold on eBay",
    statusDate: "2026-02-14",
    salePrice: 210,
  },
  {
    id: "6",
    name: "Bosch Dishwasher",
    shortName: "Dishwasher",
    brand: "Bosch",
    model: "SHPM88Z75N",
    category: "appliance",
    status: "in_use",
    draft: false,
    count: 1,
    areaId: "a1",
    purchasedAt: "2021-04-12",
    purchasePrice: 1049,
    purchaseCurrency: "USD",
    currentValue: 750,
    serialNumber: "BSH-2021-DW-88721",
    extraSerialNumbers: [],
    partNumbers: [],
    urls: [],
    warranty: { expiresAt: daysFromNow(55), receiptUrl: null, notes: "Bosch 2+1 extended plan active." },
    supplyLinks: [{ label: "Finish Quantum pods", url: "#" }, { label: "Dishwasher cleaner", url: "#" }],
    notes: "Run dishwasher cleaner monthly.",
    imageUrl: null,
    tags: ["i5", "i10"],
  },
  {
    id: "7",
    name: "4K TV",
    shortName: "LG OLED",
    brand: "LG",
    model: "OLED65C2PSA",
    category: "electronics",
    status: "in_use",
    draft: false,
    count: 1,
    areaId: "a2",
    purchasedAt: "2022-12-26",
    purchasePrice: 1799,
    purchaseCurrency: "USD",
    currentValue: 1300,
    serialNumber: "LG-OLED-22-00517",
    extraSerialNumbers: [],
    partNumbers: [],
    urls: [{ label: "LG Support", url: "#" }],
    warranty: { expiresAt: daysFromNow(600), receiptUrl: null, notes: "" },
    supplyLinks: [{ label: "HDMI 2.1 cable", url: "#" }],
    notes: '65" OLED, G-Sync Compatible.',
    imageUrl: null,
    tags: ["i11", "i12"],
  },
  {
    id: "8",
    name: "DeWalt Drill",
    shortName: "Drill",
    brand: "DeWalt",
    model: "DCD791D2",
    category: "tool",
    status: "in_use",
    draft: false,
    count: 1,
    areaId: "a6",
    purchasedAt: "2019-08-14",
    purchasePrice: 199,
    purchaseCurrency: "USD",
    currentValue: 120,
    serialNumber: "DWL-2019-00029",
    extraSerialNumbers: ["DWL-2019-00029-B"],
    partNumbers: ["DCD791D2"],
    urls: [],
    warranty: { expiresAt: null, receiptUrl: null, notes: "Warranty expired." },
    supplyLinks: [{ label: "20V MAX battery", url: "#" }, { label: "Drill bit set", url: "#" }],
    notes: "Left battery needs replacement.",
    imageUrl: null,
    tags: ["i13", "i14"],
  },
  {
    id: "9",
    name: "Circular Saw",
    shortName: "Circ Saw",
    brand: "Makita",
    model: "HS7601",
    category: "tool",
    status: "in_use",
    draft: true,
    count: 1,
    areaId: "a6",
    purchasedAt: null,
    purchasePrice: 149,
    purchaseCurrency: "USD",
    currentValue: 100,
    serialNumber: "MAK-SAW-21-441",
    extraSerialNumbers: [],
    partNumbers: [],
    urls: [],
    warranty: { expiresAt: daysFromNow(180), receiptUrl: null, notes: "1-year Makita warranty." },
    supplyLinks: [{ label: "Replacement blade", url: "#" }],
    notes: "Use only 165mm blades.",
    imageUrl: null,
    tags: ["i13", "i15", "i7"],
  },
  {
    id: "10",
    name: "Nespresso Vertuo",
    shortName: "Coffee Maker",
    brand: "Nespresso",
    model: "Vertuo Next",
    category: "appliance",
    status: "in_use",
    draft: false,
    count: 1,
    areaId: "a1",
    purchasedAt: "2023-09-01",
    purchasePrice: 199,
    purchaseCurrency: "USD",
    currentValue: 150,
    serialNumber: "NES-VN-23-009",
    extraSerialNumbers: [],
    partNumbers: [],
    urls: [],
    warranty: { expiresAt: daysFromNow(300), receiptUrl: null, notes: "2-year Nespresso warranty." },
    supplyLinks: [{ label: "Vertuo pods", url: "#" }],
    notes: "Descale every 3 months.",
    imageUrl: null,
    tags: ["i5", "i7", "i2"],
  },
  {
    id: "11",
    name: "Bose QuietComfort 45",
    shortName: "BQ45",
    brand: "Bose",
    model: "QC45",
    category: "electronics",
    status: "in_use",
    draft: false,
    count: 1,
    areaId: "a3",
    purchasedAt: "2022-07-15",
    purchasePrice: 329,
    purchaseCurrency: "USD",
    currentValue: 200,
    serialNumber: "BOSE-QC45-22-887",
    extraSerialNumbers: [],
    partNumbers: [],
    urls: [],
    warranty: { expiresAt: daysFromNow(-90), receiptUrl: null, notes: "1-year Bose warranty." },
    supplyLinks: [{ label: "Replacement ear cushions", url: "#" }],
    notes: "Charge every 2–3 days with regular use.",
    imageUrl: null,
    tags: ["i8", "i3", "i11"],
  },
]

// ─── Unified tag palette ──────────────────────────────────────
// Single source of truth for both item tags and file tags.
// color/bg/border are design-token-based Tailwind classes.
export const MOCK_TAGS: Tag[] = [
  // File/document tags (t1–t6) — kept with same IDs for backwards compat
  { id: "t1", label: "Invoice",     color: "text-chart-1",         bg: "bg-chart-1/15",        border: "border-chart-1/30" },
  { id: "t2", label: "Warranty",    color: "text-status-active",   bg: "bg-status-active/15",  border: "border-status-active/30" },
  { id: "t3", label: "Manual",      color: "text-chart-3",         bg: "bg-chart-3/15",        border: "border-chart-3/30" },
  { id: "t4", label: "Photo",       color: "text-status-expiring", bg: "bg-status-expiring/15",border: "border-status-expiring/30" },
  { id: "t5", label: "Certificate", color: "text-chart-2",         bg: "bg-chart-2/15",        border: "border-chart-2/30" },
  { id: "t6", label: "Backup",      color: "text-muted-foreground",bg: "bg-muted",             border: "border-border" },
  // Item / inventory tags (i1–i12)
  { id: "i1",  label: "laundry",    color: "text-chart-3",         bg: "bg-chart-3/15",        border: "border-chart-3/30" },
  { id: "i2",  label: "white-goods",color: "text-chart-1",         bg: "bg-chart-1/15",        border: "border-chart-1/30" },
  { id: "i3",  label: "work",       color: "text-chart-2",         bg: "bg-chart-2/15",        border: "border-chart-2/30" },
  { id: "i4",  label: "apple",      color: "text-muted-foreground",bg: "bg-muted",             border: "border-border" },
  { id: "i5",  label: "kitchen",    color: "text-status-expiring", bg: "bg-status-expiring/15",border: "border-status-expiring/30" },
  { id: "i6",  label: "samsung",    color: "text-chart-4",         bg: "bg-chart-4/15",        border: "border-chart-4/30" },
  { id: "i7",  label: "cleaning",   color: "text-status-active",   bg: "bg-status-active/15",  border: "border-status-active/30" },
  { id: "i8",  label: "audio",      color: "text-chart-1",         bg: "bg-chart-1/15",        border: "border-chart-1/30" },
  { id: "i9",  label: "sony",       color: "text-chart-5",         bg: "bg-chart-5/15",        border: "border-chart-5/30" },
  { id: "i10", label: "bosch",      color: "text-chart-3",         bg: "bg-chart-3/15",        border: "border-chart-3/30" },
  { id: "i11", label: "living-room",color: "text-chart-2",         bg: "bg-chart-2/15",        border: "border-chart-2/30" },
  { id: "i12", label: "lg",         color: "text-chart-4",         bg: "bg-chart-4/15",        border: "border-chart-4/30" },
  { id: "i13", label: "tools",      color: "text-chart-5",         bg: "bg-chart-5/15",        border: "border-chart-5/30" },
  { id: "i14", label: "dewalt",     color: "text-status-expiring", bg: "bg-status-expiring/15",border: "border-status-expiring/30" },
  { id: "i15", label: "makita",     color: "text-chart-1",         bg: "bg-chart-1/15",        border: "border-chart-1/30" },
]

/** @deprecated Use MOCK_TAGS instead */
export const FILE_TAGS: Tag[] = MOCK_TAGS.filter((t) => t.id.startsWith("t"))

// Helper: look up a Tag by ID, returns undefined if not found
export function findTag(id: string): Tag | undefined {
  return MOCK_TAGS.find((t) => t.id === id)
}

// Helper: resolve an array of tag IDs to Tag objects (omits unknown IDs)
export function resolveTags(ids: string[]): Tag[] {
  return ids.flatMap((id) => { const t = findTag(id); return t ? [t] : [] })
}

// ─── Files ────────────────────────────────────────────────────
export const MOCK_FILES: AttachedFile[] = [
  { id: "f1", name: "Miele_Warranty.pdf", size: "284 KB", uploadedAt: "2022-03-15", mimeType: "application/pdf",
    category: "document",
    attachedTo: { type: "commodity", id: "1", name: "Washing Machine" }, tags: ["t2"] },
  { id: "f2", name: "MacBook_Receipt.pdf", size: "156 KB", uploadedAt: "2021-11-08", mimeType: "application/pdf",
    category: "invoice",
    attachedTo: { type: "commodity", id: "2", name: 'MacBook Pro 16"' }, tags: ["t1", "t2"] },
  { id: "f3", name: "Samsung_Fridge_Manual.pdf", size: "4.2 MB", uploadedAt: "2020-06-20", mimeType: "application/pdf",
    category: "document",
    attachedTo: { type: "commodity", id: "3", name: "Refrigerator" }, tags: ["t3"] },
  { id: "f4", name: "Washing_Machine_photo.jpg", size: "1.8 MB", uploadedAt: "2022-03-16", mimeType: "image/jpeg",
    category: "image",
    attachedTo: { type: "commodity", id: "1", name: "Washing Machine" }, tags: ["t4"],
    thumbnailUrl: "https://images.unsplash.com/photo-1626806787461-102c1bfaaea1?w=400&q=80" },
  { id: "f5", name: "Dyson_Invoice.pdf", size: "198 KB", uploadedAt: "2023-01-10", mimeType: "application/pdf",
    category: "invoice",
    attachedTo: { type: "commodity", id: "4", name: "Dyson V15 Detect" }, tags: ["t2", "t1"] },
  { id: "f6", name: "DeWalt_Drill_photo.jpg", size: "2.1 MB", uploadedAt: "2019-08-14", mimeType: "image/jpeg",
    category: "image",
    attachedTo: { type: "commodity", id: "8", name: "DeWalt Drill" }, tags: ["t4"],
    thumbnailUrl: "https://images.unsplash.com/photo-1504148455328-c376907d081c?w=400&q=80" },
  { id: "f7", name: "Bosch_Dishwasher_Invoice.pdf", size: "88 KB", uploadedAt: "2021-04-12", mimeType: "application/pdf",
    category: "invoice",
    attachedTo: { type: "commodity", id: "6", name: "Bosch Dishwasher" }, tags: ["t1"] },
  { id: "f8", name: "LG_TV_Setup_Guide.pdf", size: "6.1 MB", uploadedAt: "2022-12-26", mimeType: "application/pdf",
    category: "document",
    attachedTo: { type: "commodity", id: "7", name: "4K TV" }, tags: ["t3"] },
  { id: "f9", name: "Home_Inventory_Backup.zip", size: "12.4 MB", uploadedAt: "2024-01-01", mimeType: "application/zip",
    category: "other",
    attachedTo: { type: "location", id: "loc1", name: "Main House" }, tags: ["t6"] },
  { id: "f10", name: "Sony_WH_Photo.jpg", size: "980 KB", uploadedAt: "2022-09-01", mimeType: "image/jpeg",
    category: "image",
    attachedTo: { type: "commodity", id: "5", name: "Sony WH-1000XM5" }, tags: ["t4"],
    thumbnailUrl: "https://images.unsplash.com/photo-1505740420928-5e560c06d30e?w=400&q=80" },
  { id: "f11", name: "Makita_Warranty_Certificate.pdf", size: "112 KB", uploadedAt: "2021-06-05", mimeType: "application/pdf",
    category: "document",
    attachedTo: { type: "commodity", id: "9", name: "Circular Saw" }, tags: ["t2", "t5"] },
  { id: "f12", name: "Miele_Invoice.pdf", size: "96 KB", uploadedAt: "2022-03-15", mimeType: "application/pdf",
    category: "invoice",
    attachedTo: { type: "commodity", id: "1", name: "Washing Machine" }, tags: ["t1"] },
  { id: "f13", name: "MacBook_Front.jpg", size: "1.2 MB", uploadedAt: "2021-11-09", mimeType: "image/jpeg",
    category: "image",
    attachedTo: { type: "commodity", id: "2", name: 'MacBook Pro 16"' }, tags: ["t4"],
    thumbnailUrl: "https://images.unsplash.com/photo-1517336714731-489689fd1ca8?w=400&q=80" },
]

export const CATEGORIES: { value: ItemCategory; label: string }[] = [
  { value: "appliance", label: "Appliance" },
  { value: "electronics", label: "Electronics" },
  { value: "tool", label: "Tool" },
  { value: "furniture", label: "Furniture" },
  { value: "vehicle", label: "Vehicle" },
  { value: "other", label: "Other" },
]

export const CATEGORY_ICONS: Record<ItemCategory, string> = {
  appliance: "🏠",
  electronics: "💻",
  tool: "🔧",
  furniture: "🪑",
  vehicle: "🚗",
  other: "📦",
}

export function areaLabel(areaId: string): string {
  const area = MOCK_AREAS.find((a) => a.id === areaId)
  if (!area) return "Unknown"
  const location = MOCK_LOCATIONS.find((l) => l.id === area.locationId)
  return location ? `${location.name} · ${area.name}` : area.name
}

export function areaName(areaId: string): string {
  return MOCK_AREAS.find((a) => a.id === areaId)?.name ?? "Unknown"
}

export const WARRANTY_STATUS_CONFIG: Record<
  WarrantyStatus,
  { label: string; color: string; bg: string }
> = {
  active: { label: "Active", color: "text-status-active", bg: "bg-status-active/10" },
  expiring: { label: "Expiring Soon", color: "text-status-expiring", bg: "bg-status-expiring/10" },
  expired: { label: "Expired", color: "text-status-expired", bg: "bg-status-expired/10" },
  none: { label: "No Warranty", color: "text-status-none", bg: "bg-status-none/10" },
}

export const COMMODITY_STATUS_CONFIG: Record<
  CommodityStatus,
  { label: string; color: string; bg: string; description: string }
> = {
  in_use: { label: "In Use", color: "text-status-active", bg: "bg-status-active/10", description: "Currently owned and in use" },
  sold: { label: "Sold", color: "text-chart-2", bg: "bg-chart-2/10", description: "Sold to someone else" },
  lost: { label: "Lost", color: "text-status-expiring", bg: "bg-status-expiring/10", description: "Cannot be located" },
  disposed: { label: "Disposed", color: "text-muted-foreground", bg: "bg-muted", description: "Thrown away or recycled" },
  written_off: { label: "Written Off", color: "text-status-expired", bg: "bg-status-expired/10", description: "Damaged beyond repair or written off" },
}

export const CURRENCIES = [
  { code: "USD", name: "US Dollar", symbol: "$" },
  { code: "EUR", name: "Euro", symbol: "€" },
  { code: "GBP", name: "British Pound", symbol: "£" },
  { code: "CZK", name: "Czech Koruna", symbol: "Kč" },
  { code: "CHF", name: "Swiss Franc", symbol: "Fr" },
  { code: "JPY", name: "Japanese Yen", symbol: "¥" },
  { code: "CAD", name: "Canadian Dollar", symbol: "CA$" },
  { code: "AUD", name: "Australian Dollar", symbol: "A$" },
  { code: "SEK", name: "Swedish Krona", symbol: "kr" },
  { code: "NOK", name: "Norwegian Krone", symbol: "kr" },
  { code: "DKK", name: "Danish Krone", symbol: "kr" },
  { code: "PLN", name: "Polish Zloty", symbol: "zł" },
  { code: "HUF", name: "Hungarian Forint", symbol: "Ft" },
  { code: "RON", name: "Romanian Leu", symbol: "lei" },
  { code: "BGN", name: "Bulgarian Lev", symbol: "лв" },
  { code: "HRK", name: "Croatian Kuna", symbol: "kn" },
  { code: "RUB", name: "Russian Ruble", symbol: "₽" },
  { code: "CNY", name: "Chinese Yuan", symbol: "¥" },
  { code: "KRW", name: "South Korean Won", symbol: "₩" },
  { code: "INR", name: "Indian Rupee", symbol: "₹" },
  { code: "BRL", name: "Brazilian Real", symbol: "R$" },
  { code: "MXN", name: "Mexican Peso", symbol: "MX$" },
  { code: "ZAR", name: "South African Rand", symbol: "R" },
  { code: "SGD", name: "Singapore Dollar", symbol: "S$" },
  { code: "NZD", name: "New Zealand Dollar", symbol: "NZ$" },
  { code: "HKD", name: "Hong Kong Dollar", symbol: "HK$" },
  { code: "TWD", name: "Taiwan Dollar", symbol: "NT$" },
  { code: "TRY", name: "Turkish Lira", symbol: "₺" },
  { code: "SAR", name: "Saudi Riyal", symbol: "﷼" },
  { code: "AED", name: "UAE Dirham", symbol: "د.إ" },
]

// ══════════════════════════════════════════════════════════════
// Admin — tenants, users, sessions, groups (mock-only)
// ══════════════════════════════════════════════════════════════

export type TenantStatus = "active" | "suspended" | "trial" | "archived"
export type TenantPlan = "free" | "starter" | "business" | "enterprise"
export type GroupStatus = "active" | "pending_deletion"

export interface UserSession {
  id: string
  device: string
  ip: string
  location: string
  lastActive: string // ISO datetime
  current: boolean
}

export interface AdminUser {
  id: string
  tenantId: string
  name: string
  email: string
  avatarInitials: string
  role: MemberRole
  isActive: boolean
  lastLogin: string | null // ISO datetime
  createdAt: string
  sessions: UserSession[]
  groupMemberships: { groupId: string; role: MemberRole }[]
}

export interface AdminGroup {
  id: string
  tenantId: string
  name: string
  status: GroupStatus
  currency: string
  memberCount: number
  createdAt: string
  members: { userId: string; role: MemberRole }[]
}

export interface Tenant {
  id: string
  name: string
  slug: string
  domain: string
  status: TenantStatus
  plan: TenantPlan
  userCount: number
  groupCount: number
  createdAt: string
}

export const TENANT_STATUS_CONFIG: Record<
  TenantStatus,
  { label: string; color: string; bg: string }
> = {
  active: { label: "Active", color: "text-status-active", bg: "bg-status-active/10" },
  trial: { label: "Trial", color: "text-chart-3", bg: "bg-chart-3/10" },
  suspended: { label: "Suspended", color: "text-status-expired", bg: "bg-status-expired/10" },
  archived: { label: "Archived", color: "text-status-none", bg: "bg-status-none/10" },
}

export const TENANT_PLAN_CONFIG: Record<TenantPlan, { label: string }> = {
  free: { label: "Free" },
  starter: { label: "Starter" },
  business: { label: "Business" },
  enterprise: { label: "Enterprise" },
}

export const GROUP_STATUS_CONFIG: Record<
  GroupStatus,
  { label: string; color: string; bg: string }
> = {
  active: { label: "Active", color: "text-status-active", bg: "bg-status-active/10" },
  pending_deletion: { label: "Pending Deletion", color: "text-status-expired", bg: "bg-status-expired/10" },
}

// ─── Tenants ─────────────────────────────────────────────────
export const MOCK_TENANTS: Tenant[] = [
  { id: "tn1",  name: "Northwind Estates",     slug: "northwind",     domain: "northwind.inventario.app",    status: "active",    plan: "business",   userCount: 24, groupCount: 6, createdAt: "2023-02-11" },
  { id: "tn2",  name: "Harbor & Co",           slug: "harbor-co",     domain: "harbor.inventario.app",       status: "active",    plan: "enterprise", userCount: 88, groupCount: 14, createdAt: "2022-09-30" },
  { id: "tn3",  name: "Cedar Valley Homes",    slug: "cedar-valley",  domain: "cedarvalley.inventario.app",  status: "trial",     plan: "starter",    userCount: 5,  groupCount: 2, createdAt: "2026-04-28" },
  { id: "tn4",  name: "Atlas Property Group",  slug: "atlas-pg",      domain: "atlas.inventario.app",        status: "active",    plan: "business",   userCount: 41, groupCount: 9, createdAt: "2023-07-19" },
  { id: "tn5",  name: "Birchwood Rentals",     slug: "birchwood",     domain: "birchwood.inventario.app",    status: "suspended", plan: "starter",    userCount: 12, groupCount: 3, createdAt: "2024-01-05" },
  { id: "tn6",  name: "Sterling Logistics",    slug: "sterling",      domain: "sterling.inventario.app",     status: "active",    plan: "enterprise", userCount: 130, groupCount: 22, createdAt: "2021-11-22" },
  { id: "tn7",  name: "Maple Lane Storage",    slug: "maple-lane",    domain: "maplelane.inventario.app",    status: "active",    plan: "free",       userCount: 3,  groupCount: 1, createdAt: "2025-03-14" },
  { id: "tn8",  name: "Ironclad Workshops",    slug: "ironclad",      domain: "ironclad.inventario.app",     status: "trial",     plan: "starter",    userCount: 7,  groupCount: 2, createdAt: "2026-05-02" },
  { id: "tn9",  name: "Greenfield Coop",       slug: "greenfield",    domain: "greenfield.inventario.app",   status: "active",    plan: "business",   userCount: 33, groupCount: 7, createdAt: "2023-05-08" },
  { id: "tn10", name: "Pinecrest Holdings",    slug: "pinecrest",     domain: "pinecrest.inventario.app",    status: "archived",  plan: "free",       userCount: 2,  groupCount: 1, createdAt: "2022-04-17" },
  { id: "tn11", name: "Lakeside Ventures",     slug: "lakeside",      domain: "lakeside.inventario.app",     status: "active",    plan: "business",   userCount: 19, groupCount: 5, createdAt: "2024-08-29" },
  { id: "tn12", name: "Summit Facilities",     slug: "summit-fac",    domain: "summit.inventario.app",       status: "active",    plan: "enterprise", userCount: 64, groupCount: 11, createdAt: "2022-12-03" },
  { id: "tn13", name: "Willow Park Mgmt",      slug: "willow-park",   domain: "willowpark.inventario.app",   status: "suspended", plan: "business",   userCount: 28, groupCount: 6, createdAt: "2023-10-11" },
  { id: "tn14", name: "Redbrick Realty",       slug: "redbrick",      domain: "redbrick.inventario.app",     status: "active",    plan: "starter",    userCount: 9,  groupCount: 3, createdAt: "2025-01-20" },
  { id: "tn15", name: "Copperline Studios",    slug: "copperline",    domain: "copperline.inventario.app",   status: "trial",     plan: "starter",    userCount: 4,  groupCount: 1, createdAt: "2026-05-12" },
  { id: "tn16", name: "Oakhaven Trust",        slug: "oakhaven",      domain: "oakhaven.inventario.app",     status: "active",    plan: "business",   userCount: 37, groupCount: 8, createdAt: "2023-03-27" },
  { id: "tn17", name: "Brightwater Group",     slug: "brightwater",   domain: "brightwater.inventario.app",  status: "active",    plan: "enterprise", userCount: 102, groupCount: 18, createdAt: "2022-06-14" },
  { id: "tn18", name: "Stonebridge Estates",   slug: "stonebridge",   domain: "stonebridge.inventario.app",  status: "active",    plan: "free",       userCount: 6,  groupCount: 2, createdAt: "2025-09-01" },
  { id: "tn19", name: "Hollow Creek Storage",  slug: "hollow-creek",  domain: "hollowcreek.inventario.app",  status: "archived",  plan: "starter",    userCount: 8,  groupCount: 2, createdAt: "2021-08-23" },
  { id: "tn20", name: "Vantage Property Co",   slug: "vantage",       domain: "vantage.inventario.app",      status: "active",    plan: "business",   userCount: 45, groupCount: 10, createdAt: "2024-02-16" },
  { id: "tn21", name: "Foxglove Rentals",      slug: "foxglove",      domain: "foxglove.inventario.app",     status: "trial",     plan: "free",       userCount: 3,  groupCount: 1, createdAt: "2026-05-15" },
  { id: "tn22", name: "Driftwood Holdings",    slug: "driftwood",     domain: "driftwood.inventario.app",    status: "active",    plan: "enterprise", userCount: 71, groupCount: 13, createdAt: "2023-01-09" },
]

// ─── Admin users — generated per tenant ──────────────────────
const ADMIN_FIRST = ["Alex", "Jordan", "Riley", "Morgan", "Casey", "Taylor", "Sam", "Jamie", "Quinn", "Avery", "Drew", "Reese"]
const ADMIN_LAST = ["Hartley", "Velez", "Okafor", "Lindqvist", "Marsh", "Delgado", "Novak", "Bauer", "Cho", "Ferreira"]
const DEVICES = ["MacBook Pro · Chrome", "iPhone 15 · Safari", "Windows 11 · Edge", "iPad Air · Safari", "Linux · Firefox"]
const CITIES = ["Berlin, DE", "Austin, US", "Prague, CZ", "London, GB", "Toronto, CA", "Lisbon, PT"]

function pick<T>(arr: T[], n: number): T {
  return arr[n % arr.length]
}

function buildAdminUsers(): AdminUser[] {
  const users: AdminUser[] = []
  let seq = 0
  for (const tenant of MOCK_TENANTS) {
    const count = Math.min(6, Math.max(2, Math.round(tenant.userCount / 8)))
    for (let i = 0; i < count; i++) {
      seq++
      const first = pick(ADMIN_FIRST, seq * 3 + i)
      const last = pick(ADMIN_LAST, seq + i * 2)
      const role: MemberRole = i === 0 ? "owner" : i === 1 ? "admin" : i % 2 === 0 ? "user" : "viewer"
      const isActive = !(seq % 9 === 0)
      const sessionCount = isActive ? (i % 3) + 1 : 0
      const sessions: UserSession[] = Array.from({ length: sessionCount }, (_, s) => ({
        id: `se-${seq}-${s}`,
        device: pick(DEVICES, seq + s),
        ip: `${20 + ((seq + s) % 200)}.${(seq * 7 + s) % 250}.${(s * 13) % 250}.${(seq + s * 3) % 250}`,
        location: pick(CITIES, seq + s * 2),
        lastActive: daysFromNow(-(s * 2) - (seq % 5)) + "T" + String(8 + ((seq + s) % 12)).padStart(2, "0") + ":" + String((seq * 7) % 60).padStart(2, "0") + ":00",
        current: s === 0,
      }))
      users.push({
        id: `au-${seq}`,
        tenantId: tenant.id,
        name: `${first} ${last}`,
        email: `${first.toLowerCase()}.${last.toLowerCase()}@${tenant.slug}.com`,
        avatarInitials: first[0] + last[0],
        role,
        isActive,
        lastLogin: isActive ? daysFromNow(-(seq % 14)) + "T09:" + String((seq * 11) % 60).padStart(2, "0") + ":00" : daysFromNow(-60 - (seq % 90)) + "T09:00:00",
        createdAt: tenant.createdAt,
        sessions,
        groupMemberships: [],
      })
    }
  }
  return users
}

export const MOCK_ADMIN_USERS: AdminUser[] = buildAdminUsers()

// ─── Admin groups — generated per tenant ─────────────────────
const GROUP_NAMES = ["Main Residence", "Downtown Office", "Warehouse A", "Storage Unit 12", "Garage Workshop", "Coastal Retreat", "North Wing", "Field Depot", "Annex Building", "Records Vault"]

function buildAdminGroups(): AdminGroup[] {
  const groups: AdminGroup[] = []
  let seq = 0
  for (const tenant of MOCK_TENANTS) {
    const tenantUsers = MOCK_ADMIN_USERS.filter((u) => u.tenantId === tenant.id)
    const count = Math.min(5, Math.max(1, Math.round(tenant.groupCount / 2)))
    for (let i = 0; i < count; i++) {
      seq++
      const status: GroupStatus = seq % 11 === 0 ? "pending_deletion" : "active"
      const memberSlice = tenantUsers.slice(0, Math.min(tenantUsers.length, (i % 3) + 2))
      // Intentional: a member's group-membership role is assigned per group and
      // is distinct from the user's tenant-level identity role. A user who is a
      // "viewer" at the tenant level can legitimately be an "owner" of a group.
      const members = memberSlice.map((u, mi) => ({
        userId: u.id,
        role: (mi === 0 ? "owner" : mi === 1 ? "admin" : mi % 2 === 0 ? "user" : "viewer") as MemberRole,
      }))
      const group: AdminGroup = {
        id: `ag-${seq}`,
        tenantId: tenant.id,
        name: pick(GROUP_NAMES, seq + i),
        status,
        currency: pick(["USD", "EUR", "GBP", "CZK", "CHF"], seq),
        memberCount: members.length,
        createdAt: daysFromNow(-(seq * 17) - 30),
        members,
      }
      groups.push(group)
      for (const m of members) {
        const u = MOCK_ADMIN_USERS.find((x) => x.id === m.userId)
        if (u) u.groupMemberships.push({ groupId: group.id, role: m.role })
      }
    }
  }
  return groups
}

export const MOCK_ADMIN_GROUPS: AdminGroup[] = buildAdminGroups()

// ─── Admin lookup helpers ────────────────────────────────────
export function tenantById(id: string): Tenant | undefined {
  return MOCK_TENANTS.find((t) => t.id === id)
}

export function adminUserById(id: string): AdminUser | undefined {
  return MOCK_ADMIN_USERS.find((u) => u.id === id)
}

export function adminGroupById(id: string): AdminGroup | undefined {
  return MOCK_ADMIN_GROUPS.find((g) => g.id === id)
}

export function usersByTenant(tenantId: string): AdminUser[] {
  return MOCK_ADMIN_USERS.filter((u) => u.tenantId === tenantId)
}

export function groupsByTenant(tenantId: string): AdminGroup[] {
  return MOCK_ADMIN_GROUPS.filter((g) => g.tenantId === tenantId)
}

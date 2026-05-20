import { useState } from "react"
import { Button } from "@/components/ui/button"
import { ButtonGroup, ButtonGroupText, ButtonGroupSeparator } from "@/components/ui/button-group"
import { Badge } from "@/components/ui/badge"
import { Input } from "@/components/ui/input"
import { Textarea } from "@/components/ui/textarea"
import { Label } from "@/components/ui/label"
import { Switch } from "@/components/ui/switch"
import { Checkbox } from "@/components/ui/checkbox"
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group"
import { Slider } from "@/components/ui/slider"
import { Progress } from "@/components/ui/progress"
import { Separator } from "@/components/ui/separator"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from "@/components/ui/card"
import { Avatar, AvatarFallback } from "@/components/ui/avatar"
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Skeleton } from "@/components/ui/skeleton"
import { SkeletonShowcase } from "@/components/SkeletonPatterns"
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
  DropdownMenuCheckboxItem,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuShortcut,
} from "@/components/ui/dropdown-menu"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog"
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from "@/components/ui/alert-dialog"
import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
} from "@/components/ui/accordion"
import {
  Table,
  TableBody,
  TableCaption,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import {
  HoverCard,
  HoverCardContent,
  HoverCardTrigger,
} from "@/components/ui/hover-card"
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover"
import { Toggle } from "@/components/ui/toggle"
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group"
import { Kbd } from "@/components/ui/kbd"
import { Spinner } from "@/components/ui/spinner"
import { Empty, EmptyHeader, EmptyMedia, EmptyTitle, EmptyDescription, EmptyContent } from "@/components/ui/empty"
import {
  ChartContainer,
  ChartTooltip,
  ChartTooltipContent,
  type ChartConfig,
} from "@/components/ui/chart"
import {
  BarChart, Bar, AreaChart, Area,
  XAxis, CartesianGrid,
} from "recharts"
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
  SheetTrigger,
  SheetFooter,
} from "@/components/ui/sheet"
import {
  Drawer,
  DrawerContent,
  DrawerDescription,
  DrawerFooter,
  DrawerHeader,
  DrawerTitle,
  DrawerTrigger,
} from "@/components/ui/drawer"
import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
  CommandSeparator,
  CommandShortcut,
} from "@/components/ui/command"
import {
  Breadcrumb,
  BreadcrumbItem,
  BreadcrumbLink,
  BreadcrumbList,
  BreadcrumbPage,
  BreadcrumbSeparator,
} from "@/components/ui/breadcrumb"
import {
  Pagination,
  PaginationContent,
  PaginationEllipsis,
  PaginationItem,
  PaginationLink,
  PaginationNext,
  PaginationPrevious,
} from "@/components/ui/pagination"
import { ScrollArea } from "@/components/ui/scroll-area"
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from "@/components/ui/collapsible"
import {
  ResizableHandle,
  ResizablePanel,
  ResizablePanelGroup,
} from "@/components/ui/resizable"
import { InputOTP, InputOTPGroup, InputOTPSlot, InputOTPSeparator } from "@/components/ui/input-otp"
import { NativeSelect } from "@/components/ui/native-select"
import {
  Item,
  ItemMedia,
  ItemContent,
  ItemTitle,
  ItemDescription,
  ItemActions,
  ItemGroup,
  ItemSeparator,
} from "@/components/ui/item"
import {
  Combobox,
  ComboboxContent,
  ComboboxEmpty,
  ComboboxInput,
  ComboboxItem,
  ComboboxList,
} from "@/components/ui/combobox"
import { Field, FieldLabel, FieldError } from "@/components/ui/field"
import { Toaster } from "@/components/ui/sonner"
import { toast } from "sonner"
import { CurrencyCombobox } from "@/components/CurrencyCombobox"
import { WarrantyBadge } from "@/components/WarrantyBadge"
import { MOCK_ITEMS } from "@/data/mock"
import { Package, Bell, Settings, Star, Bookmark, Trash2, CreditCard as Edit, Plus, Download, Upload, Search, ListFilter as Filter, CircleCheck as CheckCircle2, CircleAlert as AlertCircle, Info, Circle as XCircle, Zap, TrendingUp, Hop as Home, Users, ChevronDown, MoveHorizontal as MoreHorizontal, ChevronLeft as AlignLeft, TextAlignCenter as AlignCenter, Highlighter as AlignRight, Bold, Italic, Underline, FolderOpen, FileText, MapPin, ChevronsUpDown } from "lucide-react"
import { cn } from "@/lib/utils"

// ─── Demo data ────────────────────────────────────────────────
const chartData = [
  { month: "Jan", items: 4, value: 3200 },
  { month: "Feb", items: 6, value: 4800 },
  { month: "Mar", items: 5, value: 5100 },
  { month: "Apr", items: 9, value: 7200 },
  { month: "May", items: 7, value: 6400 },
  { month: "Jun", items: 11, value: 9800 },
]

const barConfig = {
  items: { label: "Items Added", color: "var(--chart-1)" },
  value: { label: "Portfolio Value", color: "var(--chart-2)" },
} satisfies ChartConfig

const tableItems = MOCK_ITEMS.slice(0, 5)

const frameworks = ["Next.js", "SvelteKit", "Nuxt.js", "Remix", "Astro", "React Router", "TanStack Start"]

// ─── Section wrapper ──────────────────────────────────────────
function Section({ title, subtitle, children, id }: { title: string; subtitle?: string; children: React.ReactNode; id?: string }) {
  return (
    <section className="space-y-4" id={id}>
      <div>
        <h2 className="text-lg font-semibold tracking-tight">{title}</h2>
        {subtitle && <p className="text-sm text-muted-foreground mt-0.5">{subtitle}</p>}
      </div>
      {children}
    </section>
  )
}

function DemoRow({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="flex flex-col gap-2">
      <span className="text-xs font-medium text-muted-foreground uppercase tracking-wide">{label}</span>
      <div className="flex flex-wrap items-center gap-2">{children}</div>
    </div>
  )
}

export function UIShowcaseView() {
  const [switchOn, setSwitchOn] = useState(false)
  const [checkA, setCheckA] = useState(true)
  const [checkB, setCheckB] = useState(false)
  const [radio, setRadio] = useState("option-1")
  const [sliderVal, setSliderVal] = useState([60])
  const [inputVal, setInputVal] = useState("")
  const [dropdownCheck, setDropdownCheck] = useState(true)
  const [dropdownRadio, setDropdownRadio] = useState("all")
  const [toggleVal, setToggleVal] = useState<string>("center")
  const [collapsibleOpen, setCollapsibleOpen] = useState(false)
  const [otpVal, setOtpVal] = useState("")
  const [comboboxVal, setComboboxVal] = useState<string | null>(null)
  const [currencyVal, setCurrencyVal] = useState("EUR")

  return (
    <div className="flex flex-col gap-12 p-8 max-w-5xl mx-auto w-full pb-20">
      {/* Page header */}
      <div className="space-y-1">
        <div className="flex items-center gap-2">
          <Zap className="size-5 text-primary" />
          <h1 className="scroll-m-20 text-3xl font-semibold tracking-tight">UI Showcase</h1>
        </div>
        <p className="text-muted-foreground">Every UI element available in this system, in one place.</p>
      </div>

      {/* ── BUTTONS ──────────────────────────────────────────── */}
      <Section title="Buttons" subtitle="All variants and sizes">
        <DemoRow label="Variants">
          <Button>Default</Button>
          <Button variant="secondary">Secondary</Button>
          <Button variant="outline">Outline</Button>
          <Button variant="ghost">Ghost</Button>
          <Button variant="destructive">Destructive</Button>
          <Button variant="link">Link</Button>
        </DemoRow>
        <DemoRow label="Sizes">
          <Button size="lg">Large</Button>
          <Button size="default">Default</Button>
          <Button size="sm">Small</Button>
          <Button size="icon"><Plus className="size-4" /></Button>
          <Button size="icon-sm"><Plus className="size-3.5" /></Button>
          <Button size="icon-xs"><Plus className="size-3" /></Button>
        </DemoRow>
        <DemoRow label="With icons">
          <Button className="gap-2"><Download className="size-4" />Download</Button>
          <Button variant="outline" className="gap-2"><Upload className="size-4" />Upload</Button>
          <Button variant="secondary" className="gap-2"><Plus className="size-4" />Add Item</Button>
          <Button variant="destructive" className="gap-2"><Trash2 className="size-4" />Delete</Button>
        </DemoRow>
        <DemoRow label="Loading / Disabled">
          <Button disabled className="gap-2"><Spinner className="size-4" />Saving…</Button>
          <Button disabled>Disabled</Button>
          <Button variant="outline" disabled>Disabled outline</Button>
        </DemoRow>
      </Section>

      <Separator />

      {/* ── BUTTON GROUP ─────────────────────────────────────── */}
      <Section title="Button Group" subtitle="Grouped buttons and inputs">
        <DemoRow label="Horizontal button group">
          <ButtonGroup>
            <Button variant="outline" size="sm">Cut</Button>
            <Button variant="outline" size="sm">Copy</Button>
            <Button variant="outline" size="sm">Paste</Button>
          </ButtonGroup>
          <ButtonGroup>
            <Button variant="outline" size="sm"><AlignLeft className="size-4" /></Button>
            <Button variant="outline" size="sm"><AlignCenter className="size-4" /></Button>
            <Button variant="outline" size="sm"><AlignRight className="size-4" /></Button>
          </ButtonGroup>
        </DemoRow>
        <DemoRow label="With text addon">
          <ButtonGroup>
            <ButtonGroupText>https://</ButtonGroupText>
            <Input placeholder="yoursite.com" className="w-48" />
          </ButtonGroup>
          <ButtonGroup>
            <Input placeholder="0.00" className="w-28" />
            <ButtonGroupSeparator />
            <Select defaultValue="USD">
              <SelectTrigger className="w-20"><SelectValue /></SelectTrigger>
              <SelectContent>
                <SelectItem value="USD">USD</SelectItem>
                <SelectItem value="EUR">EUR</SelectItem>
              </SelectContent>
            </Select>
          </ButtonGroup>
        </DemoRow>
        <DemoRow label="Vertical">
          <ButtonGroup orientation="vertical">
            <Button variant="outline" size="sm">Top</Button>
            <Button variant="outline" size="sm">Middle</Button>
            <Button variant="outline" size="sm">Bottom</Button>
          </ButtonGroup>
        </DemoRow>
      </Section>

      <Separator />

      {/* ── BADGES ───────────────────────────────────────────── */}
      <Section title="Badges" subtitle="Status and label badges">
        <DemoRow label="Variants">
          <Badge>Default</Badge>
          <Badge variant="secondary">Secondary</Badge>
          <Badge variant="outline">Outline</Badge>
          <Badge variant="destructive">Destructive</Badge>
        </DemoRow>
        <DemoRow label="Warranty status">
          {MOCK_ITEMS.slice(0, 4).map((item) => (
            <WarrantyBadge key={item.id} item={item} />
          ))}
        </DemoRow>
        <DemoRow label="Semantic colors">
          <Badge className="bg-status-active/10 text-status-active border-status-active/20">Active</Badge>
          <Badge className="bg-status-expiring/10 text-status-expiring border-status-expiring/20">Expiring</Badge>
          <Badge className="bg-status-expired/10 text-status-expired border-status-expired/20">Expired</Badge>
          <Badge className="bg-chart-1/10 text-chart-1">Category</Badge>
          <Badge className="bg-chart-2/10 text-chart-2">Tag</Badge>
        </DemoRow>
      </Section>

      <Separator />

      {/* ── FORM CONTROLS ────────────────────────────────────── */}
      <Section title="Form Controls" subtitle="Inputs, selects, checkboxes, switches">
        <div className="grid grid-cols-2 gap-6">
          <div className="space-y-4">
            <div className="space-y-1.5">
              <Label htmlFor="demo-input">Text Input</Label>
              <Input
                id="demo-input"
                placeholder="Enter item name…"
                value={inputVal}
                onChange={(e) => setInputVal(e.target.value)}
              />
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="demo-search">Search Input</Label>
              <div className="relative">
                <Search className="absolute left-2.5 top-1/2 -translate-y-1/2 size-4 text-muted-foreground" />
                <Input id="demo-search" className="pl-8" placeholder="Search…" />
              </div>
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="demo-textarea">Textarea</Label>
              <Textarea id="demo-textarea" placeholder="Notes and descriptions…" rows={3} />
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="demo-select">Select</Label>
              <Select defaultValue="electronics">
                <SelectTrigger id="demo-select">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="appliance">Appliance</SelectItem>
                  <SelectItem value="electronics">Electronics</SelectItem>
                  <SelectItem value="tool">Tool</SelectItem>
                  <SelectItem value="furniture">Furniture</SelectItem>
                  <SelectItem value="vehicle">Vehicle</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-1.5">
              <Label>Native Select</Label>
              <NativeSelect defaultValue="electronics">
                <option value="appliance">Appliance</option>
                <option value="electronics">Electronics</option>
                <option value="tool">Tool</option>
              </NativeSelect>
            </div>
          </div>

          <div className="space-y-5">
            <div className="space-y-3">
              <Label>Checkboxes</Label>
              <div className="space-y-2">
                {[
                  { id: "cb-a", label: "Include photos", checked: checkA, onChange: setCheckA },
                  { id: "cb-b", label: "Export warranty info", checked: checkB, onChange: setCheckB },
                  { id: "cb-c", label: "Disabled option", checked: false, onChange: () => {}, disabled: true },
                ].map(({ id, label, checked, onChange, disabled }) => (
                  <div key={id} className="flex items-center gap-2">
                    <Checkbox
                      id={id}
                      checked={checked}
                      onCheckedChange={(v) => onChange(!!v)}
                      disabled={disabled}
                    />
                    <Label htmlFor={id} className={cn("font-normal", disabled && "opacity-50")}>{label}</Label>
                  </div>
                ))}
              </div>
            </div>

            <div className="space-y-3">
              <Label>Radio Group</Label>
              <RadioGroup value={radio} onValueChange={setRadio} className="space-y-2">
                {["option-1", "option-2", "option-3"].map((v, i) => (
                  <div key={v} className="flex items-center gap-2">
                    <RadioGroupItem value={v} id={v} />
                    <Label htmlFor={v} className="font-normal">Option {i + 1}</Label>
                  </div>
                ))}
              </RadioGroup>
            </div>

            <div className="flex items-center gap-3">
              <Switch id="demo-switch" checked={switchOn} onCheckedChange={setSwitchOn} />
              <Label htmlFor="demo-switch" className="font-normal">
                {switchOn ? "Notifications enabled" : "Notifications disabled"}
              </Label>
            </div>

            <div className="space-y-2">
              <div className="flex items-center justify-between">
                <Label>Slider — {sliderVal[0]}%</Label>
              </div>
              <Slider value={sliderVal} onValueChange={setSliderVal} min={0} max={100} step={5} />
            </div>
          </div>
        </div>
      </Section>

      <Separator />

      {/* ── FIELD ────────────────────────────────────────────── */}
      <Section title="Field" subtitle="Form field wrapper with label, error, and validation states">
        <div className="grid grid-cols-2 gap-4">
          <Field>
            <FieldLabel htmlFor="field-normal">Normal field</FieldLabel>
            <Input id="field-normal" placeholder="Enter value…" />
          </Field>
          <Field data-invalid={true}>
            <FieldLabel htmlFor="field-error">Field with error</FieldLabel>
            <Input id="field-error" aria-invalid placeholder="Invalid value" />
            <FieldError errors={[{ message: "This field is required" }]} />
          </Field>
          <Field>
            <FieldLabel htmlFor="field-select">Select field</FieldLabel>
            <Select defaultValue="appliance">
              <SelectTrigger id="field-select"><SelectValue /></SelectTrigger>
              <SelectContent>
                <SelectItem value="appliance">Appliance</SelectItem>
                <SelectItem value="electronics">Electronics</SelectItem>
              </SelectContent>
            </Select>
          </Field>
          <Field>
            <FieldLabel htmlFor="field-textarea">Textarea field</FieldLabel>
            <Textarea id="field-textarea" placeholder="Notes…" rows={2} />
          </Field>
        </div>
      </Section>

      <Separator />

      {/* ── OTP INPUT ────────────────────────────────────────── */}
      <Section title="OTP Input" subtitle="One-time password / PIN entry">
        <DemoRow label="6-digit OTP">
          <InputOTP maxLength={6} value={otpVal} onChange={setOtpVal}>
            <InputOTPGroup>
              <InputOTPSlot index={0} />
              <InputOTPSlot index={1} />
              <InputOTPSlot index={2} />
            </InputOTPGroup>
            <InputOTPSeparator />
            <InputOTPGroup>
              <InputOTPSlot index={3} />
              <InputOTPSlot index={4} />
              <InputOTPSlot index={5} />
            </InputOTPGroup>
          </InputOTP>
        </DemoRow>
        <DemoRow label="4-digit PIN">
          <InputOTP maxLength={4}>
            <InputOTPGroup>
              <InputOTPSlot index={0} />
              <InputOTPSlot index={1} />
              <InputOTPSlot index={2} />
              <InputOTPSlot index={3} />
            </InputOTPGroup>
          </InputOTP>
        </DemoRow>
      </Section>

      <Separator />

      {/* ── COMBOBOX ─────────────────────────────────────────── */}
      <Section title="Combobox" subtitle="Searchable select with autocomplete">
        <div className="grid grid-cols-2 gap-6">
          <div className="space-y-1.5">
            <Label>Framework (plain strings)</Label>
            <Combobox
              items={frameworks}
              value={comboboxVal}
              onValueChange={(v) => setComboboxVal(v as string | null)}
            >
              <ComboboxInput placeholder="Search framework…" showClear />
              <ComboboxContent>
                <ComboboxEmpty>No framework found.</ComboboxEmpty>
                <ComboboxList>
                  {(item: string) => (
                    <ComboboxItem key={item} value={item}>{item}</ComboboxItem>
                  )}
                </ComboboxList>
              </ComboboxContent>
            </Combobox>
          </div>
          <div className="space-y-1.5">
            <Label>Currency (custom objects)</Label>
            <CurrencyCombobox value={currencyVal} onValueChange={setCurrencyVal} />
          </div>
        </div>
      </Section>

      <Separator />

      {/* ── TOGGLE ───────────────────────────────────────────── */}
      <Section title="Toggle & Toggle Groups" subtitle="Single and grouped toggles">
        <DemoRow label="Single Toggle">
          <Toggle aria-label="Bold"><Bold className="size-4" /></Toggle>
          <Toggle aria-label="Italic"><Italic className="size-4" /></Toggle>
          <Toggle aria-label="Underline"><Underline className="size-4" /></Toggle>
          <Toggle variant="outline"><Star className="size-4 mr-1.5" />Favourite</Toggle>
        </DemoRow>
        <DemoRow label="Toggle Group">
          <ToggleGroup type="single" value={toggleVal} onValueChange={(v) => v && setToggleVal(v)}>
            <ToggleGroupItem value="left" aria-label="Left"><AlignLeft className="size-4" /></ToggleGroupItem>
            <ToggleGroupItem value="center" aria-label="Center"><AlignCenter className="size-4" /></ToggleGroupItem>
            <ToggleGroupItem value="right" aria-label="Right"><AlignRight className="size-4" /></ToggleGroupItem>
          </ToggleGroup>
        </DemoRow>
      </Section>

      <Separator />

      {/* ── PROGRESS & SKELETON ──────────────────────────────── */}
      <Section title="Progress & Loading" subtitle="Progress bars, skeletons, spinners">
        <div className="space-y-4">
          <DemoRow label="Progress bars">
            <div className="w-full space-y-2">
              {[25, 50, 68, 90].map((v) => (
                <div key={v} className="flex items-center gap-3">
                  <span className="w-8 text-xs text-muted-foreground text-right tabular-nums">{v}%</span>
                  <Progress value={v} className="flex-1 h-2" />
                </div>
              ))}
            </div>
          </DemoRow>
          <DemoRow label="Spinners">
            <Spinner className="size-3" />
            <Spinner className="size-4" />
            <Spinner className="size-6" />
          </DemoRow>
          <DemoRow label="Skeleton primitives">
            <div className="w-full space-y-2">
              <Skeleton className="h-5 w-1/2" />
              <Skeleton className="h-4 w-3/4" />
              <Skeleton className="h-4 w-2/3" />
              <div className="flex items-center gap-3 mt-3">
                <Skeleton className="size-10 rounded-full" />
                <div className="flex-1 space-y-1.5">
                  <Skeleton className="h-4 w-1/3" />
                  <Skeleton className="h-3 w-1/2" />
                </div>
              </div>
            </div>
          </DemoRow>
        </div>
      </Section>

      <Separator />

      {/* ── SKELETON PATTERNS ────────────────────────────────── */}
      <Section
        title="Skeleton Patterns"
        subtitle="Loading state patterns for stat cards, tables, grids, and detail panels"
      >
        <SkeletonShowcase />
      </Section>

      <Separator />

      {/* ── ALERTS ───────────────────────────────────────────── */}
      <Section title="Alerts" subtitle="Info, warning, success, error states">
        <div className="space-y-3">
          <Alert>
            <Info className="size-4" />
            <AlertTitle>Information</AlertTitle>
            <AlertDescription>Your inventory was last synced 2 hours ago.</AlertDescription>
          </Alert>
          <Alert className="border-status-active/30 bg-status-active/5 text-status-active [&>svg]:text-status-active">
            <CheckCircle2 className="size-4" />
            <AlertTitle>Success</AlertTitle>
            <AlertDescription className="text-foreground/70">Item successfully added to your inventory.</AlertDescription>
          </Alert>
          <Alert className="border-status-expiring/30 bg-status-expiring/5 [&>svg]:text-status-expiring">
            <AlertCircle className="size-4" />
            <AlertTitle className="text-status-expiring">Warning</AlertTitle>
            <AlertDescription>3 warranties expire within the next 60 days.</AlertDescription>
          </Alert>
          <Alert variant="destructive">
            <XCircle className="size-4" />
            <AlertTitle>Error</AlertTitle>
            <AlertDescription>Failed to save changes. Please try again.</AlertDescription>
          </Alert>
        </div>
      </Section>

      <Separator />

      {/* ── CARDS ────────────────────────────────────────────── */}
      <Section title="Cards" subtitle="Content containers with header, body, footer">
        <div className="grid grid-cols-3 gap-4">
          <Card>
            <CardHeader className="pb-2">
              <div className="flex items-start justify-between">
                <div className="flex size-10 items-center justify-center rounded-lg bg-muted">
                  <Package className="size-5 text-muted-foreground" />
                </div>
                <Badge variant="secondary">Electronics</Badge>
              </div>
              <CardTitle className="text-base mt-2">MacBook Pro 16"</CardTitle>
              <CardDescription>Apple · M1 Pro</CardDescription>
            </CardHeader>
            <CardContent className="pb-3">
              <div className="flex items-center justify-between text-sm">
                <span className="text-muted-foreground">Purchase price</span>
                <span className="font-semibold tabular-nums">$2,499</span>
              </div>
            </CardContent>
            <CardFooter className="pt-0">
              <Button variant="outline" size="sm" className="w-full gap-1.5">
                <Edit className="size-3.5" />View Details
              </Button>
            </CardFooter>
          </Card>

          <Card>
            <CardHeader className="pb-3">
              <CardDescription>Total Portfolio Value</CardDescription>
              <CardTitle className="text-3xl font-bold tabular-nums">$9,914</CardTitle>
            </CardHeader>
            <CardContent>
              <div className="flex items-center gap-1.5 text-sm text-status-active">
                <TrendingUp className="size-3.5" />
                <span>+12.4% from last year</span>
              </div>
            </CardContent>
          </Card>

          <Card className="border-dashed flex flex-col items-center justify-center text-center gap-3 py-6">
            <div className="flex size-12 items-center justify-center rounded-xl border-2 border-dashed border-border">
              <Plus className="size-5 text-muted-foreground" />
            </div>
            <div>
              <p className="text-sm font-medium">Add New Item</p>
              <p className="text-xs text-muted-foreground mt-0.5">Track a new possession</p>
            </div>
            <Button size="sm">Get started</Button>
          </Card>
        </div>
      </Section>

      <Separator />

      {/* ── ITEM ─────────────────────────────────────────────── */}
      <Section title="Item" subtitle="Structured list item with media, content, and actions">
        <ItemGroup className="rounded-xl border border-border overflow-hidden">
          {[
            { icon: Package, title: "Washing Machine", desc: "Miele · Laundry Room · Active warranty", badge: "Appliance" },
            { icon: FileText, title: "Sony WH-1000XM5", desc: "Sony · Home Office · Warranty expired", badge: "Electronics" },
            { icon: FolderOpen, title: "DeWalt Drill", desc: "DeWalt · Workshop · No warranty", badge: "Tool" },
          ].map((row, i) => (
            <div key={row.title}>
              {i > 0 && <ItemSeparator />}
              <Item variant="outline" className="rounded-none border-0">
                <ItemMedia variant="icon">
                  <row.icon />
                </ItemMedia>
                <ItemContent>
                  <ItemTitle>{row.title}</ItemTitle>
                  <ItemDescription>{row.desc}</ItemDescription>
                </ItemContent>
                <ItemActions>
                  <Badge variant="secondary">{row.badge}</Badge>
                  <Button variant="ghost" size="icon-sm"><MoreHorizontal className="size-4" /></Button>
                </ItemActions>
              </Item>
            </div>
          ))}
        </ItemGroup>
      </Section>

      <Separator />

      {/* ── TABS ─────────────────────────────────────────────── */}
      <Section title="Tabs" subtitle="Tabbed navigation and content">
        <Tabs defaultValue="overview">
          <TabsList>
            <TabsTrigger value="overview">Overview</TabsTrigger>
            <TabsTrigger value="files">Files</TabsTrigger>
            <TabsTrigger value="warranty">Warranty</TabsTrigger>
            <TabsTrigger value="settings" disabled>Settings</TabsTrigger>
          </TabsList>
          <TabsContent value="overview" className="mt-4">
            <div className="rounded-xl border border-border bg-card p-4 text-sm text-muted-foreground">
              Overview tab — shows item details, purchase info, and location.
            </div>
          </TabsContent>
          <TabsContent value="files" className="mt-4">
            <div className="rounded-xl border border-border bg-card p-4 text-sm text-muted-foreground">
              Files tab — lists attached receipts, manuals, and photos.
            </div>
          </TabsContent>
          <TabsContent value="warranty" className="mt-4">
            <div className="rounded-xl border border-border bg-card p-4 text-sm text-muted-foreground">
              Warranty tab — shows warranty status and expiry date.
            </div>
          </TabsContent>
        </Tabs>
      </Section>

      <Separator />

      {/* ── ACCORDION & COLLAPSIBLE ──────────────────────────── */}
      <Section title="Accordion & Collapsible" subtitle="Collapsible content sections">
        <div className="grid grid-cols-2 gap-6">
          <div>
            <p className="text-xs text-muted-foreground uppercase tracking-wide font-medium mb-3">Accordion</p>
            <Accordion type="single" collapsible className="w-full" defaultValue="item-1">
              <AccordionItem value="item-1">
                <AccordionTrigger>
                  <div className="flex items-center gap-2">
                    <Home className="size-4 text-muted-foreground" />
                    Kitchen Appliances
                  </div>
                </AccordionTrigger>
                <AccordionContent>
                  Refrigerator, Dishwasher, Microwave, and 2 other items tracked in this room.
                </AccordionContent>
              </AccordionItem>
              <AccordionItem value="item-2">
                <AccordionTrigger>
                  <div className="flex items-center gap-2">
                    <Users className="size-4 text-muted-foreground" />
                    Home Office
                  </div>
                </AccordionTrigger>
                <AccordionContent>
                  MacBook Pro, Sony headphones, and external monitor tracked in this room.
                </AccordionContent>
              </AccordionItem>
              <AccordionItem value="item-3">
                <AccordionTrigger>
                  <div className="flex items-center gap-2">
                    <Settings className="size-4 text-muted-foreground" />
                    Workshop
                  </div>
                </AccordionTrigger>
                <AccordionContent>
                  DeWalt Drill, Circular Saw, and other tools tracked in this area.
                </AccordionContent>
              </AccordionItem>
            </Accordion>
          </div>
          <div>
            <p className="text-xs text-muted-foreground uppercase tracking-wide font-medium mb-3">Collapsible</p>
            <Collapsible open={collapsibleOpen} onOpenChange={setCollapsibleOpen} className="rounded-xl border border-border">
              <CollapsibleTrigger className="flex w-full items-center justify-between px-4 py-3 text-sm font-medium">
                <span className="flex items-center gap-2">
                  <MapPin className="size-4 text-muted-foreground" />
                  Main Residence — 3 locations
                </span>
                <ChevronsUpDown className="size-4 text-muted-foreground" />
              </CollapsibleTrigger>
              <CollapsibleContent>
                <Separator />
                {["Kitchen", "Living Room", "Workshop"].map((loc) => (
                  <div key={loc} className="flex items-center gap-2 px-4 py-2.5 text-sm text-muted-foreground">
                    <MapPin className="size-3.5" />{loc}
                  </div>
                ))}
              </CollapsibleContent>
            </Collapsible>
          </div>
        </div>
      </Section>

      <Separator />

      {/* ── MENUS & OVERLAYS ─────────────────────────────────── */}
      <Section title="Menus & Overlays" subtitle="Dropdowns, dialogs, sheets, drawers, popovers, tooltips">
        <div className="flex flex-wrap items-center gap-3">
          {/* Dropdown */}
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button variant="outline" className="gap-1.5">
                Actions <ChevronDown className="size-4" />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="start" className="w-48">
              <DropdownMenuLabel>Item Actions</DropdownMenuLabel>
              <DropdownMenuSeparator />
              <DropdownMenuItem>
                <Edit className="size-4 mr-2" />Edit
                <DropdownMenuShortcut>⌘E</DropdownMenuShortcut>
              </DropdownMenuItem>
              <DropdownMenuItem>
                <Download className="size-4 mr-2" />Export
              </DropdownMenuItem>
              <DropdownMenuItem>
                <Bookmark className="size-4 mr-2" />Bookmark
              </DropdownMenuItem>
              <DropdownMenuSeparator />
              <DropdownMenuCheckboxItem checked={dropdownCheck} onCheckedChange={setDropdownCheck}>
                Show in dashboard
              </DropdownMenuCheckboxItem>
              <DropdownMenuSeparator />
              <DropdownMenuLabel>Filter</DropdownMenuLabel>
              <DropdownMenuRadioGroup value={dropdownRadio} onValueChange={setDropdownRadio}>
                <DropdownMenuRadioItem value="all">All items</DropdownMenuRadioItem>
                <DropdownMenuRadioItem value="active">Active warranty</DropdownMenuRadioItem>
                <DropdownMenuRadioItem value="expired">Expired</DropdownMenuRadioItem>
              </DropdownMenuRadioGroup>
              <DropdownMenuSeparator />
              <DropdownMenuItem className="text-destructive focus:text-destructive">
                <Trash2 className="size-4 mr-2" />Delete
                <DropdownMenuShortcut>⌘⌫</DropdownMenuShortcut>
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>

          {/* Dialog */}
          <Dialog>
            <DialogTrigger asChild>
              <Button variant="outline" className="gap-1.5">
                <Plus className="size-4" />Dialog
              </Button>
            </DialogTrigger>
            <DialogContent>
              <DialogHeader>
                <DialogTitle>Add New Item</DialogTitle>
                <DialogDescription>Fill in the details to track a new possession.</DialogDescription>
              </DialogHeader>
              <div className="space-y-3 py-2">
                <div className="space-y-1.5">
                  <Label>Item Name</Label>
                  <Input placeholder="e.g. Samsung Refrigerator" />
                </div>
                <div className="space-y-1.5">
                  <Label>Category</Label>
                  <Select>
                    <SelectTrigger><SelectValue placeholder="Select…" /></SelectTrigger>
                    <SelectContent>
                      <SelectItem value="appliance">Appliance</SelectItem>
                      <SelectItem value="electronics">Electronics</SelectItem>
                    </SelectContent>
                  </Select>
                </div>
              </div>
              <DialogFooter>
                <Button variant="outline">Cancel</Button>
                <Button>Save Item</Button>
              </DialogFooter>
            </DialogContent>
          </Dialog>

          {/* AlertDialog */}
          <AlertDialog>
            <AlertDialogTrigger asChild>
              <Button variant="destructive" className="gap-1.5">
                <Trash2 className="size-4" />Alert Dialog
              </Button>
            </AlertDialogTrigger>
            <AlertDialogContent>
              <AlertDialogHeader>
                <AlertDialogTitle>Are you absolutely sure?</AlertDialogTitle>
                <AlertDialogDescription>
                  This will permanently delete the item and all its files. This action cannot be undone.
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

          {/* Sheet */}
          <Sheet>
            <SheetTrigger asChild>
              <Button variant="outline">Sheet</Button>
            </SheetTrigger>
            <SheetContent>
              <SheetHeader>
                <SheetTitle>Item Details</SheetTitle>
                <SheetDescription>View and edit the selected item's properties.</SheetDescription>
              </SheetHeader>
              <div className="space-y-4 py-4">
                <div className="space-y-1.5">
                  <Label>Name</Label>
                  <Input defaultValue="Washing Machine" />
                </div>
                <div className="space-y-1.5">
                  <Label>Notes</Label>
                  <Textarea rows={3} placeholder="Notes…" />
                </div>
              </div>
              <SheetFooter>
                <Button size="sm">Save changes</Button>
              </SheetFooter>
            </SheetContent>
          </Sheet>

          {/* Drawer */}
          <Drawer>
            <DrawerTrigger asChild>
              <Button variant="outline">Drawer</Button>
            </DrawerTrigger>
            <DrawerContent>
              <DrawerHeader>
                <DrawerTitle>Add item</DrawerTitle>
                <DrawerDescription>Quickly log a new item from mobile.</DrawerDescription>
              </DrawerHeader>
              <div className="px-4 pb-2 space-y-3">
                <Input placeholder="Item name…" />
                <Select>
                  <SelectTrigger><SelectValue placeholder="Category…" /></SelectTrigger>
                  <SelectContent>
                    <SelectItem value="electronics">Electronics</SelectItem>
                    <SelectItem value="appliance">Appliance</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <DrawerFooter>
                <Button>Save Item</Button>
              </DrawerFooter>
            </DrawerContent>
          </Drawer>

          {/* Popover */}
          <Popover>
            <PopoverTrigger asChild>
              <Button variant="outline" className="gap-1.5">
                <Filter className="size-4" />Popover
              </Button>
            </PopoverTrigger>
            <PopoverContent className="w-64">
              <div className="space-y-3">
                <p className="text-sm font-medium">Filter items</p>
                <div className="space-y-2">
                  {["Appliances", "Electronics", "Tools"].map((cat) => (
                    <div key={cat} className="flex items-center gap-2">
                      <Checkbox id={`pop-${cat}`} />
                      <Label htmlFor={`pop-${cat}`} className="font-normal">{cat}</Label>
                    </div>
                  ))}
                </div>
                <Button size="sm" className="w-full">Apply filters</Button>
              </div>
            </PopoverContent>
          </Popover>

          {/* Tooltip */}
          <Tooltip>
            <TooltipTrigger asChild>
              <Button variant="ghost" size="icon">
                <Info className="size-4" />
              </Button>
            </TooltipTrigger>
            <TooltipContent>
              <p>Hover for contextual help</p>
            </TooltipContent>
          </Tooltip>

          {/* HoverCard */}
          <HoverCard>
            <HoverCardTrigger asChild>
              <Button variant="link" className="gap-1.5">
                <Bell className="size-4" />Hover Card
              </Button>
            </HoverCardTrigger>
            <HoverCardContent className="w-72">
              <div className="flex items-start gap-3">
                <div className="flex size-10 items-center justify-center rounded-full bg-muted">
                  <Bell className="size-5 text-muted-foreground" />
                </div>
                <div>
                  <p className="text-sm font-semibold">Warranty Alerts</p>
                  <p className="text-xs text-muted-foreground mt-1">
                    You have 3 warranties expiring in the next 60 days.
                  </p>
                </div>
              </div>
            </HoverCardContent>
          </HoverCard>

          {/* Toast */}
          <Button
            variant="outline"
            onClick={() => toast("Item saved", { description: "Washing Machine has been added to your inventory." })}
          >
            Toast
          </Button>
        </div>
      </Section>

      <Separator />

      {/* ── COMMAND ──────────────────────────────────────────── */}
      <Section title="Command" subtitle="Command palette / searchable list">
        <div className="rounded-xl border border-border overflow-hidden max-w-sm">
          <Command>
            <CommandInput placeholder="Search items, locations…" />
            <CommandList>
              <CommandEmpty>No results found.</CommandEmpty>
              <CommandGroup heading="Items">
                <CommandItem><Package className="size-4 mr-2 text-muted-foreground" />Washing Machine<CommandShortcut>⌘W</CommandShortcut></CommandItem>
                <CommandItem><Package className="size-4 mr-2 text-muted-foreground" />Sony Headphones</CommandItem>
                <CommandItem><Package className="size-4 mr-2 text-muted-foreground" />MacBook Pro</CommandItem>
              </CommandGroup>
              <CommandSeparator />
              <CommandGroup heading="Navigate">
                <CommandItem><MapPin className="size-4 mr-2 text-muted-foreground" />Locations</CommandItem>
                <CommandItem><Settings className="size-4 mr-2 text-muted-foreground" />Settings</CommandItem>
              </CommandGroup>
            </CommandList>
          </Command>
        </div>
      </Section>

      <Separator />

      {/* ── BREADCRUMB & PAGINATION ──────────────────────────── */}
      <Section title="Breadcrumb & Pagination" subtitle="Navigation breadcrumbs and page controls">
        <DemoRow label="Breadcrumb">
          <Breadcrumb>
            <BreadcrumbList>
              <BreadcrumbItem>
                <BreadcrumbLink href="#">Home</BreadcrumbLink>
              </BreadcrumbItem>
              <BreadcrumbSeparator />
              <BreadcrumbItem>
                <BreadcrumbLink href="#">Main Residence</BreadcrumbLink>
              </BreadcrumbItem>
              <BreadcrumbSeparator />
              <BreadcrumbItem>
                <BreadcrumbLink href="#">Kitchen</BreadcrumbLink>
              </BreadcrumbItem>
              <BreadcrumbSeparator />
              <BreadcrumbItem>
                <BreadcrumbPage>Washing Machine</BreadcrumbPage>
              </BreadcrumbItem>
            </BreadcrumbList>
          </Breadcrumb>
        </DemoRow>
        <DemoRow label="Pagination">
          <Pagination>
            <PaginationContent>
              <PaginationItem>
                <PaginationPrevious href="#" />
              </PaginationItem>
              <PaginationItem>
                <PaginationLink href="#">1</PaginationLink>
              </PaginationItem>
              <PaginationItem>
                <PaginationLink href="#" isActive>2</PaginationLink>
              </PaginationItem>
              <PaginationItem>
                <PaginationLink href="#">3</PaginationLink>
              </PaginationItem>
              <PaginationItem>
                <PaginationEllipsis />
              </PaginationItem>
              <PaginationItem>
                <PaginationLink href="#">8</PaginationLink>
              </PaginationItem>
              <PaginationItem>
                <PaginationNext href="#" />
              </PaginationItem>
            </PaginationContent>
          </Pagination>
        </DemoRow>
      </Section>

      <Separator />

      {/* ── SCROLL AREA ──────────────────────────────────────── */}
      <Section title="Scroll Area" subtitle="Custom-styled scrollable container">
        <div className="grid grid-cols-2 gap-4">
          <div>
            <p className="text-xs text-muted-foreground uppercase tracking-wide font-medium mb-3">Vertical</p>
            <ScrollArea className="h-48 rounded-xl border border-border">
              <div className="p-4 space-y-2">
                {Array.from({ length: 15 }, (_, i) => (
                  <div key={i} className="flex items-center gap-2 text-sm">
                    <Package className="size-3.5 text-muted-foreground shrink-0" />
                    <span>Item {i + 1} — Kitchen Appliance</span>
                  </div>
                ))}
              </div>
            </ScrollArea>
          </div>
          <div>
            <p className="text-xs text-muted-foreground uppercase tracking-wide font-medium mb-3">Horizontal</p>
            <ScrollArea className="rounded-xl border border-border">
              <div className="flex gap-3 p-4 w-max">
                {Array.from({ length: 10 }, (_, i) => (
                  <div key={i} className="flex size-20 shrink-0 items-center justify-center rounded-lg border border-border bg-muted text-xs text-muted-foreground">
                    Area {i + 1}
                  </div>
                ))}
              </div>
            </ScrollArea>
          </div>
        </div>
      </Section>

      <Separator />

      {/* ── RESIZABLE ────────────────────────────────────────── */}
      <Section title="Resizable Panels" subtitle="Draggable panel layout">
        <div className="rounded-xl border border-border overflow-hidden h-40">
          <ResizablePanelGroup>
            <ResizablePanel defaultSize={35} minSize={20}>
              <div className="flex h-full items-center justify-center p-4">
                <div className="text-center">
                  <p className="text-sm font-medium">Sidebar</p>
                  <p className="text-xs text-muted-foreground mt-1">Drag handle →</p>
                </div>
              </div>
            </ResizablePanel>
            <ResizableHandle withHandle />
            <ResizablePanel defaultSize={65}>
              <div className="flex h-full items-center justify-center p-4">
                <div className="text-center">
                  <p className="text-sm font-medium">Main Content</p>
                  <p className="text-xs text-muted-foreground mt-1">Resizable panel</p>
                </div>
              </div>
            </ResizablePanel>
          </ResizablePanelGroup>
        </div>
      </Section>

      <Separator />

      {/* ── AVATARS ──────────────────────────────────────────── */}
      <Section title="Avatars" subtitle="User avatars with fallback initials">
        <DemoRow label="Sizes">
          {[
            { size: "size-6 text-[10px]", initials: "AJ" },
            { size: "size-8 text-xs", initials: "SC" },
            { size: "size-10 text-sm", initials: "MK" },
            { size: "size-14 text-base", initials: "RB" },
          ].map(({ size, initials }) => (
            <Avatar key={size} className={size}>
              <AvatarFallback>{initials}</AvatarFallback>
            </Avatar>
          ))}
        </DemoRow>
        <DemoRow label="Group">
          <div className="flex -space-x-2">
            {["AJ", "SC", "MK", "RB", "+3"].map((i) => (
              <Avatar key={i} className="size-8 text-xs ring-2 ring-background">
                <AvatarFallback>{i}</AvatarFallback>
              </Avatar>
            ))}
          </div>
        </DemoRow>
      </Section>

      <Separator />

      {/* ── TABLE ────────────────────────────────────────────── */}
      <Section title="Table" subtitle="Data display with a table component">
        <div className="rounded-xl border border-border overflow-hidden">
          <Table>
            <TableCaption className="pb-3">Recent items in your inventory</TableCaption>
            <TableHeader>
              <TableRow>
                <TableHead>Item</TableHead>
                <TableHead>Category</TableHead>
                <TableHead>Purchase Price</TableHead>
                <TableHead>Status</TableHead>
                <TableHead className="text-right">Actions</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {tableItems.map((item) => (
                <TableRow key={item.id}>
                  <TableCell>
                    <div>
                      <p className="font-medium text-sm">{item.brand} {item.name}</p>
                      <p className="text-xs text-muted-foreground">{item.model}</p>
                    </div>
                  </TableCell>
                  <TableCell>
                    <Badge variant="secondary" className="capitalize">{item.category}</Badge>
                  </TableCell>
                  <TableCell className="tabular-nums text-sm">
                    {item.purchasePrice ? `$${item.purchasePrice.toLocaleString()}` : "—"}
                  </TableCell>
                  <TableCell>
                    <WarrantyBadge item={item} />
                  </TableCell>
                  <TableCell className="text-right">
                    <Button variant="ghost" size="icon" className="size-7">
                      <MoreHorizontal className="size-4" />
                    </Button>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      </Section>

      <Separator />

      {/* ── KBD ──────────────────────────────────────────────── */}
      <Section title="Keyboard Shortcuts" subtitle="Keyboard indicator component">
        <DemoRow label="Common shortcuts">
          <span className="text-sm text-muted-foreground flex items-center gap-1.5">Add item <Kbd>⌘</Kbd><Kbd>N</Kbd></span>
          <span className="text-sm text-muted-foreground flex items-center gap-1.5">Search <Kbd>⌘</Kbd><Kbd>K</Kbd></span>
          <span className="text-sm text-muted-foreground flex items-center gap-1.5">Toggle sidebar <Kbd>⌘</Kbd><Kbd>B</Kbd></span>
          <span className="text-sm text-muted-foreground flex items-center gap-1.5">Delete <Kbd>⌫</Kbd></span>
        </DemoRow>
      </Section>

      <Separator />

      {/* ── EMPTY STATE ──────────────────────────────────────── */}
      <Section title="Empty State" subtitle="Placeholder when there is no content">
        <div className="rounded-xl border border-border overflow-hidden">
          <Empty>
            <EmptyContent>
              <EmptyHeader>
                <EmptyMedia variant="icon">
                  <Package className="size-6" />
                </EmptyMedia>
                <EmptyTitle>No items yet</EmptyTitle>
                <EmptyDescription>Start by adding your first possession to track it here.</EmptyDescription>
              </EmptyHeader>
              <Button size="sm" className="gap-1.5"><Plus className="size-3.5" />Add item</Button>
            </EmptyContent>
          </Empty>
        </div>
      </Section>

      <Separator />

      {/* ── CHARTS ───────────────────────────────────────────── */}
      <Section title="Charts" subtitle="Data visualization with Recharts">
        <div className="grid grid-cols-2 gap-6">
          <Card>
            <CardHeader>
              <CardTitle className="text-sm font-medium">Items Added per Month</CardTitle>
            </CardHeader>
            <CardContent>
              <ChartContainer config={barConfig} className="h-[160px] w-full">
                <BarChart data={chartData}>
                  <CartesianGrid vertical={false} />
                  <XAxis dataKey="month" tickLine={false} axisLine={false} tick={{ fontSize: 11 }} />
                  <ChartTooltip content={<ChartTooltipContent />} />
                  <Bar dataKey="items" fill="var(--color-items)" radius={4} />
                </BarChart>
              </ChartContainer>
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle className="text-sm font-medium">Portfolio Value Over Time</CardTitle>
            </CardHeader>
            <CardContent>
              <ChartContainer config={barConfig} className="h-[160px] w-full">
                <AreaChart data={chartData}>
                  <CartesianGrid vertical={false} />
                  <XAxis dataKey="month" tickLine={false} axisLine={false} tick={{ fontSize: 11 }} />
                  <ChartTooltip content={<ChartTooltipContent />} />
                  <Area
                    dataKey="value"
                    fill="var(--color-value)"
                    stroke="var(--color-value)"
                    fillOpacity={0.15}
                    type="monotone"
                  />
                </AreaChart>
              </ChartContainer>
            </CardContent>
          </Card>
        </div>
      </Section>

      <Separator />

      {/* ── TYPOGRAPHY ───────────────────────────────────────── */}
      <Section title="Typography" subtitle="Text styles and hierarchy">
        <div className="space-y-3">
          <h1 className="scroll-m-20 text-4xl font-extrabold tracking-tight text-balance">Heading 1 — Extrabold</h1>
          <h2 className="scroll-m-20 text-3xl font-semibold tracking-tight">Heading 2 — Semibold</h2>
          <h3 className="scroll-m-20 text-2xl font-semibold tracking-tight">Heading 3 — Semibold</h3>
          <h4 className="scroll-m-20 text-xl font-semibold tracking-tight">Heading 4 — Semibold</h4>
          <p className="leading-7">Body text — Samsung RF23M8590SG, 36-Month Warranty Active. Serial: 05TZ3CFK800123. Located in Kitchen · Main House.</p>
          <p className="text-xl text-muted-foreground">Lead paragraph — larger muted body text for introductory content.</p>
          <p className="text-sm text-muted-foreground">Small muted — used for captions, helper text, and secondary information.</p>
          <div className="flex flex-wrap gap-3">
            <code className="relative rounded bg-muted px-[0.3rem] py-[0.2rem] font-mono text-sm font-semibold">SN-MEL-2022-00412</code>
            <blockquote className="border-l-2 pl-4 italic text-muted-foreground">
              "Run maintenance cycle monthly."
            </blockquote>
          </div>
        </div>
      </Section>

      <Separator />

      {/* ── COLOR PALETTE ────────────────────────────────────── */}
      <Section title="Color Tokens" subtitle="Design system semantic colors">
        <div className="grid grid-cols-4 gap-3">
          {[
            { name: "Background", bg: "bg-background", border: "border-border" },
            { name: "Card", bg: "bg-card", border: "border-border" },
            { name: "Muted", bg: "bg-muted", border: "" },
            { name: "Primary", bg: "bg-primary", border: "", text: "text-primary-foreground" },
            { name: "Secondary", bg: "bg-secondary", border: "" },
            { name: "Accent", bg: "bg-accent", border: "" },
            { name: "Destructive", bg: "bg-destructive", border: "", text: "text-destructive-foreground" },
            { name: "Border", bg: "bg-border", border: "" },
            { name: "Status Active", bg: "bg-status-active", border: "", text: "text-white" },
            { name: "Status Expiring", bg: "bg-status-expiring", border: "", text: "text-white" },
            { name: "Status Expired", bg: "bg-status-expired", border: "", text: "text-white" },
            { name: "Chart 1–5", bg: "bg-chart-1", border: "", text: "text-white" },
          ].map(({ name, bg, border, text }) => (
            <div key={name} className={cn("rounded-xl p-3 border", bg, border || "border-transparent")}>
              <p className={cn("text-xs font-medium", text ?? "text-foreground")}>{name}</p>
            </div>
          ))}
        </div>
      </Section>

      {/* ── COMING SOON ──────────────────────────────────────── */}
      <Section title="Coming Soon" subtitle="Placeholder for features under development">
        <div className="flex flex-col items-center justify-center gap-3 rounded-xl border border-dashed border-border py-16 bg-muted/20">
          <div className="flex size-12 items-center justify-center rounded-xl bg-muted">
            <Zap className="size-5 text-muted-foreground/50" />
          </div>
          <div className="text-center space-y-1">
            <p className="text-sm font-medium">Feature coming soon</p>
            <p className="text-xs text-muted-foreground">This section is under active development.</p>
          </div>
        </div>
      </Section>

      {/* Toast provider */}
      <Toaster />
    </div>
  )
}

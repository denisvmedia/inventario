import { useState } from "react"
import { Bell, Check, Info, Loader2, Plus, Search, ShieldAlert, Sparkles } from "lucide-react"

import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import { Checkbox } from "@/components/ui/checkbox"
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
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover"
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group"
import { ScrollArea } from "@/components/ui/scroll-area"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { Separator } from "@/components/ui/separator"
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
  SheetTrigger,
} from "@/components/ui/sheet"
import { Skeleton } from "@/components/ui/skeleton"
import { Switch } from "@/components/ui/switch"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { Textarea } from "@/components/ui/textarea"
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip"
import { RouteTitle } from "@/components/routing/RouteTitle"

// UIShowcasePage — dev-only catalog of every shadcn/Radix primitive used in
// the app. Mounted by the router only when `import.meta.env.DEV` is true
// (see app/router.tsx). Designed to be opened during design-system
// migrations to spot regressions in a single screen — not a production
// feature. Loosely mirrors design-mocks/src/views/UIShowcaseView.tsx but
// restricted to the primitives our frontend actually ships
// (frontend/src/components/ui/). #1542 / design-audit #1527.
export function UIShowcasePage() {
  return (
    <>
      <RouteTitle title="UI Showcase (dev)" />
      <div
        className="mx-auto flex w-full max-w-6xl flex-col gap-8 p-6"
        data-testid="page-ui-showcase"
      >
        <header className="space-y-1">
          <div className="flex items-center gap-2">
            <h1 className="scroll-m-20 text-3xl font-semibold tracking-tight">UI Showcase</h1>
            <Badge variant="secondary">dev only</Badge>
          </div>
          <p className="text-sm text-muted-foreground">
            Reference catalog of shadcn/Radix primitives shipped in this app — kept here to spot
            regressions during design-system work.
          </p>
        </header>

        <Tabs defaultValue="buttons">
          <TabsList>
            <TabsTrigger value="buttons">Buttons</TabsTrigger>
            <TabsTrigger value="forms">Forms</TabsTrigger>
            <TabsTrigger value="overlays">Overlays</TabsTrigger>
            <TabsTrigger value="feedback">Feedback</TabsTrigger>
            <TabsTrigger value="layout">Layout</TabsTrigger>
            <TabsTrigger value="typography">Typography</TabsTrigger>
            <TabsTrigger value="tokens">Tokens</TabsTrigger>
          </TabsList>

          <TabsContent value="buttons" className="mt-6">
            <ButtonsSection />
          </TabsContent>
          <TabsContent value="forms" className="mt-6">
            <FormsSection />
          </TabsContent>
          <TabsContent value="overlays" className="mt-6">
            <OverlaysSection />
          </TabsContent>
          <TabsContent value="feedback" className="mt-6">
            <FeedbackSection />
          </TabsContent>
          <TabsContent value="layout" className="mt-6">
            <LayoutSection />
          </TabsContent>
          <TabsContent value="typography" className="mt-6">
            <TypographySection />
          </TabsContent>
          <TabsContent value="tokens" className="mt-6">
            <TokensSection />
          </TabsContent>
        </Tabs>
      </div>
    </>
  )
}

function ShowcaseRow({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div className="space-y-3 rounded-xl border border-border bg-card p-4">
      <p className="text-xs font-semibold uppercase tracking-widest text-muted-foreground">
        {title}
      </p>
      <div className="flex flex-wrap items-center gap-3">{children}</div>
    </div>
  )
}

function ButtonsSection() {
  return (
    <div className="space-y-4">
      <ShowcaseRow title="Variants">
        <Button>Default</Button>
        <Button variant="secondary">Secondary</Button>
        <Button variant="outline">Outline</Button>
        <Button variant="ghost">Ghost</Button>
        <Button variant="link">Link</Button>
        <Button variant="destructive">Destructive</Button>
      </ShowcaseRow>
      <ShowcaseRow title="Sizes">
        <Button size="sm">Small</Button>
        <Button>Default</Button>
        <Button size="lg">Large</Button>
        <Button size="icon" aria-label="add">
          <Plus className="size-4" aria-hidden="true" />
        </Button>
      </ShowcaseRow>
      <ShowcaseRow title="States">
        <Button disabled>Disabled</Button>
        <Button>
          <Loader2 className="size-4 animate-spin" aria-hidden="true" />
          Loading
        </Button>
        <Button>
          <Sparkles className="size-4" aria-hidden="true" />
          With icon
        </Button>
      </ShowcaseRow>
    </div>
  )
}

function FormsSection() {
  const [check, setCheck] = useState(true)
  const [toggle, setToggle] = useState(false)
  const [radio, setRadio] = useState("one")
  return (
    <div className="grid gap-4 md:grid-cols-2">
      <Card>
        <CardHeader>
          <CardTitle>Inputs</CardTitle>
          <CardDescription>Text, password, search, number, textarea</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="space-y-1.5">
            <Label htmlFor="showcase-text">Text</Label>
            <Input id="showcase-text" placeholder="Type something…" />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="showcase-search">Search</Label>
            <div className="relative">
              <Search
                className="pointer-events-none absolute left-2.5 top-1/2 size-4 -translate-y-1/2 text-muted-foreground"
                aria-hidden="true"
              />
              <Input id="showcase-search" placeholder="Search…" className="pl-8" />
            </div>
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="showcase-textarea">Textarea</Label>
            <Textarea id="showcase-textarea" placeholder="Multi-line…" />
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Selectors</CardTitle>
          <CardDescription>Switch, checkbox, radio, select</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="flex items-center justify-between">
            <Label htmlFor="showcase-switch">Switch</Label>
            <Switch id="showcase-switch" checked={toggle} onCheckedChange={setToggle} />
          </div>
          <div className="flex items-center gap-2">
            <Checkbox
              id="showcase-check"
              checked={check}
              onCheckedChange={(v) => setCheck(v === true)}
            />
            <Label htmlFor="showcase-check">Checkbox</Label>
          </div>
          <RadioGroup value={radio} onValueChange={setRadio} className="flex gap-4">
            <div className="flex items-center gap-2">
              <RadioGroupItem value="one" id="r-one" />
              <Label htmlFor="r-one">One</Label>
            </div>
            <div className="flex items-center gap-2">
              <RadioGroupItem value="two" id="r-two" />
              <Label htmlFor="r-two">Two</Label>
            </div>
          </RadioGroup>
          <div className="space-y-1.5">
            <Label htmlFor="showcase-select">Select</Label>
            <Select>
              <SelectTrigger id="showcase-select">
                <SelectValue placeholder="Pick one" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="apple">Apple</SelectItem>
                <SelectItem value="banana">Banana</SelectItem>
                <SelectItem value="cherry">Cherry</SelectItem>
              </SelectContent>
            </Select>
          </div>
        </CardContent>
      </Card>
    </div>
  )
}

function OverlaysSection() {
  return (
    <div className="grid gap-4 md:grid-cols-2">
      <ShowcaseRow title="Dialog">
        <Dialog>
          <DialogTrigger asChild>
            <Button variant="outline">Open dialog</Button>
          </DialogTrigger>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>Dialog title</DialogTitle>
              <DialogDescription>Sample modal dialog</DialogDescription>
            </DialogHeader>
            <p className="text-sm text-muted-foreground">Body content goes here.</p>
            <DialogFooter>
              <Button variant="ghost">Cancel</Button>
              <Button>Confirm</Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      </ShowcaseRow>
      <ShowcaseRow title="Sheet (right slide-over)">
        <Sheet>
          <SheetTrigger asChild>
            <Button variant="outline">Open sheet</Button>
          </SheetTrigger>
          <SheetContent>
            <SheetHeader>
              <SheetTitle>Sheet title</SheetTitle>
              <SheetDescription>Slide-over panel.</SheetDescription>
            </SheetHeader>
          </SheetContent>
        </Sheet>
      </ShowcaseRow>
      <ShowcaseRow title="Popover">
        <Popover>
          <PopoverTrigger asChild>
            <Button variant="outline">Open popover</Button>
          </PopoverTrigger>
          <PopoverContent className="text-sm">Anchored popover content.</PopoverContent>
        </Popover>
      </ShowcaseRow>
      <ShowcaseRow title="Tooltip">
        <Tooltip>
          <TooltipTrigger asChild>
            <Button variant="outline">Hover me</Button>
          </TooltipTrigger>
          <TooltipContent>Helpful hint</TooltipContent>
        </Tooltip>
      </ShowcaseRow>
      <ShowcaseRow title="Dropdown menu">
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button variant="outline">Open menu</Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent>
            <DropdownMenuLabel>Group</DropdownMenuLabel>
            <DropdownMenuSeparator />
            <DropdownMenuItem>One</DropdownMenuItem>
            <DropdownMenuItem>Two</DropdownMenuItem>
            <DropdownMenuItem>Three</DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </ShowcaseRow>
    </div>
  )
}

function FeedbackSection() {
  return (
    <div className="space-y-4">
      <Alert>
        <Info className="size-4" aria-hidden="true" />
        <AlertTitle>Heads up</AlertTitle>
        <AlertDescription>Informational alert content.</AlertDescription>
      </Alert>
      <Alert variant="destructive">
        <ShieldAlert className="size-4" aria-hidden="true" />
        <AlertTitle>Destructive</AlertTitle>
        <AlertDescription>Something went wrong.</AlertDescription>
      </Alert>
      <ShowcaseRow title="Badges">
        <Badge>Default</Badge>
        <Badge variant="secondary">Secondary</Badge>
        <Badge variant="outline">Outline</Badge>
        <Badge variant="destructive">Destructive</Badge>
      </ShowcaseRow>
      <ShowcaseRow title="Status pills (semantic tokens)">
        <span className="inline-flex items-center gap-1.5 rounded-full bg-status-active/10 px-2.5 py-0.5 text-xs font-medium text-status-active">
          <Check className="size-3" aria-hidden="true" />
          Active
        </span>
        <span className="inline-flex items-center gap-1.5 rounded-full bg-status-expiring/10 px-2.5 py-0.5 text-xs font-medium text-status-expiring">
          <Bell className="size-3" aria-hidden="true" />
          Expiring
        </span>
        <span className="inline-flex items-center gap-1.5 rounded-full bg-status-expired/10 px-2.5 py-0.5 text-xs font-medium text-status-expired">
          Expired
        </span>
      </ShowcaseRow>
      <ShowcaseRow title="Skeleton">
        <Skeleton className="h-6 w-32" />
        <Skeleton className="h-6 w-48" />
        <Skeleton className="h-12 w-12 rounded-full" />
      </ShowcaseRow>
    </div>
  )
}

function LayoutSection() {
  return (
    <div className="grid gap-4 md:grid-cols-2">
      <Card>
        <CardHeader>
          <CardTitle>Card</CardTitle>
          <CardDescription>Header / content / footer slots</CardDescription>
        </CardHeader>
        <CardContent>Card body content.</CardContent>
        <CardFooter className="gap-2">
          <Button variant="ghost">Cancel</Button>
          <Button>Save</Button>
        </CardFooter>
      </Card>
      <Card>
        <CardHeader>
          <CardTitle>ScrollArea</CardTitle>
          <CardDescription>Long-list scroll container</CardDescription>
        </CardHeader>
        <CardContent>
          <ScrollArea className="h-32 rounded-md border">
            <ul className="space-y-1 p-3 text-sm">
              {Array.from({ length: 20 }, (_, i) => (
                <li key={i}>Item {i + 1}</li>
              ))}
            </ul>
          </ScrollArea>
        </CardContent>
      </Card>
      <Card className="md:col-span-2">
        <CardHeader>
          <CardTitle>Separator</CardTitle>
        </CardHeader>
        <CardContent className="space-y-3 text-sm">
          <p>Section A</p>
          <Separator />
          <p>Section B</p>
        </CardContent>
      </Card>
    </div>
  )
}

function TypographySection() {
  return (
    <div className="space-y-4 rounded-xl border border-border bg-card p-6">
      <h1 className="scroll-m-20 text-3xl font-semibold tracking-tight">
        Heading 1 — Page title (3xl, semibold)
      </h1>
      <h2 className="text-2xl font-bold tracking-tight">Heading 2 — Section (2xl, bold)</h2>
      <h3 className="text-xl font-semibold tracking-tight">Heading 3 — Subsection (xl)</h3>
      <h4 className="text-lg font-semibold">Heading 4 — Card title (lg)</h4>
      <p className="text-base">Body — base size for paragraphs.</p>
      <p className="text-sm text-muted-foreground">
        Muted small — descriptions, helper text, secondary metadata.
      </p>
      <p className="text-xs text-muted-foreground">Caption — timestamps, labels.</p>
      <p className="font-mono text-sm">Mono — short_name, slugs, technical identifiers.</p>
    </div>
  )
}

function TokensSection() {
  const palette: Array<{ label: string; className: string }> = [
    { label: "background", className: "bg-background border" },
    { label: "card", className: "bg-card border" },
    { label: "muted", className: "bg-muted" },
    { label: "primary", className: "bg-primary" },
    { label: "secondary", className: "bg-secondary" },
    { label: "accent", className: "bg-accent" },
    { label: "destructive", className: "bg-destructive" },
    { label: "border", className: "bg-border" },
    { label: "status-active", className: "bg-status-active" },
    { label: "status-expiring", className: "bg-status-expiring" },
    { label: "status-expired", className: "bg-status-expired" },
  ]
  return (
    <div className="space-y-4">
      <p className="text-sm text-muted-foreground">
        Semantic colour tokens used by the design system. Light and dark themes both resolve to the
        same names; the underlying colour switches based on `[data-theme]`.
      </p>
      <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 md:grid-cols-4">
        {palette.map(({ label, className }) => (
          <div
            key={label}
            className="flex flex-col items-center gap-2 rounded-xl border border-border p-3"
          >
            <div className={`size-14 rounded-md ${className}`} aria-hidden="true" />
            <span className="font-mono text-xs">{label}</span>
          </div>
        ))}
      </div>
    </div>
  )
}

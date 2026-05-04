# Component Patterns

Anatomy and rules for every reusable component primitive in Inventario.
Engineering reference; if it's here, it's specced. If a sprint task
needs a component not on this list, add it here first.

The implementation rules (where to put files, when to extract, named
exports, etc.) live in `../components.md`. This doc is the *visual*
spec — what each primitive looks like, what variants exist, where the
spacing comes from.

## Button

```tsx
import { Button } from "@/components/ui/button"

<Button>Default</Button>
<Button variant="outline" size="sm">Small outline</Button>
<Button variant="ghost" size="icon"><Plus className="size-4" /></Button>
<Button variant="destructive" size="sm" className="gap-1.5">
  <Trash2 className="size-3.5" />
  Delete
</Button>
```

| Variant | When |
| --- | --- |
| `default` (filled primary) | The page's main CTA. Exactly one per surface. |
| `outline` | Secondary actions, modal Cancel, "Save changes" inside a card. |
| `ghost` | Tertiary actions, kebab triggers, sidebar items. |
| `destructive` | Delete / discard / leave. Always paired with an `<AlertDialog>` confirmation. |
| `secondary` | Rare. Use `outline` first. |
| `link` | Text-only inline, e.g. "Forgot password?" |

Sizes: `default` (h-9), `sm` (h-8), `lg` (h-10), `xs` (h-6),
`icon` (size-9), `icon-sm` (size-8), `icon-xs` (size-6).

Icon sizing per button size — see `06-iconography-and-illustration.md`.

## Badge

```tsx
import { Badge } from "@/components/ui/badge"
<Badge>Default</Badge>
<Badge variant="outline">Outline</Badge>
```

The status-badge pattern (domain-specific):

```tsx
<Badge variant="outline" className={`${cfg.color} ${cfg.bg} border-current/20 font-medium`}>
  <Icon className="size-3" />
  {cfg.label}
</Badge>
```

`cfg` comes from `WARRANTY_STATUS_CONFIG` / `COMMODITY_STATUS_CONFIG`
(`features/commodities/constants.ts`). Always use the config map; never
inline ternaries.

## Tag pill (`TagBadge`)

Tag pills use a closed-enum color set (`--tag-amber`, `--tag-green`,
`--tag-blue`, `--tag-orange`, `--tag-red`, `--tag-muted`), not chart
colors. Anatomy:

```tsx
<span className="inline-flex items-center gap-1 rounded-full border px-2.5 py-0.5 text-xs font-medium bg-tag-amber/15 text-tag-amber border-tag-amber/30">
  <Hash className="size-2.5 shrink-0" />
  kitchen
</span>
```

Mirror `models.TagColor` on the backend. The picker UI uses the
six-swatch chip row; see `frontend/src/components/tags/`.

## Input + Label

```tsx
<div className="space-y-1.5">
  <Label htmlFor="email">{t("auth:login.email")}</Label>
  <Input id="email" type="email" {...form.register("email")} />
  {error && <p className="text-xs text-destructive">{t(error.message ?? "")}</p>}
</div>
```

- `space-y-1.5` (6px) between label, input, error. Always.
- `<Label htmlFor>` programmatically pairs to the input's `id`.
- `aria-invalid={!!error}` on the input when there's an error;
  shadcn's `Input` styles the error state via `aria-invalid` selectors.

See `15-form-and-data-ux.md` for the validation timing.

## Select

```tsx
<Select value={value} onValueChange={setValue}>
  <SelectTrigger><SelectValue /></SelectTrigger>
  <SelectContent>
    <SelectItem value="x">x</SelectItem>
  </SelectContent>
</Select>
```

For currency fields specifically, use `<CurrencyCombobox>` instead —
it has search and full names.

## Dialog

```tsx
<Dialog open={open} onOpenChange={(o) => !o && onClose()}>
  <DialogContent className="sm:max-w-md">
    <DialogHeader>
      <DialogTitle className="flex items-center gap-2">
        <div className="flex size-7 items-center justify-center rounded-lg bg-primary/10">
          <Icon className="size-4 text-primary" />
        </div>
        Dialog Title
      </DialogTitle>
      <DialogDescription>Supporting text.</DialogDescription>
    </DialogHeader>
    <div className="space-y-4">{/* body */}</div>
    <DialogFooter className="gap-2">
      <Button variant="outline" onClick={onClose}>Cancel</Button>
      <Button onClick={onConfirm}>Confirm</Button>
    </DialogFooter>
  </DialogContent>
</Dialog>
```

Width:

- `sm:max-w-md` for confirmations and 1–3 fields.
- `sm:max-w-lg` for ≤5 fields.
- `sm:max-w-2xl` for multi-step wizards.
- Above `2xl` → use a Sheet or a full page.

Title icon: 28×28 tile (`size-7`) with a 16×16 (`size-4`) icon at
`text-primary` over `bg-primary/10`. Optional but consistent.

## AlertDialog

```tsx
<AlertDialog open={!!deleteId} onOpenChange={(o) => !o && setDeleteId(null)}>
  <AlertDialogContent>
    <AlertDialogHeader>
      <AlertDialogTitle>Delete item</AlertDialogTitle>
      <AlertDialogDescription>This action cannot be undone.</AlertDialogDescription>
    </AlertDialogHeader>
    <AlertDialogFooter>
      <AlertDialogCancel>Cancel</AlertDialogCancel>
      <AlertDialogAction
        onClick={handleDelete}
        className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
      >
        Delete
      </AlertDialogAction>
    </AlertDialogFooter>
  </AlertDialogContent>
</AlertDialog>
```

Always for destructive flows. Always paired with a destructive `Button`
trigger. The `useConfirm()` hook (`src/hooks/useConfirm.tsx`) wraps
this in a promise — prefer it over re-implementing the open state.

## Sheet

Right-sliding panel for previews and edits. Anatomy:

```tsx
<Sheet open={open} onOpenChange={setOpen}>
  <SheetContent side="right" className="sm:max-w-xl">
    <SheetHeader>
      <SheetTitle>{title}</SheetTitle>
    </SheetHeader>
    <div className="space-y-6 py-6">{/* body */}</div>
    <SheetFooter>{/* actions */}</SheetFooter>
  </SheetContent>
</Sheet>
```

Widths per use:

- Quick preview: `sm:max-w-md` (448px).
- Edit form: `sm:max-w-xl` (576px).
- Long form: `sm:max-w-2xl` (672px).

## DropdownMenu

```tsx
<DropdownMenu>
  <DropdownMenuTrigger asChild>
    <Button variant="ghost" size="icon" aria-label={t("common:actions.more")}>
      <MoreHorizontal className="size-4" />
    </Button>
  </DropdownMenuTrigger>
  <DropdownMenuContent align="end">
    <DropdownMenuItem>Edit</DropdownMenuItem>
    <DropdownMenuSeparator />
    <DropdownMenuItem className="text-destructive focus:text-destructive">
      <Trash2 className="size-4 mr-2" />
      Delete
    </DropdownMenuItem>
  </DropdownMenuContent>
</DropdownMenu>
```

Destructive items are styled inline with `text-destructive
focus:text-destructive` — Radix doesn't have a built-in destructive
variant. Group destructive items at the bottom, separated by a
`DropdownMenuSeparator`.

## Tooltip

```tsx
<TooltipProvider delayDuration={300}>
  <Tooltip>
    <TooltipTrigger asChild>
      <Button size="icon" variant="ghost"><Info className="size-4" /></Button>
    </TooltipTrigger>
    <TooltipContent>{t("…explanation")}</TooltipContent>
  </Tooltip>
</TooltipProvider>
```

- 300ms delay (the default). No instant tooltips.
- Used to *supplement* a label, never replace it. See
  `14-accessibility.md`.
- Don't tooltip text already visible.

## Popover

For density-rich pickers (filter, date range, color). Anatomy mirrors
DropdownMenu but the body is freeform JSX. `align="start"` for filters
that anchor to a leading element.

## Card

```tsx
<div className="rounded-xl border border-border bg-card p-6 space-y-5">
  <div>
    <h2 className="text-base font-semibold">{title}</h2>
    <p className="text-sm text-muted-foreground mt-0.5">{subtitle}</p>
  </div>
  <div className="space-y-4">{/* rows */}</div>
</div>
```

`rounded-xl`, 1px `border-border`, `bg-card`, `p-6`, `space-y-5`. No
shadow.

## Stat card

```tsx
<div className="rounded-xl border border-border bg-card px-4 py-3 flex items-center gap-3">
  <div className="flex size-8 items-center justify-center rounded-lg bg-muted shrink-0">
    <Icon className="size-4 text-muted-foreground" />
  </div>
  <div>
    <p className="text-xs text-muted-foreground">{label}</p>
    <p className="text-lg font-semibold leading-tight">{value}</p>
  </div>
</div>
```

`size-8` icon tile, `size-4` icon. Don't drift to `size-10` /
`size-5`.

## List row (settings / detail)

```tsx
<div className="flex items-center justify-between py-3.5">
  <div>
    <p className="text-sm font-medium">{label}</p>
    {hint && <p className="text-xs text-muted-foreground">{hint}</p>}
  </div>
  <div className="flex items-center gap-2">{/* control */}</div>
</div>
```

Wrapped in a `divide-y divide-border` container (see
`03-space-and-layout.md`).

## List row (data — items / locations / files)

Click target = full row (button or link, depending). Hover bg, kebab
revealed on hover or always-visible if it's the only action.

```tsx
<button
  onClick={() => onOpen(item.id)}
  className="flex w-full items-center gap-3 px-4 py-3 text-left hover:bg-muted/40 active:bg-muted/60 transition-colors"
>
  <Icon className="size-4 text-muted-foreground shrink-0" />
  <span className="text-sm font-medium flex-1 truncate">{item.title}</span>
  <span className="text-xs text-muted-foreground tabular-nums">{item.value}</span>
  <ChevronRight className="size-4 text-muted-foreground" />
</button>
```

Bulk-actionable lists: add `selected` state per row and a sticky
toolbar above. See `08-interaction-states.md` ("Selection").

## Switch row

```tsx
<div className="flex items-center justify-between py-3.5">
  <p className="text-sm font-medium">Warranty expiring alerts</p>
  <Switch checked={val} onCheckedChange={setVal} />
</div>
```

Settings rows save **immediately** on change — no "Save" button
afterward. Use a debounced PATCH and a sonner toast on failure.

## Skeleton

```tsx
<div className="space-y-3">
  <Skeleton className="h-4 w-1/3" />
  <Skeleton className="h-4 w-1/2" />
  <Skeleton className="h-32 w-full" />
</div>
```

Skeleton shape matches the rendered shape. Don't show a generic
spinner.

## Sonner toast

Wrapped in `useAppToast()` (`src/hooks/useAppToast.ts`):

```ts
const toast = useAppToast()
toast.success(t("commodities:toast.created"))
toast.error(t("auth:login.errorGeneric"))
```

Severity → behavior:

| Severity | Auto-dismiss | Color |
| --- | --- | --- |
| `info` | 4s | foreground |
| `success` | 4s | `--status-active` |
| `warning` | 6s | `--status-expiring` |
| `error` | manual | `--destructive` |

See `16-notifications-and-trust.md`.

## Empty state (component family)

`src/components/dashboard/EmptyState.tsx` etc. Shape:

```tsx
<div className="flex flex-col items-center justify-center gap-3 py-16">
  <div className="flex size-10 items-center justify-center rounded-lg bg-primary/10">
    <Icon className="size-5 text-primary" />
  </div>
  <div className="text-center space-y-1">
    <p className="text-base font-semibold">{title}</p>
    <p className="text-sm text-muted-foreground">{body}</p>
  </div>
  {cta && <Button size="sm" onClick={cta.onClick}>{cta.label}</Button>}
</div>
```

See `20-edge-cases.md` for the empty-state taxonomy.

## Coming-soon banner (transitional)

For features whose backend is in flight. The dialog or page has a
banner above the content saying "This is read-only until #NNNN
ships." Component lives at `src/components/coming-soon/`. Use it
sparingly — ship the real feature instead when possible.

## Sidebar / TopBar (Shell)

Owned by `src/app/Shell.tsx` + `src/components/AppSidebar.tsx`. The
Shell composes:

- `<Sidebar>` (collapsible at md breakpoint).
- `<TopBar>` (logo on mobile, title + actions on desktop).
- `<Outlet>` (the routed page).

Sidebar nav items come from a const array (`NAV_ENTRIES`) gated by
permissions. Adding a route to nav = adding the entry + the
translation under `common:nav.*`.

## Hard rules

1. **Use the primitive.** `<Button>` over `<button>`, `<Input>` over
   `<input>`, `<Dialog>` over a hand-rolled overlay.
2. **`cn(base, className)` for prop-driven classes.** No template
   literals.
3. **`asChild` for primitive composition.** When a Radix primitive
   needs to pass behavior to your own element, use `asChild` (e.g.
   `<DropdownMenuTrigger asChild><Button…/></DropdownMenuTrigger>`).
4. **Settings save immediately.** Don't wrap a switch row in a
   "Save" button.
5. **One filled-primary per page.** Multiple primary buttons fight
   for attention.

## Anti-patterns

- Two filled-primary buttons on the same surface ("Save" and "Submit").
- A confirmation dialog with no title or description.
- A dropdown with destructive actions interleaved with safe ones, no
  separator.
- A toast that takes a `<button>` for a CTA. Toasts are read-only —
  the action lives elsewhere.
- A list row that's clickable except for the kebab — make the row a
  `<button>` and the kebab a `<DropdownMenu>` with `stopPropagation`
  on the trigger.

## Cross-refs

- Engineering rules (where files live, when to extract): `../components.md`.
- Form fields specifically: `15-form-and-data-ux.md`.
- States across all primitives: `08-interaction-states.md`.
- Mock canonical: `denisvmedia/inventario-design/CLAUDE.md` §6
  (Component Patterns).

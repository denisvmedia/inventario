# Design mock fidelity — pattern micro-library

Paste-ready TSX for every recurring surface. Values match `design-mocks/` verbatim. See `SKILL.md` for workflow, rules, and the surface index.

## Badge — neutral

```tsx
<Badge>Default</Badge>
<Badge variant="secondary">Secondary</Badge>
<Badge variant="outline">Outline</Badge>
```

## Badge — status (domain pattern)

The mock uses `WARRANTY_STATUS_CONFIG[status].color / .bg / .label` and the same shape for commodity status. The frontend has *split* these into two differently-shaped constants:

- **Warranty:** `WARRANTY_STATUS_CONFIG` in `frontend/src/components/warranty/config.ts` — fields are `{ i18nKey, icon, text, bg, bgSolid, border }`. Note `text` (not `color`) for the foreground class, and `i18nKey` instead of a literal label so the chip is translatable.
- **Commodity:** `COMMODITY_STATUS_TONES` in `frontend/src/features/commodities/constants.ts` — a flat `Record<status, string>` of pre-joined utility strings (`"text-status-active border-status-active/30 bg-status-active/10"`), no separate fields. Pair with the `commodities:status.*` i18n namespace for the label.

Reach for the existing `WarrantyBadge` component for warranty surfaces — don't compose a chip from scratch. For commodity status, the canonical pattern in the frontend is:

```tsx
const tone = status ? COMMODITY_STATUS_TONES[status] : ""
<Badge variant="outline" className={cn(tone, "border-current/20 font-medium gap-1")}>
  <Icon className="size-3" />
  {status ? t(`commodities:status.${status}`) : ""}
</Badge>
```

For warranty status outside `WarrantyBadge` (e.g. dashboard breakdowns), pull from `WARRANTY_STATUS_CONFIG[status]`:

```tsx
const visual = WARRANTY_STATUS_CONFIG[status]
const Icon = visual.icon
<Badge variant="outline" className={cn(visual.text, visual.bg, visual.border, "font-medium gap-1")}>
  <Icon className="size-3" />
  {t(visual.i18nKey)}
</Badge>
```

## Tag pill (with `lucide` `Hash` glyph, color from `chart-*` cycle)

```tsx
<span className="inline-flex items-center gap-1 rounded-full border px-2.5 py-0.5 text-xs font-medium select-none bg-chart-1/15 text-chart-1 border-chart-1/30">
  <Hash className="size-2.5 shrink-0" />
  kitchen
</span>
```

## Button — sizes & icon scale (icon size MUST match button size)

| Button size | Class | Icon size |
|---|---|---|
| `default` | `h-9` | `size-4` |
| `sm` | `h-8` | `size-3.5` |
| `lg` | `h-10` | `size-4` |
| `xs` | `h-6` | `size-3` |
| `icon` | `size-9` | `size-4` |
| `icon-sm` | `size-8` | `size-3.5` |
| `icon-xs` | `size-6` | `size-3` |

```tsx
<Button>Default</Button>
<Button variant="outline" size="sm">Small outline</Button>
<Button variant="ghost" size="icon" aria-label="Add"><Plus className="size-4" /></Button>
<Button variant="destructive" size="sm" className="gap-1.5">
  <Trash2 className="size-3.5" />
  Delete
</Button>
```

## Field with label (no validation)

```tsx
<div className="space-y-1.5">
  <Label htmlFor="field-id">Label</Label>
  <Input id="field-id" placeholder="Enter value…" value={val} onChange={(e) => setVal(e.target.value)} />
</div>
```

## Field with validation (RHF + Zod)

The actual frontend pattern — see `devdocs/frontend/forms.md` and any `pages/*Page.tsx`.

The repo does not have shadcn `Field`/`FieldLabel`/`FieldError` primitives. Schemas live in `frontend/src/features/<name>/schemas.ts` and their `message` fields are *i18n keys* (`"auth:validation.emailRequired"`), not English strings — the page resolves them with `t()` at render time. The field shape:

```tsx
<div className="space-y-1.5">
  <Label htmlFor="profile-name">{t("auth:fields.name")}</Label>
  <Input
    id="profile-name"
    aria-invalid={!!form.formState.errors.name}
    {...form.register("name")}
  />
  {form.formState.errors.name ? (
    <p className="field-error text-xs text-destructive">
      {t(form.formState.errors.name.message ?? "")}
    </p>
  ) : null}
</div>
```

`field-error` is a class hook for tests / styling overrides; keep it on every error `<p>`. Pair with `data-testid` on both the input and the error node when the form has tests.

## Save button placement (always at the bottom of a form section, in `pt-2`)

```tsx
<div className="pt-2">
  <Button size="sm">Save changes</Button>
</div>
```

## Dialog (icon-headed title is the house style)

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
    {/* body */}
    <DialogFooter className="gap-2">
      <Button variant="outline" onClick={onClose}>Cancel</Button>
      <Button onClick={onConfirm}>Confirm</Button>
    </DialogFooter>
  </DialogContent>
</Dialog>
```

## AlertDialog (destructive) — never `window.confirm`

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

## Dropdown menu (with destructive-tinted item)

```tsx
<DropdownMenu>
  <DropdownMenuTrigger asChild>
    <Button variant="ghost" size="icon" aria-label="Actions"><MoreHorizontal className="size-4" /></Button>
  </DropdownMenuTrigger>
  <DropdownMenuContent align="end">
    <DropdownMenuItem>Edit</DropdownMenuItem>
    <DropdownMenuSeparator />
    <DropdownMenuItem className="text-destructive focus:text-destructive">
      <Trash2 className="size-4 mr-2" />Delete
    </DropdownMenuItem>
  </DropdownMenuContent>
</DropdownMenu>
```

## Empty state — inline (in a list/card)

```tsx
<div className="flex flex-col items-center justify-center gap-3 py-16">
  <Icon className="size-8 text-muted-foreground/30" />
  <p className="text-sm text-muted-foreground">Nothing here yet.</p>
</div>
```

## Empty state — full-page

Use the named exports from `design-mocks/src/views/EmptyStatesView.tsx` as the recipe. Patterns: `NotFoundView`, `NoLocationGroupView`, `NoGroupOnboardingView`, `NoLocationView`, `NoAreaView`, `MaintenanceView`. Don't compose your own — port the matching one.

## Hoverable list row (Dashboard "Expiring Warranties" pattern)

```tsx
<button
  className="flex w-full items-center justify-between px-6 py-3.5 text-left transition-colors hover:bg-muted/50"
  onClick={() => onItemClick(item.id)}
>
  <div>
    <p className="text-sm font-medium">{item.name}</p>
    <p className="text-xs text-muted-foreground">{item.brand} · {areaName(item.areaId)}</p>
  </div>
  <Badge variant="outline" className="text-status-expiring bg-status-expiring/10 border-current/20 shrink-0 ml-4">
    {days} days left
  </Badge>
</button>
```

## Reveal-on-hover actions (rows that show extra controls only when hovered)

```tsx
<div className="group flex items-center justify-between py-3 hover:bg-muted/40">
  <span className="text-sm">Item</span>
  <div className="flex gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
    <Button size="icon-sm" variant="ghost" aria-label="Edit"><Pencil className="size-3.5" /></Button>
    <Button size="icon-sm" variant="ghost" aria-label="Delete"><Trash2 className="size-3.5" /></Button>
  </div>
</div>
```

## Sidebar nav group (the canonical AppSidebar pattern)

Open `design-mocks/src/components/AppSidebar.tsx` and copy the `<SidebarGroup>` block — that file is short and you'll want to read it in full for the user-dropdown footer + rail + collapsed-mode classes (`group-data-[collapsible=icon]:*`).

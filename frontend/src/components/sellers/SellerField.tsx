import { useRef, useState, type ReactNode } from "react"
import { useTranslation } from "react-i18next"
import { Plus, Sparkles } from "lucide-react"

import { PortalEventBoundary } from "@/components/PortalEventBoundary"
import { SellerFormDialog } from "@/components/sellers/SellerFormDialog"
import { Button } from "@/components/ui/button"
import { Label } from "@/components/ui/label"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectSeparator,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { findSellerByName, type Seller } from "@/features/sellers/api"
import { useCreateSeller, useSellers } from "@/features/sellers/hooks"
import type { SellerFormValues } from "@/features/sellers/schemas"
import { useAppToast } from "@/hooks/useAppToast"
import { withId } from "@/lib/with-id"

// The value of the "New seller" entry. It is never stored: picking it opens
// the create dialog instead.
const CREATE_VALUE = "__create__"

export interface SellerFieldProps {
  // The selected seller's id, "" for none.
  value: string
  onChange: (sellerId: string) => void
  // The select's id, for its label.
  id: string
  error?: { message?: string }
  // A seller name the AI scan read that matches no seller of the group. The
  // field offers to create it; nothing is created until the user agrees.
  suggestion?: string
  // Called once the suggestion was used or a seller was picked anyway.
  onSuggestionDone?: () => void
  // Renders the label, for a form whose labels differ from the plain one.
  renderLabel?: (htmlFor: string, text: string) => ReactNode
}

// SellerField picks the seller an item was bought from (#2804). The list ends
// with "New seller", which opens the Sellers page's create dialog over the
// form and selects what it created, as the Location picker does for places
// (#2796). Sellers are group content, so every member who may edit the item
// may create one.
export function SellerField({
  value,
  onChange,
  id,
  error,
  suggestion = "",
  onSuggestionDone,
  renderLabel = defaultLabel,
}: SellerFieldProps) {
  const { t } = useTranslation()
  const toast = useAppToast()
  const sellers = useSellers()
  const createSeller = useCreateSeller()
  // Sellers created here, kept until the list includes them.
  const [created, setCreated] = useState<Seller[]>([])
  const listed = sellers.data ?? []
  const all = [...listed, ...created.filter((c) => !listed.some((s) => s.id === c.id))]

  // The list closes before the dialog opens, or the dialog would open under
  // it; a letter typed on the closed select picks without opening the list,
  // and then the dialog opens at once.
  const listOpen = useRef(false)
  const requested = useRef(false)
  const trigger = useRef<HTMLButtonElement>(null)
  const [dialog, setDialog] = useState<{ id: number; name: string } | null>(null)

  // Escape closes the dialog while its create still runs, and the dialog can
  // open again before the create ends. The number of the opening a create
  // was sent from tells whether that dialog is still open when it ends.
  const lastOpening = useRef(0)
  const openNow = useRef(0)

  function openDialog(name: string) {
    lastOpening.current += 1
    openNow.current = lastOpening.current
    setDialog({ id: lastOpening.current, name })
  }

  function closeDialog(opening: number) {
    if (openNow.current !== opening) return
    openNow.current = 0
    setDialog(null)
  }

  function pick(next: string) {
    if (next === CREATE_VALUE) {
      if (listOpen.current) requested.current = true
      else openDialog("")
      return
    }
    onChange(next)
    onSuggestionDone?.()
  }

  function openRequested() {
    if (!requested.current) return
    requested.current = false
    openDialog("")
  }

  // A seller of the group may carry the suggested name by now, created
  // elsewhere or after the scan; then the suggestion offers to pick it.
  const suggested = suggestion ? findSellerByName(all, suggestion) : undefined

  const opening = dialog?.id ?? 0
  async function submit(values: SellerFormValues) {
    const seller = await createSeller.mutateAsync(values)
    toast.success(t("sellers:toast.created"))
    setCreated((prev) => [...prev, seller])
    // Selected only while its dialog is still open, so a create that ends
    // after the dialog closed leaves a choice made meanwhile alone.
    if (openNow.current === opening) {
      onChange(seller.id)
      onSuggestionDone?.()
    }
  }

  return (
    <div className="flex flex-col gap-1.5">
      <div className="flex items-center justify-between gap-2">
        {renderLabel(id, t("commodities:fields.seller"))}
        {value ? (
          <button
            type="button"
            onClick={() => onChange("")}
            className="text-xs text-muted-foreground transition-colors hover:text-foreground"
            data-testid={`${id}-clear`}
          >
            {t("commodities:fields.clearSeller")}
          </button>
        ) : null}
      </div>
      {/* An empty string, not undefined, keeps the select controlled while it
          shows the placeholder, so the create entry never sticks as its value. */}
      <Select
        value={value}
        onValueChange={pick}
        onOpenChange={(open) => {
          listOpen.current = open
          if (open) requested.current = false
        }}
      >
        <SelectTrigger id={id} ref={trigger} className="w-full" aria-invalid={!!error}>
          <SelectValue placeholder={t("commodities:fields.sellerPlaceholder")} />
        </SelectTrigger>
        <SelectContent onCloseAutoFocus={openRequested}>
          {withId(all).map((s) => (
            <SelectItem key={s.id} value={s.id}>
              {s.name}
            </SelectItem>
          ))}
          {all.length > 0 ? <SelectSeparator /> : null}
          <SelectItem value={CREATE_VALUE} data-testid={`${id}-new`}>
            <Plus aria-hidden="true" />
            {t("commodities:fields.newSeller")}
          </SelectItem>
        </SelectContent>
      </Select>
      {error?.message ? (
        <p className="text-xs text-destructive" role="alert">
          {t(error.message)}
        </p>
      ) : null}
      {suggestion && !value ? (
        <div
          className="flex flex-wrap items-center gap-2 rounded-md border border-dashed px-3 py-2 text-xs text-muted-foreground"
          data-testid={`${id}-suggestion`}
        >
          <Sparkles aria-hidden="true" className="size-3.5 shrink-0" />
          <span className="min-w-0 flex-1">
            {t("commodities:fields.sellerScanned", { name: suggestion })}
          </span>
          {suggested ? (
            <Button
              type="button"
              size="sm"
              variant="outline"
              className="h-7"
              onClick={() => pick(suggested.id)}
              data-testid={`${id}-suggestion-use`}
            >
              {t("commodities:fields.sellerUseExisting", { name: suggested.name })}
            </Button>
          ) : (
            <Button
              type="button"
              size="sm"
              variant="outline"
              className="h-7"
              onClick={() => openDialog(suggestion)}
              data-testid={`${id}-suggestion-create`}
            >
              {t("commodities:fields.sellerCreateScanned")}
            </Button>
          )}
        </div>
      ) : null}
      <PortalEventBoundary>
        <SellerFormDialog
          open={dialog !== null}
          onOpenChange={(open) => {
            if (!open) closeDialog(opening)
          }}
          defaultName={dialog?.name ?? ""}
          onSubmit={submit}
          isPending={createSeller.isPending}
          onCloseAutoFocus={(event) => {
            event.preventDefault()
            trigger.current?.focus()
          }}
        />
      </PortalEventBoundary>
    </div>
  )
}

function defaultLabel(htmlFor: string, text: string) {
  return (
    <Label htmlFor={htmlFor} className="text-sm font-medium">
      {text}
    </Label>
  )
}

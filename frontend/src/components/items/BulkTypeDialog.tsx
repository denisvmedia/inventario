import { useState } from "react"
import { useTranslation } from "react-i18next"

import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Label } from "@/components/ui/label"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import {
  COMMODITY_TYPES,
  COMMODITY_TYPE_ICONS,
  type CommodityTypeValue,
} from "@/features/commodities/constants"

export interface BulkTypeDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  /** How many items the change applies to. */
  count: number
  onSubmit: (type: CommodityTypeValue) => Promise<void>
  isPending?: boolean
}

// Sets the type of several items.
export function BulkTypeDialog(props: BulkTypeDialogProps) {
  return (
    <Dialog open={props.open} onOpenChange={props.onOpenChange}>
      {props.open ? <BulkTypeForm {...props} /> : null}
    </Dialog>
  )
}

function BulkTypeForm({ onOpenChange, count, onSubmit, isPending = false }: BulkTypeDialogProps) {
  const { t } = useTranslation()
  const [type, setType] = useState<CommodityTypeValue | "">("")

  return (
    <DialogContent className="sm:max-w-md" data-testid="bulk-type-dialog">
      <DialogHeader>
        <DialogTitle>{t("commodities:bulk.typeTitle", { count })}</DialogTitle>
        <DialogDescription>{t("commodities:bulk.typeDescription")}</DialogDescription>
      </DialogHeader>
      <form
        className="flex flex-col gap-4"
        onSubmit={(e) => {
          e.preventDefault()
          if (type) void onSubmit(type)
        }}
      >
        <div className="flex flex-col gap-1.5">
          <Label htmlFor="bulk-type-select">{t("commodities:fields.type")}</Label>
          <Select value={type || undefined} onValueChange={(v) => setType(v as CommodityTypeValue)}>
            <SelectTrigger id="bulk-type-select" className="w-full" data-testid="bulk-type-select">
              <SelectValue placeholder={t("commodities:fields.typePlaceholder")} />
            </SelectTrigger>
            <SelectContent>
              {COMMODITY_TYPES.map((tp) => {
                const Icon = COMMODITY_TYPE_ICONS[tp]
                return (
                  <SelectItem key={tp} value={tp}>
                    <Icon aria-hidden="true" />
                    {t(`commodities:type.${tp}`)}
                  </SelectItem>
                )
              })}
            </SelectContent>
          </Select>
        </div>
        <DialogFooter>
          <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
            {t("common:actions.cancel")}
          </Button>
          <Button type="submit" disabled={isPending || !type} data-testid="bulk-type-confirm">
            {t("commodities:bulk.typeConfirm")}
          </Button>
        </DialogFooter>
      </form>
    </DialogContent>
  )
}

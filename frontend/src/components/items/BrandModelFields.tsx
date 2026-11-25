import { useTranslation } from "react-i18next"
import { Controller, type Control, type FieldErrors, type UseFormRegister } from "react-hook-form"

import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import {
  BRAND_MAX_LEN,
  MODEL_MAX_LEN,
  type CommodityFormInput,
} from "@/features/commodities/schemas"

import { BrandInput } from "./BrandInput"

interface BrandModelFieldsProps {
  control: Control<CommodityFormInput>
  register: UseFormRegister<CommodityFormInput>
  errors: FieldErrors<CommodityFormInput>
  // Prefix of the input ids: "commodity" in the step-by-step dialog,
  // "edit" in the item's edit form.
  idPrefix?: string
}

// The Brand and Model inputs side by side, as in the mock's Add Item and
// edit forms (design-mocks AddItemDialog.tsx BasicsStep, ItemDetail.tsx
// ItemEditForm). The brand suggests the brands the group already uses.
export function BrandModelFields({
  control,
  register,
  errors,
  idPrefix = "commodity",
}: BrandModelFieldsProps) {
  const { t } = useTranslation()
  const brandId = `${idPrefix}-brand`
  const modelId = `${idPrefix}-model`
  return (
    <div className="grid grid-cols-2 gap-3">
      <div className="flex flex-col gap-1.5">
        <Label htmlFor={brandId}>{t("commodities:fields.brand")}</Label>
        <Controller
          control={control}
          name="brand"
          render={({ field }) => (
            <BrandInput
              id={brandId}
              name={field.name}
              value={field.value ?? ""}
              onChange={field.onChange}
              onBlur={field.onBlur}
              maxLength={BRAND_MAX_LEN}
              placeholder={t("commodities:fields.brandPlaceholder")}
              aria-invalid={!!errors.brand}
              data-testid="commodity-brand-input"
            />
          )}
        />
        {errors.brand?.message ? (
          <p className="text-xs text-destructive" role="alert">
            {t(errors.brand.message)}
          </p>
        ) : null}
      </div>
      <div className="flex flex-col gap-1.5">
        <Label htmlFor={modelId}>{t("commodities:fields.model")}</Label>
        <Input
          id={modelId}
          autoComplete="off"
          maxLength={MODEL_MAX_LEN}
          placeholder={t("commodities:fields.modelPlaceholder")}
          {...register("model")}
          aria-invalid={!!errors.model}
          data-testid="commodity-model-input"
        />
        {errors.model?.message ? (
          <p className="text-xs text-destructive" role="alert">
            {t(errors.model.message)}
          </p>
        ) : null}
      </div>
    </div>
  )
}

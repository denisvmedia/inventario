import { FieldContextKey } from "vee-validate"
import { computed, inject } from "vue"
import { FORM_ITEM_INJECTION_KEY } from "./injectionKeys"

export function useFormField() {
  const fieldContext = inject(FieldContextKey)
  const fieldItemContext = inject(FORM_ITEM_INJECTION_KEY)

  if (!fieldContext)
    throw new Error("useFormField should be used within <FormField>")

  if (!fieldItemContext)
    throw new Error("useFormField should be used within <FormItem>")

  const { name, errorMessage: error, meta } = fieldContext
  const id = fieldItemContext

  const fieldState = {
    valid: computed(() => meta.valid),
    isDirty: computed(() => meta.dirty),
    isTouched: computed(() => meta.touched),
    error,
  }

  // Use the FormItem id directly as the control id so explicit
  // overrides (e.g. <FormItem id="name">) anchor the underlying
  // input to a stable DOM id that legacy Playwright e2e selectors
  // such as `#name` can find. See devdocs/frontend/migration-conventions.md.
  return {
    id,
    name,
    formItemId: id,
    formDescriptionId: `${id}-description`,
    formMessageId: `${id}-message`,
    ...fieldState,
  }
}

<script setup lang="ts">
/**
 * LocationCreateView — migrated to the design system in Phase 4 of
 * Epic #1324 (issue #1329).
 *
 * Page chrome (header, form section, error toasts) is built from
 * `@design/*` patterns. Form is wired through vee-validate + zod with
 * the shared `LocationForm.schema.ts`. Server-side validation errors
 * are mapped onto the form via `setErrors`.
 *
 * Legacy `.location-create` class anchor preserved as a no-op marker
 * for Playwright stability — see
 * devdocs/frontend/migration-conventions.md.
 */
import { useRouter } from 'vue-router'
import { useForm } from 'vee-validate'
import { toTypedSchema } from '@vee-validate/zod'

import locationService from '@/services/locationService'
import { useGroupStore } from '@/stores/groupStore'
import { getErrorMessage } from '@/utils/errorUtils'

import { Button } from '@design/ui/button'
import { Input } from '@design/ui/input'
import {
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@design/ui/form'
import PageContainer from '@design/patterns/PageContainer.vue'
import PageHeader from '@design/patterns/PageHeader.vue'
import { useAppToast } from '@design/composables/useAppToast'

import { locationFormSchema, type LocationFormInput } from './LocationForm.schema'

const router = useRouter()
const groupStore = useGroupStore()
const toast = useAppToast()

const { handleSubmit, isSubmitting, setErrors } = useForm<LocationFormInput>({
  validationSchema: toTypedSchema(locationFormSchema),
  initialValues: { name: '', address: '' },
})

const onSubmit = handleSubmit(async (values) => {
  try {
    await locationService.createLocation({
      data: {
        type: 'locations',
        attributes: { name: values.name.trim(), address: values.address.trim() },
      },
    })
    router.push(groupStore.groupPath('/locations'))
  } catch (err) {
    if (applyApiFieldErrors(err)) return
    toast.error(getErrorMessage(err as never, 'location', 'Failed to create location'))
  }
})

function applyApiFieldErrors(err: unknown): boolean {
  const apiErrors = (err as { response?: { data?: { errors?: Array<{ source?: { pointer?: string }; detail?: string }> } } })
    .response?.data?.errors
  if (!Array.isArray(apiErrors) || apiErrors.length === 0) return false
  const fieldErrors: Record<string, string> = {}
  for (const apiError of apiErrors) {
    const field = apiError.source?.pointer?.split('/').pop()
    if (field && apiError.detail) fieldErrors[field] = apiError.detail
  }
  if (Object.keys(fieldErrors).length === 0) return false
  setErrors(fieldErrors)
  return true
}

function cancel() {
  router.push(groupStore.groupPath('/locations'))
}
</script>

<template>
  <PageContainer as="div" class="location-create mx-auto max-w-2xl">
    <PageHeader title="Create New Location" />

    <form
      class="flex flex-col gap-6 rounded-md border border-border bg-card p-6 shadow-sm"
      data-testid="location-create-form"
      @submit="onSubmit"
    >
      <FormField v-slot="{ componentField }" name="name">
        <FormItem>
          <FormLabel required>Name</FormLabel>
          <FormControl>
            <Input
              v-bind="componentField"
              type="text"
              placeholder="Enter location name"
              data-testid="location-create-form-name"
            />
          </FormControl>
          <FormMessage />
        </FormItem>
      </FormField>

      <FormField v-slot="{ componentField }" name="address">
        <FormItem>
          <FormLabel required>Address</FormLabel>
          <FormControl>
            <Input
              v-bind="componentField"
              type="text"
              placeholder="Enter location address"
              data-testid="location-create-form-address"
            />
          </FormControl>
          <FormMessage />
        </FormItem>
      </FormField>

      <div class="flex justify-end gap-2">
        <Button type="button" variant="outline" @click="cancel">Cancel</Button>
        <Button
          type="submit"
          :disabled="isSubmitting"
          data-testid="location-create-form-submit"
        >
          {{ isSubmitting ? 'Creating...' : 'Create Location' }}
        </Button>
      </div>
    </form>
  </PageContainer>
</template>

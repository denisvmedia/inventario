<script setup lang="ts">
/**
 * LocationEditView — migrated to the design system in Phase 4 of
 * Epic #1324 (issue #1329).
 *
 * Page chrome (header, form section, NotFound branch, error toasts)
 * is built from `@design/*` patterns. Form is wired through
 * vee-validate + zod with the shared `LocationForm.schema.ts`. The
 * 404 branch reuses the legacy `ResourceNotFound` component for now
 * (a `NotFoundCard` design pattern is planned for a follow-up).
 *
 * Legacy `.location-edit` class anchor preserved as a no-op marker
 * for Playwright stability — see
 * devdocs/frontend/migration-conventions.md.
 */
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useForm } from 'vee-validate'
import { toTypedSchema } from '@vee-validate/zod'

import locationService from '@/services/locationService'
import { useGroupStore } from '@/stores/groupStore'
import {
  is404Error as checkIs404Error,
  get404Message,
  get404Title,
  getErrorMessage,
} from '@/utils/errorUtils'

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

import ResourceNotFound from '@/components/ResourceNotFound.vue'

import { locationFormSchema, type LocationFormInput } from './LocationForm.schema'

type ApiResource = { id: string; attributes: Record<string, unknown> }

const route = useRoute()
const router = useRouter()
const groupStore = useGroupStore()
const toast = useAppToast()

const id = route.params.id as string
const loading = ref<boolean>(true)
const lastError = ref<unknown>(null)
const is404 = computed(() => !!lastError.value && checkIs404Error(lastError.value as never))

const { handleSubmit, isSubmitting, setErrors, setValues } = useForm<LocationFormInput>({
  validationSchema: toTypedSchema(locationFormSchema),
  initialValues: { name: '', address: '' },
})

async function loadLocation(): Promise<void> {
  loading.value = true
  lastError.value = null
  try {
    const response = await locationService.getLocation(id)
    const resource = response.data.data as ApiResource
    setValues({
      name: (resource.attributes.name as string) ?? '',
      address: (resource.attributes.address as string) ?? '',
    })
  } catch (err) {
    lastError.value = err
    if (!checkIs404Error(err as never)) {
      toast.error(getErrorMessage(err as never, 'location', 'Failed to load location'))
    }
  } finally {
    loading.value = false
  }
}

const onSubmit = handleSubmit(async (values) => {
  try {
    await locationService.updateLocation(id, {
      data: {
        id,
        type: 'locations',
        attributes: { name: values.name.trim(), address: values.address.trim() },
      },
    })
    router.push(groupStore.groupPath(`/locations/${id}`))
  } catch (err) {
    if (applyApiFieldErrors(err)) return
    toast.error(getErrorMessage(err as never, 'location', 'Failed to update location'))
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

function goBack() {
  router.go(-1)
}

function goBackToList() {
  router.push(groupStore.groupPath('/locations'))
}

onMounted(loadLocation)
</script>

<template>
  <PageContainer as="div" class="location-edit mx-auto max-w-2xl">
    <div v-if="loading" class="py-12 text-center text-sm text-muted-foreground">Loading...</div>

    <ResourceNotFound
      v-else-if="is404"
      resource-type="location"
      :title="get404Title('location')"
      :message="get404Message('location')"
      go-back-text="Back to Locations"
      @go-back="goBackToList"
      @try-again="loadLocation"
    />

    <template v-else>
      <!-- `header` is a strangler-fig anchor preserved for
           `e2e/tests/includes/user-isolation-auth.ts:393`, which waits
           for `.header` to confirm successful access to the edit
           page (legacy template wrapped the title block in
           `<div class="header">`). -->
      <PageHeader class="header" title="Edit Location">
        <template #actions>
          <Button variant="outline" @click="goBack">Go Back</Button>
        </template>
      </PageHeader>

      <form
        class="flex flex-col gap-6 rounded-md border border-border bg-card p-6 shadow-sm"
        data-testid="location-edit-form"
        @submit="onSubmit"
      >
        <FormField v-slot="{ componentField }" name="name">
          <FormItem id="name">
            <FormLabel required>Name</FormLabel>
            <FormControl>
              <Input
                v-bind="componentField"
                type="text"
                placeholder="Enter location name"
                data-testid="location-edit-form-name"
              />
            </FormControl>
            <FormMessage />
          </FormItem>
        </FormField>

        <FormField v-slot="{ componentField }" name="address">
          <FormItem id="address">
            <FormLabel required>Address</FormLabel>
            <FormControl>
              <Input
                v-bind="componentField"
                type="text"
                placeholder="Enter location address"
                data-testid="location-edit-form-address"
              />
            </FormControl>
            <FormMessage />
          </FormItem>
        </FormField>

        <div class="flex justify-end gap-2">
          <Button type="button" variant="outline" @click="goBack">Cancel</Button>
          <Button
            type="submit"
            :disabled="isSubmitting"
            data-testid="location-edit-form-submit"
          >
            {{ isSubmitting ? 'Saving...' : 'Save Changes' }}
          </Button>
        </div>
      </form>
    </template>
  </PageContainer>
</template>


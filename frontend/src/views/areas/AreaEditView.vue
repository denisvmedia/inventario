<script setup lang="ts">
/**
 * AreaEditView — migrated to the design system in Phase 4 of Epic
 * #1324 (issue #1329).
 *
 * Page chrome (header, form section, NotFound branch, error toasts)
 * is built from `@design/*` patterns. Form is wired through
 * vee-validate + zod with the shared `AreaForm.schema.ts` (the
 * `areaEditFormSchema` variant that adds `location_id`). The 404
 * branch reuses the legacy `ResourceNotFound` component.
 *
 * Legacy `.area-edit` class anchor preserved as a no-op marker for
 * Playwright stability — see
 * devdocs/frontend/migration-conventions.md.
 */
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useForm } from 'vee-validate'
import { toTypedSchema } from '@vee-validate/zod'

import areaService from '@/services/areaService'
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
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@design/ui/select'
import PageContainer from '@design/patterns/PageContainer.vue'
import PageHeader from '@design/patterns/PageHeader.vue'
import { useAppToast } from '@design/composables/useAppToast'

import ResourceNotFound from '@/components/ResourceNotFound.vue'

import { areaEditFormSchema, type AreaEditFormInput } from './AreaForm.schema'

type ApiResource = { id: string; attributes: Record<string, unknown>; relationships?: Record<string, { data?: { id: string } }> }

const route = useRoute()
const router = useRouter()
const groupStore = useGroupStore()
const toast = useAppToast()

const id = route.params.id as string
const loading = ref<boolean>(true)
const lastError = ref<unknown>(null)
const is404 = computed(() => !!lastError.value && checkIs404Error(lastError.value as never))
const locations = ref<ApiResource[]>([])

const { handleSubmit, isSubmitting, setErrors, setValues, values } = useForm<AreaEditFormInput>({
  validationSchema: toTypedSchema(areaEditFormSchema),
  initialValues: { name: '', location_id: '' },
})

async function loadData(): Promise<void> {
  loading.value = true
  lastError.value = null
  try {
    const [areaResponse, locationsResponse] = await Promise.all([
      areaService.getArea(id),
      locationService.getLocations(),
    ])
    const area = areaResponse.data.data as ApiResource
    locations.value = (locationsResponse.data.data as ApiResource[]) ?? []
    const locationId =
      (area.attributes.location_id as string | undefined) ??
      area.relationships?.location?.data?.id ??
      ''
    setValues({
      name: (area.attributes.name as string) ?? '',
      location_id: locationId,
    })
  } catch (err) {
    lastError.value = err
    if (!checkIs404Error(err as never)) {
      toast.error(getErrorMessage(err as never, 'area', 'Failed to load area'))
    }
  } finally {
    loading.value = false
  }
}

const onSubmit = handleSubmit(async (formValues) => {
  try {
    await areaService.updateArea(id, {
      data: {
        id,
        type: 'areas',
        attributes: {
          name: formValues.name.trim(),
          location_id: formValues.location_id,
        },
      },
    })
    router.push({
      path: groupStore.groupPath('/locations'),
      query: { areaId: id, locationId: formValues.location_id },
    })
  } catch (err) {
    if (applyApiFieldErrors(err)) return
    toast.error(getErrorMessage(err as never, 'area', 'Failed to update area'))
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
  router.push({
    path: groupStore.groupPath('/locations'),
    query: { areaId: id, locationId: values.location_id },
  })
}

function goBackToList() {
  router.push(groupStore.groupPath('/locations'))
}

onMounted(loadData)
</script>

<template>
  <PageContainer as="div" class="area-edit mx-auto max-w-2xl">
    <div v-if="loading" class="py-12 text-center text-sm text-muted-foreground">Loading...</div>

    <ResourceNotFound
      v-else-if="is404"
      resource-type="area"
      :title="get404Title('area')"
      :message="get404Message('area')"
      go-back-text="Back to Locations"
      @go-back="goBackToList"
      @try-again="loadData"
    />

    <template v-else>
      <PageHeader title="Edit Area">
        <template #actions>
          <Button variant="outline" @click="goBack">Go Back</Button>
        </template>
      </PageHeader>

      <form
        class="flex flex-col gap-6 rounded-md border border-border bg-card p-6 shadow-sm"
        data-testid="area-edit-form"
        @submit="onSubmit"
      >
        <FormField v-slot="{ componentField }" name="name">
          <FormItem>
            <FormLabel required>Name</FormLabel>
            <FormControl>
              <Input
                v-bind="componentField"
                type="text"
                placeholder="Enter area name"
                data-testid="area-edit-form-name"
              />
            </FormControl>
            <FormMessage />
          </FormItem>
        </FormField>

        <FormField v-slot="{ componentField, value }" name="location_id">
          <FormItem>
            <FormLabel required>Location</FormLabel>
            <FormControl>
              <Select
                :model-value="value as string | undefined"
                @update:model-value="(v) => componentField['onUpdate:modelValue'](v)"
              >
                <SelectTrigger class="w-full" data-testid="area-edit-form-location">
                  <SelectValue placeholder="Select a location" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem
                    v-for="loc in locations"
                    :key="loc.id"
                    :value="loc.id"
                  >
                    {{ loc.attributes.name }}
                  </SelectItem>
                </SelectContent>
              </Select>
            </FormControl>
            <FormMessage />
          </FormItem>
        </FormField>

        <div class="flex justify-end gap-2">
          <Button type="button" variant="outline" @click="goBack">Cancel</Button>
          <Button
            type="submit"
            :disabled="isSubmitting"
            data-testid="area-edit-form-submit"
          >
            {{ isSubmitting ? 'Saving...' : 'Save Changes' }}
          </Button>
        </div>
      </form>
    </template>
  </PageContainer>
</template>

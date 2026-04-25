<script setup lang="ts">
/**
 * LocationListView — migrated to the design system in Phase 4 of
 * Epic #1324 (issue #1329).
 *
 * Page chrome (header, sections, banners, empty state, area cards,
 * inline create forms, error toasts, confirm dialogs) is built from
 * `@design/*` patterns. The expandable per-location row is kept
 * inline because it has list-view-specific behaviour (areas-under-
 * location with "Add Area" toggle).
 *
 * Legacy CSS class anchors (`location-list`, `location-card`,
 * `location-container`, `areas-container`, `area-card`,
 * `area-highlight`, `new-location-button`, `grand-total-card`) are
 * preserved as no-op markers so existing Playwright selectors keep
 * resolving through the strangler-fig migration window — see
 * devdocs/frontend/migration-conventions.md.
 */
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { toTypedSchema } from '@vee-validate/zod'
import { ChevronDown, ChevronRight, Eye, Pencil, Plus, Trash2, X } from 'lucide-vue-next'

import locationService from '@/services/locationService'
import areaService from '@/services/areaService'
import valueService from '@/services/valueService'
import { fetchAll } from '@/utils/paginationUtils'
import { useSettingsStore } from '@/stores/settingsStore'
import { useGroupStore } from '@/stores/groupStore'
import { formatPrice } from '@/services/currencyService'
import { getErrorMessage } from '@/utils/errorUtils'

import { Button } from '@design/ui/button'
import { Input } from '@design/ui/input'
import { Card } from '@design/ui/card'
import {
  Form,
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@design/ui/form'
import EmptyState from '@design/patterns/EmptyState.vue'
import IconButton from '@design/patterns/IconButton.vue'
import PageContainer from '@design/patterns/PageContainer.vue'
import PageHeader from '@design/patterns/PageHeader.vue'
import AreaCard from '@design/patterns/AreaCard.vue'
import { useAppToast } from '@design/composables/useAppToast'
import { useConfirm } from '@design/composables/useConfirm'

import PaginationControls from '@/components/PaginationControls.vue'

import { locationFormSchema, type LocationFormInput } from './LocationForm.schema'
import { areaFormSchema as areaZodSchema, type AreaFormInput } from '@/views/areas/AreaForm.schema'

type AnyRecord = Record<string, unknown>
type ApiResource = { id: string; attributes: AnyRecord }

const route = useRoute()
const router = useRouter()
const settingsStore = useSettingsStore()
const groupStore = useGroupStore()
const toast = useAppToast()
const { confirmDelete } = useConfirm()

const locations = ref<ApiResource[]>([])
const areas = ref<ApiResource[]>([])
const loading = ref<boolean>(true)

const currentPage = ref(1)
const pageSize = ref(50)
const totalLocations = ref(0)
const totalPages = computed(() => Math.ceil(totalLocations.value / pageSize.value))

const areaTotals = ref<Array<{ id: string; value: string | number }>>([])
const locationTotals = ref<Array<{ id: string; value: string | number }>>([])
const globalTotal = ref<number>(0)
const valuesLoading = ref<boolean>(true)

const mainCurrency = computed(() => settingsStore.mainCurrency)

const showLocationForm = ref(false)
const showAreaFormForLocation = ref<string | null>(null)
const expandedLocations = ref<string[]>([])
const areaToFocus = ref<string | null>(null)

async function loadValues() {
  valuesLoading.value = true
  try {
    const response = await valueService.getValues()
    const data = (response.data?.data?.attributes ?? {}) as AnyRecord
    if (data.global_total !== undefined && data.global_total !== null) {
      globalTotal.value =
        typeof data.global_total === 'string' ? parseFloat(data.global_total) : (data.global_total as number)
    }
    areaTotals.value = normaliseTotals(data.area_totals)
    locationTotals.value = normaliseTotals(data.location_totals)
  } catch (err) {
    toast.error(getErrorMessage(err as never, 'value', 'Failed to load inventory values'))
  } finally {
    valuesLoading.value = false
  }
}

function normaliseTotals(input: unknown): Array<{ id: string; value: string | number }> {
  if (Array.isArray(input)) return input as Array<{ id: string; value: string | number }>
  if (input && typeof input === 'object') {
    return Object.entries(input as Record<string, string | number>).map(([id, value]) => ({ id, value }))
  }
  return []
}

function getAreaValueLabel(areaId: string): string {
  if (valuesLoading.value) return 'Loading...'
  const entry = areaTotals.value.find((a) => a.id === areaId)
  if (!entry) return `0.00 ${mainCurrency.value}`
  const v = typeof entry.value === 'string' ? parseFloat(entry.value) : entry.value
  return isNaN(v) ? `0.00 ${mainCurrency.value}` : formatPrice(v, mainCurrency.value)
}

function getLocationValueLabel(locationId: string): string {
  if (valuesLoading.value) return 'Loading...'
  const entry = locationTotals.value.find((l) => l.id === locationId)
  if (!entry) return `0.00 ${mainCurrency.value}`
  const v = typeof entry.value === 'string' ? parseFloat(entry.value) : entry.value
  return isNaN(v) ? `0.00 ${mainCurrency.value}` : formatPrice(v, mainCurrency.value)
}

async function loadLocations() {
  loading.value = true
  try {
    const [locationsResponse, allAreas] = await Promise.all([
      locationService.getLocations({ page: currentPage.value, per_page: pageSize.value }),
      fetchAll((params) => areaService.getAreas(params)),
      loadValues(),
    ])
    locations.value = locationsResponse.data.data
    totalLocations.value = locationsResponse.data.meta.locations
    areas.value = allAreas

    const areaId = route.query.areaId as string
    const locationId = route.query.locationId as string
    if (areaId && locationId) {
      if (!expandedLocations.value.includes(locationId)) {
        expandedLocations.value.push(locationId)
      }
      areaToFocus.value = areaId
      await nextTick()
      scrollToArea(areaId)
    } else if (locations.value.length === 1) {
      expandedLocations.value = [locations.value[0].id]
    }
  } catch (err) {
    toast.error(getErrorMessage(err as never, 'location', 'Failed to load locations'))
  } finally {
    loading.value = false
  }
}

function scrollToArea(areaId: string) {
  const el = document.getElementById(`area-${areaId}`)
  if (!el) return
  el.scrollIntoView({ behavior: 'smooth', block: 'center' })
  el.classList.add('area-highlight')
  window.setTimeout(() => el.classList.remove('area-highlight'), 2000)
}

function toggleLocationExpanded(locationId: string) {
  if (expandedLocations.value.includes(locationId)) {
    expandedLocations.value = expandedLocations.value.filter((id) => id !== locationId)
  } else {
    expandedLocations.value.push(locationId)
  }
}

function toggleAreaForm(locationId: string) {
  showAreaFormForLocation.value = showAreaFormForLocation.value === locationId ? null : locationId
}

function getAreasForLocation(locationId: string): ApiResource[] {
  return areas.value.filter((area) => (area.attributes as AnyRecord).location_id === locationId)
}

// Two inline forms (location + area) live in the same view. To keep
// each form's vee-validate context isolated we use the `<Form>`
// component (which scopes its own provide/inject) instead of two
// `useForm()` calls in the same setup. Submit handlers below receive
// values plus a `SubmissionContext` from the slot so they can reset
// the form or surface server-side field errors.
const locationFormSchemaTyped = toTypedSchema(locationFormSchema)
const areaFormSchemaTyped = toTypedSchema(areaZodSchema)

interface SubmissionContext {
  setErrors: (_errors: Record<string, string>) => void
  resetForm: (_state?: { values?: Record<string, unknown> }) => void
}

async function onLocationSubmit(
  values: LocationFormInput,
  ctx: SubmissionContext,
): Promise<void> {
  try {
    const response = await locationService.createLocation({
      data: {
        type: 'locations',
        attributes: { name: values.name.trim(), address: values.address.trim() },
      },
    })
    const newLocation = response.data.data as ApiResource
    showLocationForm.value = false
    expandedLocations.value.push(newLocation.id)
    ctx.resetForm({ values: { name: '', address: '' } })
    await loadLocations()
  } catch (err) {
    if (applyApiFieldErrors(err, ctx.setErrors)) return
    toast.error(getErrorMessage(err as never, 'location', 'Failed to create location'))
  }
}

function cancelLocationForm(resetForm: SubmissionContext['resetForm']): void {
  resetForm({ values: { name: '', address: '' } })
  showLocationForm.value = false
}

async function onAreaSubmit(
  values: AreaFormInput,
  ctx: SubmissionContext,
): Promise<void> {
  const locationId = showAreaFormForLocation.value
  if (!locationId) return
  try {
    await areaService.createArea({
      data: {
        type: 'areas',
        attributes: { name: values.name.trim(), location_id: locationId },
      },
    })
    showAreaFormForLocation.value = null
    ctx.resetForm({ values: { name: '' } })
    await loadLocations()
  } catch (err) {
    if (applyApiFieldErrors(err, ctx.setErrors)) return
    toast.error(getErrorMessage(err as never, 'area', 'Failed to create area'))
  }
}

function applyApiFieldErrors(
  err: unknown,
  setErrors: (_errors: Record<string, string>) => void,
): boolean {
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

function viewLocation(id: string) {
  router.push(groupStore.groupPath(`/locations/${id}`))
}

function editLocation(id: string) {
  router.push(groupStore.groupPath(`/locations/${id}/edit`))
}

async function onDeleteLocation(id: string) {
  const confirmed = await confirmDelete('location')
  if (!confirmed) return
  try {
    await locationService.deleteLocation(id)
    expandedLocations.value = expandedLocations.value.filter((lid) => lid !== id)
    await loadLocations()
  } catch (err) {
    toast.error(getErrorMessage(err as never, 'location', 'Failed to delete location'))
  }
}

function viewArea(id: string) {
  router.push(groupStore.groupPath(`/areas/${id}`))
}

function editArea(id: string) {
  router.push(groupStore.groupPath(`/areas/${id}/edit`))
}

async function onDeleteArea(id: string) {
  const confirmed = await confirmDelete('area')
  if (!confirmed) return
  try {
    await areaService.deleteArea(id)
    await loadLocations()
  } catch (err) {
    toast.error(getErrorMessage(err as never, 'area', 'Failed to delete area'))
  }
}

onMounted(async () => {
  await settingsStore.fetchMainCurrency()
  currentPage.value = Number(route.query.page) || 1
  await loadLocations()
})

watch(
  () => route.query.page,
  (newPage) => {
    currentPage.value = Number(newPage) || 1
    loadLocations()
  },
)
</script>

<template>
  <PageContainer as="div" class="location-list">
    <PageHeader title="Locations">
      <template #actions>
        <Button
          :variant="showLocationForm ? 'outline' : 'default'"
          class="new-location-button"
          @click="showLocationForm = !showLocationForm"
        >
          <component :is="showLocationForm ? X : Plus" class="size-4" aria-hidden="true" />
          {{ showLocationForm ? 'Cancel' : 'New' }}
        </Button>
      </template>
    </PageHeader>

    <Card
      v-if="!valuesLoading && globalTotal > 0"
      class="grand-total-card mb-6 border-l-4 border-l-primary p-6"
    >
      <div class="flex items-center justify-between">
        <h3 class="m-0 text-base font-semibold text-foreground">Total Inventory Value</h3>
        <div class="text-2xl font-bold text-primary">
          {{ formatPrice(globalTotal, mainCurrency) }}
        </div>
      </div>
    </Card>

    <Form
      v-if="showLocationForm"
      v-slot="{ isSubmitting, resetForm }"
      as="form"
      :validation-schema="locationFormSchemaTyped"
      :initial-values="{ name: '', address: '' }"
      class="location-form mb-6 flex flex-col gap-4 rounded-md border border-border bg-card p-4 shadow-sm"
      data-testid="location-list-location-form"
      @submit="(values, ctx) => onLocationSubmit(values as LocationFormInput, ctx as SubmissionContext)"
    >
      <div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
        <FormField v-slot="{ componentField }" name="name">
          <FormItem id="name">
            <FormLabel required>Name</FormLabel>
            <FormControl>
              <Input
                v-bind="componentField"
                type="text"
                placeholder="Enter location name"
                data-testid="location-list-location-form-name"
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
                data-testid="location-list-location-form-address"
              />
            </FormControl>
            <FormMessage />
          </FormItem>
        </FormField>
      </div>

      <div class="flex justify-end gap-2">
        <Button type="button" variant="outline" @click="cancelLocationForm(resetForm)">Cancel</Button>
        <Button
          type="submit"
          :disabled="isSubmitting"
          data-testid="location-list-location-form-submit"
        >
          {{ isSubmitting ? 'Creating...' : 'Create Location' }}
        </Button>
      </div>
    </Form>

    <div v-if="loading" class="py-12 text-center text-sm text-muted-foreground">Loading...</div>

    <EmptyState
      v-else-if="locations.length === 0"
      title="No locations yet"
      description="No locations found. Create your first location!"
    >
      <template #actions>
        <Button @click="showLocationForm = true">
          <Plus class="size-4" aria-hidden="true" />
          Create Location
        </Button>
      </template>
    </EmptyState>

    <div v-else class="locations-list flex flex-col gap-6">
      <div
        v-for="location in locations"
        :key="location.id"
        class="location-container flex flex-col"
      >
        <Card
          class="location-card flex-row items-start justify-between gap-4 px-6 py-5 transition-shadow hover:shadow-md cursor-pointer"
          :data-location-id="location.id"
          @click="toggleLocationExpanded(location.id)"
        >
          <div class="location-content min-w-0 flex-1">
            <div class="flex items-center justify-between gap-4">
              <h3 class="truncate text-lg font-semibold text-foreground">
                {{ location.attributes.name }}
              </h3>
              <component
                :is="expandedLocations.includes(location.id) ? ChevronDown : ChevronRight"
                class="size-4 shrink-0 text-muted-foreground"
                aria-hidden="true"
              />
            </div>
            <p
              v-if="location.attributes.address"
              class="address mt-1 truncate text-sm italic text-muted-foreground"
            >
              {{ location.attributes.address }}
            </p>
            <div v-if="!valuesLoading" class="location-value mt-2 text-sm font-medium text-primary">
              <span class="text-muted-foreground font-normal">Total value:</span>
              {{ getLocationValueLabel(location.id) }}
            </div>
          </div>
          <div class="location-actions flex shrink-0 items-center gap-1">
            <IconButton
              aria-label="View location"
              @click.stop="viewLocation(location.id)"
            >
              <Eye class="size-4" aria-hidden="true" />
            </IconButton>
            <IconButton
              aria-label="Edit location"
              @click.stop="editLocation(location.id)"
            >
              <Pencil class="size-4" aria-hidden="true" />
            </IconButton>
            <IconButton
              aria-label="Delete location"
              class="text-destructive hover:text-destructive"
              @click.stop="onDeleteLocation(location.id)"
            >
              <Trash2 class="size-4" aria-hidden="true" />
            </IconButton>
          </div>
        </Card>

        <div
          v-if="expandedLocations.includes(location.id)"
          class="areas-container ml-8 mt-2 rounded-md border-l-4 border-l-primary bg-muted/40 p-4"
        >
          <div class="areas-header mb-3 flex items-center justify-between">
            <h4 class="m-0 text-sm font-semibold text-foreground">Areas</h4>
            <Button size="sm" @click="toggleAreaForm(location.id)">
              {{ showAreaFormForLocation === location.id ? 'Cancel' : 'Add Area' }}
            </Button>
          </div>

          <Form
            v-if="showAreaFormForLocation === location.id"
            v-slot="{ isSubmitting }"
            as="form"
            :validation-schema="areaFormSchemaTyped"
            :initial-values="{ name: '' }"
            class="mb-4 flex flex-col gap-3 rounded-md border border-border bg-card p-3 shadow-sm"
            :data-testid="`location-list-area-form-${location.id}`"
            @submit="(values, ctx) => onAreaSubmit(values as AreaFormInput, ctx as SubmissionContext)"
          >
            <FormField v-slot="{ componentField }" name="name">
              <FormItem id="name">
                <FormLabel required>Area Name</FormLabel>
                <FormControl>
                  <Input
                    v-bind="componentField"
                    type="text"
                    placeholder="Enter area name"
                    :data-testid="`location-list-area-form-${location.id}-name`"
                  />
                </FormControl>
                <FormMessage />
              </FormItem>
            </FormField>
            <div class="flex justify-end gap-2">
              <Button
                type="button"
                variant="outline"
                @click="showAreaFormForLocation = null"
              >
                Cancel
              </Button>
              <Button
                type="submit"
                :disabled="isSubmitting"
                :data-testid="`location-list-area-form-${location.id}-submit`"
              >
                {{ isSubmitting ? 'Creating...' : 'Create Area' }}
              </Button>
            </div>
          </Form>

          <div v-if="getAreasForLocation(location.id).length > 0" class="areas-list flex flex-col gap-3">
            <AreaCard
              v-for="area in getAreasForLocation(location.id)"
              :id="`area-${area.id}`"
              :key="area.id"
              :area="(area as never)"
              :class="{ 'area-highlight': areaToFocus === area.id }"
              :subtitle="!valuesLoading ? `Total value: ${getAreaValueLabel(area.id)}` : ''"
              @view="viewArea"
              @edit="editArea"
              @delete="onDeleteArea"
            />
          </div>
          <div
            v-else
            class="no-areas rounded-md bg-card p-4 text-center text-sm text-muted-foreground"
          >
            No areas found for this location. Add your first area using the button above.
          </div>
        </div>
      </div>
    </div>

    <PaginationControls
      v-if="!loading"
      :current-page="currentPage"
      :total-pages="totalPages"
      :page-size="pageSize"
      :total-items="totalLocations"
      item-label="locations"
    />
  </PageContainer>
</template>


<script setup lang="ts">
/**
 * ExportCreateView — migrated to the design system in Phase 4 of
 * Epic #1324 (issue #1329).
 *
 * Lets the user kick off a new export — picking a description, an
 * export type, optionally a hierarchical selection of locations /
 * areas / commodities, and the include-file-data flag.
 *
 * Legacy DOM anchors preserved verbatim because Playwright suites
 * (`exports-crud.spec.ts`, `e2e/tests/includes/exports.ts`) drive
 * the selection tree and the type dropdown by class / data-attr:
 *   .export-create, .export-form, .form-section, .form-group,
 *   .selection-tree, .hierarchical-selector, .inclusion-toggle,
 *   .tree-item.{location,area,commodity}-item, .item-header,
 *   .item-checkbox, .item-content, .item-name, .item-type,
 *   data-location_id / data-area_id / data-commodity_id,
 *   .checkbox-label, .form-help, .field-error, .form-error,
 *   .p-select[id="type"], .p-select-option-label, .btn[type=submit].
 */
import { ref, computed, onMounted, reactive, nextTick } from 'vue'
import { useRouter } from 'vue-router'
import { Loader2, Plus } from 'lucide-vue-next'

import exportService from '@/services/exportService'
import locationService from '@/services/locationService'
import areaService from '@/services/areaService'
import commodityService from '@/services/commodityService'
import type { Export, ExportType, Location, Area, Commodity, ResourceObject } from '@/types'
import { useGroupStore } from '@/stores/groupStore'

import { Button } from '@design/ui/button'
import { Label } from '@design/ui/label'
import { Textarea } from '@design/ui/textarea'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@design/ui/select'
import Banner from '@design/patterns/Banner.vue'
import PageContainer from '@design/patterns/PageContainer.vue'
import PageHeader from '@design/patterns/PageHeader.vue'
import PageSection from '@design/patterns/PageSection.vue'
import FormFooter from '@design/patterns/FormFooter.vue'

const router = useRouter()
const groupStore = useGroupStore()

const exportTypeOptions = [
  { label: 'Full Database', value: 'full_database' as const },
  { label: 'Locations Only', value: 'locations' as const },
  { label: 'Areas Only', value: 'areas' as const },
  { label: 'Commodities Only', value: 'commodities' as const },
  { label: 'Selected Items', value: 'selected_items' as const },
]

const exportData = ref<Partial<Export>>({
  type: '' as ExportType,
  description: '',
  include_file_data: false,
  selected_items: [],
})

const locations = ref<Array<{ id: string; name: string; areas: string[] }>>([])
const areas = ref<Array<{ id: string; name: string; location_id: string }>>([])
const commodities = ref<Array<{ id: string; name: string; area_id?: string; location_id: string }>>([])

const locationInclusions = ref<Map<string, boolean>>(new Map())
const areaInclusions = ref<Map<string, boolean>>(new Map())

const creating = ref(false)
const formError = ref<string | null>(null)
const formErrors = reactive<Record<string, string>>({
  description: '',
  type: '',
  selected_items: '',
  include_file_data: '',
})
const loadingItems = ref(false)

const canSubmit = computed(() => {
  if (!exportData.value.type || !exportData.value.description?.trim()) return false
  if (exportData.value.type === 'selected_items') {
    return !!exportData.value.selected_items && exportData.value.selected_items.length > 0
  }
  return true
})

async function loadItems() {
  loadingItems.value = true
  try {
    const [locationsResponse, areasResponse, commoditiesResponse] = await Promise.all([
      locationService.getLocations(),
      areaService.getAreas(),
      commodityService.getCommodities(),
    ])
    if (locationsResponse.data?.data) {
      locations.value = locationsResponse.data.data.map((item: ResourceObject<Location>) => ({
        id: item.id,
        name: item.attributes.name,
        areas: item.attributes.areas || [],
      }))
    }
    if (areasResponse.data?.data) {
      areas.value = areasResponse.data.data.map((item: ResourceObject<Area>) => ({
        id: item.id,
        name: item.attributes.name,
        location_id: item.attributes.location_id,
      }))
    }
    if (commoditiesResponse.data?.data) {
      commodities.value = commoditiesResponse.data.data.map((item: ResourceObject<Commodity>) => ({
        id: item.id,
        name: item.attributes.name,
        area_id: item.attributes.area_id,
        location_id: item.attributes.location_id,
      }))
    }
  } catch (err) {
    console.error('Error loading items:', err)
  } finally {
    loadingItems.value = false
  }
}

function onTypeChange(value: string) {
  exportData.value.type = value as ExportType
  exportData.value.selected_items = []
  locationInclusions.value.clear()
  areaInclusions.value.clear()
  if (value === 'selected_items') loadItems()
}

const isLocationSelected = (id: string) =>
  exportData.value.selected_items?.some(i => i.id === id && i.type === 'location') || false
const isAreaSelected = (id: string) =>
  exportData.value.selected_items?.some(i => i.id === id && i.type === 'area') || false
const isCommoditySelected = (id: string) =>
  exportData.value.selected_items?.some(i => i.id === id && i.type === 'commodity') || false

const getLocationInclusion = (id: string) => locationInclusions.value.get(id) ?? true
const getAreaInclusion = (id: string) => areaInclusions.value.get(id) ?? true
const setLocationInclusion = (id: string, include: boolean) => locationInclusions.value.set(id, include)
const setAreaInclusion = (id: string, include: boolean) => areaInclusions.value.set(id, include)

const getAreasForLocation = (locationId: string) =>
  areas.value.filter(a => a.location_id === locationId)
const getCommoditiesForArea = (areaId: string) =>
  commodities.value.filter(c => c.area_id === areaId)

function toggleLocation(locationId: string, selected: boolean) {
  if (!exportData.value.selected_items) exportData.value.selected_items = []
  if (selected) {
    if (!isLocationSelected(locationId)) {
      exportData.value.selected_items.push({ id: locationId, type: 'location' })
      locationInclusions.value.set(locationId, true)
    }
  } else {
    exportData.value.selected_items = exportData.value.selected_items.filter(
      i => !(i.id === locationId && i.type === 'location'),
    )
    locationInclusions.value.delete(locationId)
    getAreasForLocation(locationId).forEach(area => {
      exportData.value.selected_items = exportData.value.selected_items!.filter(
        i => !(i.id === area.id && i.type === 'area'),
      )
      areaInclusions.value.delete(area.id)
      getCommoditiesForArea(area.id).forEach(c => {
        exportData.value.selected_items = exportData.value.selected_items!.filter(
          i => !(i.id === c.id && i.type === 'commodity'),
        )
      })
    })
  }
}

function toggleArea(areaId: string, selected: boolean) {
  if (!exportData.value.selected_items) exportData.value.selected_items = []
  if (selected) {
    if (!isAreaSelected(areaId)) {
      exportData.value.selected_items.push({ id: areaId, type: 'area' })
      areaInclusions.value.set(areaId, true)
    }
  } else {
    exportData.value.selected_items = exportData.value.selected_items.filter(
      i => !(i.id === areaId && i.type === 'area'),
    )
    areaInclusions.value.delete(areaId)
    getCommoditiesForArea(areaId).forEach(c => {
      exportData.value.selected_items = exportData.value.selected_items!.filter(
        i => !(i.id === c.id && i.type === 'commodity'),
      )
    })
  }
}

function toggleCommodity(commodityId: string, selected: boolean) {
  if (!exportData.value.selected_items) exportData.value.selected_items = []
  if (selected) {
    if (!isCommoditySelected(commodityId)) {
      exportData.value.selected_items.push({ id: commodityId, type: 'commodity' })
    }
  } else {
    exportData.value.selected_items = exportData.value.selected_items.filter(
      i => !(i.id === commodityId && i.type === 'commodity'),
    )
  }
}

function scrollToFirstError() {
  nextTick(() => {
    const el = document.querySelector('.field-error, .form-error')
    if (el) {
      const group = el.closest('.form-group') ?? el
      group.scrollIntoView({ behavior: 'smooth', block: 'center' })
    }
  })
}

function validateForm(): boolean {
  let isValid = true
  Object.keys(formErrors).forEach(k => { formErrors[k] = '' })
  if (!exportData.value.description?.trim()) {
    formErrors.description = 'Description is required'
    isValid = false
  }
  if (!exportData.value.type) {
    formErrors.type = 'Export type is required'
    isValid = false
  }
  if (exportData.value.type === 'selected_items') {
    if (!exportData.value.selected_items || exportData.value.selected_items.length === 0) {
      formErrors.selected_items = 'At least one item must be selected'
      isValid = false
    }
  }
  if (!isValid) scrollToFirstError()
  return isValid
}

async function createExport() {
  if (!canSubmit.value) return
  if (!validateForm()) return
  try {
    creating.value = true
    formError.value = null
    const selectedItemsWithInclusion = (exportData.value.selected_items || []).map(item => {
      const enriched: typeof item & { include_all?: boolean } = { ...item }
      if (item.type === 'location') enriched.include_all = getLocationInclusion(item.id)
      else if (item.type === 'area') enriched.include_all = getAreaInclusion(item.id)
      return enriched
    })
    const requestData = {
      data: {
        type: 'exports',
        attributes: {
          type: exportData.value.type,
          description: exportData.value.description?.trim(),
          include_file_data: exportData.value.include_file_data,
          selected_items: selectedItemsWithInclusion,
          status: 'pending',
        },
      },
    }
    const response = await exportService.createExport(requestData)
    if (response.data?.data) {
      router.push(groupStore.groupPath(`/exports/${response.data.data.id}`))
    } else {
      router.push(groupStore.groupPath('/exports'))
    }
  } catch (err: unknown) {
    const e = err as { response?: { status?: number; data?: { errors?: Array<{ error?: { error?: { data?: { attributes?: Record<string, string> } } } }> } }; message?: string }
    if (e.response) {
      const apiErrors = e.response.data?.errors?.[0]?.error?.error?.data?.attributes || {}
      if (apiErrors && Object.keys(apiErrors).length > 0) {
        const unknownErrors: Record<string, string> = {}
        Object.keys(apiErrors).forEach(key => {
          const camelKey = key.replace(/_([a-z])/g, (_, l) => l.toUpperCase())
          if (Object.prototype.hasOwnProperty.call(formErrors, camelKey)) {
            formErrors[camelKey] = apiErrors[key]
          } else {
            unknownErrors[key] = apiErrors[key]
          }
        })
        formError.value = Object.keys(unknownErrors).length === 0
          ? 'Please correct the errors above.'
          : 'Please correct the errors above. Additional errors: ' + JSON.stringify(unknownErrors)
        scrollToFirstError()
      } else {
        formError.value = `Failed to create export: ${e.response.status} - ${JSON.stringify(e.response.data)}`
      }
    } else {
      formError.value = 'Failed to create export: ' + (e.message || 'Unknown error')
    }
  } finally {
    creating.value = false
  }
}

onMounted(() => {
  loadItems()
})
</script>

<template>
  <PageContainer as="div" class="export-create mx-auto max-w-3xl">
    <div class="breadcrumb-nav mb-2 text-sm">
      <router-link
        :to="groupStore.groupPath('/exports')"
        class="breadcrumb-link inline-flex items-center gap-1 text-primary hover:underline"
      >
        ← Back to Exports
      </router-link>
    </div>

    <PageHeader title="Create New Export" />

    <Banner v-if="formError" variant="error" class="mb-4">{{ formError }}</Banner>

    <form
      class="export-form rounded-md border bg-card p-6 shadow-sm"
      @submit.prevent="createExport"
    >
      <PageSection title="Export Details">
        <div>
          <Label for="description" class="mb-2 inline-block">Description</Label>
          <Textarea
            id="description"
            v-model="exportData.description"
            placeholder="Enter a description for this export..."
            rows="3"
            maxlength="500"
            required
            :class="{ 'is-invalid border-destructive': formErrors.description }"
          />
          <p v-if="formErrors.description" class="field-error mt-1 text-sm text-destructive">
            {{ formErrors.description }}
          </p>
          <p class="field-help mt-1 text-xs text-muted-foreground">Describe what this export contains</p>
        </div>

        <div class="mt-6">
          <Label for="type" class="mb-2 inline-block">Export Type</Label>
          <Select :model-value="exportData.type" @update:model-value="(v) => onTypeChange(String(v))">
            <SelectTrigger
              id="type"
              :class="['p-select w-full', { 'is-invalid border-destructive': formErrors.type }]"
            >
              <SelectValue placeholder="Select export type..." />
            </SelectTrigger>
            <SelectContent>
              <SelectItem
                v-for="opt in exportTypeOptions"
                :key="opt.value"
                :value="opt.value"
                class="p-select-option-label"
              >
                {{ opt.label }}
              </SelectItem>
            </SelectContent>
          </Select>
          <p v-if="formErrors.type" class="field-error mt-1 text-sm text-destructive">
            {{ formErrors.type }}
          </p>
          <p class="field-help mt-1 text-xs text-muted-foreground">
            Choose what data to include in the export
          </p>
        </div>

        <div v-if="exportData.type === 'selected_items'" class="mt-6">
          <Label class="mb-2 inline-block">Selected Items</Label>
          <div class="hierarchical-selector max-h-[400px] overflow-y-auto rounded-md border">
            <div v-if="loadingItems" class="loading p-4 text-sm text-muted-foreground">
              Loading items...
            </div>
            <div v-else class="selection-tree p-4">
              <div
                v-for="location in locations"
                :key="location.id"
                :data-location_id="location.id"
                class="tree-item location-item mb-2 border-l-[3px] border-l-blue-500 pl-2"
              >
                <div class="item-header mb-2">
                  <label class="item-checkbox flex cursor-pointer items-center gap-2 rounded p-2 hover:bg-muted/40">
                    <input
                      type="checkbox"
                      :checked="isLocationSelected(location.id)"
                      class="size-4"
                      @change="toggleLocation(location.id, ($event.target as HTMLInputElement).checked)"
                    />
                    <span class="item-name font-semibold text-foreground">{{ location.name }}</span>
                    <span class="item-type ml-auto rounded bg-muted px-1.5 py-0.5 text-xs uppercase text-muted-foreground">Location</span>
                  </label>
                </div>
                <div
                  v-if="isLocationSelected(location.id)"
                  class="item-content mt-2 pl-5"
                  :data-location_id="location.id"
                >
                  <div class="inclusion-toggle mb-3 rounded bg-muted/40 p-2">
                    <label class="checkbox-label flex cursor-pointer items-center gap-2">
                      <input
                        type="checkbox"
                        :checked="getLocationInclusion(location.id)"
                        class="size-4"
                        @change="setLocationInclusion(location.id, ($event.target as HTMLInputElement).checked)"
                      />
                      <span>Include all areas and commodities in this location</span>
                    </label>
                  </div>
                  <div v-if="!getLocationInclusion(location.id)" class="sub-items mt-2">
                    <div
                      v-for="area in getAreasForLocation(location.id)"
                      :key="area.id"
                      :data-location_id="location.id"
                      :data-area_id="area.id"
                      class="tree-item area-item mb-2 ml-5 border-l-[3px] border-l-orange-500 pl-2"
                    >
                      <div class="item-header mb-2">
                        <label class="item-checkbox flex cursor-pointer items-center gap-2 rounded p-2 hover:bg-muted/40">
                          <input
                            type="checkbox"
                            :checked="isAreaSelected(area.id)"
                            class="size-4"
                            @change="toggleArea(area.id, ($event.target as HTMLInputElement).checked)"
                          />
                          <span class="item-name font-semibold text-foreground">{{ area.name }}</span>
                          <span class="item-type ml-auto rounded bg-muted px-1.5 py-0.5 text-xs uppercase text-muted-foreground">Area</span>
                        </label>
                      </div>
                      <div
                        v-if="isAreaSelected(area.id)"
                        class="item-content mt-2 pl-5"
                        :data-location_id="location.id"
                        :data-area_id="area.id"
                      >
                        <div class="inclusion-toggle mb-3 rounded bg-muted/40 p-2">
                          <label class="checkbox-label flex cursor-pointer items-center gap-2">
                            <input
                              type="checkbox"
                              :checked="getAreaInclusion(area.id)"
                              class="size-4"
                              @change="setAreaInclusion(area.id, ($event.target as HTMLInputElement).checked)"
                            />
                            <span>Include all commodities in this area</span>
                          </label>
                        </div>
                        <div v-if="!getAreaInclusion(area.id)" class="sub-items mt-2">
                          <div
                            v-for="commodity in getCommoditiesForArea(area.id)"
                            :key="commodity.id"
                            :data-location_id="location.id"
                            :data-area_id="area.id"
                            :data-commodity_id="commodity.id"
                            class="tree-item commodity-item mb-2 ml-10 border-l-[3px] border-l-green-500 pl-2"
                          >
                            <div class="item-header mb-2">
                              <label class="item-checkbox flex cursor-pointer items-center gap-2 rounded p-2 hover:bg-muted/40">
                                <input
                                  type="checkbox"
                                  :checked="isCommoditySelected(commodity.id)"
                                  class="size-4"
                                  @change="toggleCommodity(commodity.id, ($event.target as HTMLInputElement).checked)"
                                />
                                <span class="item-name font-semibold text-foreground">{{ commodity.name }}</span>
                                <span class="item-type ml-auto rounded bg-muted px-1.5 py-0.5 text-xs uppercase text-muted-foreground">Commodity</span>
                              </label>
                            </div>
                          </div>
                        </div>
                      </div>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </div>
          <p v-if="formErrors.selected_items" class="field-error mt-1 text-sm text-destructive">
            {{ formErrors.selected_items }}
          </p>
          <p class="field-help mt-1 text-xs text-muted-foreground">
            Selected: {{ exportData.selected_items ? exportData.selected_items.length : 0 }} item(s)
          </p>
        </div>

        <div class="mt-6">
          <label class="checkbox-label flex cursor-pointer items-center gap-2">
            <input v-model="exportData.include_file_data" type="checkbox" class="size-4" />
            <span>Include file data (images, invoices, manuals)</span>
          </label>
          <p class="field-help mt-1 text-xs text-muted-foreground">
            When enabled, exported XML will include base64-encoded file data.
            This makes the export larger but fully self-contained.
          </p>
        </div>
      </PageSection>

      <FormFooter class="mt-6">
        <Button variant="outline" type="button" as-child>
          <router-link :to="groupStore.groupPath('/exports')">Cancel</router-link>
        </Button>
        <Button type="submit" data-testid="export-submit" :disabled="!canSubmit || creating">
          <Loader2 v-if="creating" class="size-4 motion-safe:animate-spin" aria-hidden="true" />
          <Plus v-else class="size-4" aria-hidden="true" />
          {{ creating ? 'Creating...' : 'Create Export' }}
        </Button>
      </FormFooter>
    </form>
  </PageContainer>
</template>


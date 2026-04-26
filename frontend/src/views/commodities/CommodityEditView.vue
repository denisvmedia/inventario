<script setup lang="ts">
/**
 * CommodityEditView — migrated to the design system in Phase 4 of
 * Epic #1324 (issue #1329).
 *
 * Mirrors `CommodityCreateView` with an extra data-fetch step for the
 * existing commodity, plus the legacy "back to area / commodities /
 * commodity" breadcrumb that depends on the `source`, `areaId`, and
 * `directEdit` query params.
 *
 * Legacy DOM anchors (`.commodity-edit`, `#name`, `#shortName`,
 * `#count`, `#originalPrice`, `#serialNumber`, `#purchaseDate`,
 * `.p-select[id="…"]`, `.p-select-option-label`, `.array-input`,
 * `.array-item`) are preserved as no-op markers so existing
 * Playwright selectors keep resolving — see
 * devdocs/frontend/migration-conventions.md.
 */
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useForm } from 'vee-validate'
import { toTypedSchema } from '@vee-validate/zod'
import { ArrowLeft } from 'lucide-vue-next'

import commodityService from '@/services/commodityService'
import settingsService from '@/services/settingsService'
import areaService from '@/services/areaService'
import locationService from '@/services/locationService'
import { useSettingsStore } from '@/stores/settingsStore'
import { useGroupStore } from '@/stores/groupStore'
import { COMMODITY_TYPES } from '@/constants/commodityTypes'
import { COMMODITY_STATUSES } from '@/constants/commodityStatuses'
import { is404Error as checkIs404Error, getErrorMessage } from '@/utils/errorUtils'

import { Button } from '@design/ui/button'
import { Checkbox } from '@design/ui/checkbox'
import { Input } from '@design/ui/input'
import { Label } from '@design/ui/label'
import { Textarea } from '@design/ui/textarea'
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
  SelectGroup,
  SelectItem,
  SelectLabel,
  SelectTrigger,
  SelectValue,
} from '@design/ui/select'
import FormFooter from '@design/patterns/FormFooter.vue'
import FormSection from '@design/patterns/FormSection.vue'
import InlineListEditor from '@design/patterns/InlineListEditor.vue'
import PageContainer from '@design/patterns/PageContainer.vue'
import PageHeader from '@design/patterns/PageHeader.vue'
import { useAppToast } from '@design/composables/useAppToast'

import {
  commodityFormSchema,
  defaultCommodityFormValues,
  type CommodityFormInput,
} from './CommodityForm.schema'
import {
  buildCommodityAttributes,
  buildCurrencyOptions,
  buildGroupedAreas,
  extractApiFieldErrors,
  type ApiResource,
  type AreaAttributes,
  type CurrencyOption,
  type LocationAttributes,
} from './CommodityForm.helpers'

const route = useRoute()
const router = useRouter()
const groupStore = useGroupStore()
const settingsStore = useSettingsStore()
const toast = useAppToast()

const id = route.params.id as string

const sourceIsArea = computed<boolean>(() => route.query.source === 'area')
const areaIdFromQuery = computed<string>(() => (route.query.areaId as string) ?? '')
const isDirectEdit = computed<boolean>(() => route.query.directEdit === 'true')

const loading = ref<boolean>(true)
const loadError = ref<string | null>(null)
const lastError = ref<unknown>(null)
const is404 = computed<boolean>(() => !!lastError.value && checkIs404Error(lastError.value as never))

const areas = ref<ApiResource<AreaAttributes>[]>([])
const locations = ref<ApiResource<LocationAttributes>[]>([])
const currencies = ref<CurrencyOption[]>([])

const mainCurrency = computed<string>(() => settingsStore.mainCurrency)
const groupedAreas = computed(() => buildGroupedAreas(areas.value, locations.value))

const { handleSubmit, isSubmitting, setErrors, setFieldValue, setValues, values } =
  useForm<CommodityFormInput>({
    validationSchema: toTypedSchema(commodityFormSchema),
    initialValues: defaultCommodityFormValues(),
  })

const showConvertedOriginalPrice = computed(
  () => values.originalPriceCurrency !== mainCurrency.value,
)

const isPriceUsedInCalculations = computed(
  () => !values.draft && values.status === 'in_use',
)

const hasSuitablePrice = computed(() => {
  if (!isPriceUsedInCalculations.value) return false
  return (
    (values.currentPrice ?? 0) > 0 ||
    (values.originalPriceCurrency === mainCurrency.value && (values.originalPrice ?? 0) > 0) ||
    (values.convertedOriginalPrice ?? 0) > 0
  )
})

const priceCalculationHint = computed<string>(() => {
  if (values.draft) {
    return 'This item is a draft and will not be included in value calculations.'
  }
  if (values.status !== 'in_use') {
    const status = COMMODITY_STATUSES.find((s) => s.id === values.status)
    return `This item has status "${status ? status.name : values.status}" and will not be included in value calculations.`
  }
  if ((values.currentPrice ?? 0) > 0) {
    return `Current Price will be used in value calculations (in ${mainCurrency.value}).`
  }
  if (values.originalPriceCurrency === mainCurrency.value && (values.originalPrice ?? 0) > 0) {
    return `Original Price will be used in value calculations (in ${mainCurrency.value}).`
  }
  if (showConvertedOriginalPrice.value && (values.convertedOriginalPrice ?? 0) > 0) {
    return `Converted Original Price will be used in value calculations (in ${mainCurrency.value}).`
  }
  const needsConverted = values.originalPriceCurrency !== mainCurrency.value
  return `No suitable price found for calculations. Please enter Current Price${needsConverted ? ', Converted Original Price' : ''}, or Original Price in ${mainCurrency.value}.`
})

function onCurrencyChange(next: string) {
  setFieldValue('originalPriceCurrency', next)
  if (next === mainCurrency.value) {
    setFieldValue('convertedOriginalPrice', 0)
  }
}

async function loadCommodity(): Promise<void> {
  loading.value = true
  loadError.value = null
  lastError.value = null
  try {
    await settingsStore.fetchMainCurrency()
    const [commodityResp, areasResp, locationsResp, currenciesResp] = await Promise.all([
      commodityService.getCommodity(id),
      areaService.getAreas(),
      locationService.getLocations(),
      settingsService.getCurrencies(),
    ])
    areas.value = (areasResp.data.data as ApiResource<AreaAttributes>[]) ?? []
    locations.value = (locationsResp.data.data as ApiResource<LocationAttributes>[]) ?? []
    currencies.value = buildCurrencyOptions(currenciesResp.data as string[])

    const attrs = commodityResp.data.data.attributes as Record<string, unknown>
    setValues(
      defaultCommodityFormValues({
        name: (attrs.name as string) ?? '',
        shortName: (attrs.short_name as string) ?? '',
        type: (attrs.type as string) ?? COMMODITY_TYPES[0].id,
        areaId: (attrs.area_id as string) ?? '',
        count: (attrs.count as number) ?? 1,
        originalPrice: (attrs.original_price as number) ?? 0,
        originalPriceCurrency:
          (attrs.original_price_currency as string) ?? mainCurrency.value,
        convertedOriginalPrice: (attrs.converted_original_price as number) ?? 0,
        currentPrice: (attrs.current_price as number) ?? 0,
        serialNumber: (attrs.serial_number as string) ?? '',
        extraSerialNumbers: (attrs.extra_serial_numbers as string[]) ?? [],
        partNumbers: (attrs.part_numbers as string[]) ?? [],
        tags: (attrs.tags as string[]) ?? [],
        status: (attrs.status as string) ?? 'in_use',
        purchaseDate: (attrs.purchase_date as string) ?? '',
        urls: (attrs.urls as string[]) ?? [],
        comments: (attrs.comments as string) ?? '',
        draft: (attrs.draft as boolean) ?? false,
      }),
    )
  } catch (err) {
    lastError.value = err
    if (!checkIs404Error(err as never)) {
      loadError.value = getErrorMessage(err as never, 'commodity', 'Failed to load commodity')
    }
  } finally {
    loading.value = false
  }
}

const onSubmit = handleSubmit(async (formValues) => {
  try {
    await commodityService.updateCommodity(id, {
      data: {
        id,
        type: 'commodities',
        attributes: buildCommodityAttributes(formValues),
      },
    })
    if (isDirectEdit.value) {
      if (sourceIsArea.value && areaIdFromQuery.value) {
        router.push({
          path: groupStore.groupPath(`/areas/${areaIdFromQuery.value}`),
          query: { highlightCommodityId: id },
        })
      } else {
        router.push({
          path: groupStore.groupPath('/commodities'),
          query: { highlightCommodityId: id },
        })
      }
    } else {
      router.push({
        path: groupStore.groupPath(`/commodities/${id}`),
        query: { source: route.query.source, areaId: route.query.areaId },
      })
    }
  } catch (err) {
    const fieldErrors = extractApiFieldErrors(err)
    if (Object.keys(fieldErrors).length > 0) {
      setErrors(fieldErrors)
      return
    }
    toast.error(getErrorMessage(err as never, 'commodity', 'Failed to update commodity'))
  }
})

function goBack() {
  if (isDirectEdit.value) {
    if (sourceIsArea.value && areaIdFromQuery.value) {
      router.push(groupStore.groupPath(`/areas/${areaIdFromQuery.value}`))
    } else {
      router.push(groupStore.groupPath('/commodities'))
    }
  } else {
    router.push({
      path: groupStore.groupPath(`/commodities/${id}`),
      query: { source: route.query.source, areaId: route.query.areaId },
    })
  }
}

function goBackToList() {
  router.push(groupStore.groupPath('/commodities'))
}

onMounted(loadCommodity)
</script>

<template>
  <PageContainer as="div" class="commodity-edit mx-auto max-w-3xl">
    <div v-if="loading" class="loading py-12 text-center text-muted-foreground">Loading...</div>
    <div v-else-if="is404" class="not-found py-12 text-center">
      <h2 class="mb-2 text-xl font-semibold">Commodity not found</h2>
      <p class="mb-4 text-muted-foreground">The commodity you are looking for does not exist or has been removed.</p>
      <Button type="button" variant="outline" @click="goBackToList">
        <ArrowLeft class="size-4" aria-hidden="true" /> Back to Commodities
      </Button>
    </div>
    <div v-else-if="loadError" class="error rounded-md border border-destructive/50 bg-destructive/10 p-4 text-destructive">
      {{ loadError }}
    </div>
    <template v-else>
      <div class="mb-2 text-sm">
        <a
          href="#"
          class="breadcrumb-link inline-flex items-center gap-1 text-primary hover:underline"
          @click.prevent="goBack"
        >
          <ArrowLeft class="size-4" aria-hidden="true" />
          <span v-if="sourceIsArea && isDirectEdit">Back to Area</span>
          <span v-else-if="isDirectEdit">Back to Commodities</span>
          <span v-else>Back to Commodity</span>
        </a>
      </div>

      <PageHeader title="Edit Commodity" />

      <form
        class="commodity-form flex flex-col gap-8 rounded-md border border-border bg-card p-6 shadow-sm"
        data-testid="commodity-edit-form"
        @submit.prevent="onSubmit"
      >
        <FormSection title="Basic Information">
          <FormField v-slot="{ componentField }" name="name">
            <FormItem id="name">
              <FormLabel required>Name</FormLabel>
              <FormControl>
                <Input v-bind="componentField" type="text" placeholder="Enter the full name of the commodity" />
              </FormControl>
              <FormMessage />
            </FormItem>
          </FormField>

          <FormField v-slot="{ componentField }" name="shortName">
            <FormItem id="shortName">
              <FormLabel required>Short Name</FormLabel>
              <FormControl>
                <Input v-bind="componentField" type="text" maxlength="20" placeholder="Enter a short identifier (max 20 chars)" />
              </FormControl>
              <FormMessage />
            </FormItem>
          </FormField>

          <FormField v-slot="{ componentField, value }" name="type">
            <FormItem>
              <FormLabel required>Type</FormLabel>
              <Select
                :model-value="value as string | undefined"
                @update:model-value="(v) => componentField['onUpdate:modelValue'](v)"
              >
                <SelectTrigger id="type" class="p-select w-full">
                  <SelectValue placeholder="Select a type" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem
                    v-for="type in COMMODITY_TYPES"
                    :key="type.id"
                    :value="type.id"
                    class="p-select-option-label"
                  >
                    {{ type.name }}
                  </SelectItem>
                </SelectContent>
              </Select>
              <FormMessage />
            </FormItem>
          </FormField>

          <FormField v-slot="{ componentField, value }" name="areaId">
            <FormItem>
              <FormLabel required>Area</FormLabel>
              <Select
                :model-value="value as string | undefined"
                @update:model-value="(v) => componentField['onUpdate:modelValue'](v)"
              >
                <SelectTrigger id="areaId" class="p-select w-full">
                  <SelectValue placeholder="Select an area" />
                </SelectTrigger>
                <SelectContent>
                  <SelectGroup v-for="group in groupedAreas" :key="group.code">
                    <SelectLabel class="location-group-label">{{ group.label }}</SelectLabel>
                    <SelectItem
                      v-for="area in group.items"
                      :key="area.id"
                      :value="area.id"
                      class="p-select-option-label"
                    >
                      {{ area.attributes.name }}
                    </SelectItem>
                  </SelectGroup>
                </SelectContent>
              </Select>
              <FormMessage />
            </FormItem>
          </FormField>

          <FormField v-slot="{ componentField }" name="count">
            <FormItem id="count">
              <FormLabel required>Count</FormLabel>
              <FormControl>
                <Input v-bind="componentField" type="number" min="1" placeholder="Enter quantity (minimum 1)" />
              </FormControl>
              <FormMessage />
            </FormItem>
          </FormField>
        </FormSection>


        <FormSection title="Price Information">
          <p
            class="rounded-md px-3 py-2 text-sm italic"
            :class="{
              'bg-primary/10 text-foreground': isPriceUsedInCalculations && hasSuitablePrice,
              'bg-destructive/10 text-destructive': !isPriceUsedInCalculations,
              'bg-amber-100 text-amber-900': isPriceUsedInCalculations && !hasSuitablePrice,
            }"
          >
            {{ priceCalculationHint }}
          </p>

          <FormField v-slot="{ componentField }" name="originalPrice">
            <FormItem id="originalPrice">
              <FormLabel>Original Price</FormLabel>
              <FormControl>
                <Input v-bind="componentField" type="number" min="0" step="0.01" placeholder="Enter the purchase price (minimum 0)" />
              </FormControl>
              <FormMessage />
            </FormItem>
          </FormField>

          <FormField v-slot="{ value }" name="originalPriceCurrency">
            <FormItem>
              <FormLabel>Original Price Currency</FormLabel>
              <Select :model-value="value as string | undefined" @update:model-value="(v) => onCurrencyChange(v as string)">
                <SelectTrigger id="originalPriceCurrency" class="p-select w-full">
                  <SelectValue placeholder="Select a currency" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem
                    v-for="c in currencies"
                    :key="c.code"
                    :value="c.code"
                    class="p-select-option-label"
                  >
                    {{ c.label }}
                  </SelectItem>
                </SelectContent>
              </Select>
              <FormMessage />
            </FormItem>
          </FormField>

          <FormField v-if="showConvertedOriginalPrice" v-slot="{ componentField }" name="convertedOriginalPrice">
            <FormItem id="convertedOriginalPrice">
              <FormLabel>Converted Original Price</FormLabel>
              <FormControl>
                <Input v-bind="componentField" type="number" min="0" step="0.01" placeholder="Enter the price converted to main currency" />
              </FormControl>
              <FormMessage />
            </FormItem>
          </FormField>

          <FormField v-slot="{ componentField }" name="currentPrice">
            <FormItem id="currentPrice">
              <FormLabel required>Current Price</FormLabel>
              <FormControl>
                <Input v-bind="componentField" type="number" min="0" step="0.01" placeholder="Enter the current estimated value" />
              </FormControl>
              <FormMessage />
            </FormItem>
          </FormField>
        </FormSection>

        <FormSection title="Serial Numbers and Part Numbers">
          <FormField v-slot="{ componentField }" name="serialNumber">
            <FormItem id="serialNumber">
              <FormLabel>Serial Number</FormLabel>
              <FormControl>
                <Input v-bind="componentField" type="text" placeholder="Enter the main serial number if available" />
              </FormControl>
              <FormMessage />
            </FormItem>
          </FormField>

          <FormField v-slot="{ value, handleChange }" name="extraSerialNumbers">
            <FormItem>
              <FormLabel>Extra Serial Numbers</FormLabel>
              <InlineListEditor
                :model-value="(value as string[]) ?? []"
                add-label="Add Serial Number"
                remove-text="Remove"
                placeholder="Enter additional serial number"
                :new-item="() => ''"
                class="array-input"
                row-class="array-item"
                @update:model-value="handleChange"
              />
              <FormMessage />
            </FormItem>
          </FormField>

          <FormField v-slot="{ value, handleChange }" name="partNumbers">
            <FormItem>
              <FormLabel>Part Numbers</FormLabel>
              <InlineListEditor
                :model-value="(value as string[]) ?? []"
                add-label="Add Part Number"
                remove-text="Remove"
                placeholder="Enter part or model number"
                :new-item="() => ''"
                class="array-input"
                row-class="array-item"
                @update:model-value="handleChange"
              />
              <FormMessage />
            </FormItem>
          </FormField>
        </FormSection>

        <FormSection title="Tags and Status">
          <FormField v-slot="{ value, handleChange }" name="tags">
            <FormItem>
              <FormLabel>Tags</FormLabel>
              <InlineListEditor
                :model-value="(value as string[]) ?? []"
                add-label="Add Tag"
                remove-text="Remove"
                placeholder="Enter a tag for categorization"
                :new-item="() => ''"
                class="array-input"
                row-class="array-item"
                @update:model-value="handleChange"
              />
              <FormMessage />
            </FormItem>
          </FormField>

          <FormField v-slot="{ componentField, value }" name="status">
            <FormItem>
              <FormLabel required>Status</FormLabel>
              <Select
                :model-value="value as string | undefined"
                @update:model-value="(v) => componentField['onUpdate:modelValue'](v)"
              >
                <SelectTrigger id="status" class="p-select w-full">
                  <SelectValue placeholder="Select a status" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem
                    v-for="s in COMMODITY_STATUSES"
                    :key="s.id"
                    :value="s.id"
                    class="p-select-option-label"
                  >
                    {{ s.name }}
                  </SelectItem>
                </SelectContent>
              </Select>
              <FormMessage />
            </FormItem>
          </FormField>

          <FormField v-slot="{ componentField }" name="purchaseDate">
            <FormItem>
              <FormLabel required>Purchase Date</FormLabel>
              <FormControl>
                <div id="purchaseDate">
                  <Input v-bind="componentField" type="date" placeholder="Select the date of purchase" />
                </div>
              </FormControl>
              <FormMessage />
            </FormItem>
          </FormField>
        </FormSection>

        <FormSection title="URLs and Comments">
          <FormField v-slot="{ value, handleChange }" name="urls">
            <FormItem>
              <FormLabel>URLs</FormLabel>
              <InlineListEditor
                :model-value="(value as string[]) ?? []"
                add-label="Add URL"
                remove-text="Remove"
                placeholder="Enter a relevant URL (e.g., product page, manual)"
                :new-item="() => ''"
                class="array-input"
                row-class="array-item"
                @update:model-value="handleChange"
              />
              <FormMessage />
            </FormItem>
          </FormField>

          <FormField v-slot="{ componentField }" name="comments">
            <FormItem id="comments">
              <FormLabel>Comments</FormLabel>
              <FormControl>
                <Textarea v-bind="componentField" rows="4" placeholder="Enter any additional notes or comments about this item (max 1000 characters)" />
              </FormControl>
              <FormMessage />
            </FormItem>
          </FormField>

          <FormField v-slot="{ value, handleChange }" name="draft">
            <FormItem>
              <Label class="checkbox-label inline-flex items-center gap-2">
                <Checkbox
                  :model-value="!!value"
                  @update:model-value="(v) => handleChange(!!v)"
                />
                <span>Draft</span>
              </Label>
              <FormMessage />
            </FormItem>
          </FormField>
        </FormSection>

        <FormFooter>
          <Button type="button" variant="outline" @click="goBack">Cancel</Button>
          <Button type="submit" :disabled="isSubmitting">
            {{ isSubmitting ? 'Saving Commodity...' : 'Save Commodity' }}
          </Button>
        </FormFooter>
      </form>
    </template>
  </PageContainer>
</template>

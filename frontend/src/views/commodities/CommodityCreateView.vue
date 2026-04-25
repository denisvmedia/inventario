<script setup lang="ts">
/**
 * CommodityCreateView — migrated to the design system in Phase 4 of
 * Epic #1324 (issue #1329).
 *
 * Page chrome (header, form sections, footer) is built from
 * `@design/*` patterns. Form is wired through vee-validate + zod with
 * the shared `CommodityForm.schema.ts`. The legacy
 * `CommodityForm.vue` was deleted; its data shaping helpers live in
 * `CommodityForm.helpers.ts`.
 *
 * Legacy DOM anchors (`.commodity-create`, `#name`, `#shortName`,
 * `#count`, `#originalPrice`, `#serialNumber`, `#purchaseDate`,
 * `.p-select[id="…"]`, `.p-select-option-label`, `.array-input`,
 * `.array-item`) are preserved as no-op markers so existing
 * Playwright selectors keep resolving through the strangler-fig
 * window — see devdocs/frontend/migration-conventions.md.
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
import { getErrorMessage } from '@/utils/errorUtils'

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

const router = useRouter()
const route = useRoute()
const groupStore = useGroupStore()
const settingsStore = useSettingsStore()
const toast = useAppToast()

const areaFromUrl = ref<string | null>(null)
const areas = ref<ApiResource<AreaAttributes>[]>([])
const locations = ref<ApiResource<LocationAttributes>[]>([])
const currencies = ref<CurrencyOption[]>([])

const mainCurrency = computed<string>(() => settingsStore.mainCurrency)
const groupedAreas = computed(() => buildGroupedAreas(areas.value, locations.value))

const { handleSubmit, isSubmitting, setErrors, setFieldValue, values } =
  useForm<CommodityFormInput>({
    validationSchema: toTypedSchema(commodityFormSchema),
    initialValues: defaultCommodityFormValues({ originalPriceCurrency: mainCurrency.value }),
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

async function loadData(): Promise<void> {
  try {
    if (route.query.area) {
      areaFromUrl.value = route.query.area as string
    }
    await settingsStore.fetchMainCurrency()
    const [areasResponse, locationsResponse, currenciesResponse] = await Promise.all([
      areaService.getAreas(),
      locationService.getLocations(),
      settingsService.getCurrencies(),
    ])
    areas.value = (areasResponse.data.data as ApiResource<AreaAttributes>[]) ?? []
    locations.value = (locationsResponse.data.data as ApiResource<LocationAttributes>[]) ?? []
    currencies.value = buildCurrencyOptions(currenciesResponse.data as string[])
    setFieldValue('originalPriceCurrency', mainCurrency.value)
    if (locations.value.length === 0 || areas.value.length === 0) {
      router.push(groupStore.groupPath('/commodities'))
      return
    }
    if (areaFromUrl.value) {
      setFieldValue('areaId', areaFromUrl.value)
    }
  } catch (err) {
    toast.error(getErrorMessage(err as never, 'commodity', 'Failed to load form data'))
  }
}

const onSubmit = handleSubmit(async (formValues) => {
  try {
    const response = await commodityService.createCommodity({
      data: {
        type: 'commodities',
        attributes: buildCommodityAttributes(formValues),
      },
    })
    const newId = response.data.data.id as string
    if (areaFromUrl.value) {
      router.push({
        path: groupStore.groupPath(`/commodities/${newId}`),
        query: { source: 'area', areaId: areaFromUrl.value },
      })
    } else {
      router.push({
        path: groupStore.groupPath(`/commodities/${newId}`),
        query: { source: 'commodities' },
      })
    }
  } catch (err) {
    const fieldErrors = extractApiFieldErrors(err)
    if (Object.keys(fieldErrors).length > 0) {
      setErrors(fieldErrors)
      return
    }
    toast.error(getErrorMessage(err as never, 'commodity', 'Failed to create commodity'))
  }
})

function cancel() {
  if (areaFromUrl.value) {
    router.push(groupStore.groupPath(`/areas/${areaFromUrl.value}`))
  } else {
    router.push(groupStore.groupPath('/commodities'))
  }
}

function backToList() {
  router.push(groupStore.groupPath('/commodities'))
}

function backToArea() {
  if (areaFromUrl.value) {
    router.push(groupStore.groupPath(`/areas/${areaFromUrl.value}`))
  }
}

onMounted(loadData)
</script>


<template>
  <PageContainer as="div" class="commodity-create mx-auto max-w-3xl">
    <div class="mb-2 text-sm">
      <a
        v-if="areaFromUrl"
        href="#"
        class="breadcrumb-link inline-flex items-center gap-1 text-primary hover:underline"
        @click.prevent="backToArea"
      >
        <ArrowLeft class="size-4" aria-hidden="true" /> Back to Area
      </a>
      <a
        v-else
        href="#"
        class="breadcrumb-link inline-flex items-center gap-1 text-primary hover:underline"
        @click.prevent="backToList"
      >
        <ArrowLeft class="size-4" aria-hidden="true" /> Back to Commodities
      </a>
    </div>

    <PageHeader title="Create New Commodity" />

    <form
      class="form flex flex-col gap-8 rounded-md border border-border bg-card p-6 shadow-sm"
      data-testid="commodity-create-form"
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
              :disabled="!!areaFromUrl"
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
        <Button type="button" variant="outline" @click="cancel">Cancel</Button>
        <Button type="submit" :disabled="isSubmitting">
          {{ isSubmitting ? 'Creating...' : 'Create Commodity' }}
        </Button>
      </FormFooter>
    </form>
  </PageContainer>
</template>

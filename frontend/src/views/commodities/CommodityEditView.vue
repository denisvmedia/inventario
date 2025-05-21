<template>
  <div class="commodity-edit">
    <div class="breadcrumb-nav">
      <a href="#" class="breadcrumb-link" @click.prevent="goBack">
        <font-awesome-icon icon="arrow-left" />
        <span v-if="sourceIsArea && isDirectEdit">Back to Area</span>
        <span v-else-if="isDirectEdit">Back to Commodities</span>
        <span v-else>Back to Commodity</span>
      </a>
    </div>
    <h1>Edit Commodity</h1>

    <div v-if="loading" class="loading">Loading...</div>
    <div v-else-if="error" class="error">{{ error }}</div>
    <div v-else-if="!commodity" class="not-found">Commodity not found</div>
    <div v-else>
      <CommodityForm
        ref="commodityForm"
        :initial-data="form"
        :areas="groupedAreas"
        :currencies="currencies"
        :main-currency="mainCurrency"
        :is-submitting="isSubmitting"
        submit-button-text="Save Commodity"
        submit-button-loading-text="Saving Commodity..."
        @submit="submitForm"
        @cancel="goBack"
        @validate="handleValidation"
      />

      <div v-if="formError" class="form-error">{{ formError }}</div>
      <div v-if="debugInfo" class="debug-info">
        <h3>Debug Info</h3>
        <pre>{{ debugInfo }}</pre>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted, computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import commodityService from '@/services/commodityService'
import areaService from '@/services/areaService'
import locationService from '@/services/locationService'
import settingsService from '@/services/settingsService'
import { useSettingsStore } from '@/stores/settingsStore'
import { COMMODITY_STATUS_IN_USE } from '@/constants/commodityStatuses'
import CommodityForm from '@/components/CommodityForm.vue'

const route = useRoute()
const router = useRouter()
const settingsStore = useSettingsStore()
const id = route.params.id as string
const commodityForm = ref(null)

// Navigation source tracking
const sourceIsArea = computed(() => route.query.source === 'area')
const areaId = computed(() => route.query.areaId as string || '')
const isDirectEdit = computed(() => route.query.directEdit === 'true')

const commodity = ref<any>(null)
const areas = ref<any[]>([])
const locations = ref<any[]>([])
const loading = ref<boolean>(true)
const error = ref<string | null>(null)
const isSubmitting = ref<boolean>(false)
const formError = ref<string | null>(null)
const debugInfo = ref<string | null>(null)


const currencies = ref<any[]>([])

// Use the main currency from the store
const mainCurrency = computed(() => settingsStore.mainCurrency)

// Group areas by their locations for the dropdown
const groupedAreas = computed(() => {
  // Create a map of locations by ID for quick lookup
  const locationMap = new Map()
  locations.value.forEach(location => {
    locationMap.set(location.id, location)
  })

  // Group areas by location
  const groupedByLocation = {}

  // Create a group for each location
  locations.value.forEach(location => {
    groupedByLocation[location.id] = {
      label: location.attributes.name,
      code: location.id,
      items: []
    }
  })

  // Add areas to their respective location groups
  areas.value.forEach(area => {
    const locationId = area.attributes.location_id
    if (groupedByLocation[locationId]) {
      groupedByLocation[locationId].items.push({
        id: area.id,
        attributes: {
          name: area.attributes.name
        }
      })
    }
  })

  // Convert the object to an array of location groups
  return Object.values(groupedByLocation).filter(group => group.items.length > 0)
})

const form = reactive({
  name: '',
  shortName: '',
  type: '',
  areaId: '',
  count: 1,
  originalPrice: 0,
  originalPriceCurrency: computed(() => settingsStore.mainCurrency).value,
  convertedOriginalPrice: 0,
  currentPrice: 0,
  serialNumber: '',
  extraSerialNumbers: [] as string[],
  partNumbers: [] as string[],
  tags: [] as string[],
  status: COMMODITY_STATUS_IN_USE,
  purchaseDate: new Date().toISOString().split('T')[0],
  urls: [] as string[],
  comments: '',
  draft: false
})

const errors = reactive({
  name: '',
  shortName: '',
  type: '',
  areaId: '',
  count: '',
  originalPrice: '',
  originalPriceCurrency: '',
  convertedOriginalPrice: '',
  currentPrice: '',
  serialNumber: '',
  status: '',
  purchaseDate: '',
  comments: ''
})

onMounted(async () => {
  try {
    // Fetch main currency from the store
    await settingsStore.fetchMainCurrency()

    // Load commodity, areas, locations, and currencies in parallel
    const [commodityResponse, areasResponse, locationsResponse, currenciesResponse] = await Promise.all([
      commodityService.getCommodity(id),
      areaService.getAreas(),
      locationService.getLocations(),
      settingsService.getCurrencies()
    ])

    // Process commodity data
    commodity.value = commodityResponse.data.data
    areas.value = areasResponse.data.data
    locations.value = locationsResponse.data.data

    // Process currencies with proper names
    const currencyCodes = currenciesResponse.data
    const currencyNames = new Intl.DisplayNames(['en'], { type: 'currency' })

    currencies.value = currencyCodes.map((code: string) => {
      let currencyName = code
      try {
        // Try to get the localized currency name
        currencyName = currencyNames.of(code)
      /* eslint-disable no-unused-vars */
      } catch (_) {
      /* eslint-enable no-unused-vars */
        console.warn(`Could not get display name for currency: ${code}`)
      }

      return {
        code: code,
        label: `${currencyName} (${code})`
      }
    })

    // Main currency is already set from the store

    // Initialize form with commodity data
    const attrs = commodity.value.attributes
    form.name = attrs.name
    form.shortName = attrs.short_name
    form.type = attrs.type
    form.areaId = attrs.area_id
    form.count = attrs.count
    form.originalPrice = attrs.original_price
    form.originalPriceCurrency = attrs.original_price_currency
    form.convertedOriginalPrice = attrs.converted_original_price
    form.currentPrice = attrs.current_price
    form.serialNumber = attrs.serial_number || ''
    form.extraSerialNumbers = attrs.extra_serial_numbers || []
    form.partNumbers = attrs.part_numbers || []
    form.tags = attrs.tags || []
    form.status = attrs.status
    form.purchaseDate = attrs.purchase_date
    form.urls = attrs.urls || []
    form.comments = attrs.comments || ''
    form.draft = attrs.draft || false

    loading.value = false
  } catch (err: any) {
    console.error('Error loading commodity data:', err)
    loading.value = false
    error.value = 'Failed to load commodity data: ' + (err.message || 'Unknown error')
  }
})

const handleValidation = (isValid: boolean, validationErrors: any) => {
  // Update our local errors object with validation errors from the form component
  if (!isValid) {
    Object.assign(errors, validationErrors)
  } else {
    // Reset errors when the form becomes valid
    Object.keys(errors).forEach(key => {
      errors[key] = ''
    })
  }
}

const submitForm = async (formData: any) => {
  isSubmitting.value = true
  formError.value = null
  debugInfo.value = null

  try {
    // Create the payload with snake_case keys as expected by the API
    const payload = {
      data: {
        id: id,
        type: 'commodities',
        attributes: {
          name: formData.name.trim(),
          short_name: formData.shortName.trim(),
          type: formData.type,
          area_id: formData.areaId,
          count: formData.count,
          original_price: formData.originalPrice,
          original_price_currency: formData.originalPriceCurrency,
          converted_original_price: formData.convertedOriginalPrice,
          current_price: formData.currentPrice,
          serial_number: formData.serialNumber || null,
          extra_serial_numbers: formData.extraSerialNumbers.length > 0 ? formData.extraSerialNumbers : null,
          part_numbers: formData.partNumbers.length > 0 ? formData.partNumbers : null,
          tags: formData.tags.length > 0 ? formData.tags : null,
          status: formData.status,
          purchase_date: formData.purchaseDate,
          urls: formData.urls.length > 0 ? formData.urls : null,
          comments: formData.comments || null,
          draft: formData.draft
        }
      }
    }

    // Log what we're sending
    console.log('Updating commodity with payload:', JSON.stringify(payload, null, 2))
    debugInfo.value = `Sending: ${JSON.stringify(payload, null, 2)}`

    // Make the API call
    const response = await commodityService.updateCommodity(id, payload)

    console.log('Success response:', response.data)
    debugInfo.value += `\n\nResponse: ${JSON.stringify(response.data, null, 2)}`

    // Handle navigation based on the edit context
    if (isDirectEdit.value) {
      if (sourceIsArea.value && areaId.value) {
        // Go back to the area view with the commodity ID for highlighting
        router.push({
          path: `/areas/${areaId.value}`,
          query: {
            highlightCommodityId: id
          }
        })
      } else {
        // Go back to the commodities list with the commodity ID for highlighting
        router.push({
          path: '/commodities',
          query: {
            highlightCommodityId: id
          }
        })
      }
    } else {
      // Navigate back to commodity details with source context preserved
      router.push({
        path: `/commodities/${id}`,
        query: {
          source: route.query.source,
          areaId: route.query.areaId
        }
      })
    }
  } catch (err: any) {
    console.error('Error updating commodity:', err)

    if (err.response) {
      console.error('Response status:', err.response.status)
      console.error('Response data:', err.response.data)

      debugInfo.value += `\n\nError Response: ${JSON.stringify(err.response.data, null, 2)}`

      // Extract validation errors if present
      const apiErrors = err.response.data.errors?.[0]?.error?.error?.data?.attributes || {}

      // Set errors on the form component
      if (commodityForm.value && commodityForm.value.setErrors) {
        commodityForm.value.setErrors(apiErrors)
      }

      if (Object.values(apiErrors).some(e => e)) {
        formError.value = 'Please correct the errors above.'
      } else {
        formError.value = `Failed to update commodity: ${err.response.status} - ${JSON.stringify(err.response.data)}`
      }
    } else {
      formError.value = 'Failed to update commodity: ' + (err.message || 'Unknown error')
    }
  } finally {
    isSubmitting.value = false
  }
}

const goBack = () => {
  if (isDirectEdit.value) {
    if (sourceIsArea.value && areaId.value) {
      router.push(`/areas/${areaId.value}`)
    } else {
      router.push('/commodities')
    }
  } else {
    router.push({
      path: `/commodities/${id}`,
      query: {
        source: route.query.source,
        areaId: route.query.areaId
      }
    })
  }
}
</script>

<style lang="scss" scoped>
@use '@/assets/main' as *;

.commodity-edit {
  max-width: 800px;
  margin: 0 auto;
  padding: 1rem;
}

h1 {
  margin-bottom: 2rem;
}

.loading, .error, .not-found {
  text-align: center;
  padding: 2rem;
  background: white;
  border-radius: $default-radius;
  box-shadow: $box-shadow;
}

.error {
  color: $danger-color;
}

.form-error {
  margin-top: 1rem;
  padding: 0.75rem;
  background-color: #f8d7da;
  color: $error-text-color;
  border-radius: $default-radius;
}

.debug-info {
  margin-top: 2rem;
  background-color: $light-bg-color;
  padding: 1rem;
  border-radius: $default-radius;
  border: 1px solid $border-color;

  pre {
    white-space: pre-wrap;
    word-wrap: break-word;
    overflow-x: auto;
  }
}
</style>

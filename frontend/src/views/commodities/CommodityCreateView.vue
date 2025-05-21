<template>
  <div class="commodity-create">
    <div class="breadcrumb-nav">
      <a v-if="areaFromUrl" href="#" class="breadcrumb-link" @click.prevent="navigateToArea">
        <font-awesome-icon icon="arrow-left" /> Back to Area
      </a>
      <a v-else href="#" class="breadcrumb-link" @click.prevent="navigateToCommodities">
        <font-awesome-icon icon="arrow-left" /> Back to Commodities
      </a>
    </div>
    <h1>Create New Commodity</h1>

    <CommodityForm
      ref="commodityForm"
      :initial-data="form"
      :areas="groupedAreas"
      :currencies="currencies"
      :main-currency="mainCurrency"
      :area-from-url="areaFromUrl"
      :is-submitting="isSubmitting"
      submit-button-text="Create Commodity"
      submit-button-loading-text="Creating..."
      @submit="submitForm"
      @cancel="cancel"
      @validate="handleValidation"
    />

    <div v-if="formError" class="form-error">{{ formError }}</div>
    <div v-if="debugInfo" class="debug-info">
      <h3>Debug Information</h3>
      <pre>{{ debugInfo }}</pre>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted, computed } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import axios from 'axios'
import commodityService from '@/services/commodityService'
import settingsService from '@/services/settingsService'
import { useSettingsStore } from '@/stores/settingsStore'
import { COMMODITY_STATUS_IN_USE } from '@/constants/commodityStatuses'
import CommodityForm from '@/components/CommodityForm.vue'

const router = useRouter()
const route = useRoute()
const settingsStore = useSettingsStore()
const commodityForm = ref(null)
const isSubmitting = ref(false)
const error = ref<string | null>(null)
const formError = ref<string | null>(null)
const debugInfo = ref<string | null>(null)
const areas = ref<any[]>([])
const locations = ref<any[]>([])

const currencies = ref<any[]>([])
const areaFromUrl = ref<string | null>(null)
const areaName = ref<string>('')

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

const today = new Date().toISOString().split('T')[0]

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
  purchaseDate: today,
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
    // Check if area ID is provided in the URL
    if (route.query.area) {
      areaFromUrl.value = route.query.area as string
      console.log('Area ID from URL:', areaFromUrl.value)
    }

    // Fetch main currency from the store
    // (fetched in App.vue)
    await settingsStore.fetchMainCurrency()

    // Fetch areas, locations, and currencies in parallel
    const [areasResponse, locationsResponse, currenciesResponse] = await Promise.all([
      axios.get('/api/v1/areas', {
        headers: {
          'Accept': 'application/vnd.api+json'
        }
      }),
      axios.get('/api/v1/locations', {
        headers: {
          'Accept': 'application/vnd.api+json'
        }
      }),
      settingsService.getCurrencies()
    ])

    areas.value = areasResponse.data.data
    locations.value = locationsResponse.data.data
    console.log('Loaded areas:', areas.value)
    console.log('Loaded locations:', locations.value)

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

    console.log('Loaded currencies:', currencies.value)

    // Set the form's currency to the main currency
    form.originalPriceCurrency = mainCurrency.value

    // Check if we have locations and areas
    if (locations.value.length === 0 || areas.value.length === 0) {
      // Redirect to commodities list which will show the appropriate message
      router.push('/commodities')
      return
    }

    // If area ID is provided in the URL, set it in the form
    if (areaFromUrl.value) {
      form.areaId = areaFromUrl.value

      // Find the area name for display
      const selectedArea = areas.value.find(area => area.id === areaFromUrl.value)
      if (selectedArea) {
        areaName.value = selectedArea.attributes.name
      }
    }
  } catch (err: any) {
    error.value = 'Failed to load data: ' + (err.message || 'Unknown error')
    console.error('Failed to load data:', err)
  }
})

// Validation is now handled by the CommodityForm component

const submitForm = async (formData: any) => {
  console.log('CommodityCreateView: submitForm called with formData:', formData)
  isSubmitting.value = true
  error.value = null
  formError.value = null
  debugInfo.value = null

  try {
    // Create the payload with snake_case keys as expected by the API
    const payload = {
      data: {
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
    console.log('Submitting commodity with payload:', JSON.stringify(payload, null, 2))
    debugInfo.value = `Sending: ${JSON.stringify(payload, null, 2)}`

    // Make the API call using commodityService
    const response = await commodityService.createCommodity(payload)

    console.log('Success response:', response.data)
    debugInfo.value += `\n\nResponse: ${JSON.stringify(response.data, null, 2)}`

    // On success, navigate to commodity details page with context preserved
    const newCommodityId = response.data.data.id

    // If coming from an area, preserve that context
    if (areaFromUrl.value) {
      router.push({
        path: `/commodities/${newCommodityId}`,
        query: {
          source: 'area',
          areaId: areaFromUrl.value
        }
      })
    } else {
      // Otherwise, navigate with commodities as source
      router.push({
        path: `/commodities/${newCommodityId}`,
        query: {
          source: 'commodities'
        }
      })
    }
  } catch (err: any) {
    console.error('Error creating commodity:', err)

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
        formError.value = `Failed to create commodity: ${err.response.status} - ${JSON.stringify(err.response.data)}`
      }
    } else {
      formError.value = 'Failed to create commodity: ' + (err.message || 'Unknown error')
    }
  } finally {
    isSubmitting.value = false
  }
}

const navigateToArea = () => {
  // Navigate back to the area detail view
  if (areaFromUrl.value) {
    router.push(`/areas/${areaFromUrl.value}`)
  }
}

const navigateToCommodities = () => {
  // Navigate back to the commodities list
  router.push('/commodities')
}

const cancel = () => {
  // If coming from an area, navigate back to that area
  if (areaFromUrl.value) {
    router.push(`/areas/${areaFromUrl.value}`)
  } else {
    // Otherwise, navigate to the commodities list
    router.push('/commodities')
  }
}



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
</script>

<style lang="scss" scoped>
@use '@/assets/main.scss' as *;

.commodity-create {
  max-width: 800px;
  margin: 0 auto;
  padding: 1rem;
}

h1 {
  margin-bottom: 2rem;
}

.form {
  background: white;
  padding: 2rem;
  border-radius: $default-radius;
  box-shadow: $box-shadow;
}

.form-section {
  margin-bottom: 2rem;
  padding-bottom: 1rem;
  border-bottom: 1px solid #eee;

  h2 {
    margin-bottom: 1.5rem;
    font-size: 1.5rem;
  }
}

.form-group {
  margin-bottom: 1.5rem;
}

.form-error {
  margin-top: 1rem;
  padding: 0.75rem;
  background-color: #f8d7da;
  color: $error-text-color;
  border-radius: $default-radius;
}

.array-input {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}

.array-item {
  display: flex;
  gap: 0.5rem;
}

.checkbox-label {
  display: flex;
  align-items: center;
  gap: 0.5rem;
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

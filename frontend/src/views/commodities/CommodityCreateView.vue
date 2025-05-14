<template>
  <div class="commodity-create">
    <div class="breadcrumb-nav">
      <a v-if="areaFromUrl" href="#" @click.prevent="navigateToArea" class="breadcrumb-link">
        <font-awesome-icon icon="arrow-left" /> Back to Area
      </a>
      <a v-else href="#" @click.prevent="navigateToCommodities" class="breadcrumb-link">
        <font-awesome-icon icon="arrow-left" /> Back to Commodities
      </a>
    </div>
    <h1>Create New Commodity</h1>

    <div v-if="error" class="error-alert">{{ error }}</div>

    <CommodityForm
      :initial-data="form"
      :areas="areas"
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

    <div v-if="debugInfo" class="debug-info">
      <h3>Debug Information</h3>
      <pre>{{ debugInfo }}</pre>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import axios from 'axios'
import commodityService from '@/services/commodityService'
import areaService from '@/services/areaService'
import locationService from '@/services/locationService'
import settingsService from '@/services/settingsService'
import { COMMODITY_TYPES } from '@/constants/commodityTypes'
import { COMMODITY_STATUSES, COMMODITY_STATUS_IN_USE } from '@/constants/commodityStatuses'
import { CURRENCY_CZK } from '@/constants/currencies'
import CommodityForm from '@/components/CommodityForm.vue'

const router = useRouter()
const route = useRoute()
const isSubmitting = ref(false)
const error = ref<string | null>(null)
const debugInfo = ref<string | null>(null)
const areas = ref<any[]>([])
const commodityTypes = ref(COMMODITY_TYPES)
const commodityStatuses = ref(COMMODITY_STATUSES)
const currencies = ref<any[]>([])
const areaFromUrl = ref<string | null>(null)
const areaName = ref<string>('')
const mainCurrency = ref<string>(CURRENCY_CZK)

const today = new Date().toISOString().split('T')[0]

const form = reactive({
  name: '',
  shortName: '',
  type: '',
  areaId: '',
  count: 1,
  originalPrice: 0,
  originalPriceCurrency: CURRENCY_CZK,
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

    // Fetch areas, locations, currencies, and main currency in parallel
    const [areasResponse, locationsResponse, currenciesResponse, mainCurrencyResponse] = await Promise.all([
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
      settingsService.getCurrencies(),
      settingsService.getMainCurrency()
    ])

    areas.value = areasResponse.data.data
    const locations = locationsResponse.data.data
    console.log('Loaded areas:', areas.value)
    console.log('Loaded locations:', locations)

    // Process currencies with proper names
    const currencyCodes = currenciesResponse.data
    const currencyNames = new Intl.DisplayNames(['en'], { type: 'currency' })

    currencies.value = currencyCodes.map((code: string) => {
      let currencyName = code
      try {
        // Try to get the localized currency name
        currencyName = currencyNames.of(code)
      } catch (e) {
        console.warn(`Could not get display name for currency: ${code}`)
      }

      return {
        code: code,
        label: `${currencyName} (${code})`
      }
    })

    console.log('Loaded currencies:', currencies.value)

    // Set main currency if available
    if (mainCurrencyResponse) {
      mainCurrency.value = mainCurrencyResponse
      form.originalPriceCurrency = mainCurrency.value
    }

    // Check if we have locations and areas
    if (locations.length === 0 || areas.value.length === 0) {
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

const validateForm = () => {
  let isValid = true

  // Reset errors
  errors.name = ''
  errors.shortName = ''
  errors.type = ''
  errors.areaId = ''
  errors.count = ''
  errors.originalPrice = ''
  errors.originalPriceCurrency = ''
  errors.convertedOriginalPrice = ''
  errors.currentPrice = ''
  errors.serialNumber = ''
  errors.status = ''
  errors.purchaseDate = ''
  errors.comments = ''

  if (!form.name.trim()) {
    errors.name = 'Name is required'
    isValid = false
  }

  if (!form.shortName.trim()) {
    errors.shortName = 'Short Name is required'
    isValid = false
  }

  if (!form.type) {
    errors.type = 'Type is required'
    isValid = false
  }

  if (!form.areaId) {
    errors.areaId = 'Area is required'
    isValid = false
  }

  if (form.count < 1) {
    errors.count = 'Count must be at least 1'
    isValid = false
  }

  if (form.originalPrice < 0) {
    errors.originalPrice = 'Original Price cannot be negative'
    isValid = false
  }

  if (!form.originalPriceCurrency) {
    errors.originalPriceCurrency = 'Original Price Currency is required'
    isValid = false
  }

  if (form.convertedOriginalPrice < 0) {
    errors.convertedOriginalPrice = 'Converted Original Price cannot be negative'
    isValid = false
  }

  if (form.currentPrice < 0) {
    errors.currentPrice = 'Current Price cannot be negative'
    isValid = false
  }

  if (!form.status) {
    errors.status = 'Status is required'
    isValid = false
  }

  if (!form.purchaseDate) {
    errors.purchaseDate = 'Purchase Date is required'
    isValid = false
  }

  if (form.purchaseDate! > today) {
    errors.purchaseDate = 'Purchase Date cannot be in the future'
    isValid = false
  }

  if (form.comments! && form.comments! > 1000) {
    errors.comments = 'Comments cannot exceed 1000 characters'
    isValid = false
  }

  return isValid
}

const submitForm = async () => {
  if (!validateForm()) return

  isSubmitting.value = true
  error.value = null
  debugInfo.value = null

  try {
    // Create the payload with snake_case keys as expected by the API
    const payload = {
      data: {
        type: 'commodities',
        attributes: {
          name: form.name.trim(),
          short_name: form.shortName.trim(),
          type: form.type,
          area_id: form.areaId,
          count: form.count,
          original_price: form.originalPrice,
          original_price_currency: form.originalPriceCurrency,
          converted_original_price: form.convertedOriginalPrice,
          current_price: form.currentPrice,
          serial_number: form.serialNumber || null,
          extra_serial_numbers: form.extraSerialNumbers.length > 0 ? form.extraSerialNumbers : null,
          part_numbers: form.partNumbers.length > 0 ? form.partNumbers : null,
          tags: form.tags.length > 0 ? form.tags : null,
          status: form.status,
          purchase_date: form.purchaseDate,
          urls: form.urls.length > 0 ? form.urls : null,
          comments: form.comments || null,
          draft: form.draft
        }
      }
    }

    // Log what we're sending
    console.log('Submitting commodity with payload:', JSON.stringify(payload, null, 2))
    debugInfo.value = `Sending: ${JSON.stringify(payload, null, 2)}`

    // Make the API call
    const response = await axios.post('/api/v1/commodities', payload, {
      headers: {
        'Content-Type': 'application/vnd.api+json',
        'Accept': 'application/vnd.api+json'
      }
    })

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

      // Map API errors to form errors
      Object.keys(apiErrors).forEach(key => {
        const camelKey = key.replace(/_([a-z])/g, (_, letter) => letter.toUpperCase())
        if (errors.hasOwnProperty(camelKey)) {
          errors[camelKey] = apiErrors[key]
        }
      })

      if (Object.values(errors).some(e => e)) {
        error.value = 'Please correct the errors above.'
      } else {
        error.value = `Failed to create commodity: ${err.response.status} - ${JSON.stringify(err.response.data)}`
      }
    } else {
      error.value = 'Failed to create commodity: ' + (err.message || 'Unknown error')
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

const addExtraSerialNumber = () => {
  form.extraSerialNumbers.push('')
}

const removeExtraSerialNumber = (index: number) => {
  form.extraSerialNumbers.splice(index, 1)
}

const addPartNumber = () => {
  form.partNumbers.push('')
}

const removePartNumber = (index: number) => {
  form.partNumbers.splice(index, 1)
}

const addTag = () => {
  form.tags.push('')
}

const removeTag = (index: number) => {
  form.tags.splice(index, 1)
}

const addUrl = () => {
  form.urls.push('')
}

const removeUrl = (index: number) => {
  form.urls.splice(index, 1)
}

const handleValidation = (isValid: boolean, validationErrors: any) => {
  // Update our local errors object with validation errors from the form component
  if (!isValid) {
    Object.assign(errors, validationErrors)
  }
}
</script>

<style lang="scss" scoped>
@import '../../assets/main.scss';

.commodity-create {
  max-width: 800px;
  margin: 0 auto;
  padding: 1rem;
}

.breadcrumb-nav {
  margin-bottom: 1rem;
}

.breadcrumb-link {
  color: $secondary-color;
  font-size: 0.9rem;
  text-decoration: none;
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
  transition: color 0.2s;

  &:hover {
    color: $primary-color;
    text-decoration: none;
  }
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

label {
  display: block;
  margin-bottom: 0.5rem;
  font-weight: 500;
}

.form-control {
  width: 100%;
  padding: 0.75rem;
  border: 1px solid $border-color;
  border-radius: $default-radius;
  font-size: 1rem;

  &:focus {
    outline: none;
    border-color: $primary-color;
    box-shadow: 0 0 0 2px rgba($primary-color, 0.2);
  }

  &.is-invalid {
    border-color: $danger-color;
  }
}

textarea.form-control {
  resize: vertical;
}

.error-message {
  color: $danger-color;
  font-size: 0.875rem;
  margin-top: 0.25rem;
}

.form-actions {
  display: flex;
  justify-content: flex-end;
  gap: 1rem;
  margin-top: 2rem;
}

.btn {
  padding: 0.75rem 1.5rem;
  border: none;
  border-radius: $default-radius;
  cursor: pointer;
  font-weight: 500;

  &:disabled {
    background-color: #cccccc;
    cursor: not-allowed;
  }
}

.btn-primary {
  background-color: $primary-color;
  color: white;
}

.btn-secondary {
  background-color: $light-bg-color;
  color: $text-color;
}

.btn-danger {
  background-color: $danger-color;
  color: white;
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

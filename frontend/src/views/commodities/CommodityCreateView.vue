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

    <form @submit.prevent="submitForm" class="form">
      <!-- Basic Information -->
      <div class="form-section">
        <h2>Basic Information</h2>

        <div class="form-group">
          <label for="name">Name</label>
          <input
            type="text"
            id="name"
            v-model="form.name"
            required
            class="form-control"
            :class="{ 'is-invalid': errors.name }"
          >
          <div v-if="errors.name" class="error-message">{{ errors.name }}</div>
        </div>

        <div class="form-group">
          <label for="shortName">Short Name</label>
          <input
            type="text"
            id="shortName"
            v-model="form.shortName"
            required
            maxlength="20"
            class="form-control"
            :class="{ 'is-invalid': errors.shortName }"
          >
          <div v-if="errors.shortName" class="error-message">{{ errors.shortName }}</div>
        </div>

        <div class="form-group">
          <label for="type">Type</label>
          <select
            id="type"
            v-model="form.type"
            required
            class="form-control"
            :class="{ 'is-invalid': errors.type }"
          >
            <option value="" disabled>Select a type</option>
            <option v-for="type in commodityTypes" :key="type.id" :value="type.id">
              {{ type.name }}
            </option>
          </select>
          <div v-if="errors.type" class="error-message">{{ errors.type }}</div>
        </div>

        <div class="form-group">
          <label for="area">Area</label>
          <div v-if="areaFromUrl">
            <input
              type="text"
              id="area"
              :value="areaName"
              disabled
              class="form-control"
            >
            <input type="hidden" v-model="form.areaId">
          </div>
          <select
            v-else
            id="area"
            v-model="form.areaId"
            required
            class="form-control"
            :class="{ 'is-invalid': errors.areaId }"
          >
            <option value="" disabled>Select an area</option>
            <option v-for="area in areas" :key="area.id" :value="area.id">
              {{ area.attributes.name }}
            </option>
          </select>
          <div v-if="errors.areaId" class="error-message">{{ errors.areaId }}</div>
        </div>

        <div class="form-group">
          <label for="count">Count</label>
          <input
            type="number"
            id="count"
            v-model.number="form.count"
            required
            min="1"
            class="form-control"
            :class="{ 'is-invalid': errors.count }"
          >
          <div v-if="errors.count" class="error-message">{{ errors.count }}</div>
        </div>
      </div>

      <!-- Price Information -->
      <div class="form-section">
        <h2>Price Information</h2>

        <div class="form-group">
          <label for="originalPrice">Original Price</label>
          <input
            type="number"
            id="originalPrice"
            v-model.number="form.originalPrice"
            required
            min="0"
            step="0.01"
            class="form-control"
            :class="{ 'is-invalid': errors.originalPrice }"
          >
          <div v-if="errors.originalPrice" class="error-message">{{ errors.originalPrice }}</div>
        </div>

        <div class="form-group">
          <label for="originalPriceCurrency">Original Price Currency</label>
          <select
            id="originalPriceCurrency"
            v-model="form.originalPriceCurrency"
            required
            class="form-control"
            :class="{ 'is-invalid': errors.originalPriceCurrency }"
          >
            <option value="" disabled>Select a currency</option>
            <option v-for="currency in currencies" :key="currency.id" :value="currency.id">
              {{ currency.name }}
            </option>
          </select>
          <div v-if="errors.originalPriceCurrency" class="error-message">{{ errors.originalPriceCurrency }}</div>
        </div>

        <div class="form-group">
          <label for="convertedOriginalPrice">Converted Original Price</label>
          <input
            type="number"
            id="convertedOriginalPrice"
            v-model.number="form.convertedOriginalPrice"
            required
            min="0"
            step="0.01"
            class="form-control"
            :class="{ 'is-invalid': errors.convertedOriginalPrice }"
          >
          <div v-if="errors.convertedOriginalPrice" class="error-message">{{ errors.convertedOriginalPrice }}</div>
        </div>

        <div class="form-group">
          <label for="currentPrice">Current Price</label>
          <input
            type="number"
            id="currentPrice"
            v-model.number="form.currentPrice"
            required
            min="0"
            step="0.01"
            class="form-control"
            :class="{ 'is-invalid': errors.currentPrice }"
          >
          <div v-if="errors.currentPrice" class="error-message">{{ errors.currentPrice }}</div>
        </div>
      </div>

      <!-- Serial Numbers and Part Numbers -->
      <div class="form-section">
        <h2>Serial Numbers and Part Numbers</h2>

        <div class="form-group">
          <label for="serialNumber">Serial Number</label>
          <input
            type="text"
            id="serialNumber"
            v-model="form.serialNumber"
            class="form-control"
            :class="{ 'is-invalid': errors.serialNumber }"
          >
          <div v-if="errors.serialNumber" class="error-message">{{ errors.serialNumber }}</div>
        </div>

        <div class="form-group">
          <label>Extra Serial Numbers</label>
          <div class="array-input">
            <div v-for="(item, index) in form.extraSerialNumbers" :key="index" class="array-item">
              <input
                type="text"
                v-model="form.extraSerialNumbers[index]"
                class="form-control"
              >
              <button type="button" class="btn btn-danger" @click="removeExtraSerialNumber(index)">Remove</button>
            </div>
            <button type="button" class="btn btn-secondary" @click="addExtraSerialNumber">Add Serial Number</button>
          </div>
        </div>

        <div class="form-group">
          <label>Part Numbers</label>
          <div class="array-input">
            <div v-for="(item, index) in form.partNumbers" :key="index" class="array-item">
              <input
                type="text"
                v-model="form.partNumbers[index]"
                class="form-control"
              >
              <button type="button" class="btn btn-danger" @click="removePartNumber(index)">Remove</button>
            </div>
            <button type="button" class="btn btn-secondary" @click="addPartNumber">Add Part Number</button>
          </div>
        </div>
      </div>

      <!-- Tags and Status -->
      <div class="form-section">
        <h2>Tags and Status</h2>

        <div class="form-group">
          <label>Tags</label>
          <div class="array-input">
            <div v-for="(item, index) in form.tags" :key="index" class="array-item">
              <input
                type="text"
                v-model="form.tags[index]"
                class="form-control"
              >
              <button type="button" class="btn btn-danger" @click="removeTag(index)">Remove</button>
            </div>
            <button type="button" class="btn btn-secondary" @click="addTag">Add Tag</button>
          </div>
        </div>

        <div class="form-group">
          <label for="status">Status</label>
          <select
            id="status"
            v-model="form.status"
            required
            class="form-control"
            :class="{ 'is-invalid': errors.status }"
          >
            <option value="" disabled>Select a status</option>
            <option v-for="status in commodityStatuses" :key="status.id" :value="status.id">
              {{ status.name }}
            </option>
          </select>
          <div v-if="errors.status" class="error-message">{{ errors.status }}</div>
        </div>

        <div class="form-group">
          <label for="purchaseDate">Purchase Date</label>
          <input
            type="date"
            id="purchaseDate"
            v-model="form.purchaseDate"
            required
            class="form-control"
            :class="{ 'is-invalid': errors.purchaseDate }"
          >
          <div v-if="errors.purchaseDate" class="error-message">{{ errors.purchaseDate }}</div>
        </div>
      </div>

      <!-- URLs and Comments -->
      <div class="form-section">
        <h2>URLs and Comments</h2>

        <div class="form-group">
          <label>URLs</label>
          <div class="array-input">
            <div v-for="(item, index) in form.urls" :key="index" class="array-item">
              <input
                type="url"
                v-model="form.urls[index]"
                class="form-control"
              >
              <button type="button" class="btn btn-danger" @click="removeUrl(index)">Remove</button>
            </div>
            <button type="button" class="btn btn-secondary" @click="addUrl">Add URL</button>
          </div>
        </div>

        <div class="form-group">
          <label for="comments">Comments</label>
          <textarea
            id="comments"
            v-model="form.comments"
            class="form-control"
            :class="{ 'is-invalid': errors.comments }"
            rows="4"
          ></textarea>
          <div v-if="errors.comments" class="error-message">{{ errors.comments }}</div>
        </div>

        <div class="form-group">
          <label class="checkbox-label">
            <input type="checkbox" v-model="form.draft">
            <span>Draft</span>
          </label>
        </div>
      </div>

      <div class="form-actions">
        <button type="button" class="btn btn-secondary" @click="cancel">Cancel</button>
        <button type="submit" class="btn btn-primary" :disabled="isSubmitting">
          {{ isSubmitting ? 'Creating...' : 'Create Commodity' }}
        </button>
      </div>

      <div v-if="error" class="form-error">{{ error }}</div>

      <!-- Debug information -->
      <div v-if="debugInfo" class="debug-info">
        <h3>Debug Information</h3>
        <pre>{{ debugInfo }}</pre>
      </div>
    </form>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import axios from 'axios'
import commodityService from '@/services/commodityService'
import { COMMODITY_TYPES } from '@/constants/commodityTypes'
import { COMMODITY_STATUSES, COMMODITY_STATUS_IN_USE } from '@/constants/commodityStatuses'
import { CURRENCIES, CURRENCY_CZK } from '@/constants/currencies'

const router = useRouter()
const route = useRoute()
const isSubmitting = ref(false)
const error = ref<string | null>(null)
const debugInfo = ref<string | null>(null)
const areas = ref<any[]>([])
const commodityTypes = ref(COMMODITY_TYPES)
const commodityStatuses = ref(COMMODITY_STATUSES)
const currencies = ref(CURRENCIES)
const areaFromUrl = ref<string | null>(null)
const areaName = ref<string>('')

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

    // Fetch areas
    const response = await axios.get('/api/v1/areas', {
      headers: {
        'Accept': 'application/vnd.api+json'
      }
    })
    areas.value = response.data.data
    console.log('Loaded areas:', areas.value)

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
    error.value = 'Failed to load areas: ' + (err.message || 'Unknown error')
    console.error('Failed to load areas:', err)
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
</script>

<style scoped>
.commodity-create {
  max-width: 800px;
  margin: 0 auto;
  padding: 1rem;
}

.breadcrumb-nav {
  margin-bottom: 1rem;
}

.breadcrumb-link {
  color: #6c757d;
  font-size: 0.9rem;
  text-decoration: none;
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
  transition: color 0.2s;
}

.breadcrumb-link:hover {
  color: #4CAF50;
  text-decoration: none;
}

h1 {
  margin-bottom: 2rem;
}

.form {
  background: white;
  padding: 2rem;
  border-radius: 8px;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
}

.form-section {
  margin-bottom: 2rem;
  padding-bottom: 1rem;
  border-bottom: 1px solid #eee;
}

.form-section h2 {
  margin-bottom: 1.5rem;
  font-size: 1.5rem;
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
  border: 1px solid #ddd;
  border-radius: 4px;
  font-size: 1rem;
}

textarea.form-control {
  resize: vertical;
}

.form-control:focus {
  outline: none;
  border-color: #4CAF50;
  box-shadow: 0 0 0 2px rgba(76, 175, 80, 0.2);
}

.form-control.is-invalid {
  border-color: #dc3545;
}

.error-message {
  color: #dc3545;
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
  border-radius: 4px;
  cursor: pointer;
  font-weight: 500;
}

.btn-primary {
  background-color: #4CAF50;
  color: white;
}

.btn-secondary {
  background-color: #f0f0f0;
  color: #333;
}

.btn-danger {
  background-color: #dc3545;
  color: white;
}

.btn:disabled {
  background-color: #cccccc;
  cursor: not-allowed;
}

.form-error {
  margin-top: 1rem;
  padding: 0.75rem;
  background-color: #f8d7da;
  color: #721c24;
  border-radius: 4px;
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
  background-color: #f8f9fa;
  padding: 1rem;
  border-radius: 4px;
  border: 1px solid #ddd;
}

pre {
  white-space: pre-wrap;
  word-wrap: break-word;
  overflow-x: auto;
}
</style>

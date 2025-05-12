<template>
  <div class="commodity-edit">
    <div class="breadcrumb-nav">
      <a href="#" @click.prevent="goBack" class="breadcrumb-link">
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
            <select
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
              min="1"
              required
              class="form-control"
              :class="{ 'is-invalid': errors.count }"
            >
            <div v-if="errors.count" class="error-message">{{ errors.count }}</div>
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

        <!-- Price Information -->
        <div class="form-section">
          <h2>Price Information</h2>

          <div class="form-group">
            <label for="originalPrice">Original Price</label>
            <input
              type="number"
              id="originalPrice"
              v-model.number="form.originalPrice"
              step="0.01"
              min="0"
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
              step="0.01"
              min="0"
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
              step="0.01"
              min="0"
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

          <!-- Extra Serial Numbers -->
          <div class="form-group">
            <label>Extra Serial Numbers</label>
            <div v-for="(serial, index) in form.extraSerialNumbers" :key="index" class="array-input-row">
              <input
                type="text"
                v-model="form.extraSerialNumbers[index]"
                class="form-control"
              >
              <button type="button" class="btn btn-danger btn-sm" @click="removeExtraSerialNumber(index)">
                Remove
              </button>
            </div>
            <button type="button" class="btn btn-secondary btn-sm" @click="addExtraSerialNumber">
              Add Serial Number
            </button>
          </div>

          <!-- Part Numbers -->
          <div class="form-group">
            <label>Part Numbers</label>
            <div v-for="(part, index) in form.partNumbers" :key="index" class="array-input-row">
              <input
                type="text"
                v-model="form.partNumbers[index]"
                class="form-control"
              >
              <button type="button" class="btn btn-danger btn-sm" @click="removePartNumber(index)">
                Remove
              </button>
            </div>
            <button type="button" class="btn btn-secondary btn-sm" @click="addPartNumber">
              Add Part Number
            </button>
          </div>
        </div>

        <!-- Tags -->
        <div class="form-section">
          <h2>Tags</h2>
          <div class="form-group">
            <div v-for="(tag, index) in form.tags" :key="index" class="array-input-row">
              <input
                type="text"
                v-model="form.tags[index]"
                class="form-control"
              >
              <button type="button" class="btn btn-danger btn-sm" @click="removeTag(index)">
                Remove
              </button>
            </div>
            <button type="button" class="btn btn-secondary btn-sm" @click="addTag">
              Add Tag
            </button>
          </div>
        </div>

        <!-- URLs -->
        <div class="form-section">
          <h2>URLs</h2>
          <div class="form-group">
            <div v-for="(url, index) in form.urls" :key="index" class="array-input-row">
              <input
                type="url"
                v-model="form.urls[index]"
                class="form-control"
              >
              <button type="button" class="btn btn-danger btn-sm" @click="removeUrl(index)">
                Remove
              </button>
            </div>
            <button type="button" class="btn btn-secondary btn-sm" @click="addUrl">
              Add URL
            </button>
          </div>
        </div>

        <!-- Comments -->
        <div class="form-section">
          <h2>Comments</h2>
          <div class="form-group">
            <textarea
              id="comments"
              v-model="form.comments"
              rows="4"
              class="form-control"
              :class="{ 'is-invalid': errors.comments }"
            ></textarea>
            <div v-if="errors.comments" class="error-message">{{ errors.comments }}</div>
          </div>
        </div>

        <!-- Draft Status -->
        <div class="form-section">
          <div class="form-group">
            <label class="checkbox-label">
              <input type="checkbox" v-model="form.draft">
              Draft
            </label>
          </div>
        </div>

        <div class="form-actions">
          <button type="button" class="btn btn-secondary" @click="goBack">Cancel</button>
          <button type="submit" class="btn btn-primary" :disabled="isSubmitting">
            {{ isSubmitting ? 'Saving...' : 'Save Changes' }}
          </button>
        </div>

        <div v-if="formError" class="form-error">{{ formError }}</div>
        <div v-if="debugInfo" class="debug-info">
          <h3>Debug Info</h3>
          <pre>{{ debugInfo }}</pre>
        </div>
      </form>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted, computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import commodityService from '@/services/commodityService'
import areaService from '@/services/areaService'
import { COMMODITY_TYPES } from '@/constants/commodityTypes'
import { COMMODITY_STATUSES, COMMODITY_STATUS_IN_USE } from '@/constants/commodityStatuses'
import { CURRENCIES, CURRENCY_CZK } from '@/constants/currencies'

const route = useRoute()
const router = useRouter()
const id = route.params.id as string

// Navigation source tracking
const sourceIsArea = computed(() => route.query.source === 'area')
const areaId = computed(() => route.query.areaId as string || '')
const isDirectEdit = computed(() => route.query.directEdit === 'true')

const commodity = ref<any>(null)
const areas = ref<any[]>([])
const loading = ref<boolean>(true)
const error = ref<string | null>(null)
const isSubmitting = ref<boolean>(false)
const formError = ref<string | null>(null)
const debugInfo = ref<string | null>(null)

const commodityTypes = ref(COMMODITY_TYPES)
const commodityStatuses = ref(COMMODITY_STATUSES)
const currencies = ref(CURRENCIES)

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
    // Load commodity and areas in parallel
    const [commodityResponse, areasResponse] = await Promise.all([
      commodityService.getCommodity(id),
      areaService.getAreas()
    ])

    commodity.value = commodityResponse.data.data
    areas.value = areasResponse.data.data

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
    console.error('Error loading data:', err)
    error.value = 'Failed to load commodity: ' + (err.message || 'Unknown error')
    loading.value = false
  }
})

const validateForm = () => {
  let isValid = true

  // Reset errors
  Object.keys(errors).forEach(key => {
    errors[key] = ''
  })

  if (!form.name.trim()) {
    errors.name = 'Name is required'
    isValid = false
  }

  if (!form.shortName.trim()) {
    errors.shortName = 'Short name is required'
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

  if (!form.count || form.count < 1) {
    errors.count = 'Count must be at least 1'
    isValid = false
  }

  if (!form.status) {
    errors.status = 'Status is required'
    isValid = false
  }

  if (!form.purchaseDate) {
    errors.purchaseDate = 'Purchase date is required'
    isValid = false
  }

  return isValid
}

const submitForm = async () => {
  if (!validateForm()) return

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

      // Extract validation errors if present
      const apiErrors = err.response.data.errors?.[0]?.error?.error?.data?.attributes || {}

      // Map API errors to form fields
      Object.keys(apiErrors).forEach(key => {
        const formKey = key.replace(/_([a-z])/g, (_, letter) => letter.toUpperCase())
        if (errors[formKey] !== undefined) {
          errors[formKey] = apiErrors[key]
        }
      })

      if (Object.values(errors).some(e => e)) {
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
  // If this was a direct edit from a list, go back to the appropriate list
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
}

// Helper methods for array fields
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

<style lang="scss" scoped>
@import '../../assets/main.scss';

.commodity-edit {
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
    margin-bottom: 1rem;
    color: $text-color;
  }
}

.form-group {
  margin-bottom: 1rem;

  label {
    display: block;
    margin-bottom: 0.5rem;
    font-weight: 500;
  }
}

.form-control {
  width: 100%;
  padding: 0.75rem;
  border: 1px solid $border-color;
  border-radius: $default-radius;
  font-size: 1rem;

  &.is-invalid {
    border-color: $danger-color;
  }
}

.error-message {
  color: $danger-color;
  font-size: 0.875rem;
  margin-top: 0.25rem;
}

.array-input-row {
  display: flex;
  gap: 0.5rem;
  margin-bottom: 0.5rem;
  align-items: center;

  .form-control {
    flex: 1;
  }
}

.checkbox-label {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  cursor: pointer;
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
  text-decoration: none;
  display: inline-block;
}

.btn-primary {
  background-color: $primary-color;
  color: white;
}

.btn-secondary {
  background-color: $secondary-color;
  color: white;
}

.btn-danger {
  background-color: $danger-color;
  color: white;
}

.btn-sm {
  padding: 0.25rem 0.5rem;
  font-size: 0.875rem;
}

.form-error {
  margin-top: 1rem;
  padding: 1rem;
  background-color: #f8d7da;
  color: $error-text-color;
  border-radius: $default-radius;
}

.debug-info {
  margin-top: 2rem;
  padding: 1rem;
  background-color: $light-bg-color;
  border-radius: $default-radius;
  overflow-x: auto;

  pre {
    margin: 0;
    white-space: pre-wrap;
    word-break: break-all;
  }
}
</style>

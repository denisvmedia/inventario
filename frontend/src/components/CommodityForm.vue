<template>
  <form class="form" @submit.prevent="onSubmit">
    <!-- Basic Information -->
    <div class="form-section">
      <h2>Basic Information</h2>

      <div class="form-group">
        <label for="name">Name</label>
        <input
          id="name"
          v-model="formData.name"
          type="text"
          required
          class="form-control"
          :class="{ 'is-invalid': formErrors.name }"
        >
        <div v-if="formErrors.name" class="error-message">{{ formErrors.name }}</div>
      </div>

      <div class="form-group">
        <label for="shortName">Short Name</label>
        <input
          id="shortName"
          v-model="formData.shortName"
          type="text"
          required
          maxlength="20"
          class="form-control"
          :class="{ 'is-invalid': formErrors.shortName }"
        >
        <div v-if="formErrors.shortName" class="error-message">{{ formErrors.shortName }}</div>
      </div>

      <div class="form-group">
        <label for="type">Type</label>
        <Select
          id="type"
          v-model="formData.type"
          :options="commodityTypes"
          option-label="name"
          option-value="id"
          placeholder="Select a type"
          class="w-100"
          :class="{ 'is-invalid': formErrors.type }"
        />
        <div v-if="formErrors.type" class="error-message">{{ formErrors.type }}</div>
      </div>

      <div class="form-group">
        <label for="areaId">Area</label>
        <Select
          id="areaId"
          v-model="formData.areaId"
          :options="areas"
          option-label="attributes.name"
          option-value="id"
          option-group-label="label"
          option-group-children="items"
          placeholder="Select an area"
          class="w-100"
          :class="{ 'is-invalid': formErrors.areaId }"
          :disabled="!!areaFromUrl"
        >
          <template #optiongroup="slotProps">
            <div class="location-group-label">{{ slotProps.option.label }}</div>
          </template>
        </Select>
        <div v-if="formErrors.areaId" class="error-message">{{ formErrors.areaId }}</div>
      </div>

      <div class="form-group">
        <label for="count">Count</label>
        <input
          id="count"
          v-model.number="formData.count"
          type="number"
          required
          min="1"
          class="form-control"
          :class="{ 'is-invalid': formErrors.count }"
        >
        <div v-if="formErrors.count" class="error-message">{{ formErrors.count }}</div>
      </div>
    </div>

    <!-- Price Information -->
    <div class="form-section">
      <h2>Price Information</h2>
      <div
class="price-calculation-hint" :class="{
        'inactive-hint': !isPriceUsedInCalculations,
        'warning-hint': !hasSuitablePrice && isPriceUsedInCalculations
      }" v-html="getPriceCalculationHint">
      </div>

      <div class="form-group">
        <label for="originalPrice">Original Price</label>
        <input
          id="originalPrice"
          v-model.number="formData.originalPrice"
          type="number"
          required
          min="0"
          step="0.01"
          class="form-control"
          :class="{ 'is-invalid': formErrors.originalPrice }"
        >
        <div v-if="formErrors.originalPrice" class="error-message">{{ formErrors.originalPrice }}</div>
      </div>

      <div class="form-group">
        <label for="originalPriceCurrency">Original Price Currency</label>
        <Select
          id="originalPriceCurrency"
          v-model="formData.originalPriceCurrency"
          :options="currencies"
          option-label="label"
          option-value="code"
          placeholder="Select a currency"
          class="w-100"
          :class="{ 'is-invalid': formErrors.originalPriceCurrency }"
          :filter="true"
        />
        <div v-if="formErrors.originalPriceCurrency" class="error-message">{{ formErrors.originalPriceCurrency }}</div>
      </div>

      <div v-if="showConvertedOriginalPrice" class="form-group">
        <label for="convertedOriginalPrice">Converted Original Price</label>
        <input
          id="convertedOriginalPrice"
          v-model.number="formData.convertedOriginalPrice"
          type="number"
          required
          min="0"
          step="0.01"
          class="form-control"
          :class="{ 'is-invalid': formErrors.convertedOriginalPrice }"
        >
        <div v-if="formErrors.convertedOriginalPrice" class="error-message">{{ formErrors.convertedOriginalPrice }}</div>
      </div>

      <div class="form-group">
        <label for="currentPrice">Current Price</label>
        <input
          id="currentPrice"
          v-model.number="formData.currentPrice"
          type="number"
          required
          min="0"
          step="0.01"
          class="form-control"
          :class="{ 'is-invalid': formErrors.currentPrice }"
        >
        <div v-if="formErrors.currentPrice" class="error-message">{{ formErrors.currentPrice }}</div>
      </div>
    </div>

    <!-- Serial Numbers and Part Numbers -->
    <div class="form-section">
      <h2>Serial Numbers and Part Numbers</h2>

      <div class="form-group">
        <label for="serialNumber">Serial Number</label>
        <input
          id="serialNumber"
          v-model="formData.serialNumber"
          type="text"
          class="form-control"
          :class="{ 'is-invalid': formErrors.serialNumber }"
        >
        <div v-if="formErrors.serialNumber" class="error-message">{{ formErrors.serialNumber }}</div>
      </div>

      <div class="form-group">
        <label>Extra Serial Numbers</label>
        <div class="array-input">
          <div v-for="(item, index) in formData.extraSerialNumbers" :key="index" class="array-item">
            <input
              v-model="formData.extraSerialNumbers[index]"
              type="text"
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
          <div v-for="(item, index) in formData.partNumbers" :key="index" class="array-item">
            <input
              v-model="formData.partNumbers[index]"
              type="text"
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
          <div v-for="(item, index) in formData.tags" :key="index" class="array-item">
            <input
              v-model="formData.tags[index]"
              type="text"
              class="form-control"
            >
            <button type="button" class="btn btn-danger" @click="removeTag(index)">Remove</button>
          </div>
          <button type="button" class="btn btn-secondary" @click="addTag">Add Tag</button>
        </div>
      </div>

      <div class="form-group">
        <label for="status">Status</label>
        <Select
          id="status"
          v-model="formData.status"
          :options="commodityStatuses"
          option-label="name"
          option-value="id"
          placeholder="Select a status"
          class="w-100"
          :class="{ 'is-invalid': formErrors.status }"
        />
        <div v-if="formErrors.status" class="error-message">{{ formErrors.status }}</div>
      </div>

      <div class="form-group">
        <label for="purchaseDate">Purchase Date</label>
        <input
          id="purchaseDate"
          v-model="formData.purchaseDate"
          type="date"
          required
          class="form-control"
          :class="{ 'is-invalid': formErrors.purchaseDate }"
        >
        <div v-if="formErrors.purchaseDate" class="error-message">{{ formErrors.purchaseDate }}</div>
      </div>
    </div>

    <!-- URLs and Comments -->
    <div class="form-section">
      <h2>URLs and Comments</h2>

      <div class="form-group">
        <label>URLs</label>
        <div class="array-input">
          <div v-for="(item, index) in formData.urls" :key="index" class="array-item">
            <input
              v-model="formData.urls[index]"
              type="url"
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
          v-model="formData.comments"
          class="form-control"
          :class="{ 'is-invalid': formErrors.comments }"
          rows="4"
        ></textarea>
        <div v-if="formErrors.comments" class="error-message">{{ formErrors.comments }}</div>
      </div>

      <div class="form-group">
        <label class="checkbox-label">
          <input v-model="formData.draft" type="checkbox">
          <span>Draft</span>
        </label>
      </div>
    </div>

    <div class="form-actions">
      <button type="button" class="btn btn-secondary" @click="onCancel">Cancel</button>
      <button type="submit" class="btn btn-primary" :disabled="isSubmitting">
        {{ isSubmitting ? submitButtonLoadingText : submitButtonText }}
      </button>
    </div>
  </form>
</template>

<script setup lang="ts">
import { ref, reactive, watch, nextTick, computed } from 'vue'
import { COMMODITY_TYPES } from '@/constants/commodityTypes'
import { COMMODITY_STATUSES, COMMODITY_STATUS_IN_USE } from '@/constants/commodityStatuses'
import { CURRENCY_CZK } from '@/constants/currencies'
import Select from 'primevue/select'

const props = defineProps({
  initialData: {
    type: Object,
    default: () => ({
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
      extraSerialNumbers: [],
      partNumbers: [],
      tags: [],
      status: COMMODITY_STATUS_IN_USE,
      purchaseDate: new Date().toISOString().split('T')[0],
      urls: [],
      comments: '',
      draft: false
    })
  },
  isSubmitting: {
    type: Boolean,
    default: false
  },
  submitButtonText: {
    type: String,
    default: 'Save'
  },
  submitButtonLoadingText: {
    type: String,
    default: 'Saving...'
  },
  areas: {
    type: Array,
    default: () => []
  },
  currencies: {
    type: Array,
    default: () => []
  },
  mainCurrency: {
    type: String,
    default: CURRENCY_CZK
  },
  areaFromUrl: {
    type: String,
    default: null
  }
})

const emit = defineEmits(['submit', 'cancel', 'validate', 'update:errors'])

const commodityTypes = ref(COMMODITY_TYPES)
const commodityStatuses = ref(COMMODITY_STATUSES)

const formData = reactive({ ...props.initialData })
const formErrors = reactive({
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

// Watch for changes in initialData
watch(() => props.initialData, (newValue) => {
  // Update formData with new values
  Object.assign(formData, newValue)
}, { deep: true })

// Watch for areaFromUrl changes
watch(() => props.areaFromUrl, (newValue) => {
  if (newValue) {
    formData.areaId = newValue
  }
}, { immediate: true })

// Method to set errors from outside (for backend validation errors)
const setErrors = (backendErrors) => {
  // Map backend errors to form errors
  if (backendErrors) {
    Object.keys(backendErrors).forEach(key => {
      // Convert snake_case to camelCase
      const camelKey = key.replace(/_([a-z])/g, (_, letter) => letter.toUpperCase())
      if (formErrors.hasOwnProperty(camelKey)) {
        formErrors[camelKey] = backendErrors[key]
      }
    })

    // If there are errors, scroll to the first one
    if (Object.values(backendErrors).some(e => e)) {
      scrollToFirstError()
    }
  }
}

// Function to scroll to the first error in the form
const scrollToFirstError = () => {
  // Use nextTick to ensure the DOM has updated with error messages
  nextTick(() => {
    // Find the first element with an error message
    const firstErrorElement = document.querySelector('.error-message')
    if (firstErrorElement) {
      // Find the parent form group to scroll to
      const formGroup = firstErrorElement.closest('.form-group')
      if (formGroup) {
        // Scroll the form group into view with some padding at the top
        formGroup.scrollIntoView({ behavior: 'smooth', block: 'center' })
      }
    }
  })
}

// Expose the setErrors method
defineExpose({ setErrors })

const validateForm = () => {
  let isValid = true

  // Reset errors
  Object.keys(formErrors).forEach(key => {
    formErrors[key] = ''
  })

  // Validation logic
  if (!formData.name.trim()) {
    formErrors.name = 'Name is required'
    isValid = false
  }

  if (!formData.shortName.trim()) {
    formErrors.shortName = 'Short Name is required'
    isValid = false
  }

  if (!formData.type) {
    formErrors.type = 'Type is required'
    isValid = false
  }

  if (!formData.areaId) {
    formErrors.areaId = 'Area is required'
    isValid = false
  }

  if (formData.count < 1) {
    formErrors.count = 'Count must be at least 1'
    isValid = false
  }

  if (formData.originalPrice < 0) {
    formErrors.originalPrice = 'Original Price cannot be negative'
    isValid = false
  }

  if (!formData.originalPriceCurrency) {
    formErrors.originalPriceCurrency = 'Original Price Currency is required'
    isValid = false
  }

  if (showConvertedOriginalPrice.value && formData.convertedOriginalPrice < 0) {
    formErrors.convertedOriginalPrice = 'Converted Original Price cannot be negative'
    isValid = false
  }

  if (formData.currentPrice < 0) {
    formErrors.currentPrice = 'Current Price cannot be negative'
    isValid = false
  }

  if (!formData.status) {
    formErrors.status = 'Status is required'
    isValid = false
  }

  if (!formData.purchaseDate) {
    formErrors.purchaseDate = 'Purchase Date is required'
    isValid = false
  }

  const today = new Date().toISOString().split('T')[0]
  if (formData.purchaseDate > today) {
    formErrors.purchaseDate = 'Purchase Date cannot be in the future'
    isValid = false
  }

  if (formData.comments && formData.comments.length > 1000) {
    formErrors.comments = 'Comments cannot exceed 1000 characters'
    isValid = false
  }

  emit('validate', isValid, formErrors)

  // If validation failed, scroll to the first error
  if (!isValid) {
    scrollToFirstError()
  }

  return isValid
}

const onSubmit = () => {
  console.log('CommodityForm: onSubmit called')
  if (!validateForm()) {
    console.log('CommodityForm: Form validation failed')
    return
  }
  console.log('CommodityForm: Form validation passed, emitting submit event with data:', formData)
  emit('submit', formData)
}

const onCancel = () => {
  emit('cancel')
}

// Helper methods for array fields
const addExtraSerialNumber = () => {
  formData.extraSerialNumbers.push('')
}

const removeExtraSerialNumber = (index: number) => {
  formData.extraSerialNumbers.splice(index, 1)
}

const addPartNumber = () => {
  formData.partNumbers.push('')
}

const removePartNumber = (index: number) => {
  formData.partNumbers.splice(index, 1)
}

const addTag = () => {
  formData.tags.push('')
}

const removeTag = (index: number) => {
  formData.tags.splice(index, 1)
}

const addUrl = () => {
  formData.urls.push('')
}

const removeUrl = (index: number) => {
  formData.urls.splice(index, 1)
}

// Computed properties for price hints
const isPriceUsedInCalculations = computed(() => {
  return !formData.draft && formData.status === COMMODITY_STATUS_IN_USE
})

const hasSuitablePrice = computed(() => {
  if (!isPriceUsedInCalculations.value) {
    return false
  }

  return formData.currentPrice > 0 ||
    (formData.originalPriceCurrency === props.mainCurrency && formData.originalPrice > 0) ||
    formData.convertedOriginalPrice > 0
})

// Determine if we should show the converted original price field
const showConvertedOriginalPrice = computed(() => {
  return formData.originalPriceCurrency !== props.mainCurrency
})

// Reset converted original price when original currency is changed to main currency
watch(() => formData.originalPriceCurrency, (newCurrency) => {
  if (newCurrency === props.mainCurrency) {
    formData.convertedOriginalPrice = 0
  }
})

// Note: We're using getPriceCalculationHint for displaying price calculation information in the template

const getPriceCalculationHint = computed(() => {
  // If the item is a draft or not in use, explain why it's excluded from calculations
  if (formData.draft) {
    return 'This item is a draft and will not be included in value calculations.'
  }

  if (formData.status !== COMMODITY_STATUS_IN_USE) {
    const statusName = getStatusName(formData.status)
    return `This item has status "${statusName}" and will not be included in value calculations.`
  }

  // Determine which price will be used in calculations
  if (formData.currentPrice > 0) {
    return `<strong>Current Price</strong> will be used in value calculations (in ${props.mainCurrency}).`
  }

  if (formData.originalPriceCurrency === props.mainCurrency && formData.originalPrice > 0) {
    return `<strong>Original Price</strong> will be used in value calculations (in ${props.mainCurrency}).`
  }

  if (showConvertedOriginalPrice.value && formData.convertedOriginalPrice > 0) {
    return `<strong>Converted Original Price</strong> will be used in value calculations (in ${props.mainCurrency}).`
  }

  const needsConvertedPrice = formData.originalPriceCurrency !== props.mainCurrency;
  return `No suitable price found for calculations. Please enter Current Price${needsConvertedPrice ? ', Converted Original Price' : ''}, or Original Price in ${props.mainCurrency}.`
})

const getStatusName = (statusId: string) => {
  const status = commodityStatuses.value.find(s => s.id === statusId)
  return status ? status.name : statusId
}
</script>

<style lang="scss" scoped>
@import '../assets/main.scss';

.price-calculation-hint {
  font-size: 0.9rem;
  margin: 0.5rem 0 1.5rem;
  padding: 0.75rem;
  border-radius: $default-radius;
  background-color: rgba($primary-color, 0.1);
  color: $text-color;
  font-style: italic;

  &.inactive-hint {
    color: $danger-color;
    background-color: rgba($danger-color, 0.1);
  }

  &.warning-hint {
    color: #856404; /* Warning text color - dark amber */
    background-color: #fff3cd; /* Light amber background */
  }
}

.form {
  background: white;
  padding: 2rem;
  border-radius: $default-radius;
  box-shadow: $box-shadow;
}

.location-group-label {
  font-weight: bold;
  padding: 0.5rem;
  background-color: $light-bg-color;
  border-bottom: 1px solid $border-color;
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

.w-100 {
  width: 100%;
}

textarea.form-control {
  resize: vertical;
}

.error-message {
  color: $danger-color;
  font-size: 0.875rem;
  margin-top: 0.25rem;
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

.btn-sm {
  padding: 0.5rem 1rem;
  font-size: 0.875rem;
}
</style>

<template>
  <form @submit.prevent="onSubmit" class="form">
    <!-- Basic Information -->
    <div class="form-section">
      <h2>Basic Information</h2>

      <div class="form-group">
        <label for="name">Name</label>
        <input
          type="text"
          id="name"
          v-model="formData.name"
          required
          class="form-control"
          :class="{ 'is-invalid': formErrors.name }"
        >
        <div v-if="formErrors.name" class="error-message">{{ formErrors.name }}</div>
      </div>

      <!-- Other fields from both forms -->
      <!-- ... -->

      <div class="form-group">
        <label class="checkbox-label">
          <input type="checkbox" v-model="formData.draft">
          Draft
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
import { ref, reactive, onMounted } from 'vue'
import { COMMODITY_TYPES } from '@/constants/commodityTypes'
import { COMMODITY_STATUSES } from '@/constants/commodityStatuses'
import { CURRENCIES } from '@/constants/currencies'

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
      originalPriceCurrency: '',
      convertedOriginalPrice: 0,
      currentPrice: 0,
      serialNumber: '',
      extraSerialNumbers: [],
      partNumbers: [],
      tags: [],
      status: '',
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
  }
})

const emit = defineEmits(['submit', 'cancel', 'validate'])

const commodityTypes = ref(COMMODITY_TYPES)
const commodityStatuses = ref(COMMODITY_STATUSES)
const currencies = ref(CURRENCIES)

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

const validateForm = () => {
  let isValid = true

  // Reset errors
  Object.keys(formErrors).forEach(key => {
    formErrors[key] = ''
  })

  // Validation logic from both forms
  if (!formData.name.trim()) {
    formErrors.name = 'Name is required'
    isValid = false
  }

  // Add other validations...

  emit('validate', isValid, formErrors)
  return isValid
}

const onSubmit = () => {
  if (!validateForm()) return
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

// Similar methods for other array fields...
</script>
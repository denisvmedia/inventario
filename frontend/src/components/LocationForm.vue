<template>
  <div class="inline-form location-form">
    <form @submit.prevent="submitForm">
      <div class="form-row">
        <div class="form-group">
          <label for="name">Name</label>
          <input
            type="text"
            id="name"
            v-model="form.name"
            required
            class="form-control"
            :class="{ 'is-invalid': errors.name }"
            placeholder="Enter location name"
          >
          <div v-if="errors.name" class="error-message">{{ errors.name }}</div>
        </div>
        
        <div class="form-group">
          <label for="address">Address</label>
          <input
            type="text"
            id="address"
            v-model="form.address"
            required
            class="form-control"
            :class="{ 'is-invalid': errors.address }"
            placeholder="Enter location address"
          >
          <div v-if="errors.address" class="error-message">{{ errors.address }}</div>
        </div>
      </div>

      <div class="form-actions">
        <button type="button" class="btn btn-secondary" @click="cancel">Cancel</button>
        <button type="submit" class="btn btn-primary" :disabled="isSubmitting">
          {{ isSubmitting ? 'Creating...' : 'Create Location' }}
        </button>
      </div>

      <div v-if="error" class="form-error">{{ error }}</div>
    </form>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive } from 'vue'
import locationService from '@/services/locationService'

const emit = defineEmits(['created', 'cancel'])

const isSubmitting = ref(false)
const error = ref<string | null>(null)

const form = reactive({
  name: '',
  address: ''
})

const errors = reactive({
  name: '',
  address: ''
})

const validateForm = () => {
  let isValid = true

  // Reset errors
  errors.name = ''
  errors.address = ''

  if (!form.name.trim()) {
    errors.name = 'Name is required'
    isValid = false
  }

  if (!form.address.trim()) {
    errors.address = 'Address is required'
    isValid = false
  }

  return isValid
}

const submitForm = async () => {
  if (!validateForm()) return

  isSubmitting.value = true
  error.value = null

  try {
    // Create the payload
    const payload = {
      data: {
        type: 'locations',
        attributes: {
          name: form.name.trim(),
          address: form.address.trim()
        }
      }
    }

    const response = await locationService.createLocation(payload)
    
    // Reset form
    form.name = ''
    form.address = ''
    
    // Emit created event with the new location
    emit('created', response.data.data)
  } catch (err: any) {
    if (err.response && err.response.data && err.response.data.errors) {
      const apiErrors = err.response.data.errors
      apiErrors.forEach((apiError: any) => {
        if (apiError.source && apiError.source.pointer) {
          const field = apiError.source.pointer.split('/').pop()
          if (field === 'name') {
            errors.name = apiError.detail
          } else if (field === 'address') {
            errors.address = apiError.detail
          }
        }
      })

      if (Object.values(errors).some(e => e)) {
        error.value = 'Please correct the errors above.'
      } else {
        error.value = `Failed to create location: ${err.response.status} - ${JSON.stringify(err.response.data)}`
      }
    } else {
      error.value = 'Failed to create location: ' + (err.message || 'Unknown error')
    }
  } finally {
    isSubmitting.value = false
  }
}

const cancel = () => {
  // Reset form
  form.name = ''
  form.address = ''
  
  // Emit cancel event
  emit('cancel')
}
</script>

<style scoped>
.inline-form {
  background: white;
  padding: 1.5rem;
  border-radius: 8px;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
  margin-bottom: 1.5rem;
}

.form-row {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 1rem;
}

.form-group {
  margin-bottom: 1rem;
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
  gap: 0.5rem;
  margin-top: 1rem;
}

.form-error {
  color: #dc3545;
  margin-top: 1rem;
  padding: 0.5rem;
  background-color: rgba(220, 53, 69, 0.1);
  border-radius: 4px;
}

@media (max-width: 768px) {
  .form-row {
    grid-template-columns: 1fr;
  }
}
</style>

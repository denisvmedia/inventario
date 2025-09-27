<template>
  <div class="inline-form location-form">
    <form @submit.prevent="submitForm">
      <div class="form-row">
        <div class="form-group">
          <label for="name">Name</label>
          <input
            id="name"
            v-model="form.name"
            type="text"
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
            id="address"
            v-model="form.address"
            type="text"
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
  } catch (err: unknown) {
    if (err && typeof err === 'object' && 'response' in err) {
      const errorResponse = err as { response?: { data?: { errors?: Array<{ source?: { pointer?: string }, detail?: string }> } } }
      if (errorResponse.response?.data?.errors) {
        const apiErrors = errorResponse.response.data.errors
        apiErrors.forEach((apiError) => {
          if (apiError.source && apiError.source.pointer) {
            const field = apiError.source.pointer.split('/').pop()
            if (field === 'name') {
              errors.name = apiError.detail || ''
            } else if (field === 'address') {
              errors.address = apiError.detail || ''
            }
          }
        })

        if (Object.values(errors).some(e => e)) {
          error.value = 'Please correct the errors above.'
        } else {
          error.value = `Failed to create location: ${errorResponse.response?.status || 'Unknown'} - ${JSON.stringify(errorResponse.response?.data || {})}`
        }
      } else {
        error.value = 'Failed to create location: Unknown server error'
      }
    } else {
      const errorMessage = err && typeof err === 'object' && 'message' in err ? (err as Error).message : 'Unknown error'
      error.value = 'Failed to create location: ' + errorMessage
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

<style lang="scss" scoped>
@use '@/assets/main.scss' as *;

.inline-form {
  background: white;
  padding: 1.5rem;
  border-radius: $default-radius;
  box-shadow: $box-shadow;
  margin-bottom: 1.5rem;
}

.form-row {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 1rem;

  @media (width <= 768px) {
    grid-template-columns: 1fr;
  }
}

.form-group {
  margin-bottom: 1rem;
}

.form-actions {
  gap: 0.5rem;
  margin-top: 1rem;
}

.form-error {
  color: $danger-color;
  margin-top: 1rem;
  padding: 0.5rem;
  background-color: rgba($danger-color, 0.1);
  border-radius: $default-radius;
}
</style>

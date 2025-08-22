<template>
  <div class="location-create">
    <h1>Create New Location</h1>

    <form class="form" @submit.prevent="submitForm">
      <div class="form-group">
        <label for="name">Name</label>
        <input
          id="name"
          v-model="form.name"
          type="text"
          required
          class="form-control"
          :class="{ 'is-invalid': errors.name }"
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
        >
        <div v-if="errors.address" class="error-message">{{ errors.address }}</div>
      </div>

      <div class="form-actions">
        <button type="button" class="btn btn-secondary" @click="cancel">Cancel</button>
        <button type="submit" class="btn btn-primary" :disabled="isSubmitting">
          {{ isSubmitting ? 'Creating...' : 'Create Location' }}
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
import { ref, reactive } from 'vue'
import { useRouter } from 'vue-router'
import locationService from '@/services/locationService'

const router = useRouter()
const isSubmitting = ref(false)
const error = ref<string | null>(null)
const debugInfo = ref<string | null>(null)

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
  debugInfo.value = null

  try {
    // Create the payload directly
    const payload = {
      data: {
        type: 'locations',
        attributes: {
          name: form.name.trim(),
          address: form.address.trim()
        }
      }
    }

    // Log what we're sending
    console.log('Submitting with payload:', JSON.stringify(payload, null, 2))
    debugInfo.value = `Sending: ${JSON.stringify(payload, null, 2)}`

    // Use the location service which includes authentication
    const response = await locationService.createLocation(payload)

    console.log('Success response:', response.data)
    debugInfo.value += `\n\nResponse: ${JSON.stringify(response.data, null, 2)}`

    // On success, navigate to locations list
    router.push('/locations')
  } catch (err: any) {
    console.error('Error creating location:', err)

    if (err.response) {
      console.error('Response status:', err.response.status)
      console.error('Response data:', err.response.data)

      debugInfo.value += `\n\nError Response: ${JSON.stringify(err.response.data, null, 2)}`

      // Extract validation errors if present
      const apiErrors = err.response.data.errors?.[0]?.error?.error?.data?.attributes || {}

      if (apiErrors.name) {
        errors.name = apiErrors.name
      }

      if (apiErrors.address) {
        errors.address = apiErrors.address
      }

      if (errors.name || errors.address) {
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
  router.push('/locations')
}
</script>

<style lang="scss" scoped>
@use '@/assets/main' as *;

.location-create {
  max-width: 600px;
  margin: 0 auto;
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

.form-error {
  margin-top: 1rem;
  padding: 0.75rem;
  background-color: #f8d7da;
  color: $error-text-color;
  border-radius: $default-radius;
}

.debug-info {
  margin-top: 2rem;
  padding: 1rem;
  background-color: $light-bg-color;
  border-radius: $default-radius;
  border: 1px solid $border-color;

  pre {
    white-space: pre-wrap;
    word-wrap: break-word;
    overflow-x: auto;
  }
}
</style>

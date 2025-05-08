<template>
  <div class="area-create">
    <h1>Create New Area</h1>

    <form @submit.prevent="submitForm" class="form">
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
        <label for="location">Location</label>
        <select
          id="location"
          v-model="form.locationId"
          required
          class="form-control"
          :class="{ 'is-invalid': errors.locationId }"
        >
          <option value="" disabled>Select a location</option>
          <option v-for="location in locations" :key="location.id" :value="location.id">
            {{ location.attributes.name }}
          </option>
        </select>
        <div v-if="errors.locationId" class="error-message">{{ errors.locationId }}</div>
      </div>

      <div class="form-actions">
        <button type="button" class="btn btn-secondary" @click="cancel">Cancel</button>
        <button type="submit" class="btn btn-primary" :disabled="isSubmitting">
          {{ isSubmitting ? 'Creating...' : 'Create Area' }}
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

const router = useRouter()
const route = useRoute()
const isSubmitting = ref(false)
const error = ref<string | null>(null)
const debugInfo = ref<string | null>(null)
const locations = ref<any[]>([])

const form = reactive({
  name: '',
  locationId: ''
})

const errors = reactive({
  name: '',
  locationId: ''
})

onMounted(async () => {
  try {
    // Fetch locations for the dropdown
    const response = await axios.get('/api/v1/locations', {
      headers: {
        'Accept': 'application/vnd.api+json'
      }
    })
    locations.value = response.data.data

    // Check if location ID was passed in the URL
    const locationId = route.query.location as string
    if (locationId) {
      form.locationId = locationId
    }
  } catch (err: any) {
    console.error('Failed to load locations:', err)
    error.value = 'Failed to load locations. Please refresh the page.'
  }
})

const validateForm = () => {
  let isValid = true

  // Reset errors
  errors.name = ''
  errors.locationId = ''

  if (!form.name.trim()) {
    errors.name = 'Name is required'
    isValid = false
  }

  if (!form.locationId) {
    errors.locationId = 'Location is required'
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
    // Create the payload with the exact structure expected by the server
    const payload = {
      data: {
        type: 'areas',
        attributes: {
          name: form.name.trim(),
          location_id: form.locationId
        }
      }
    }

    // Log what we're sending
    console.log('Submitting with payload:', JSON.stringify(payload, null, 2))
    debugInfo.value = `Sending: ${JSON.stringify(payload, null, 2)}`

    // Make a direct axios call with explicit content type
    const response = await axios({
      method: 'post',
      url: '/api/v1/areas',
      data: payload,
      headers: {
        'Content-Type': 'application/vnd.api+json',
        'Accept': 'application/vnd.api+json'
      }
    })

    console.log('Success response:', response.data)
    debugInfo.value += `\n\nResponse: ${JSON.stringify(response.data, null, 2)}`

    // On success, navigate to areas list
    router.push('/areas')
  } catch (err: any) {
    console.error('Error creating area:', err)

    if (err.response) {
      console.error('Response status:', err.response.status)
      console.error('Response data:', err.response.data)

      debugInfo.value += `\n\nError Response: ${JSON.stringify(err.response.data, null, 2)}`

      // Extract validation errors if present
      const apiErrors = err.response.data.errors?.[0]?.error?.error?.data?.attributes || {}

      if (apiErrors.name) {
        errors.name = apiErrors.name
      }

      if (apiErrors.location_id) {
        errors.locationId = apiErrors.location_id
      }

      if (errors.name || errors.locationId) {
        error.value = 'Please correct the errors above.'
      } else {
        error.value = `Failed to create area: ${err.response.status} - ${JSON.stringify(err.response.data)}`
      }
    } else {
      error.value = 'Failed to create area: ' + (err.message || 'Unknown error')
    }
  } finally {
    isSubmitting.value = false
  }
}

const cancel = () => {
  router.push('/areas')
}
</script>

<template>
  <div class="area-create">
    <div class="breadcrumb-nav">
      <a href="#" class="breadcrumb-link" @click.prevent="navigateToLocations">
        <font-awesome-icon icon="arrow-left" /> Back to Locations
      </a>
    </div>
    <h1>Create New Area</h1>

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
        <label for="location">Location</label>
        <Select
          id="location"
          v-model="form.locationId"
          :options="locations"
          option-label="attributes.name"
          option-value="id"
          placeholder="Select a location"
          class="w-100"
          :class="{ 'is-invalid': errors.locationId }"
        />
        <div v-if="errors.locationId" class="error-message">{{ errors.locationId }}</div>
      </div>

      <div class="form-actions">
        <button type="button" class="btn btn-secondary" @click="navigateToLocations">Cancel</button>
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
import Select from 'primevue/select'

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

    // Get the ID of the newly created area
    const newAreaId = response.data.data.id

    // On success, navigate to locations list with the new area focused
    router.push({
      path: '/locations',
      query: {
        areaId: newAreaId,
        locationId: form.locationId
      }
    })
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

const navigateToLocations = () => {
  // Navigate to locations list with location context if available
  if (form.locationId) {
    router.push({
      path: '/locations',
      query: {
        locationId: form.locationId
      }
    })
  } else {
    router.push('/locations')
  }
}
</script>

<style lang="scss" scoped>
@use '@/assets/main.scss' as *;

.area-create {
  max-width: 600px;
  margin: 0 auto;
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
</style>

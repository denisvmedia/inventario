<template>
  <div class="area-edit">
    <div class="header">
      <h1>Edit Area</h1>
      <button class="btn btn-secondary" @click="goBack">Go Back</button>
    </div>

    <div v-if="loading" class="loading">Loading...</div>
    <div v-else-if="error" class="error">{{ error }}</div>
    <div v-else-if="!area" class="not-found">Area not found</div>
    <div v-else class="form-container">
      <form @submit.prevent="submitForm" class="form">
        <div class="form-group">
          <label for="name">Name</label>
          <input
            type="text"
            id="name"
            v-model="form.name"
            class="form-control"
            :class="{ 'is-invalid': errors.name }"
            required
          />
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
          <button type="button" class="btn btn-secondary" @click="goBack">Cancel</button>
          <button type="submit" class="btn btn-primary" :disabled="isSubmitting">
            {{ isSubmitting ? 'Saving...' : 'Save Changes' }}
          </button>
        </div>

        <div v-if="formError" class="form-error">{{ formError }}</div>

        <!-- Debug information -->
        <div v-if="debugInfo" class="debug-info">
          <h3>Debug Information</h3>
          <pre>{{ debugInfo }}</pre>
        </div>
      </form>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import areaService from '@/services/areaService'
import locationService from '@/services/locationService'

const route = useRoute()
const router = useRouter()
const id = route.params.id as string

const area = ref<any>(null)
const locations = ref<any[]>([])
const loading = ref<boolean>(true)
const error = ref<string | null>(null)
const isSubmitting = ref<boolean>(false)
const formError = ref<string | null>(null)
const debugInfo = ref<string | null>(null)

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
    // Load area and locations in parallel
    const [areaResponse, locationsResponse] = await Promise.all([
      areaService.getArea(id),
      locationService.getLocations()
    ])

    area.value = areaResponse.data.data
    locations.value = locationsResponse.data.data

    // Initialize form with area data
    form.name = area.value.attributes.name

    // Set the location ID if available in the area data
    if (area.value.attributes.location_id) {
      form.locationId = area.value.attributes.location_id
    } else if (area.value.relationships &&
               area.value.relationships.location &&
               area.value.relationships.location.data) {
      form.locationId = area.value.relationships.location.data.id
    }

    loading.value = false
  } catch (err: any) {
    console.error('Error loading data:', err)
    error.value = 'Failed to load area: ' + (err.message || 'Unknown error')
    loading.value = false
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
  formError.value = null
  debugInfo.value = null

  try {
    const payload = {
      data: {
        id: id,
        type: 'areas',
        attributes: {
          name: form.name.trim(),
          location_id: form.locationId
        }
      }
    }

    // Log what we're sending
    console.log('Updating area with payload:', JSON.stringify(payload, null, 2))
    debugInfo.value = `Sending: ${JSON.stringify(payload, null, 2)}`

    await areaService.updateArea(id, payload)

    // On success, navigate back to area detail
    router.push(`/areas/${id}`)
  } catch (err: any) {
    console.error('Error updating area:', err)

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
        formError.value = 'Please correct the errors above.'
      } else {
        formError.value = `Failed to update area: ${err.response.status} - ${JSON.stringify(err.response.data)}`
      }
    } else {
      formError.value = 'Failed to update area: ' + (err.message || 'Unknown error')
    }
  } finally {
    isSubmitting.value = false
  }
}

const goBack = () => {
  router.go(-1)
}
</script>

<style scoped>
.area-edit {
  max-width: 800px;
  margin: 0 auto;
  padding: 20px;
}

.header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 2rem;
}

.loading, .error, .not-found {
  text-align: center;
  padding: 2rem;
  background: white;
  border-radius: 8px;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
}

.error {
  color: #dc3545;
}

.form-container {
  background: white;
  padding: 2rem;
  border-radius: 8px;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
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

.form-error {
  margin-top: 1rem;
  padding: 0.75rem;
  background-color: #f8d7da;
  color: #721c24;
  border-radius: 4px;
}

.debug-info {
  margin-top: 2rem;
  padding: 1rem;
  background-color: #f8f9fa;
  border-radius: 4px;
  overflow: auto;
}

.debug-info pre {
  white-space: pre-wrap;
  font-size: 0.875rem;
}

.btn {
  padding: 0.5rem 1rem;
  border-radius: 4px;
  font-weight: 500;
  cursor: pointer;
  border: none;
}

.btn-primary {
  background-color: #4CAF50;
  color: white;
}

.btn-primary:hover {
  background-color: #43a047;
}

.btn-secondary {
  background-color: #f8f9fa;
  color: #333;
  border: 1px solid #ddd;
}

.btn-secondary:hover {
  background-color: #e9ecef;
}

.btn:disabled {
  opacity: 0.65;
  cursor: not-allowed;
}
</style>
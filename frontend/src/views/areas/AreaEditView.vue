<template>
  <div class="area-edit">
    <div class="breadcrumb-nav">
      <a href="#" class="breadcrumb-link" @click.prevent="navigateToLocations">
        <font-awesome-icon icon="arrow-left" /> Back to Locations
      </a>
    </div>
    <div class="header">
      <h1>Edit Area</h1>
    </div>

    <div v-if="loading" class="loading">Loading...</div>
    <div v-else-if="error" class="error">{{ error }}</div>
    <div v-else-if="!area" class="not-found">Area not found</div>
    <div v-else>
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
import Select from 'primevue/select'

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

    // On success, navigate back to locations list with area focused
    router.push({
      path: '/locations',
      query: {
        areaId: id,
        locationId: form.locationId
      }
    })
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
  // Navigate to locations list with area and location context
  router.push({
    path: '/locations',
    query: {
      areaId: id,
      locationId: form.locationId
    }
  })
}

const navigateToLocations = () => {
  // Navigate to locations list with area and location context
  router.push({
    path: '/locations',
    query: {
      areaId: id,
      locationId: form.locationId
    }
  })
}

// Navigation is handled by the onSubmit and onCancel functions
</script>

<style lang="scss" scoped>
@import '../../assets/main.scss';

.area-edit {
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
  background-color: lighten($danger-color, 40%);
  color: $error-text-color;
  border-radius: $default-radius;
}

.debug-info {
  margin-top: 2rem;
  padding: 1rem;
  background-color: $light-bg-color;
  border-radius: $default-radius;
  overflow: auto;

  pre {
    white-space: pre-wrap;
    font-size: 0.875rem;
  }
}

.btn {
  padding: 0.5rem 1rem;
  border-radius: $default-radius;
  font-weight: 500;
  cursor: pointer;
  border: none;

  &:disabled {
    opacity: 0.65;
    cursor: not-allowed;
  }
}

.btn-primary {
  background-color: $primary-color;
  color: white;

  &:hover {
    background-color: $primary-hover-color;
  }
}

.btn-secondary {
  background-color: $light-bg-color;
  color: $text-color;
  border: 1px solid $border-color;

  &:hover {
    background-color: $light-hover-bg-color;
  }
}
</style>

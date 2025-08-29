<template>
  <div class="location-edit">
    <div v-if="loading" class="loading">Loading...</div>
    <ResourceNotFound
      v-else-if="is404Error"
      resource-type="location"
      :title="get404Title('location')"
      :message="get404Message('location')"
      go-back-text="Back to Locations"
      @go-back="goBackToList"
      @try-again="loadLocation"
    />
    <div v-else-if="error" class="error">{{ error }}</div>
    <div v-else-if="!location" class="not-found">Location not found</div>
    <div v-else>
      <div class="header">
        <h1>Edit Location</h1>
        <button class="btn btn-secondary" @click="goBack">Go Back</button>
      </div>
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
          <button type="button" class="btn btn-secondary" @click="goBack">Cancel</button>
          <button type="submit" class="btn btn-primary" :disabled="isSubmitting">
            {{ isSubmitting ? 'Saving...' : 'Save Changes' }}
          </button>
        </div>

        <div v-if="formError" class="form-error">{{ formError }}</div>
      </form>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import locationService from '@/services/locationService'
import ResourceNotFound from '@/components/ResourceNotFound.vue'
import { is404Error as checkIs404Error, get404Message, get404Title } from '@/utils/errorUtils'

const route = useRoute()
const router = useRouter()
const id = route.params.id as string

const location = ref<any>(null)
const loading = ref<boolean>(true)
const error = ref<string | null>(null)
const lastError = ref<any>(null) // Store the last error object for 404 detection
const isSubmitting = ref<boolean>(false)
const formError = ref<string | null>(null)

// Error state computed properties
const is404Error = computed(() => lastError.value && checkIs404Error(lastError.value))

const form = ref({
  name: '',
  address: ''
})

const errors = ref({
  name: '',
  address: ''
})

const loadLocation = async () => {
  loading.value = true
  error.value = null
  lastError.value = null

  try {
    const response = await locationService.getLocation(id)
    location.value = response.data.data

    // Initialize form with location data
    form.value.name = location.value.attributes.name
    form.value.address = location.value.attributes.address || ''

    loading.value = false
  } catch (err: any) {
    lastError.value = err
    loading.value = false
    if (checkIs404Error(err)) {
      // 404 errors will be handled by the ResourceNotFound component
    } else {
      error.value = 'Failed to load location: ' + (err.message || 'Unknown error')
    }
  }
}

onMounted(() => {
  loadLocation()
})

const validateForm = () => {
  let isValid = true
  errors.value.name = ''
  errors.value.address = ''

  if (!form.value.name.trim()) {
    errors.value.name = 'Name is required'
    isValid = false
  }

  if (!form.value.address.trim()) {
    errors.value.address = 'Address is required'
    isValid = false
  }

  return isValid
}

const submitForm = async () => {
  if (!validateForm()) return

  isSubmitting.value = true
  formError.value = null

  try {
    const payload = {
      data: {
        id: id,
        type: 'locations',
        attributes: {
          name: form.value.name.trim(),
          address: form.value.address.trim()
        }
      }
    }

    await locationService.updateLocation(id, payload)

    router.push(`/locations/${id}`)
  } catch (err: any) {
    console.error('Error updating location:', err)

    if (err.response) {
      console.error('Response status:', err.response.status)
      console.error('Response data:', err.response.data)

      // Extract validation errors if present
      const apiErrors = err.response.data.errors?.[0]?.error?.error?.data?.attributes || {}

      if (apiErrors.name) {
        errors.value.name = apiErrors.name
      }

      if (apiErrors.address) {
        errors.value.address = apiErrors.address
      }

      if (errors.value.name || errors.value.address) {
        formError.value = 'Please correct the errors above.'
      } else {
        formError.value = `Failed to update location: ${err.response.status} - ${JSON.stringify(err.response.data)}`
      }
    } else {
      formError.value = 'Failed to update location: ' + (err.message || 'Unknown error')
    }
  } finally {
    isSubmitting.value = false
  }
}

const goBack = () => {
  router.go(-1)
}

const goBackToList = () => {
  router.push('/locations')
}
</script>

<style lang="scss" scoped>
@use '@/assets/main' as *;

.location-edit {
  max-width: 600px;
  margin: 0 auto;
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

h1 {
  margin-bottom: 0;
}
</style>

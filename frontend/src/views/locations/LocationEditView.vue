<template>
  <div class="location-edit">
    <div class="header">
      <h1>Edit Location</h1>
      <button class="btn btn-secondary" @click="goBack">Go Back</button>
    </div>

    <div v-if="loading" class="loading">Loading...</div>
    <div v-else-if="error" class="error">{{ error }}</div>
    <div v-else-if="!location" class="not-found">Location not found</div>
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
          <label for="address">Address</label>
          <textarea
            id="address"
            v-model="form.address"
            class="form-control"
            :class="{ 'is-invalid': errors.address }"
            rows="3"
            required
          ></textarea>
          <div v-if="errors.address" class="error-message">{{ errors.address }}</div>
        </div>

        <div class="form-actions">
          <button type="button" class="btn btn-secondary" @click="goBack">Cancel</button>
          <button type="submit" class="btn btn-primary" :disabled="isSubmitting">Save Changes</button>
        </div>

        <div v-if="formError" class="form-error">{{ formError }}</div>
      </form>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import locationService from '@/services/locationService'

const route = useRoute()
const router = useRouter()
const id = route.params.id as string

const location = ref<any>(null)
const loading = ref<boolean>(true)
const error = ref<string | null>(null)
const isSubmitting = ref<boolean>(false)
const formError = ref<string | null>(null)

const form = ref({
  name: '',
  address: ''
})

const errors = ref({
  name: '',
  address: ''
})

onMounted(async () => {
  try {
    const response = await locationService.getLocation(id)
    location.value = response.data.data

    // Initialize form with location data
    form.value.name = location.value.attributes.name
    form.value.address = location.value.attributes.address || ''

    loading.value = false
  } catch (err: any) {
    error.value = 'Failed to load location: ' + (err.message || 'Unknown error')
    loading.value = false
  }
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
</script>

<style scoped>
.location-edit {
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

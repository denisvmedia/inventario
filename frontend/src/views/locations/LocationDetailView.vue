<template>
  <div class="location-detail">
    <div v-if="loading" class="loading">Loading...</div>
    <div v-else-if="error" class="error">{{ error }}</div>
    <div v-else-if="!location" class="not-found">Location not found</div>
    <div v-else>
      <div class="header">
        <h1>{{ location.attributes.name }}</h1>
        <div class="actions">
          <button class="btn btn-secondary" @click="editLocation">Edit</button>
          <button class="btn btn-danger" @click="confirmDelete">Delete</button>
          <button class="btn btn-primary" @click="testCreateLocation">Test API</button>
        </div>
      </div>

      <div class="location-info">
        <div class="info-card">
          <h2>Details</h2>
          <div class="info-row">
            <span class="label">Address:</span>
            <span>{{ location.attributes.address || 'N/A' }}</span>
          </div>
        </div>
      </div>

      <!-- Test API Results Section -->
      <div v-if="testResult || testError" class="test-section info-card">
        <h2>API Test Results</h2>
        <div v-if="testResult" class="test-result">
          <h3>Success:</h3>
          <pre>{{ testResult }}</pre>
        </div>
        <div v-if="testError" class="test-error">
          <h3>Error:</h3>
          <pre>{{ testError }}</pre>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import axios from 'axios'

const route = useRoute()
const router = useRouter()
const loading = ref<boolean>(true)
const error = ref<string | null>(null)
const location = ref<any>(null)

// Test API variables
const testResult = ref('')
const testError = ref('')

onMounted(async () => {
  const id = route.params.id as string

  try {
    // Uncomment when you have the service
    // const response = await locationService.getLocation(id)
    // location.value = response.data.data

    // Mock data for now
    location.value = {
      id,
      attributes: {
        name: 'Warehouse A',
        address: '123 Main St'
      }
    }
    loading.value = false
  } catch (err: any) {
    error.value = 'Failed to load location: ' + (err.message || 'Unknown error')
    loading.value = false
  }
})

const editLocation = () => {
  // Implement edit functionality
  alert('Edit functionality not implemented yet')
}

const confirmDelete = () => {
  if (confirm('Are you sure you want to delete this location?')) {
    deleteLocation()
  }
}

const deleteLocation = async () => {
  try {
    // Uncomment when you have the service
    // await locationService.deleteLocation(location.value.id)
    alert('Location deleted successfully')
    router.push('/locations')
  } catch (err: any) {
    error.value = 'Failed to delete location: ' + (err.message || 'Unknown error')
  }
}

// Add test function
const testCreateLocation = async () => {
  testResult.value = ''
  testError.value = ''

  try {
    // Create a test payload with a unique timestamp and ensure non-empty values
    const timestamp = new Date().toISOString()
    const payload = {
      data: {
        type: 'locations',
        attributes: {
          name: `Test Location ${timestamp}`,
          address: `Test Address ${timestamp}`
        }
      }
    }

    console.log('Sending payload:', JSON.stringify(payload, null, 2))

    // Make a direct axios call with detailed logging
    const response = await axios.post('/api/v1/locations', payload, {
      headers: {
        'Content-Type': 'application/vnd.api+json',
        'Accept': 'application/vnd.api+json'
      }
    })

    console.log('Response status:', response.status)
    console.log('Response headers:', response.headers)
    console.log('Response data:', response.data)

    testResult.value = JSON.stringify(response.data, null, 2)
  } catch (err: any) {
    console.error('Error details:', err)

    if (err.response) {
      console.error('Response status:', err.response.status)
      console.error('Response data:', err.response.data)

      // Extract validation errors if present
      const apiErrors = err.response.data.errors?.[0]?.error?.error?.data?.attributes || {}
      if (apiErrors.name || apiErrors.address) {
        testError.value = `Validation errors:\n- Name: ${apiErrors.name || 'none'}\n- Address: ${apiErrors.address || 'none'}`
      } else {
        testError.value = JSON.stringify(err.response.data, null, 2)
      }
    } else {
      testError.value = 'Error: ' + err.message
    }
  }
}
</script>

<style scoped>
.location-detail {
  max-width: 1200px;
  margin: 0 auto;
}

.header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 2rem;
}

.actions {
  display: flex;
  gap: 0.5rem;
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

.location-info, .test-section {
  margin-bottom: 2rem;
}

.info-card {
  background: white;
  border-radius: 8px;
  padding: 1.5rem;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
}

.info-card h2 {
  margin-bottom: 1rem;
  padding-bottom: 0.5rem;
  border-bottom: 1px solid #eee;
}

.info-row {
  display: flex;
  margin-bottom: 0.5rem;
}

.label {
  font-weight: bold;
  width: 100px;
}

.test-result, .test-error {
  margin-top: 1rem;
  padding: 1rem;
  border-radius: 4px;
}

.test-result {
  background-color: #e6f7e6;
}

.test-error {
  background-color: #f7e6e6;
}

pre {
  white-space: pre-wrap;
  word-wrap: break-word;
  overflow-x: auto;
  background: #f8f9fa;
  padding: 0.5rem;
  border-radius: 4px;
}

.btn-info {
  background-color: #17a2b8;
  color: white;
}

.btn-info:hover {
  background-color: #138496;
}
</style>

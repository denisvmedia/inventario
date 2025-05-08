<template>
  <div>
    <h1>Test Area API</h1>
    <button @click="testCreateArea" :disabled="isLoading">
      Test Create Area
    </button>
    <div v-if="result" class="result">
      <h3>Result:</h3>
      <pre>{{ result }}</pre>
    </div>
    <div v-if="error" class="error">
      <h3>Error:</h3>
      <pre>{{ error }}</pre>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import axios from 'axios'

const isLoading = ref(false)
const result = ref('')
const error = ref('')
const locations = ref<any[]>([])

onMounted(async () => {
  try {
    const response = await axios.get('/api/v1/locations', {
      headers: {
        'Accept': 'application/vnd.api+json'
      }
    })
    locations.value = response.data.data
    console.log('Available locations:', locations.value)
  } catch (err: any) {
    console.error('Failed to load locations:', err)
  }
})

const testCreateArea = async () => {
  if (locations.value.length === 0) {
    error.value = 'No locations available. Please create a location first.'
    return
  }

  isLoading.value = true
  result.value = ''
  error.value = ''

  try {
    // Get the first location ID
    const locationId = locations.value[0].id
    
    // Create a test payload
    const payload = {
      data: {
        type: 'areas',
        attributes: {
          name: 'Test Area ' + new Date().toISOString(),
          location_id: locationId
        }
      }
    }

    console.log('Sending direct test payload:', JSON.stringify(payload, null, 2))

    // Make a direct axios call to avoid any service layer issues
    const response = await axios.post('/api/v1/areas', payload, {
      headers: {
        'Content-Type': 'application/vnd.api+json',
        'Accept': 'application/vnd.api+json'
      }
    })

    result.value = JSON.stringify(response.data, null, 2)
    console.log('Direct test succeeded:', response.data)
  } catch (err: any) {
    error.value = JSON.stringify(err.response?.data || err.message, null, 2)
    console.error('Direct test failed:', err.response?.data || err.message)
  } finally {
    isLoading.value = false
  }
}
</script>

<style scoped>
h1 {
  margin-bottom: 1rem;
}

button {
  padding: 0.5rem 1rem;
  background-color: #4CAF50;
  color: white;
  border: none;
  border-radius: 4px;
  cursor: pointer;
  margin-bottom: 1rem;
}

button:disabled {
  background-color: #cccccc;
  cursor: not-allowed;
}

.result, .error {
  margin-top: 1rem;
  padding: 1rem;
  border-radius: 4px;
}

.result {
  background-color: #e8f5e9;
  border: 1px solid #4CAF50;
}

.error {
  background-color: #ffebee;
  border: 1px solid #f44336;
}

pre {
  white-space: pre-wrap;
  word-wrap: break-word;
  overflow-x: auto;
}
</style>
<template>
  <div class="test-view">
    <h1>Test Location API</h1>

    <div class="test-card">
      <h2>Create Location Test</h2>
      <button @click="testCreateLocation" :disabled="isLoading" class="btn btn-primary">
        {{ isLoading ? 'Testing...' : 'Test Create Location' }}
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
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import axios from 'axios'

const isLoading = ref(false)
const result = ref('')
const error = ref('')

const testCreateLocation = async () => {
  isLoading.value = true
  result.value = ''
  error.value = ''

  try {
    // Create a test payload
    const payload = {
      data: {
        type: 'locations',
        attributes: {
          name: 'Test Location ' + new Date().toISOString(),
          address: 'Test Address ' + new Date().toISOString()
        }
      }
    }

    console.log('Sending direct test payload:', JSON.stringify(payload, null, 2))

    // Make a direct axios call to avoid any service layer issues
    const response = await axios.post('/api/v1/locations', payload, {
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
.test-view {
  max-width: 800px;
  margin: 0 auto;
  padding: 1rem;
}

.test-card {
  background: white;
  border-radius: 8px;
  padding: 1.5rem;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
  margin-top: 2rem;
}

.test-card h2 {
  margin-bottom: 1rem;
  padding-bottom: 0.5rem;
  border-bottom: 1px solid #eee;
}

.result, .error {
  margin-top: 1.5rem;
  padding: 1rem;
  border-radius: 4px;
}

.result {
  background-color: #e6f7e6;
}

.error {
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

.btn {
  display: inline-block;
  padding: 0.5rem 1rem;
  border-radius: 4px;
  font-weight: 500;
  text-align: center;
  transition: background-color 0.2s, color 0.2s;
  border: none;
  cursor: pointer;
}

.btn-primary {
  background-color: #4CAF50;
  color: white;
}

.btn-primary:hover:not(:disabled) {
  background-color: #43a047;
}

.btn:disabled {
  opacity: 0.7;
  cursor: not-allowed;
}
</style>

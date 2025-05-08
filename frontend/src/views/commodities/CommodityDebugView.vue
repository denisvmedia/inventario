<template>
  <div class="debug-container">
    <h1>Commodity API Debug</h1>
    
    <div class="debug-section">
      <h2>API Endpoints</h2>
      
      <div class="endpoint-test">
        <h3>GET /areas</h3>
        <button @click="testGetAreas" :disabled="isLoading">Test Get Areas</button>
        <div v-if="areasResult" class="result">
          <pre>{{ areasResult }}</pre>
        </div>
        <div v-if="areasError" class="error">
          <pre>{{ areasError }}</pre>
        </div>
      </div>
      
      <div class="endpoint-test">
        <h3>GET /commodities</h3>
        <button @click="testGetCommodities" :disabled="isLoading">Test Get Commodities</button>
        <div v-if="commoditiesResult" class="result">
          <pre>{{ commoditiesResult }}</pre>
        </div>
        <div v-if="commoditiesError" class="error">
          <pre>{{ commoditiesError }}</pre>
        </div>
      </div>
      
      <div class="endpoint-test">
        <h3>POST /commodities</h3>
        <button @click="testCreateCommodity" :disabled="isLoading">Test Create Commodity</button>
        <div v-if="createResult" class="result">
          <pre>{{ createResult }}</pre>
        </div>
        <div v-if="createError" class="error">
          <pre>{{ createError }}</pre>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import axios from 'axios'
import { COMMODITY_TYPE_ELECTRONICS } from '@/constants/commodityTypes'
import { COMMODITY_STATUS_IN_USE } from '@/constants/commodityStatuses'
import { CURRENCY_CZK } from '@/constants/currencies'

const isLoading = ref(false)
const areasResult = ref('')
const areasError = ref('')
const commoditiesResult = ref('')
const commoditiesError = ref('')
const createResult = ref('')
const createError = ref('')

const testGetAreas = async () => {
  isLoading.value = true
  areasResult.value = ''
  areasError.value = ''
  
  try {
    // Try both API paths to see which one works
    try {
      const response = await axios.get('/api/areas', {
        headers: {
          'Accept': 'application/vnd.api+json'
        }
      })
      areasResult.value = JSON.stringify(response.data, null, 2)
      console.log('Areas API response:', response.data)
    } catch (err: any) {
      console.log('First attempt failed, trying v1 path')
      const response = await axios.get('/api/v1/areas', {
        headers: {
          'Accept': 'application/vnd.api+json'
        }
      })
      areasResult.value = JSON.stringify(response.data, null, 2)
      console.log('Areas API response:', response.data)
    }
  } catch (err: any) {
    console.error('Error fetching areas:', err)
    areasError.value = err.response ? 
      `Status: ${err.response.status}\nData: ${JSON.stringify(err.response.data, null, 2)}` : 
      `Error: ${err.message}`
  } finally {
    isLoading.value = false
  }
}

const testGetCommodities = async () => {
  isLoading.value = true
  commoditiesResult.value = ''
  commoditiesError.value = ''
  
  try {
    // Try both API paths to see which one works
    try {
      const response = await axios.get('/api/commodities', {
        headers: {
          'Accept': 'application/vnd.api+json'
        }
      })
      commoditiesResult.value = JSON.stringify(response.data, null, 2)
      console.log('Commodities API response:', response.data)
    } catch (err: any) {
      console.log('First attempt failed, trying v1 path')
      const response = await axios.get('/api/v1/commodities', {
        headers: {
          'Accept': 'application/vnd.api+json'
        }
      })
      commoditiesResult.value = JSON.stringify(response.data, null, 2)
      console.log('Commodities API response:', response.data)
    }
  } catch (err: any) {
    console.error('Error fetching commodities:', err)
    commoditiesError.value = err.response ? 
      `Status: ${err.response.status}\nData: ${JSON.stringify(err.response.data, null, 2)}` : 
      `Error: ${err.message}`
  } finally {
    isLoading.value = false
  }
}

const testCreateCommodity = async () => {
  isLoading.value = true
  createResult.value = ''
  createError.value = ''
  
  try {
    // First get an area ID
    let areaId = ''
    try {
      const areasResponse = await axios.get('/api/v1/areas', {
        headers: {
          'Accept': 'application/vnd.api+json'
        }
      })
      if (areasResponse.data.data && areasResponse.data.data.length > 0) {
        areaId = areasResponse.data.data[0].id
      } else {
        createError.value = 'No areas found. Please create an area first.'
        isLoading.value = false
        return
      }
    } catch (err: any) {
      createError.value = 'Failed to fetch areas. Please create an area first.'
      isLoading.value = false
      return
    }
    
    // Create a test commodity
    const today = new Date().toISOString().split('T')[0]
    const testName = 'Test Commodity ' + new Date().toISOString().substring(0, 19)
    
    const payload = {
      data: {
        type: 'commodities',
        attributes: {
          name: testName,
          short_name: 'Test',
          type: COMMODITY_TYPE_ELECTRONICS,
          area_id: areaId,
          count: 1,
          original_price: 100,
          original_price_currency: CURRENCY_CZK,
          converted_original_price: 100,
          current_price: 100,
          status: COMMODITY_STATUS_IN_USE,
          purchase_date: today
        }
      }
    }
    
    console.log('Sending test commodity payload:', JSON.stringify(payload, null, 2))
    
    // Try both API paths
    try {
      const response = await axios.post('/api/commodities', payload, {
        headers: {
          'Content-Type': 'application/vnd.api+json',
          'Accept': 'application/vnd.api+json'
        }
      })
      createResult.value = JSON.stringify(response.data, null, 2)
      console.log('Create commodity response:', response.data)
    } catch (err
<template>
  <div class="home">
    <h1>Welcome to Inventario</h1>
    <p>A modern inventory management system</p>

    <!-- Total Value Summary -->
    <div v-if="!settingsLoading && mainCurrency" class="value-summary">
      <div class="summary-card">
        <h2>Total Inventory Value</h2>
        <div v-if="valuesLoading" class="value-loading">
          <font-awesome-icon icon="spinner" spin /> Loading...
        </div>
        <div v-else-if="globalTotal" class="value-amount">
          {{ formatPrice(globalTotal, mainCurrency) }}
        </div>
        <div v-else class="value-empty">
          No valued items in inventory
        </div>
      </div>
    </div>

    <!-- Navigation Cards -->
    <div class="navigation-cards">
      <div class="card" @click="navigateTo('/locations')">
        <h2>Locations</h2>
        <p>Manage storage locations and areas</p>
      </div>
      <div class="card" @click="navigateTo('/commodities')">
        <h2>Commodities</h2>
        <p>Manage inventory items</p>
      </div>
      <div class="card" @click="navigateTo('/files')">
        <h2>Files</h2>
        <p>Upload and manage standalone files</p>
      </div>
      <div class="card" @click="navigateTo('/settings')">
        <h2>Settings</h2>
        <p>Configure application settings</p>
      </div>
    </div>

    <!-- Location Values -->
    <div v-if="locationTotals.length > 0" class="location-values">
      <h2>Value by Location</h2>
      <div class="values-grid">
        <div v-for="location in locationTotals" :key="location.id" class="value-item">
          <div class="value-item-name">{{ location.name }}</div>
          <div class="value-item-amount">{{ formatPrice(location.value, mainCurrency) }}</div>
        </div>
      </div>
    </div>

    <!-- Area Values -->
    <div v-if="areaTotals.length > 0" class="area-values">
      <h2>Value by Area</h2>
      <div class="values-grid">
        <div v-for="area in areaTotals" :key="area.id" class="value-item">
          <div class="value-item-name">{{ area.name }}</div>
          <div class="value-item-amount">{{ formatPrice(area.value, mainCurrency) }}</div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useRouter } from 'vue-router'
import { useSettingsStore } from '@/stores/settingsStore'
import valueService from '@/services/valueService'
import { formatPrice } from '@/services/currencyService'

const router = useRouter()
const settingsStore = useSettingsStore()

// Values data
const globalTotal = ref<number>(0)
const locationTotals = ref<any[]>([])
const areaTotals = ref<any[]>([])
const valuesLoading = ref<boolean>(true)
const valuesError = ref<string | null>(null)

// Settings
const settingsLoading = computed(() => settingsStore.isLoading)
const mainCurrency = computed(() => settingsStore.mainCurrency)

function navigateTo(path: string) {
  router.push(path)
}

async function loadValues() {
  valuesLoading.value = true
  valuesError.value = null

  try {
    const response = await valueService.getValues()
    const data = response.data.data.attributes

    // Parse the decimal string to a number
    globalTotal.value = parseFloat(data.global_total)
    locationTotals.value = data.location_totals || []
    areaTotals.value = data.area_totals || []
  } catch (error) {
    console.error('Error loading values:', error)
    valuesError.value = 'Failed to load inventory values'
  } finally {
    valuesLoading.value = false
  }
}

onMounted(async () => {
  // Make sure we have the main currency
  if (!mainCurrency.value) {
    await settingsStore.fetchMainCurrency()
  }

  // Load values
  await loadValues()
})
</script>

<style lang="scss" scoped>
@use '@/assets/main.scss' as *;

.home {
  max-width: $container-max-width;
  margin: 0 auto;
  padding: 20px;
  text-align: center;
}

h1 {
  margin-bottom: 10px;
  color: $secondary-color;
}

h2 {
  color: $secondary-color;
  margin-bottom: 10px;
}

.value-summary {
  margin: 30px auto;
  max-width: 500px;
}

.summary-card {
  background-color: white;
  border-radius: $default-radius;
  box-shadow: $box-shadow;
  padding: 20px;
  margin-bottom: 30px;

  .value-amount {
    font-size: 2rem;
    font-weight: bold;
    color: $primary-color;
    margin: 15px 0;
  }

  .value-loading, .value-empty {
    font-size: 1.2rem;
    color: $text-secondary-color;
    margin: 15px 0;
  }
}

.navigation-cards {
  display: flex;
  justify-content: center;
  gap: 20px;
  margin: 40px auto;
  flex-wrap: wrap;
}

.card {
  background-color: white;
  border-radius: $default-radius;
  box-shadow: $box-shadow;
  padding: 20px;
  width: 300px;
  cursor: pointer;
  transition: transform 0.3s, box-shadow 0.3s;

  &:hover {
    transform: translateY(-5px);
    box-shadow: 0 4px 12px rgb(0 0 0 / 15%);
  }

  h2 {
    color: $secondary-color;
    margin-bottom: 10px;
  }

  p {
    color: $text-color;
  }
}

.location-values, .area-values {
  margin: 40px auto;
  max-width: 800px;

  h2 {
    margin-bottom: 20px;
  }
}

.values-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(250px, 1fr));
  gap: 15px;
}

.value-item {
  background-color: white;
  border-radius: $default-radius;
  box-shadow: $box-shadow;
  padding: 15px;
  display: flex;
  flex-direction: column;

  .value-item-name {
    font-weight: bold;
    margin-bottom: 8px;
    color: $secondary-color;
  }

  .value-item-amount {
    font-size: 1.2rem;
    color: $primary-color;
  }
}
</style>

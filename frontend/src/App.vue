<template>
  <div class="app">
    <!-- Global Toast component -->
    <Toast />

    <!-- Global confirmation dialog component -->
    <header v-if="!isPrintRoute">
      <div class="header-content">
        <div class="logo-container">
          <router-link to="/">
            <img src="/favicon.png" alt="Inventario Logo" class="logo" />
          </router-link>
        </div>
        <nav>
          <router-link to="/" :class="{ 'custom-active': isHomeActive }">Home</router-link> |
          <router-link to="/locations" :class="{ 'custom-active': isLocationsActive }">Locations</router-link> |
          <router-link to="/commodities" :class="{ 'custom-active': isCommoditiesActive }">Commodities</router-link> |
          <router-link to="/files" :class="{ 'custom-active': isFilesActive }">Files</router-link> |
          <router-link to="/exports" :class="{ 'custom-active': isExportsActive }">Exports</router-link> |
          <router-link to="/settings" :class="{ 'custom-active': isSettingsActive }">Settings</router-link>
        </nav>
      </div>
    </header>

    <!-- Debug information -->
    <div v-if="false" class="debug-info">
      <p>Current route: {{ $route.path }}</p>
    </div>

    <main :class="{ 'container': !isPrintRoute, 'print-container': isPrintRoute }">
      <router-view />
    </main>

    <footer v-if="!isPrintRoute">
      <p>Inventario &copy; {{ new Date().getFullYear() }}</p>
    </footer>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { useSettingsStore } from '@/stores/settingsStore'
import Toast from 'primevue/toast'

const route = useRoute()
const settingsStore = useSettingsStore()

// Check if current route is a print route
const isPrintRoute = computed(() => {
  return route.path.includes('/print')
})

// Computed properties to determine active navigation sections
const isHomeActive = computed(() => {
  return route.path === '/'
})

const isLocationsActive = computed(() => {
  return route.path.startsWith('/locations') || route.path.startsWith('/areas')
})

const isCommoditiesActive = computed(() => {
  return route.path.startsWith('/commodities')
})

const isFilesActive = computed(() => {
  return route.path.startsWith('/files')
})

const isExportsActive = computed(() => {
  return route.path.startsWith('/exports')
})

const isSettingsActive = computed(() => {
  return route.path.startsWith('/settings')
})

// Initialize global settings when the app starts
onMounted(async () => {
  // Fetch main currency
  await settingsStore.fetchMainCurrency()
})
</script>

<style lang="scss">
// @use './assets/main.scss' as *;

.print-container {
  max-width: 100%;
  margin: 0;
  padding: 0;
}

.header-content {
  display: flex;
  align-items: center;
  max-width: 1200px;
  margin: 0 auto;
  padding: 0 1rem;
  justify-content: center;
}

.logo-container {
  margin-right: 2rem;
}

.logo {
  height: 40px;
  width: auto;
  vertical-align: middle;
  transition: transform 0.2s ease;

  &:hover {
    transform: scale(1.05);
  }
}

@media (width <= 768px) {
  .header-content {
    flex-direction: column;
    align-items: center;
  }

  .logo-container {
    margin-right: 0;
    margin-bottom: 1rem;
  }

  .logo {
    height: 35px;
  }
}

@media print {
  .app {
    padding: 0;
    margin: 0;
  }
}
</style>

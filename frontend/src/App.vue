<template>
  <div class="app">
    <header v-if="!isPrintRoute">
      <div class="header-content">
        <div class="logo-container">
          <router-link to="/">
            <img src="/favicon.png" alt="Inventario Logo" class="logo" />
          </router-link>
        </div>
        <nav>
          <router-link to="/">Home</router-link> |
          <router-link to="/locations">Locations</router-link> |
          <router-link to="/commodities">Commodities</router-link> |
          <router-link to="/settings">Settings</router-link>
        </nav>
      </div>
    </header>

    <!-- Debug information -->
    <div class="debug-info" v-if="!isPrintRoute && showDebugInfo">
      <p>Current route: {{ $route.path }}</p>
    </div>

    <main :class="{ 'container': !isPrintRoute, 'print-container': isPrintRoute }">
      <!-- Settings required notification -->
      <div class="notification-container" v-if="!isPrintRoute">
        <NotificationBanner
          v-if="showSettingsRequiredNotification"
          type="warning"
          :dismissible="false"
        >
          <strong>Settings Required:</strong> Please configure your system settings before using the application.
        </NotificationBanner>
      </div>

      <router-view />
    </main>

    <footer v-if="!isPrintRoute">
      <p>Inventario &copy; {{ new Date().getFullYear() }}</p>
    </footer>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import NotificationBanner from './components/NotificationBanner.vue'
import settingsCheckService from './services/settingsCheckService'

const route = useRoute()

// State
const showDebugInfo = ref(false)
const showSettingsRequiredNotification = ref(false)

// Check if current route is a print route
const isPrintRoute = computed(() => {
  return route.path.includes('/print')
})

// Check if we're on the settings page
const isSettingsPage = computed(() => {
  return route.path.startsWith('/settings')
})

// Watch for route changes to update notification state
watch(() => route.query, (query) => {
  if (query.required === 'true' && isSettingsPage.value) {
    showSettingsRequiredNotification.value = true
  } else {
    showSettingsRequiredNotification.value = false
  }
}, { immediate: true })

// Check settings on mount
onMounted(async () => {
  try {
    // Load debug info setting
    const response = await settingsCheckService.hasSettings()
    if (!response) {
      // If we're not already on the settings page with the required query param,
      // we'll let the router guard handle the redirect
      if (isSettingsPage.value) {
        showSettingsRequiredNotification.value = true
      }
    }
  } catch (error) {
    console.error('Error checking settings:', error)
  }
})
</script>

<style lang="scss">
.print-container {
  max-width: 100%;
  margin: 0;
  padding: 0;
}

.notification-container {
  margin-bottom: 1rem;
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

@media (max-width: 768px) {
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

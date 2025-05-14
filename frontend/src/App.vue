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
    <div class="debug-info" v-if="false">
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

const route = useRoute()
const settingsStore = useSettingsStore()

// Check if current route is a print route
const isPrintRoute = computed(() => {
  return route.path.includes('/print')
})

// Initialize global settings when the app starts
onMounted(async () => {
  // Fetch main currency
  await settingsStore.fetchMainCurrency()
})
</script>

<style lang="scss">
@import './assets/main.scss';

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

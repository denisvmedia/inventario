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
import { computed } from 'vue'
import { useRoute } from 'vue-router'

const route = useRoute()

// Check if current route is a print route
const isPrintRoute = computed(() => {
  return route.path.includes('/print')
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

/* Global PrimeVue Dropdown Styling - Simplified */
.p-dropdown {
  border: 1px solid $border-color !important;
  border-radius: $default-radius !important;
  background-color: white !important;
  transition: border-color 0.2s, box-shadow 0.2s !important;

  &:hover {
    border-color: darken($border-color, 10%) !important;
  }

  &:focus, &.p-focus {
    border-color: $primary-color !important;
    box-shadow: 0 0 0 2px rgba($primary-color, 0.2) !important;
    outline: none !important;
  }

  .p-dropdown-label {
    padding: 0.75rem !important;
    font-size: 1rem !important;
    color: $text-color !important;
  }

  .p-dropdown-trigger {
    width: 3rem !important;
    color: $text-secondary-color !important;
  }
}

/* Dropdown Panel */
.p-dropdown-panel {
  border: 1px solid $border-color !important;
  border-radius: $default-radius !important;
  background-color: white !important;
  box-shadow: $box-shadow !important;
  margin-top: 0.25rem !important;
  z-index: 1000 !important;
}

/* Dropdown Items */
.p-dropdown-items,
.p-dropdown-items-wrapper,
.p-dropdown-items-list {
  background-color: white !important;
  padding: 0.5rem 0 !important;
}

/* Dropdown Item */
.p-dropdown-item,
.p-select-option {
  background-color: white !important;
  color: $text-color !important;
  padding: 0.75rem 1rem !important;
  font-size: 1rem !important;
  transition: background-color 0.2s, color 0.2s !important;

  &:hover {
    background-color: $light-bg-color !important;
    color: $primary-color !important;
  }

  &.p-highlight {
    background-color: rgba($primary-color, 0.1) !important;
    color: $primary-color !important;
    font-weight: 500 !important;
  }
}

/* Filter Container */
.p-dropdown-filter-container {
  padding: 0.5rem !important;
  background-color: white !important;
  border-bottom: 1px solid $border-color !important;

  .p-dropdown-filter {
    padding: 0.5rem 0.75rem !important;
    font-size: 1rem !important;
    width: 100% !important;
    border: 1px solid $border-color !important;
    border-radius: $default-radius !important;

    &:focus {
      outline: none !important;
      border-color: $primary-color !important;
      box-shadow: 0 0 0 2px rgba($primary-color, 0.1) !important;
    }
  }

  .p-dropdown-filter-icon {
    color: $text-secondary-color !important;
    right: 1rem !important;
  }
}

/* Currency dropdown specific styles */
.currency-dropdown {
  .p-dropdown-item {
    background-color: white !important;
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

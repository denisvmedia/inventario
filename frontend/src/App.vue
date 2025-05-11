<template>
  <div class="app">
    <header v-if="!isPrintRoute">
      <nav>
        <router-link to="/">Home</router-link> |
        <router-link to="/locations">Locations</router-link> |
        <router-link to="/commodities">Commodities</router-link> |
      </nav>
    </header>

    <!-- Debug information -->
    <div class="debug-info" v-if="!isPrintRoute">
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

<style>
.print-container {
  max-width: 100%;
  margin: 0;
  padding: 0;
}

@media print {
  .app {
    padding: 0;
    margin: 0;
  }
}
</style>

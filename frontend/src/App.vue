<template>
  <div class="app">
    <header>
      <nav>
        <router-link to="/">Home</router-link> |
        <router-link to="/areas">Areas</router-link> |
        <router-link to="/locations">Locations</router-link> |
        <router-link to="/commodities">Commodities</router-link> |
      </nav>
    </header>

    <!-- Debug information -->
    <div class="debug-info">
      <p>Current route: {{ $route.path }}</p>
    </div>

    <main class="container">
      <router-view />
    </main>

    <footer>
      <p>Inventario &copy; {{ new Date().getFullYear() }}</p>
    </footer>
  </div>
</template>

<script setup lang="ts">
// No TypeScript logic needed for this component
</script>

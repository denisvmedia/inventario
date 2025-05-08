<template>
  <div class="app">
    <header>
      <nav>
        <router-link to="/">Home</router-link> |
        <router-link to="/areas">Areas</router-link> |
        <router-link to="/locations">Locations</router-link> |
        <router-link to="/commodities">Commodities</router-link> |
        <router-link to="/locations/test">Test API</router-link>
      </nav>
    </header>

    <!-- Debug information -->
    <div style="background-color: #ffffcc; padding: 10px; margin: 10px 0; border: 1px solid #e6e600;">
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

<style>
.app {
  display: flex;
  flex-direction: column;
  min-height: 100vh;
}

header {
  background-color: #2c3e50;
  color: white;
  padding: 1rem;
}

nav {
  display: flex;
  justify-content: center;
  gap: 1rem;
}

nav a {
  color: white;
  text-decoration: none;
}

nav a.router-link-active {
  font-weight: bold;
  text-decoration: underline;
}

main {
  flex: 1;
  padding: 2rem 0;
}

footer {
  background-color: #2c3e50;
  color: white;
  padding: 1rem;
  text-align: center;
}
</style>

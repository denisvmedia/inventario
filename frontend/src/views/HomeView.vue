<template>
  <div class="home">
    <h1>Welcome to Inventario</h1>
    <p>A modern inventory management system</p>
    <div class="navigation-cards">
      <div class="card" @click="navigateTo('/areas')">
        <h2>Areas</h2>
        <p>Manage storage areas</p>
      </div>
      <div class="card" @click="navigateTo('/locations')">
        <h2>Locations</h2>
        <p>Manage storage locations</p>
      </div>
      <div class="card" @click="navigateTo('/commodities')">
        <h2>Commodities</h2>
        <p>Manage inventory items</p>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useRouter } from 'vue-router'

const router = useRouter()

function navigateTo(path: string) {
  router.push(path)
}
</script>

<style scoped>
.home {
  max-width: 1200px;
  margin: 0 auto;
  padding: 20px;
  text-align: center;
}

h1 {
  margin-bottom: 10px;
  color: #2c3e50;
}

.navigation-cards {
  display: flex;
  justify-content: center;
  gap: 20px;
  margin-top: 40px;
  flex-wrap: wrap;
}

.card {
  background-color: white;
  border-radius: 8px;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
  padding: 20px;
  width: 300px;
  cursor: pointer;
  transition: transform 0.3s, box-shadow 0.3s;
}

.card:hover {
  transform: translateY(-5px);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
}

.card h2 {
  color: #2c3e50;
  margin-bottom: 10px;
}

.card p {
  color: #666;
}
</style>

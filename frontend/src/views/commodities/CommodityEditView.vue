<template>
  <div class="commodity-edit">
    <h1>Edit Commodity</h1>
    <p>Editing commodity with ID: {{ id || commodityId }}</p>
    <div class="debug-info">
      <h3>Debug Info</h3>
      <p>Route params: {{ JSON.stringify($route.params) }}</p>
      <p>Route path: {{ $route.path }}</p>
      <p>Route name: {{ $route.name }}</p>
    </div>
    <button @click="goBack">Go Back</button>
  </div>
</template>

<script setup lang="ts">
import { useRoute, useRouter } from 'vue-router'

const route = useRoute()
const router = useRouter()
const id = route.params.id
const commodityId = route.params.commodityId

console.log('CommodityEditView loaded')
console.log('Route params:', route.params)
console.log('id:', id)
console.log('commodityId:', commodityId)

function goBack() {
  router.go(-1)
}
</script>

<style scoped>
.commodity-edit {
  max-width: 800px;
  margin: 0 auto;
  padding: 20px;
  background-color: white;
  border-radius: 8px;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
}

.debug-info {
  background-color: #f8f9fa;
  padding: 15px;
  border-radius: 4px;
  margin: 20px 0;
}

button {
  background-color: #007bff;
  color: white;
  padding: 8px 16px;
  border: none;
  border-radius: 4px;
  cursor: pointer;
  margin-top: 20px;
}

button:hover {
  background-color: #0056b3;
}
</style>

<template>
  <div class="area-edit">
    <h1>Edit Area</h1>
    <p>Editing area with ID: {{ id }}</p>
    <button @click="goBack">Go Back</button>
  </div>
</template>

<script setup lang="ts">
import { useRoute, useRouter } from 'vue-router'

const route = useRoute()
const router = useRouter()
const id = route.params.id

console.log('AreaEditView loaded with ID:', id)

function goBack() {
  router.go(-1)
}
</script>

<style scoped>
.area-edit {
  max-width: 800px;
  margin: 0 auto;
  padding: 20px;
  background-color: white;
  border-radius: 8px;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
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
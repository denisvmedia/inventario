<template>
  <div class="debug-component">
    <h1>Debug Component</h1>
    <p>This is a completely separate component file.</p>
    <p>ID: {{ id }}</p>
  </div>
</template>

<script setup lang="ts">
import { useRoute } from 'vue-router'

const route = useRoute()
const id = route.params.id

console.log('CommodityEditDebug component loaded with ID:', id)
</script>

<style scoped>
.debug-component {
  background-color: #ffcccc;
  padding: 20px;
  border: 3px solid purple;
  margin: 20px;
}
</style>
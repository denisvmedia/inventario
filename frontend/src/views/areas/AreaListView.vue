<template>
  <div class="area-list">
    <div class="header">
      <h1>Areas</h1>
      <router-link to="/areas/new" class="btn btn-primary">Create New Area</router-link>
    </div>

    <div v-if="loading" class="loading">Loading...</div>
    <div v-else-if="error" class="error">{{ error }}</div>
    <div v-else-if="areas.length === 0" class="empty">No areas found. Create your first area!</div>

    <div v-else class="areas-grid">
      <div v-for="area in areas" :key="area.id" class="area-card" @click="viewArea(area.id)">
        <h3>{{ area.attributes.name }}</h3>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import areaService from '@/services/areaService'

const router = useRouter()
const areas = ref<any[]>([])
const loading = ref<boolean>(true)
const error = ref<string | null>(null)

onMounted(async () => {
  try {
    const response = await areaService.getAreas()
    areas.value = response.data.data
    loading.value = false
  } catch (err: any) {
    error.value = 'Failed to load areas: ' + (err.message || 'Unknown error')
    loading.value = false
  }
})

const viewArea = (id: string) => {
  router.push(`/areas/${id}`)
}
</script>

<style scoped>
.area-list {
  max-width: 1200px;
  margin: 0 auto;
}

.header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 2rem;
}

.loading, .error, .empty {
  text-align: center;
  padding: 2rem;
  background: white;
  border-radius: 8px;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
}

.error {
  color: #dc3545;
}

.areas-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(300px, 1fr));
  gap: 1.5rem;
}

.area-card {
  background: white;
  border-radius: 8px;
  padding: 1.5rem;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
  cursor: pointer;
  transition: transform 0.2s, box-shadow 0.2s;
}

.area-card:hover {
  transform: translateY(-5px);
  box-shadow: 0 5px 15px rgba(0, 0, 0, 0.1);
}
</style>

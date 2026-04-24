<template>
  <div class="area-list">
    <div class="header">
      <h1>Areas</h1>
      <!-- Areas are created inline on a location's detail page (#1321):
           there is no standalone /areas/new route. Point the CTA at the
           locations list so the user can pick a location and open the
           inline "Add Area" form from there. -->
      <router-link :to="groupStore.groupPath('/locations')" class="btn btn-primary new-area-button"><font-awesome-icon icon="plus" /> New</router-link>
    </div>

    <!-- Error Notification Stack -->
    <ErrorNotificationStack
      :errors="errors"
      @dismiss="removeError"
    />

    <div v-if="loading" class="loading">Loading...</div>
    <div v-else-if="areas.length === 0" class="empty">
      <div class="empty-message">
        <p>No areas found. Create your first area!</p>
        <div class="action-button">
          <router-link :to="groupStore.groupPath('/locations')" class="btn btn-primary">Create Area</router-link>
        </div>
      </div>
    </div>

    <div v-else class="areas-grid-container">
    <div class="areas-grid">
      <div v-for="area in areas" :key="area.id" class="area-card" @click="viewArea(area.id)">
        <div class="area-content">
          <h3>{{ area.attributes.name }}</h3>
          <div v-if="area.attributes.location_id" class="area-meta">
            <span class="location">Location: {{ getLocationName(area.attributes.location_id) }}</span>
          </div>
        </div>
        <div class="area-actions">
          <button class="btn btn-secondary btn-sm" title="Edit" @click.stop="editArea(area.id)">
            <font-awesome-icon icon="edit" />
          </button>
          <button class="btn btn-danger btn-sm" title="Delete" @click.stop="confirmDelete(area.id)">
            <font-awesome-icon icon="trash" />
          </button>
        </div>
      </div>
    </div>

    <!-- Pagination -->
    <PaginationControls
      :current-page="currentPage"
      :total-pages="totalPages"
      :page-size="pageSize"
      :total-items="totalAreas"
      item-label="areas"
    />
    </div>

    <!-- Area Delete Confirmation Dialog -->
    <Confirmation
      v-model:visible="showDeleteDialog"
      title="Confirm Delete"
      message="Are you sure you want to delete this area?"
      confirm-label="Delete"
      cancel-label="Cancel"
      confirm-button-class="danger"
      confirmationIcon="exclamation-triangle"
      @confirm="onConfirmDelete"
      @cancel="onCancelDelete"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount, watch } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import areaService from '@/services/areaService'
import locationService from '@/services/locationService'
import Confirmation from "@/components/Confirmation.vue"
import ErrorNotificationStack from '@/components/ErrorNotificationStack.vue'
import PaginationControls from "@/components/PaginationControls.vue"
import { useErrorState } from '@/utils/errorUtils'
import { fetchAll } from '@/utils/paginationUtils'
import { useGroupStore } from '@/stores/groupStore'

const router = useRouter()
const groupStore = useGroupStore()
const route = useRoute()
const areas = ref<any[]>([])
const locations = ref<any[]>([])
const loading = ref<boolean>(true)

// Pagination state
const currentPage = ref(1)
const pageSize = ref(50)
const totalAreas = ref(0)
const totalPages = computed(() => Math.ceil(totalAreas.value / pageSize.value))

// Error state management
const { errors, handleError, removeError, cleanup } = useErrorState()

const loadAreas = async () => {
  loading.value = true
  try {
    const [areasResponse, allLocations] = await Promise.all([
      areaService.getAreas({ page: currentPage.value, per_page: pageSize.value }),
      fetchAll(params => locationService.getLocations(params)),
    ])

    areas.value = areasResponse.data.data
    totalAreas.value = areasResponse.data.meta.areas
    locations.value = allLocations
    loading.value = false
  } catch (err: any) {
    handleError(err, 'area', 'Failed to load areas')
    loading.value = false
  }
}

onMounted(async () => {
  currentPage.value = Number(route.query.page) || 1
  await loadAreas()
})

watch(() => route.query.page, (newPage) => {
  currentPage.value = Number(newPage) || 1
  loadAreas()
})

const getLocationName = (locationId: string) => {
  const location = locations.value.find(l => l.id === locationId)
  return location ? location.attributes.name : 'Unknown Location'
}

const viewArea = (id: string) => {
  router.push(groupStore.groupPath(`/areas/${id}`))
}

const editArea = (id: string) => {
  router.push(groupStore.groupPath(`/areas/${id}/edit`))
}

const areaToDelete = ref<string | null>(null)
const showDeleteDialog = ref(false)

const confirmDelete = (id: string) => {
  areaToDelete.value = id
  showDeleteDialog.value = true
}

const onConfirmDelete = () => {
  if (areaToDelete.value) {
    deleteArea(areaToDelete.value)
    showDeleteDialog.value = false
    areaToDelete.value = null
  }
}

const onCancelDelete = () => {
  showDeleteDialog.value = false
  areaToDelete.value = null
}

const deleteArea = async (id: string) => {
  try {
    await areaService.deleteArea(id)
    // Reload the current page to reflect deletion with accurate pagination
    await loadAreas()
  } catch (err: any) {
    handleError(err, 'area', 'Failed to delete area')
  }
}

// Add cleanup when component unmounts
onBeforeUnmount(() => {
  cleanup()
})
</script>

<style lang="scss" scoped>
@use '@/assets/main' as *;

.area-list {
  max-width: $container-max-width;
  margin: 0 auto;
  padding: 20px;
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
  border-radius: $default-radius;
  box-shadow: $box-shadow;
}

.error {
  color: $danger-color;
}

.areas-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(300px, 1fr));
  gap: 1.5rem;
}

.area-card {
  background: white;
  border-radius: $default-radius;
  padding: 1.5rem;
  box-shadow: $box-shadow;
  cursor: pointer;
  transition: transform 0.2s, box-shadow 0.2s;
  display: flex;
  justify-content: space-between;
  align-items: flex-start;

  &:hover {
    transform: translateY(-5px);
    box-shadow: 0 5px 15px rgb(0 0 0 / 10%);
  }
}

.area-content {
  flex: 1;
  cursor: pointer;
}

.area-actions {
  display: flex;
  gap: 0.5rem;
  margin-left: 1rem;
  cursor: pointer;
}

.btn-sm {
  padding: 0.25rem 0.5rem;
  font-size: 0.875rem;
}

.area-meta {
  margin-top: 0.5rem;
  font-size: 0.9rem;
  color: $text-color;
}

.location {
  font-style: italic;
}

.empty-message {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 1.5rem;
}

.empty-message p {
  margin-bottom: 0;
  font-size: 1.1rem;
}

.action-button {
  margin-top: 0.5rem;
}

/* Use global button styles from main.scss */

/* Button styles are inherited from main.scss */

.areas-grid-container {
  display: flex;
  flex-direction: column;
  gap: 1.5rem;
}

</style>

<template>
  <div class="location-detail">
    <!-- Error Notification Stack -->
    <ErrorNotificationStack
      :errors="errors"
      @dismiss="removeError"
    />

    <div v-if="loading" class="loading">Loading...</div>
    <ResourceNotFound
      v-else-if="is404Error"
      resource-type="location"
      :title="get404Title('location')"
      :message="get404Message('location')"
      go-back-text="Back to Locations"
      @go-back="goBackToList"
      @try-again="loadLocation"
    />
    <div v-else-if="!location" class="not-found">Location not found</div>
    <div v-else>
      <div class="header">
        <div class="title-section">
          <h1>
              {{ location.attributes.name }}
          </h1>
          <p class="address">
              {{ location.attributes.address || 'No address provided' }}
          </p>
        </div>
        <div class="actions">
          <button class="btn btn-danger" @click="confirmDelete">Delete</button>
        </div>
      </div>

      <div class="areas-section">
        <div class="section-header">
          <h2>Areas</h2>
          <button class="btn btn-primary btn-sm" @click="showAreaForm = !showAreaForm">
            {{ showAreaForm ? 'Cancel' : 'Add Area' }}
          </button>
        </div>

        <!-- Inline Area Creation Form -->
        <AreaForm
          v-if="showAreaForm"
          :location-id="location.id"
          @created="handleAreaCreated"
          @cancel="showAreaForm = false"
        />

        <div v-if="areas.length > 0" class="areas-grid">
          <div v-for="area in areas" :key="area.id" class="area-card" @click="viewArea(area.id)">
            <div class="area-content">
              <h3>{{ area.attributes.name }}</h3>
            </div>
            <div class="area-actions">
              <button class="btn btn-secondary btn-sm" @click.stop="editArea(area.id)">
                Edit
              </button>
              <button class="btn btn-danger btn-sm" @click.stop="confirmDeleteArea(area.id)">
                Delete
              </button>
            </div>
          </div>
        </div>
        <div v-else class="no-areas">
          <p>No areas found for this location. Use the button above to add your first area.</p>
        </div>
      </div>

      <!-- Location Delete Confirmation Dialog -->
      <Confirmation
        v-model:visible="showDeleteDialog"
        title="Confirm Delete"
        message="Are you sure you want to delete this location?"
        confirm-label="Delete"
        cancel-label="Cancel"
        confirm-button-class="danger"
        confirmationIcon="exclamation-triangle"
        @confirm="onConfirmDelete"
        @cancel="onCancelDelete"
      />

      <!-- Area Delete Confirmation Dialog -->
      <Confirmation
        v-model:visible="showDeleteAreaDialog"
        title="Confirm Delete"
        message="Are you sure you want to delete this area?"
        confirm-label="Delete"
        cancel-label="Cancel"
        confirm-button-class="danger"
        confirmationIcon="exclamation-triangle"
        @confirm="onConfirmDeleteArea"
        @cancel="onCancelDeleteArea"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount, computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import locationService from '@/services/locationService'
import areaService from '@/services/areaService'
import AreaForm from '@/components/AreaForm.vue'
import Confirmation from "@/components/Confirmation.vue"
import ErrorNotificationStack from '@/components/ErrorNotificationStack.vue'
import ResourceNotFound from '@/components/ResourceNotFound.vue'
import { useErrorState, is404Error as checkIs404Error, get404Message, get404Title } from '@/utils/errorUtils'

const route = useRoute()
const router = useRouter()
const loading = ref<boolean>(true)
const location = ref<any>(null)
const areas = ref<any[]>([])
const lastError = ref<any>(null) // Store the last error object for 404 detection

// Error state management
const { errors, handleError, removeError, cleanup } = useErrorState()

// Error state computed properties
const is404Error = computed(() => lastError.value && checkIs404Error(lastError.value))

// State for inline forms
const showAreaForm = ref(false)

onMounted(() => {
  loadLocation()
})

const loadLocation = async () => {
  const id = route.params.id as string
  loading.value = true
  lastError.value = null

  try {
    // Load location and areas in parallel
    const [locationResponse, areasResponse] = await Promise.all([
      locationService.getLocation(id),
      areaService.getAreas()
    ])

    location.value = locationResponse.data.data

    // Filter areas that belong to this location
    areas.value = areasResponse.data.data.filter(
      (area: any) => area.attributes.location_id === id
    )

    loading.value = false
  } catch (err: any) {
    lastError.value = err
    if (checkIs404Error(err)) {
      // 404 errors will be handled by the ResourceNotFound component
    } else {
      handleError(err, 'location', 'Failed to load location')
    }
    loading.value = false
  }
}

const goBack = () => {
  router.push('/locations')
}

const goBackToList = () => {
  router.push('/locations')
}





const showDeleteDialog = ref(false)

const confirmDelete = () => {
  showDeleteDialog.value = true
}

const onConfirmDelete = () => {
  deleteLocation()
  showDeleteDialog.value = false
}

const onCancelDelete = () => {
  showDeleteDialog.value = false
}

const deleteLocation = async () => {
  try {
    await locationService.deleteLocation(location.value.id)
    router.push('/locations')
  } catch (err: any) {
    handleError(err, 'location', 'Failed to delete location')
  }
}

const viewArea = (id: string) => {
  router.push(`/areas/${id}`)
}

const editArea = (id: string) => {
  router.push(`/areas/${id}/edit`)
}

const areaToDelete = ref<string | null>(null)
const showDeleteAreaDialog = ref(false)

const confirmDeleteArea = (id: string) => {
  areaToDelete.value = id
  showDeleteAreaDialog.value = true
}

const onConfirmDeleteArea = () => {
  if (areaToDelete.value) {
    deleteArea(areaToDelete.value)
    showDeleteAreaDialog.value = false
    areaToDelete.value = null
  }
}

const onCancelDeleteArea = () => {
  showDeleteAreaDialog.value = false
  areaToDelete.value = null
}

const deleteArea = async (id: string) => {
  try {
    await areaService.deleteArea(id)
    // Remove the deleted area from the list
    areas.value = areas.value.filter(area => area.id !== id)
  } catch (err: any) {
    handleError(err, 'area', 'Failed to delete area')
  }
}

// Handle area creation
const handleAreaCreated = (newArea: any) => {
  areas.value.push(newArea)
  showAreaForm.value = false
}

// Add cleanup when component unmounts
onBeforeUnmount(() => {
  cleanup()
})
</script>

<style lang="scss" scoped>
@use 'sass:color';
@use '@/assets/main' as *;

.location-detail {
  max-width: $container-max-width;
  margin: 0 auto;
  padding: 20px;
}

.header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 2rem;
}

.title-section {
  display: flex;
  flex-direction: column;

  h1 {
    margin-bottom: 0.5rem;
  }
}

.address {
  color: $text-color;
  font-style: italic;
  margin-top: 0;
}

.actions {
  display: flex;
  gap: 0.5rem;
}

.loading, .error, .not-found, .no-areas {
  text-align: center;
  padding: 2rem;
  background: white;
  border-radius: $default-radius;
  box-shadow: $box-shadow;
  margin-bottom: 2rem;
}

.error {
  color: $danger-color;
}

.info-card {
  background: white;
  border-radius: $default-radius;
  padding: 1.5rem;
  box-shadow: $box-shadow;
  margin-bottom: 2rem;
}

.full-width {
  width: 100%;
}

.areas-section {
  margin-bottom: 2rem;
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

.section-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 1rem;
  padding-bottom: 0.5rem;
  border-bottom: 1px solid $border-color;
}

.btn-primary {
  background-color: $primary-color;
  color: white;
  text-decoration: none;
  padding: 0.5rem 1rem;
  border-radius: $default-radius;
  display: inline-block;
  margin-top: 1rem;
}

.btn-sm {
  padding: 0.25rem 0.5rem;
  font-size: 0.875rem;
  margin-top: 0;
  border-radius: $default-radius;
}

pre {
  white-space: pre-wrap;
  word-wrap: break-word;
  overflow-x: auto;
  background: $light-bg-color;
  padding: 0.5rem;
  border-radius: $default-radius;
}

.btn-info {
  background-color: #17a2b8;
  color: white;

  &:hover {
    background-color: #138496;
  }
}
</style>

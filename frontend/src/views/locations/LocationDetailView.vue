<template>
  <div class="location-detail">
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

      <!-- Images Section -->
      <div class="info-card full-width location-images">
        <div class="section-header">
          <h2>Images</h2>
          <button
            class="btn btn-sm"
            :class="showImageUploader ? 'btn-secondary-alt' : 'btn-primary'"
            @click="showImageUploader = !showImageUploader"
          >
            {{ showImageUploader ? 'Cancel' : 'Add Images' }}
          </button>
        </div>

        <Transition name="file-uploader" mode="out-in">
          <FileUploader
            v-if="showImageUploader"
            ref="imageUploaderRef"
            :multiple="true"
            accept=".gif,.jpg,.jpeg,.png,.webp,image/gif,image/jpeg,image/png,image/webp"
            upload-prompt="Drag and drop images here"
            upload-hint="Supports image formats (GIF, JPG, PNG, WebP)"
            @upload="uploadImages"
          />
        </Transition>

        <div v-if="loadingImages" class="loading">Loading images...</div>
        <FileViewer
          v-else
          :files="images"
          :signed-urls="imagesSignedUrls"
          file-type="images"
          :entity-id="location.id"
          entity-type="locations"
          @delete="deleteImage"
          @update="updateLocationFile"
          @download="downloadLocationFile"
        />
      </div>

      <!-- Files Section -->
      <div class="info-card full-width location-files">
        <div class="section-header">
          <h2>Files</h2>
          <button
            class="btn btn-sm"
            :class="showFileUploader ? 'btn-secondary-alt' : 'btn-primary'"
            @click="showFileUploader = !showFileUploader"
          >
            {{ showFileUploader ? 'Cancel' : 'Add Files' }}
          </button>
        </div>

        <Transition name="file-uploader" mode="out-in">
          <FileUploader
            v-if="showFileUploader"
            ref="fileUploaderRef"
            :multiple="true"
            upload-prompt="Drag and drop files here"
            upload-hint="Supports any file type"
            @upload="uploadFiles"
          />
        </Transition>

        <div v-if="loadingFiles" class="loading">Loading files...</div>
        <FileViewer
          v-else
          :files="locationFiles"
          :signed-urls="filesSignedUrls"
          file-type="files"
          :entity-id="location.id"
          entity-type="locations"
          @delete="deleteLocationFileEntry"
          @update="updateLocationFile"
          @download="downloadLocationFile"
        />
      </div>

      <!-- Location Delete Confirmation Dialog -->
      <AppConfirmDialog
        v-model:open="showDeleteDialog"
        title="Confirm Delete"
        message="Are you sure you want to delete this location?"
        confirm-label="Delete"
        cancel-label="Cancel"
        variant="danger"
        @confirm="onConfirmDelete"
        @cancel="onCancelDelete"
      />

      <!-- Area Delete Confirmation Dialog -->
      <AppConfirmDialog
        v-model:open="showDeleteAreaDialog"
        title="Confirm Delete"
        message="Are you sure you want to delete this area?"
        confirm-label="Delete"
        cancel-label="Cancel"
        variant="danger"
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
import fileService from '@/services/fileService'
import areaService from '@/services/areaService'
import AreaForm from '@/components/AreaForm.vue'
import FileViewer from '@/components/FileViewer.vue'
import FileUploader from '@/components/FileUploader.vue'
import AppConfirmDialog from "@design/patterns/AppConfirmDialog.vue"
import ResourceNotFound from '@/components/ResourceNotFound.vue'
import { useErrorState, is404Error as checkIs404Error, get404Message, get404Title } from '@/utils/errorUtils'
import { useGroupStore } from '@/stores/groupStore'

const route = useRoute()
const router = useRouter()
const groupStore = useGroupStore()
const loading = ref<boolean>(true)
const location = ref<any>(null)
const areas = ref<any[]>([])
const lastError = ref<any>(null) // Store the last error object for 404 detection

// Error state management
// Errors now surface as toasts via useErrorState (#1330 PR 5.7).
const { handleError, cleanup } = useErrorState()

// Error state computed properties
const is404Error = computed(() => lastError.value && checkIs404Error(lastError.value))

// State for inline forms
const showAreaForm = ref(false)

// Images state
const images = ref<any[]>([])
const imagesSignedUrls = ref<Record<string, any>>({})
const loadingImages = ref(false)
const showImageUploader = ref(false)
const imageUploaderRef = ref<any>(null)

// Files state
const locationFiles = ref<any[]>([])
const filesSignedUrls = ref<Record<string, any>>({})
const loadingFiles = ref(false)
const showFileUploader = ref(false)
const fileUploaderRef = ref<any>(null)

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

    // Load images and files after location is loaded
    loadImages(id)
    loadLocationFiles(id)
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

const loadImages = async (id: string) => {
  loadingImages.value = true
  try {
    const response = await locationService.getImages(id)
    images.value = response.data?.data || []
    imagesSignedUrls.value = response.data?.meta?.signed_urls || {}
  } catch (err: any) {
    console.error('Failed to load location images:', err)
  } finally {
    loadingImages.value = false
  }
}

const loadLocationFiles = async (id: string) => {
  loadingFiles.value = true
  try {
    const response = await locationService.getFiles(id)
    locationFiles.value = response.data?.data || []
    filesSignedUrls.value = response.data?.meta?.signed_urls || {}
  } catch (err: any) {
    console.error('Failed to load location files:', err)
  } finally {
    loadingFiles.value = false
  }
}

const goBackToList = () => {
  router.push(groupStore.groupPath('/locations'))
}

// Image upload/delete/update handlers
const uploadImages = async (files: File[]) => {
  if (!location.value || files.length === 0) return
  try {
    for (const file of files) {
      await locationService.uploadImage(location.value.id, file)
    }
    imageUploaderRef.value?.markUploadCompleted()
    showImageUploader.value = false
    await loadImages(location.value.id)
  } catch (err: any) {
    handleError(err, 'location', 'Failed to upload image')
    imageUploaderRef.value?.markUploadFailed()
  }
}

const deleteImage = async (image: any) => {
  if (!location.value) return
  try {
    await locationService.deleteImage(location.value.id, image.id)
    images.value = images.value.filter((img: any) => img.id !== image.id)
  } catch (err: any) {
    handleError(err, 'location', 'Failed to delete image')
  }
}

// File upload/delete handlers
const uploadFiles = async (files: File[]) => {
  if (!location.value || files.length === 0) return
  try {
    for (const file of files) {
      await locationService.uploadFile(location.value.id, file)
    }
    fileUploaderRef.value?.markUploadCompleted()
    showFileUploader.value = false
    await loadLocationFiles(location.value.id)
  } catch (err: any) {
    handleError(err, 'location', 'Failed to upload file')
    fileUploaderRef.value?.markUploadFailed()
  }
}

const deleteLocationFileEntry = async (file: any) => {
  if (!location.value) return
  try {
    await locationService.deleteFile(location.value.id, file.id)
    locationFiles.value = locationFiles.value.filter((f: any) => f.id !== file.id)
  } catch (err: any) {
    handleError(err, 'location', 'Failed to delete file')
  }
}

// Shared update handler for both images and files (updates filename/path via generic files API)
const updateLocationFile = async (data: any) => {
  if (!location.value) return
  // Find the existing file record to preserve its current title, description, tags and linkage.
  const existing =
    images.value.find((f: any) => f.id === data.id) ||
    locationFiles.value.find((f: any) => f.id === data.id)
  const attrs = existing?.attributes ?? existing ?? {}
  try {
    await fileService.updateFile(data.id, {
      path: data.path,
      title: data.title ?? attrs.title ?? data.path,
      description: data.description ?? attrs.description ?? '',
      tags: data.tags ?? attrs.tags ?? [],
      linked_entity_type: attrs.linked_entity_type ?? 'location',
      linked_entity_id: attrs.linked_entity_id ?? location.value.id,
      linked_entity_meta: attrs.linked_entity_meta,
    })
    // Update in images list
    const imgIdx = images.value.findIndex((f: any) => f.id === data.id)
    if (imgIdx !== -1) images.value[imgIdx].attributes = { ...attrs, path: data.path }
    // Update in files list
    const fileIdx = locationFiles.value.findIndex((f: any) => f.id === data.id)
    if (fileIdx !== -1) locationFiles.value[fileIdx].attributes = { ...attrs, path: data.path }
  } catch (err: any) {
    handleError(err, 'location', 'Failed to update file')
  }
}

// Download handler - uses direct download URL
const downloadLocationFile = (file: any) => {
  if (!location.value) return
  const ext = file.attributes?.ext || file.ext || ''
  const path = file.attributes?.path || file.path || file.id
  const link = document.createElement('a')
  // Use the appropriate endpoint based on the linked entity meta
  const meta = file.attributes?.linked_entity_meta || file.linked_entity_meta || 'files'
  link.href = `/api/v1/locations/${location.value.id}/${meta}/${file.id}${ext}`
  link.download = path + ext
  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)
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
    router.push(groupStore.groupPath('/locations'))
  } catch (err: any) {
    handleError(err, 'location', 'Failed to delete location')
  }
}

const viewArea = (id: string) => {
  router.push(groupStore.groupPath(`/areas/${id}`))
}

const editArea = (id: string) => {
  router.push(groupStore.groupPath(`/areas/${id}/edit`))
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
  overflow-wrap: break-word;
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

.btn-secondary-alt {
  background-color: $secondary-color;
  color: white;
  border: none;

  &:hover {
    opacity: 0.85;
  }
}

.file-uploader-enter-active,
.file-uploader-leave-active {
  transition: opacity 0.2s ease, transform 0.2s ease;
}

.file-uploader-enter-from,
.file-uploader-leave-to {
  opacity: 0;
  transform: translateY(-8px);
}
</style>

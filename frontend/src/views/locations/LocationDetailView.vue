<script setup lang="ts">
/**
 * LocationDetailView — migrated to the design system in Phase 4 of
 * Epic #1324 (issue #1329).
 *
 * Page chrome (header, sections, empty state, area cards, inline area
 * creation form, error toasts and confirm dialogs) is built from
 * `@design/*` patterns. The image / file panes still embed the legacy
 * `FileViewer` and `FileUploader` components — those will be replaced by
 * a `MediaGallery` + `FileViewerDialog` pattern in a later commit on
 * the same branch (`design-system/phase-4-detail-form-views`).
 *
 * Legacy CSS class anchors (`location-detail`, `areas-grid`) are kept
 * as no-op markers on the wrapper and grid so existing Playwright
 * selectors continue to resolve through the strangler-fig window —
 * see devdocs/frontend/migration-conventions.md.
 */
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useForm } from 'vee-validate'
import { toTypedSchema } from '@vee-validate/zod'
import { ArrowLeft, Plus, Trash2, X } from 'lucide-vue-next'

import locationService from '@/services/locationService'
import fileService from '@/services/fileService'
import areaService from '@/services/areaService'
import { useGroupStore } from '@/stores/groupStore'
import {
  is404Error as checkIs404Error,
  get404Message,
  get404Title,
  getErrorMessage,
} from '@/utils/errorUtils'

import { Button } from '@design/ui/button'
import { Input } from '@design/ui/input'
import {
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@design/ui/form'
import AreaCard from '@design/patterns/AreaCard.vue'
import Banner from '@design/patterns/Banner.vue'
import EmptyState from '@design/patterns/EmptyState.vue'
import PageContainer from '@design/patterns/PageContainer.vue'
import PageHeader from '@design/patterns/PageHeader.vue'
import PageSection from '@design/patterns/PageSection.vue'
import { useAppToast } from '@design/composables/useAppToast'
import { useConfirm } from '@design/composables/useConfirm'

import FileViewer from '@/components/FileViewer.vue'
import FileUploader from '@/components/FileUploader.vue'

import {
  areaFormSchema as locationDetailAreaFormSchema,
  type AreaFormInput as LocationDetailAreaFormInput,
} from '@/views/areas/AreaForm.schema'

type AnyRecord = Record<string, unknown>
type ApiResource = { id: string; attributes: AnyRecord }

const route = useRoute()
const router = useRouter()
const groupStore = useGroupStore()
const toast = useAppToast()
const { confirmDelete } = useConfirm()

const loading = ref<boolean>(true)
const location = ref<ApiResource | null>(null)
const areas = ref<ApiResource[]>([])
const lastError = ref<unknown>(null)
const is404 = computed(() => !!lastError.value && checkIs404Error(lastError.value as never))

const showAreaForm = ref(false)

const images = ref<ApiResource[]>([])
const imagesSignedUrls = ref<Record<string, unknown>>({})
const loadingImages = ref(false)
const showImageUploader = ref(false)
const imageUploaderRef = ref<{ markUploadCompleted: () => void; markUploadFailed: () => void } | null>(null)

const locationFiles = ref<ApiResource[]>([])
const filesSignedUrls = ref<Record<string, unknown>>({})
const loadingFiles = ref(false)
const showFileUploader = ref(false)
const fileUploaderRef = ref<{ markUploadCompleted: () => void; markUploadFailed: () => void } | null>(null)

const locationsListPath = computed(() => groupStore.groupPath('/locations'))

onMounted(() => {
  loadLocation()
})

async function loadLocation() {
  const id = route.params.id as string
  loading.value = true
  lastError.value = null

  try {
    const [locationResponse, areasResponse] = await Promise.all([
      locationService.getLocation(id),
      areaService.getAreas(),
    ])
    location.value = locationResponse.data.data
    areas.value = areasResponse.data.data.filter(
      (area: ApiResource) => (area.attributes as AnyRecord).location_id === id,
    )
    loading.value = false
    loadImages(id)
    loadLocationFiles(id)
  } catch (err) {
    lastError.value = err
    if (!checkIs404Error(err as never)) {
      toast.error(getErrorMessage(err as never, 'location', 'Failed to load location'))
    }
    loading.value = false
  }
}

async function loadImages(id: string) {
  loadingImages.value = true
  try {
    const response = await locationService.getImages(id)
    images.value = response.data?.data || []
    imagesSignedUrls.value = response.data?.meta?.signed_urls || {}
  } catch (err) {
    toast.error(getErrorMessage(err as never, 'location', 'Failed to load images'))
  } finally {
    loadingImages.value = false
  }
}

async function loadLocationFiles(id: string) {
  loadingFiles.value = true
  try {
    const response = await locationService.getFiles(id)
    locationFiles.value = response.data?.data || []
    filesSignedUrls.value = response.data?.meta?.signed_urls || {}
  } catch (err) {
    toast.error(getErrorMessage(err as never, 'location', 'Failed to load files'))
  } finally {
    loadingFiles.value = false
  }
}

function goBackToList() {
  router.push(locationsListPath.value)
}

function viewArea(id: string) {
  router.push(groupStore.groupPath(`/areas/${id}`))
}

function editArea(id: string) {
  router.push(groupStore.groupPath(`/areas/${id}/edit`))
}

async function onDeleteArea(id: string) {
  const confirmed = await confirmDelete('area')
  if (!confirmed) return
  try {
    await areaService.deleteArea(id)
    areas.value = areas.value.filter((area) => area.id !== id)
  } catch (err) {
    toast.error(getErrorMessage(err as never, 'area', 'Failed to delete area'))
  }
}

async function onDeleteLocation() {
  const confirmed = await confirmDelete('location')
  if (!confirmed) return
  try {
    if (!location.value) return
    await locationService.deleteLocation(location.value.id)
    router.push(locationsListPath.value)
  } catch (err) {
    toast.error(getErrorMessage(err as never, 'location', 'Failed to delete location'))
  }
}

const {
  handleSubmit: handleAreaSubmit,
  isSubmitting: isAreaSubmitting,
  setErrors: setAreaErrors,
  resetForm: resetAreaForm,
} = useForm<LocationDetailAreaFormInput>({
  validationSchema: toTypedSchema(locationDetailAreaFormSchema),
  initialValues: { name: '' },
})

const submitAreaForm = handleAreaSubmit(async (values) => {
  if (!location.value) return
  try {
    const response = await areaService.createArea({
      data: {
        type: 'areas',
        attributes: {
          name: values.name.trim(),
          location_id: location.value.id,
        },
      },
    })
    areas.value.push(response.data.data)
    resetAreaForm({ values: { name: '' } })
    showAreaForm.value = false
  } catch (err) {
    const apiErrors = (err as { response?: { data?: { errors?: Array<{ source?: { pointer?: string }; detail?: string }> } } })
      .response?.data?.errors
    if (Array.isArray(apiErrors) && apiErrors.length > 0) {
      const fieldErrors: Record<string, string> = {}
      for (const apiError of apiErrors) {
        const field = apiError.source?.pointer?.split('/').pop()
        if (field === 'name' && apiError.detail) fieldErrors.name = apiError.detail
      }
      if (Object.keys(fieldErrors).length > 0) {
        setAreaErrors(fieldErrors)
        return
      }
    }
    toast.error(getErrorMessage(err as never, 'area', 'Failed to create area'))
  }
})

function cancelAreaForm() {
  resetAreaForm({ values: { name: '' } })
  showAreaForm.value = false
}

async function uploadImages(files: File[]) {
  if (!location.value || files.length === 0) return
  try {
    for (const file of files) {
      await locationService.uploadImage(location.value.id, file)
    }
    imageUploaderRef.value?.markUploadCompleted()
    showImageUploader.value = false
    await loadImages(location.value.id)
  } catch (err) {
    toast.error(getErrorMessage(err as never, 'location', 'Failed to upload image'))
    imageUploaderRef.value?.markUploadFailed()
  }
}

async function deleteImage(image: ApiResource) {
  if (!location.value) return
  try {
    await locationService.deleteImage(location.value.id, image.id)
    images.value = images.value.filter((img) => img.id !== image.id)
  } catch (err) {
    toast.error(getErrorMessage(err as never, 'location', 'Failed to delete image'))
  }
}

async function uploadFiles(files: File[]) {
  if (!location.value || files.length === 0) return
  try {
    for (const file of files) {
      await locationService.uploadFile(location.value.id, file)
    }
    fileUploaderRef.value?.markUploadCompleted()
    showFileUploader.value = false
    await loadLocationFiles(location.value.id)
  } catch (err) {
    toast.error(getErrorMessage(err as never, 'location', 'Failed to upload file'))
    fileUploaderRef.value?.markUploadFailed()
  }
}

async function deleteLocationFileEntry(file: ApiResource) {
  if (!location.value) return
  try {
    await locationService.deleteFile(location.value.id, file.id)
    locationFiles.value = locationFiles.value.filter((f) => f.id !== file.id)
  } catch (err) {
    toast.error(getErrorMessage(err as never, 'location', 'Failed to delete file'))
  }
}

async function updateLocationFile(data: { id: string; path: string; title?: string; description?: string; tags?: string[] }) {
  if (!location.value) return
  const existing =
    images.value.find((f) => f.id === data.id) ||
    locationFiles.value.find((f) => f.id === data.id)
  const attrs = (existing?.attributes ?? {}) as AnyRecord
  try {
    await fileService.updateFile(data.id, {
      path: data.path,
      title: data.title ?? (attrs.title as string) ?? data.path,
      description: data.description ?? (attrs.description as string) ?? '',
      tags: data.tags ?? (attrs.tags as string[]) ?? [],
      linked_entity_type: (attrs.linked_entity_type as string) ?? 'location',
      linked_entity_id: (attrs.linked_entity_id as string) ?? location.value.id,
      linked_entity_meta: attrs.linked_entity_meta as string | undefined,
    })
    const imgIdx = images.value.findIndex((f) => f.id === data.id)
    if (imgIdx !== -1) images.value[imgIdx].attributes = { ...attrs, path: data.path }
    const fileIdx = locationFiles.value.findIndex((f) => f.id === data.id)
    if (fileIdx !== -1) locationFiles.value[fileIdx].attributes = { ...attrs, path: data.path }
  } catch (err) {
    toast.error(getErrorMessage(err as never, 'location', 'Failed to update file'))
  }
}

function downloadLocationFile(file: ApiResource & { ext?: string; path?: string; linked_entity_meta?: string }) {
  if (!location.value) return
  const attrs = (file.attributes ?? {}) as AnyRecord
  const ext = (attrs.ext as string) || file.ext || ''
  const path = (attrs.path as string) || file.path || file.id
  const meta = (attrs.linked_entity_meta as string) || file.linked_entity_meta || 'files'
  const link = document.createElement('a')
  link.href = `/api/v1/locations/${location.value.id}/${meta}/${file.id}${ext}`
  link.download = path + ext
  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)
}
</script>

<template>
  <PageContainer as="div" class="location-detail">
    <div v-if="loading" class="py-12 text-center text-sm text-muted-foreground">Loading...</div>

    <EmptyState
      v-else-if="is404"
      :title="get404Title('location')"
      :description="get404Message('location')"
    >
      <template #actions>
        <Button variant="outline" @click="goBackToList">
          <ArrowLeft class="size-4" aria-hidden="true" />
          Back to Locations
        </Button>
        <Button @click="loadLocation">Try Again</Button>
      </template>
    </EmptyState>

    <Banner v-else-if="!location" variant="warning">Location not found</Banner>

    <template v-else>
      <PageHeader
        :title="location.attributes.name as string"
        :description="(location.attributes.address as string) || 'No address provided'"
      >
        <template #actions>
          <Button variant="destructive" @click="onDeleteLocation">
            <Trash2 class="size-4" aria-hidden="true" />
            Delete
          </Button>
        </template>
      </PageHeader>

      <PageSection title="Areas" class="mb-8">
        <template #actions>
          <Button
            :variant="showAreaForm ? 'outline' : 'default'"
            size="sm"
            @click="showAreaForm = !showAreaForm"
          >
            <component :is="showAreaForm ? X : Plus" class="size-4" aria-hidden="true" />
            {{ showAreaForm ? 'Cancel' : 'Add Area' }}
          </Button>
        </template>

        <form
          v-if="showAreaForm"
          class="mb-6 flex flex-col gap-4 rounded-md border border-border bg-card p-4 shadow-sm sm:ml-8"
          data-testid="location-detail-area-form"
          @submit="submitAreaForm"
        >
          <FormField v-slot="{ componentField }" name="name">
            <FormItem id="name">
              <FormLabel required>Area Name</FormLabel>
              <FormControl>
                <Input
                  v-bind="componentField"
                  type="text"
                  placeholder="Enter area name"
                  data-testid="location-detail-area-form-name"
                />
              </FormControl>
              <FormMessage />
            </FormItem>
          </FormField>

          <div class="flex justify-end gap-2">
            <Button type="button" variant="outline" @click="cancelAreaForm">Cancel</Button>
            <Button type="submit" :disabled="isAreaSubmitting" data-testid="location-detail-area-form-submit">
              {{ isAreaSubmitting ? 'Creating...' : 'Create Area' }}
            </Button>
          </div>
        </form>

        <div
          v-if="areas.length > 0"
          class="areas-grid grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-3"
        >
          <AreaCard
            v-for="area in areas"
            :key="area.id"
            :area="(area as never)"
            @view="viewArea"
            @edit="editArea"
            @delete="onDeleteArea"
          />
        </div>
        <EmptyState
          v-else
          title="No areas yet"
          description="No areas found for this location. Use the button above to add your first area."
        />
      </PageSection>

      <!-- `.location-images` is a strangler-fig anchor preserved for
           `e2e/tests/location-file-uploads.spec.ts:16,44,52,59`, which
           waits for the section, scopes the upload helper to it, and
           asserts that uploaded `.file-item` rows render inside it.
           See devdocs/frontend/migration-conventions.md. -->
      <PageSection title="Images" class="location-images mb-8">
        <template #actions>
          <Button
            :variant="showImageUploader ? 'outline' : 'default'"
            size="sm"
            @click="showImageUploader = !showImageUploader"
          >
            <component :is="showImageUploader ? X : Plus" class="size-4" aria-hidden="true" />
            {{ showImageUploader ? 'Cancel' : 'Add Images' }}
          </Button>
        </template>

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

        <div v-if="loadingImages" class="py-6 text-center text-sm text-muted-foreground">
          Loading images...
        </div>
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
      </PageSection>

      <!-- `.location-files` is a strangler-fig anchor preserved for
           `e2e/tests/location-file-uploads.spec.ts:48,53,60`, which
           scopes the upload helper to this section and asserts that
           uploaded `.file-item` rows render inside it. See
           devdocs/frontend/migration-conventions.md. -->
      <PageSection title="Files" class="location-files">
        <template #actions>
          <Button
            :variant="showFileUploader ? 'outline' : 'default'"
            size="sm"
            @click="showFileUploader = !showFileUploader"
          >
            <component :is="showFileUploader ? X : Plus" class="size-4" aria-hidden="true" />
            {{ showFileUploader ? 'Cancel' : 'Add Files' }}
          </Button>
        </template>

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

        <div v-if="loadingFiles" class="py-6 text-center text-sm text-muted-foreground">
          Loading files...
        </div>
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
      </PageSection>
    </template>
  </PageContainer>
</template>

<script setup lang="ts">
/**
 * FileCreateView — migrated to the design system in Phase 4 of
 * Epic #1324 (issue #1329).
 *
 * Thin wrapper around the existing `FileUploader` component (kept
 * as-is for now — its 600+ LOC of drag/drop, capacity-check and
 * progress logic is out of scope for this migration). The page
 * chrome (back link, header, action footer, error banner) is now
 * built from `@design/*` patterns.
 *
 * Legacy DOM anchors (`.file-create-view`, `.breadcrumb-link`) are
 * preserved as no-op markers so existing Playwright selectors keep
 * resolving — see devdocs/frontend/migration-conventions.md.
 */
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { ArrowLeft, Upload } from 'lucide-vue-next'

import FileUploader from '@/components/FileUploader.vue'
import fileService from '@/services/fileService'
import { useGroupStore } from '@/stores/groupStore'
import { getErrorMessage } from '@/utils/errorUtils'

import { Button } from '@design/ui/button'
import Banner from '@design/patterns/Banner.vue'
import PageContainer from '@design/patterns/PageContainer.vue'
import PageHeader from '@design/patterns/PageHeader.vue'
import PageSection from '@design/patterns/PageSection.vue'
import FormFooter from '@design/patterns/FormFooter.vue'

const router = useRouter()
const groupStore = useGroupStore()

const uploading = ref<boolean>(false)
const error = ref<string | null>(null)
const hasSelectedFiles = ref<boolean>(false)
const fileUploader = ref<InstanceType<typeof FileUploader> | null>(null)

function handleFilesCleared() {
  hasSelectedFiles.value = false
  error.value = null
}

function handleFilesSelected() {
  hasSelectedFiles.value = true
}

async function uploadFile() {
  const selectedFiles = fileUploader.value?.selectedFiles
  if (!selectedFiles || selectedFiles.length === 0) return

  const file = selectedFiles[0]
  uploading.value = true
  error.value = null

  try {
    const onProgress = (current: number, total: number, currentFile: string) => {
      fileUploader.value?.updateProgress(current, total, currentFile)
    }
    const response = await fileService.uploadFile(file, onProgress)
    fileUploader.value?.markUploadCompleted()

    const responseFiles = response.data.data
    if (Array.isArray(responseFiles) && responseFiles.length > 0) {
      router.push(groupStore.groupPath(`/files/${responseFiles[0].id}`))
    } else if (responseFiles && (responseFiles as { id?: string }).id) {
      router.push(groupStore.groupPath(`/files/${(responseFiles as { id: string }).id}`))
    } else {
      router.push(groupStore.groupPath('/files'))
    }
  } catch (err) {
    error.value = getErrorMessage(err as never, 'file', 'Failed to upload file')
    fileUploader.value?.markUploadFailed()
  } finally {
    uploading.value = false
  }
}

function onUploadCapacityFailed(capacityError: { message?: string }) {
  error.value = `Upload capacity unavailable: ${capacityError.message || 'Try again later'}`
}

function clearError() {
  error.value = null
}

function goBack() {
  router.push(groupStore.groupPath('/files'))
}
</script>

<template>
  <PageContainer as="div" class="file-create-view mx-auto max-w-3xl">
    <div class="mb-2 text-sm">
      <a
        href="#"
        class="breadcrumb-link inline-flex items-center gap-1 text-primary hover:underline"
        @click.prevent="goBack"
      >
        <ArrowLeft class="size-4" aria-hidden="true" />
        <span>Back to Files</span>
      </a>
    </div>

    <PageHeader title="Upload Files" />

    <PageSection title="Select File">
      <p class="mb-4 text-sm text-muted-foreground">
        File will be uploaded with auto-detected metadata. You can edit the
        details after upload.
      </p>
      <FileUploader
        ref="fileUploader"
        :multiple="false"
        accept="*/*"
        upload-prompt="Drag and drop a file here"
        upload-hint="Supports images, documents, videos, audio files, and archives"
        operation-name="file_upload"
        :require-slots="true"
        :hide-upload-button="true"
        @filesCleared="handleFilesCleared"
        @filesSelected="handleFilesSelected"
        @upload-capacity-failed="onUploadCapacityFailed"
      />

      <!-- `upload-actions` is a strangler-fig anchor preserved for
           `e2e/tests/user-isolation.spec.ts:200` and
           `e2e/tests/file-management.spec.ts`, which click
           `.upload-actions button:has-text("Upload File")` to trigger
           the upload (legacy template wrapped the buttons in
           `<div class="upload-actions">`). -->
      <FormFooter v-if="hasSelectedFiles" class="upload-actions mt-6">
        <Button variant="outline" :disabled="uploading" @click="goBack">
          Cancel
        </Button>
        <Button :disabled="uploading" @click="uploadFile">
          <Upload class="size-4" aria-hidden="true" />
          {{ uploading ? 'Uploading...' : 'Upload File' }}
        </Button>
      </FormFooter>
    </PageSection>

    <Banner
      v-if="error"
      variant="error"
      class="mt-4"
      dismissible
      @dismiss="clearError"
    >
      <div>
        <p class="m-0 font-semibold">Upload Failed</p>
        <p class="m-0 mt-1">{{ error }}</p>
      </div>
    </Banner>
  </PageContainer>
</template>

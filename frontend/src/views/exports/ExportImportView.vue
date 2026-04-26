<script setup lang="ts">
/**
 * ExportImportView — migrated to the design system in Phase 4 of
 * Epic #1324 (issue #1329).
 *
 * Wraps the design-system `FileUploader` for the XML export upload, plus a
 * description text input. Once the file finishes uploading the user
 * triggers `Import Export` which kicks off the async import job and
 * navigates to the export detail view; status is polled in the
 * background and surfaced through `vue-sonner` toasts.
 *
 * Legacy DOM anchors (`.export-detail-page`, `.breadcrumb-link`,
 * `.export-card`, `.field-error`) preserved as no-op markers for
 * Playwright stability.
 */
import { ref, computed, onBeforeUnmount } from 'vue'
import { useRouter } from 'vue-router'
import { ArrowLeft, Loader2, Upload } from 'lucide-vue-next'

import FileUploader from '@design/patterns/FileUploader.vue'
import exportService from '@/services/exportService'
import api from '@/services/api'
import { useGroupStore } from '@/stores/groupStore'

import { Button } from '@design/ui/button'
import { Input } from '@design/ui/input'
import { Label } from '@design/ui/label'
import Banner from '@design/patterns/Banner.vue'
import PageContainer from '@design/patterns/PageContainer.vue'
import PageHeader from '@design/patterns/PageHeader.vue'
import PageSection from '@design/patterns/PageSection.vue'
import FormFooter from '@design/patterns/FormFooter.vue'
import { useAppToast } from '@design/composables/useAppToast'

const router = useRouter()
const groupStore = useGroupStore()
const toast = useAppToast()

const fileUploader = ref<InstanceType<typeof FileUploader> | null>(null)
const error = ref<string>('')
const creating = ref<boolean>(false)
const formError = ref<Record<string, string> | null>(null)
const uploadedFilePath = ref<string>('')

const form = ref({
  description: '',
  source_file_path: '',
})

const canSubmit = computed(
  () => !!form.value.description.trim() && !!uploadedFilePath.value && !creating.value,
)

async function handleFileUpload(files: File[]) {
  if (files.length === 0) return
  const file = files[0]

  if (!file.name.toLowerCase().endsWith('.xml')) {
    error.value = 'Please select a valid XML file'
    return
  }

  try {
    error.value = ''
    const formData = new FormData()
    formData.append('files', file)

    const response = await api.post('/api/v1/uploads/restores', formData, {
      headers: { 'Content-Type': 'multipart/form-data' },
    })

    uploadedFilePath.value = response.data.attributes.fileNames[0]
    form.value.source_file_path = response.data.attributes.fileNames[0]

    if (!form.value.description) {
      const fileName = file.name.replace(/\.[^/.]+$/, '')
      form.value.description = `Imported from ${fileName}`
    }

    toast.success(`File "${file.name}" uploaded successfully`)
    fileUploader.value?.markUploadCompleted()
  } catch (err: unknown) {
    const e = err as { response?: { data?: { errors?: Array<{ detail?: string }> } }; message?: string }
    error.value = e.response?.data?.errors?.[0]?.detail || e.message || 'Failed to upload file'
    fileUploader.value?.markUploadFailed()
  }
}

function handleFilesCleared() {
  uploadedFilePath.value = ''
  form.value.source_file_path = ''
  error.value = ''
}

function scrollToFirstError() {
  setTimeout(() => {
    const firstError = document.querySelector('.field-error, .form-error')
    if (firstError) {
      firstError.scrollIntoView({ behavior: 'smooth', block: 'center' })
    }
  }, 100)
}

// Polling lifecycle: the view navigates away as soon as the import
// is kicked off, so the chained `setTimeout` loop must be cancellable
// to stop background API calls and toasts after unmount.
let pollTimer: ReturnType<typeof setTimeout> | null = null
let pollAborted = false

function clearPollTimer() {
  if (pollTimer !== null) {
    clearTimeout(pollTimer)
    pollTimer = null
  }
}

onBeforeUnmount(() => {
  pollAborted = true
  clearPollTimer()
})

function pollImportStatus(exportId: string) {
  let attempts = 0
  const maxAttempts = 300
  const intervalMs = 2000

  const poll = async () => {
    pollTimer = null
    if (pollAborted) return
    try {
      attempts++
      const response = await exportService.getExport(exportId)
      if (pollAborted) return
      const exportData = response.data.data.attributes

      if (exportData.status === 'completed') {
        toast.success(`Import "${form.value.description}" completed successfully`, { duration: 8000 })
        return
      }
      if (exportData.status === 'failed') {
        toast.error(exportData.error_message || 'Import operation failed', { duration: 10000 })
        return
      }
      if (attempts >= maxAttempts) {
        toast.warning(
          'Lost connection to import status updates. Check the export details page for current status.',
          { duration: 8000 },
        )
        return
      }
      if (exportData.status === 'pending' || exportData.status === 'in_progress') {
        pollTimer = setTimeout(poll, intervalMs)
      }
    } catch {
      if (pollAborted) return
      toast.warning(
        'Lost connection to import status updates. Check the export details page for current status.',
        { duration: 8000 },
      )
    }
  }
  pollTimer = setTimeout(poll, intervalMs)
}

async function handleSubmit() {
  if (!canSubmit.value) return

  try {
    creating.value = true
    error.value = ''
    formError.value = null

    const requestData = {
      data: {
        type: 'exports',
        attributes: {
          description: form.value.description.trim(),
          source_file_path: form.value.source_file_path,
        },
      },
    }

    const response = await exportService.importExport(requestData)
    const exportId = response.data.data.id

    toast.success(`Import "${form.value.description}" started in the background`, { duration: 5000 })
    pollImportStatus(exportId)
    router.push(groupStore.groupPath(`/exports/${exportId}`))
  } catch (err: unknown) {
    const e = err as { response?: { data?: { errors?: Array<{ detail?: string; source?: { pointer?: string } }> } } }
    if (e.response?.data?.errors) {
      const apiErrors = e.response.data.errors
      const errorObj: Record<string, string> = {}
      apiErrors.forEach((apiErr) => {
        if (apiErr.source?.pointer && apiErr.detail) {
          const field = apiErr.source.pointer.replace('/data/attributes/', '')
          errorObj[field] = apiErr.detail
        }
      })
      if (Object.keys(errorObj).length > 0) {
        formError.value = errorObj
        scrollToFirstError()
        return
      }
    }
    error.value = e.response?.data?.errors?.[0]?.detail || 'Failed to import export'
  } finally {
    creating.value = false
  }
}

function goBack() {
  router.push(groupStore.groupPath('/exports'))
}
</script>

<template>
  <PageContainer as="div" class="export-detail-page mx-auto max-w-3xl">
    <div class="mb-2 text-sm">
      <a
        href="#"
        class="breadcrumb-link inline-flex items-center gap-1 text-primary hover:underline"
        @click.prevent="goBack"
      >
        <ArrowLeft class="size-4" aria-hidden="true" />
        <span>Back to Exports</span>
      </a>
    </div>

    <PageHeader title="Upload and Register Export File" />

    <Banner v-if="error" variant="error" class="mb-4">{{ error }}</Banner>

    <form class="export-card" @submit.prevent="handleSubmit">
      <PageSection title="Upload XML Export File">
        <p class="mb-4 rounded-md border-l-4 border-l-primary bg-primary/5 p-4 text-sm text-foreground">
          This must be an XML export file created by Inventario. Once uploaded,
          it will be available in your list of exports for restoration.
        </p>

        <div class="mb-6">
          <Label for="file-upload" class="mb-2 inline-block">XML Export File</Label>
          <FileUploader
            ref="fileUploader"
            :multiple="false"
            accept=".xml,application/xml,text/xml"
            upload-prompt="Drag and drop XML export file here"
            upload-hint="Supports XML export files created by Inventario"
            @upload="handleFileUpload"
            @filesCleared="handleFilesCleared"
          />
          <p class="mt-1 text-xs text-muted-foreground">
            Select an XML export file that was previously generated by Inventario
          </p>
          <p v-if="formError?.source_file_path" class="field-error mt-1 text-sm text-destructive">
            {{ formError.source_file_path }}
          </p>
        </div>

        <div class="mb-2">
          <Label for="description" class="mb-2 inline-block">Description</Label>
          <Input
            id="description"
            v-model="form.description"
            type="text"
            placeholder="Enter a description for this imported export"
            maxlength="500"
            required
          />
          <p class="mt-1 text-xs text-muted-foreground">
            Provide a description to identify this imported export
          </p>
          <p v-if="formError?.description" class="field-error mt-1 text-sm text-destructive">
            {{ formError.description }}
          </p>
        </div>

        <FormFooter class="mt-6">
          <Button variant="outline" type="button" :disabled="creating" @click="goBack">
            Cancel
          </Button>
          <Button type="submit" :disabled="!canSubmit">
            <Loader2 v-if="creating" class="size-4 motion-safe:animate-spin" aria-hidden="true" />
            <Upload v-else class="size-4" aria-hidden="true" />
            {{ creating ? 'Importing...' : 'Import Export' }}
          </Button>
        </FormFooter>
      </PageSection>
    </form>

    <Banner
      v-if="formError && Object.keys(formError).length > 0"
      variant="error"
      class="form-error mt-4"
    >
      <p class="m-0 font-semibold">Please correct the following errors:</p>
      <ul class="m-0 mt-2 list-disc pl-5">
        <li v-for="(message, field) in formError" :key="field">
          <strong>{{ field }}:</strong> {{ message }}
        </li>
      </ul>
    </Banner>
  </PageContainer>
</template>


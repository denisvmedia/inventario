<script setup lang="ts">
/**
 * RestoreCreateView — migrated to the design system in Phase 4 of
 * Epic #1324 (issue #1329).
 *
 * Lets the user start a restore operation against a completed export.
 * Replaces the legacy SCSS / PrimeVue markup with design-system
 * primitives (PageContainer / PageHeader / PageSection / Banner /
 * Button / RadioGroup / Checkbox / Textarea / Label / FormFooter)
 * and Tailwind v4 utilities. Lucide icons replace FontAwesome.
 *
 * Toast notifications go through `useAppToast` (vue-sonner) instead of
 * PrimeVue's `useToast`.
 */
import { ref, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ArrowLeft, Loader2, RefreshCw, Upload } from 'lucide-vue-next'

import exportService from '@/services/exportService'
import type { Export, RestoreRequest, RestoreOptions } from '@/types'
import { useGroupStore } from '@/stores/groupStore'

import { Button } from '@design/ui/button'
import { Textarea } from '@design/ui/textarea'
import { Label } from '@design/ui/label'
import { RadioGroup, RadioGroupItem } from '@design/ui/radio-group'
import { Checkbox } from '@design/ui/checkbox'
import Banner from '@design/patterns/Banner.vue'
import PageContainer from '@design/patterns/PageContainer.vue'
import PageHeader from '@design/patterns/PageHeader.vue'
import PageSection from '@design/patterns/PageSection.vue'
import FormFooter from '@design/patterns/FormFooter.vue'
import { useAppToast } from '@design/composables/useAppToast'

const route = useRoute()
const router = useRouter()
const groupStore = useGroupStore()
const toast = useAppToast()

const exportId = route.params.id as string
const exportData = ref<Export | null>(null)
const loading = ref(true)
const error = ref('')
const creating = ref(false)
const formError = ref<Record<string, string> | null>(null)

const form = ref<RestoreRequest>({
  description: '',
  source_file_path: '',
  options: {
    strategy: 'merge_add',
    include_file_data: true,
    dry_run: false,
  } as RestoreOptions,
})

const formErrors = ref<Record<string, string>>({})

const canSubmit = computed(() => {
  return Boolean(
    exportData.value &&
      form.value.description.trim() &&
      exportData.value.file_path &&
      !creating.value,
  )
})

const formatExportType = (type: string) => {
  const typeMap = {
    full_database: 'Full Database',
    selected_items: 'Selected Items',
    locations: 'Locations',
    areas: 'Areas',
    commodities: 'Commodities',
  }
  return typeMap[type as keyof typeof typeMap] || type
}

const formatDate = (dateString?: string) => {
  if (!dateString) return '-'
  try { return new Date(dateString).toLocaleString() } catch { return dateString }
}

const formatFileSize = (bytes: number): string => {
  if (bytes === 0) return '0 Bytes'
  const k = 1024
  const sizes = ['Bytes', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
}

const loadExport = async () => {
  try {
    loading.value = true
    error.value = ''
    const response = await exportService.getExport(exportId)
    if (response.data && response.data.data) {
      exportData.value = {
        id: response.data.data.id,
        ...response.data.data.attributes,
      }
      form.value.source_file_path = exportData.value.file_path || ''
      if (!form.value.description) {
        form.value.description = `Restore from "${exportData.value.description}"`
      }
    }
  } catch (err: any) {
    error.value = err.response?.data?.errors?.[0]?.detail || 'Failed to load export'
    console.error('Error loading export:', err)
  } finally {
    loading.value = false
  }
}

const validateForm = (): boolean => {
  formErrors.value = {}
  if (!form.value.description.trim()) {
    formErrors.value.description = 'Description is required'
  }
  if (!form.value.options.strategy) {
    formErrors.value.strategy = 'Restore strategy is required'
  }
  return Object.keys(formErrors.value).length === 0
}

const scrollToFirstError = () => {
  const firstErrorElement = document.querySelector('.error-message')
  if (firstErrorElement) {
    firstErrorElement.scrollIntoView({ behavior: 'smooth', block: 'center' })
  }
}

const createRestore = async () => {
  if (!validateForm()) {
    scrollToFirstError()
    return
  }
  try {
    creating.value = true
    error.value = ''
    formError.value = null

    const requestData = {
      data: {
        type: 'restores',
        attributes: {
          description: form.value.description,
          options: form.value.options,
        },
      },
    }

    const response = await exportService.createRestore(exportId, requestData)
    const restore = response.data.data.attributes

    toast.success('Restore Started', {
      description: `Restore operation "${form.value.description}" has been started and is running in the background`,
      duration: 5000,
    })

    exportService
      .pollRestoreStatus(exportId, restore.id, (updatedRestore) => {
        console.log('Restore status update:', updatedRestore.status)
      })
      .then((finalRestore) => {
        if (finalRestore.status === 'completed') {
          toast.success('Restore Completed', {
            description: `Restore operation "${form.value.description}" completed successfully`,
            duration: 8000,
          })
        } else if (finalRestore.status === 'failed') {
          toast.error('Restore Failed', {
            description: finalRestore.error_message || 'Restore operation failed',
            duration: 10000,
          })
        }
      })
      .catch((err) => {
        console.error('Error polling restore status:', err)
        toast.warning('Restore Monitoring Lost', {
          description:
            'Lost connection to restore status updates. Check the export details page for current status.',
          duration: 8000,
        })
      })

    router.push(groupStore.groupPath(`/exports/${exportId}`))
  } catch (err: any) {
    console.error('Error creating restore:', err)
    if (err.response?.data?.errors) {
      const apiErrors = err.response.data.errors
      const errorObj: Record<string, string> = {}
      apiErrors.forEach((apiError: any) => {
        if (apiError.source?.pointer) {
          const field = apiError.source.pointer.replace('/data/attributes/', '')
          errorObj[field] = apiError.detail
        }
      })
      if (Object.keys(errorObj).length > 0) {
        formError.value = errorObj
        scrollToFirstError()
        return
      }
    }
    const apiError = err.response?.data?.errors?.[0]
    if (apiError?.error?.msg) {
      error.value = apiError.error.msg
    } else if (apiError?.detail) {
      error.value = apiError.detail
    } else {
      error.value = 'Failed to create restore operation'
    }
  } finally {
    creating.value = false
  }
}

onMounted(() => {
  loadExport()
})
</script>

<template>
  <PageContainer as="div" class="export-detail-page mx-auto max-w-3xl">
    <div class="breadcrumb-nav mb-2 text-sm">
      <router-link
        :to="groupStore.groupPath(`/exports/${exportId}`)"
        class="breadcrumb-link inline-flex items-center gap-1 text-primary hover:underline"
      >
        <ArrowLeft class="size-4" aria-hidden="true" />
        <span>Back to Export Details</span>
      </router-link>
    </div>

    <PageHeader title="Restore from Export" />

    <div v-if="loading" class="loading mt-4 inline-flex items-center gap-2 text-sm text-muted-foreground">
      <Loader2 class="size-4 motion-safe:animate-spin" aria-hidden="true" />
      <span>Loading export details...</span>
    </div>

    <div v-else-if="error" class="mt-4 flex flex-col gap-2">
      <Banner variant="error">{{ error }}</Banner>
      <div>
        <Button variant="outline" @click="loadExport">
          <RefreshCw class="size-4" aria-hidden="true" />
          Retry
        </Button>
      </div>
    </div>

    <template v-else>
      <PageSection
        v-if="exportData"
        title="Export Information"
        class="export-card mt-4 rounded-md border bg-card p-6 shadow-sm"
      >
        <div class="info-grid grid grid-cols-1 gap-4 sm:grid-cols-2">
          <div class="info-item">
            <label class="block text-xs font-medium uppercase tracking-wide text-muted-foreground">Description</label>
            <div class="value mt-1 text-sm">{{ exportData.description || 'No description' }}</div>
          </div>
          <div class="info-item">
            <label class="block text-xs font-medium uppercase tracking-wide text-muted-foreground">Type</label>
            <div class="value mt-1">
              <span
                :class="[
                  'type-badge inline-flex items-center rounded-full border px-2 py-0.5 text-xs font-medium',
                  `type-${exportData.type}`,
                ]"
              >
                {{ formatExportType(exportData.type) }}
              </span>
            </div>
          </div>
          <div class="info-item">
            <label class="block text-xs font-medium uppercase tracking-wide text-muted-foreground">Include File Data</label>
            <div class="value mt-1">
              <span
                :class="[
                  'bool-badge inline-flex items-center rounded-full border px-2 py-0.5 text-xs font-medium',
                  exportData.include_file_data
                    ? 'border-success/40 bg-success/10 text-success yes'
                    : 'border-muted-foreground/30 bg-muted text-muted-foreground no',
                ]"
              >
                {{ exportData.include_file_data ? 'Yes' : 'No' }}
              </span>
            </div>
          </div>
          <div class="info-item">
            <label class="block text-xs font-medium uppercase tracking-wide text-muted-foreground">Created</label>
            <div class="value mt-1 text-sm">{{ formatDate(exportData.created_date) }}</div>
          </div>
          <div v-if="exportData.file_size" class="info-item">
            <label class="block text-xs font-medium uppercase tracking-wide text-muted-foreground">File Size</label>
            <div class="value mt-1 text-sm">{{ formatFileSize(exportData.file_size) }}</div>
          </div>
          <div v-if="exportData.file_path" class="info-item sm:col-span-2">
            <label class="block text-xs font-medium uppercase tracking-wide text-muted-foreground">File Location</label>
            <div class="value file-path mt-1 break-all rounded bg-muted/40 p-2 font-mono text-xs">
              {{ exportData.file_path }}
            </div>
          </div>
        </div>
      </PageSection>

      <form class="restore-form mt-4 flex flex-col gap-4" @submit.prevent="createRestore">
        <PageSection title="Restore Description" class="rounded-md border bg-card p-6 shadow-sm">
          <div class="flex flex-col gap-2">
            <Label for="description" class="text-sm font-semibold">Description</Label>
            <Textarea
              id="description"
              v-model="form.description"
              placeholder="Enter a description for this restore operation..."
              rows="3"
              maxlength="500"
              required
              :class="[{ 'is-invalid border-destructive': formErrors.description }]"
            />
            <div v-if="formErrors.description" class="error-message text-sm text-destructive">
              {{ formErrors.description }}
            </div>
            <div class="field-help text-xs text-muted-foreground">
              Describe what this restore operation will accomplish
            </div>
          </div>
        </PageSection>

        <PageSection title="Restore Strategy" class="rounded-md border bg-card p-6 shadow-sm">
          <RadioGroup v-model="form.options.strategy" class="strategy-options gap-3">
            <Label
              :class="[
                'strategy-option flex cursor-pointer items-start gap-3 rounded-md border-2 p-4 transition-colors hover:border-primary hover:bg-primary/5',
                form.options.strategy === 'merge_add'
                  ? 'selected border-primary bg-primary/10'
                  : 'border-border',
              ]"
              for="strategy-merge-add"
            >
              <RadioGroupItem id="strategy-merge-add" value="merge_add" class="mt-1" />
              <span class="strategy-label flex flex-1 flex-col gap-1">
                <strong class="text-sm font-semibold">Merge Add</strong>
                <span class="strategy-description text-xs text-muted-foreground">
                  Only add data from backup that is missing in current database
                </span>
              </span>
            </Label>

            <Label
              :class="[
                'strategy-option flex cursor-pointer items-start gap-3 rounded-md border-2 p-4 transition-colors hover:border-primary hover:bg-primary/5',
                form.options.strategy === 'merge_update'
                  ? 'selected border-primary bg-primary/10'
                  : 'border-border',
              ]"
              for="strategy-merge-update"
            >
              <RadioGroupItem id="strategy-merge-update" value="merge_update" class="mt-1" />
              <span class="strategy-label flex flex-1 flex-col gap-1">
                <strong class="text-sm font-semibold">Merge Update</strong>
                <span class="strategy-description text-xs text-muted-foreground">
                  Create if missing, update if exists, leave other records untouched
                </span>
              </span>
            </Label>


            <Label
              :class="[
                'strategy-option flex cursor-pointer items-start gap-3 rounded-md border-2 p-4 transition-colors hover:border-primary hover:bg-primary/5',
                form.options.strategy === 'full_replace'
                  ? 'selected border-primary bg-primary/10'
                  : 'border-border',
              ]"
              for="strategy-full-replace"
            >
              <RadioGroupItem id="strategy-full-replace" value="full_replace" class="mt-1" />
              <span class="strategy-label flex flex-1 flex-col gap-1">
                <strong class="text-sm font-semibold">Full Replace</strong>
                <span class="strategy-description text-xs text-muted-foreground">
                  Clear all existing data and restore everything from backup
                </span>
              </span>
            </Label>
          </RadioGroup>
          <div v-if="formErrors.strategy" class="error-message mt-2 text-sm text-destructive">
            {{ formErrors.strategy }}
          </div>
        </PageSection>

        <PageSection title="Options" class="rounded-md border bg-card p-6 shadow-sm">
          <div class="flex flex-col gap-2">
            <Label for="include-file-data" class="checkbox-label inline-flex items-center gap-2 text-sm font-normal">
              <Checkbox
                id="include-file-data"
                :model-value="!!form.options.include_file_data"
                @update:model-value="(v) => (form.options.include_file_data = !!v)"
              />
              <span>Include file data (images, invoices, manuals)</span>
            </Label>
            <div class="field-help text-xs text-muted-foreground">
              When enabled, restores binary file data along with database records
            </div>
          </div>

          <div class="mt-4 flex flex-col gap-2">
            <Label for="dry-run" class="checkbox-label inline-flex items-center gap-2 text-sm font-normal">
              <Checkbox
                id="dry-run"
                :model-value="!!form.options.dry_run"
                @update:model-value="(v) => (form.options.dry_run = !!v)"
              />
              <span>Dry run (preview changes without applying them)</span>
            </Label>
            <div class="field-help text-xs text-muted-foreground">
              When enabled, shows what would be restored without making actual changes
            </div>
          </div>
        </PageSection>

        <FormFooter class="mt-2">
          <router-link :to="groupStore.groupPath(`/exports/${exportId}`)">
            <Button variant="outline" type="button">Cancel</Button>
          </router-link>
          <Button type="submit" :disabled="!canSubmit || creating">
            <Loader2 v-if="creating" class="size-4 motion-safe:animate-spin" aria-hidden="true" />
            <Upload v-else class="size-4" aria-hidden="true" />
            {{ creating ? 'Starting Restore...' : (form.options.dry_run ? 'Preview Restore' : 'Start Restore') }}
          </Button>
        </FormFooter>
      </form>

      <Banner v-if="formError" variant="error" class="mt-4">
        <div class="flex flex-col gap-1">
          <strong class="text-sm font-semibold">Validation Errors:</strong>
          <ul class="ml-6 list-disc text-sm">
            <li v-for="(err, field) in formError" :key="field">
              <strong>{{ field }}:</strong> {{ err }}
            </li>
          </ul>
        </div>
      </Banner>
    </template>
  </PageContainer>
</template>

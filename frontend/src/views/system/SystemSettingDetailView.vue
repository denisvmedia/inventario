<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ArrowLeft, Loader2 } from 'lucide-vue-next'

import { Button } from '@design/ui/button'
import { Checkbox } from '@design/ui/checkbox'
import { Input } from '@design/ui/input'
import { Label } from '@design/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@design/ui/select'
import FormGrid from '@design/patterns/FormGrid.vue'
import FormSection from '@design/patterns/FormSection.vue'
import PageContainer from '@design/patterns/PageContainer.vue'
import PageHeader from '@design/patterns/PageHeader.vue'
import { useAppToast } from '@design/composables/useAppToast'

import settingsService from '@/services/settingsService'
import { useGroupStore } from '@/stores/groupStore'

const route = useRoute()
const router = useRouter()
const groupStore = useGroupStore()
const toast = useAppToast()

const settingId = computed(() => route.params.id as string)

const loading = ref(true)
const error = ref<string | null>(null)
const isSubmitting = ref(false)

const themeOptions = [
  { id: 'light', name: 'Light' },
  { id: 'dark', name: 'Dark' },
  { id: 'system', name: 'System' },
]

const dateFormatOptions = [
  { id: 'YYYY-MM-DD', name: 'YYYY-MM-DD' },
  { id: 'MM/DD/YYYY', name: 'MM/DD/YYYY' },
  { id: 'DD/MM/YYYY', name: 'DD/MM/YYYY' },
  { id: 'DD.MM.YYYY', name: 'DD.MM.YYYY' },
]

const uiConfig = ref({
  theme: 'light',
  show_debug_info: false,
  default_page_size: 20,
  default_date_format: 'YYYY-MM-DD',
})

const formErrors = ref({
  theme: '',
  default_date_format: '',
})

const settingTitle = computed(() => {
  switch (settingId.value) {
    case 'ui_config':
      return 'User Interface Settings'
    default:
      return formatSettingName(settingId.value)
  }
})

const backToSystemHref = computed(() => groupStore.groupPath('/system'))

onMounted(async () => {
  await loadSetting()
})

async function loadSetting() {
  loading.value = true
  error.value = null
  try {
    const response = await settingsService.getSettings()
    const settings = response.data
    if (settingId.value === 'ui_config') {
      uiConfig.value = {
        theme: settings.Theme || 'light',
        show_debug_info: settings.ShowDebugInfo || false,
        default_page_size: 20,
        default_date_format: settings.DefaultDateFormat || 'YYYY-MM-DD',
      }
    }
  } catch (err: any) {
    error.value = 'Failed to load settings: ' + (err.message || 'Unknown error')
  } finally {
    loading.value = false
  }
}

function goBack() {
  router.push(backToSystemHref.value)
}

async function saveUIConfig() {
  formErrors.value.theme = ''
  formErrors.value.default_date_format = ''

  let isValid = true
  if (!uiConfig.value.theme) {
    formErrors.value.theme = 'Theme is required'
    isValid = false
  }
  if (!uiConfig.value.default_date_format) {
    formErrors.value.default_date_format = 'Date Format is required'
    isValid = false
  }
  if (!isValid) return

  isSubmitting.value = true
  try {
    await settingsService.updateTheme(uiConfig.value.theme)
    await settingsService.updateShowDebugInfo(uiConfig.value.show_debug_info)
    await settingsService.updateDefaultDateFormat(uiConfig.value.default_date_format)
    router.push({ path: backToSystemHref.value, query: { success: 'true' } })
  } catch (err: any) {
    toast.error(err?.message ?? 'Failed to save UI config')
  } finally {
    isSubmitting.value = false
  }
}

function formatSettingName(id: string): string {
  return id
    .replace(/[-_]/g, ' ')
    .replace(/\w\S*/g, (txt) => txt.charAt(0).toUpperCase() + txt.substr(1).toLowerCase())
}
</script>

<template>
  <PageContainer width="narrow">
    <PageHeader :title="settingTitle">
      <template #breadcrumbs>
        <router-link
          :to="backToSystemHref"
          class="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground"
        >
          <ArrowLeft class="size-3.5" aria-hidden="true" />
          Back to System
        </router-link>
      </template>
    </PageHeader>

    <div
      v-if="loading"
      class="rounded-md border border-border bg-card p-12 text-center text-muted-foreground shadow-sm"
    >
      Loading setting...
    </div>

    <div
      v-else-if="error"
      class="rounded-md border border-destructive/50 bg-destructive/10 p-12 text-center text-destructive shadow-sm"
    >
      {{ error }}
    </div>

    <form
      v-else-if="settingId === 'ui_config'"
      class="flex flex-col gap-6"
      @submit.prevent="saveUIConfig"
    >
      <FormSection title="Display">
        <FormGrid cols="1">
          <div class="flex flex-col gap-1.5">
            <Label for="theme">Theme</Label>
            <Select v-model="uiConfig.theme">
              <SelectTrigger id="theme" aria-label="Theme">
                <SelectValue placeholder="Select a theme" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem
                  v-for="opt in themeOptions"
                  :key="opt.id"
                  :value="opt.id"
                >
                  {{ opt.name }}
                </SelectItem>
              </SelectContent>
            </Select>
            <p
              v-if="formErrors.theme"
              class="text-sm text-destructive"
              role="alert"
            >
              {{ formErrors.theme }}
            </p>
          </div>

          <div class="flex flex-col gap-1.5">
            <Label for="date-format">Date Format</Label>
            <Select v-model="uiConfig.default_date_format">
              <SelectTrigger id="date-format" aria-label="Date Format">
                <SelectValue placeholder="Select a date format" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem
                  v-for="opt in dateFormatOptions"
                  :key="opt.id"
                  :value="opt.id"
                >
                  {{ opt.name }}
                </SelectItem>
              </SelectContent>
            </Select>
            <p
              v-if="formErrors.default_date_format"
              class="text-sm text-destructive"
              role="alert"
            >
              {{ formErrors.default_date_format }}
            </p>
          </div>
        </FormGrid>
      </FormSection>

      <FormSection title="Behavior">
        <FormGrid cols="1">
          <div class="flex items-center gap-2">
            <Checkbox id="show-debug" v-model="uiConfig.show_debug_info" />
            <Label for="show-debug" class="cursor-pointer">
              Show Debug Information
            </Label>
          </div>

          <div class="flex flex-col gap-1.5">
            <Label for="page-size">Default Page Size</Label>
            <Input
              id="page-size"
              v-model.number="uiConfig.default_page_size"
              type="number"
              min="5"
              max="100"
              class="w-32"
            />
          </div>
        </FormGrid>
      </FormSection>

      <div class="flex justify-end gap-2">
        <Button type="button" variant="outline" @click="goBack">Cancel</Button>
        <Button type="submit" :disabled="isSubmitting">
          <Loader2
            v-if="isSubmitting"
            class="size-4 animate-spin"
            aria-hidden="true"
          />
          {{ isSubmitting ? 'Saving...' : 'Save' }}
        </Button>
      </div>
    </form>
  </PageContainer>
</template>

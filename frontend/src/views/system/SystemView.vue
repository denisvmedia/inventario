<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  ChartLine,
  CheckCircle2,
  ChevronRight,
  GitBranch,
  Server,
} from 'lucide-vue-next'

import Banner from '@design/patterns/Banner.vue'
import PageContainer from '@design/patterns/PageContainer.vue'
import PageHeader from '@design/patterns/PageHeader.vue'
import PageSection from '@design/patterns/PageSection.vue'

import systemService, { type SystemInfo } from '@/services/systemService'
import { useGroupStore } from '@/stores/groupStore'

const router = useRouter()
const route = useRoute()
const groupStore = useGroupStore()

const systemInfo = ref<SystemInfo>({
  version: '',
  commit: '',
  build_date: '',
  go_version: '',
  platform: '',
  database_backend: '',
  file_storage_backend: '',
  operating_system: '',
  uptime: '',
  memory_usage: '',
  num_goroutines: 0,
  num_cpu: 0,
  settings: {},
})
const loading = ref(true)
const error = ref<string | null>(null)
const showSuccessMessage = ref(false)

watch(
  () => route.query.success,
  (newValue) => {
    if (newValue === 'true') {
      showSuccessMessage.value = true
    }
  },
  { immediate: true },
)

function dismissSuccessMessage() {
  showSuccessMessage.value = false
  if (route.query.success) {
    router.replace({ query: { ...route.query, success: undefined } })
  }
}

onMounted(async () => {
  try {
    const response = await systemService.getSystemInfo()
    systemInfo.value = response.data
  } catch (err: any) {
    error.value = 'Failed to load system information: ' + (err.message || 'Unknown error')
  } finally {
    loading.value = false
  }
})

function navigateToSetting(id: string) {
  router.push(groupStore.groupPath(`/system/settings/${id}`))
}

const versionRows = computed(() => [
  { label: 'Version', value: systemInfo.value.version },
  { label: 'Build Date', value: systemInfo.value.build_date },
  { label: 'Commit', value: systemInfo.value.commit },
  { label: 'Go Version', value: systemInfo.value.go_version },
  { label: 'Platform', value: systemInfo.value.platform },
])

const backendRows = computed(() => [
  { label: 'Database', value: systemInfo.value.database_backend },
  { label: 'File Storage', value: systemInfo.value.file_storage_backend },
  { label: 'Operating System', value: systemInfo.value.operating_system },
])

const runtimeRows = computed(() => [
  { label: 'Uptime', value: systemInfo.value.uptime },
  { label: 'Memory Usage', value: systemInfo.value.memory_usage },
  { label: 'Goroutines', value: String(systemInfo.value.num_goroutines) },
  { label: 'CPU Cores', value: String(systemInfo.value.num_cpu) },
])
</script>

<template>
  <PageContainer>
    <PageHeader title="System" />

    <Banner
      v-if="showSuccessMessage"
      variant="success"
      dismissible
      class="mb-6"
      @dismiss="dismissSuccessMessage"
    >
      Settings updated successfully!
    </Banner>

    <div
      v-if="loading"
      class="rounded-md border border-border bg-card p-12 text-center text-muted-foreground shadow-sm"
    >
      Loading system information...
    </div>

    <div
      v-else-if="error"
      class="rounded-md border border-destructive/50 bg-destructive/10 p-12 text-center text-destructive shadow-sm"
    >
      {{ error }}
    </div>

    <div v-else class="flex flex-col gap-10">
      <PageSection title="System Information" as="h2">
        <div class="grid grid-cols-1 gap-4 lg:grid-cols-3">
          <article class="rounded-md border border-border bg-card p-6 shadow-sm">
            <header class="mb-4 flex items-center gap-2 border-b border-border pb-2">
              <GitBranch class="size-4 text-muted-foreground" aria-hidden="true" />
              <h3 class="text-base font-semibold text-foreground">Version Information</h3>
            </header>
            <table class="w-full text-sm">
              <tbody>
                <tr v-for="row in versionRows" :key="row.label" class="align-baseline">
                  <th class="py-1 pr-3 text-left font-medium text-muted-foreground">
                    {{ row.label }}
                  </th>
                  <td class="py-1 text-right">
                    <code class="rounded bg-muted px-2 py-0.5 text-xs text-foreground">
                      {{ row.value || '—' }}
                    </code>
                  </td>
                </tr>
              </tbody>
            </table>
          </article>

          <article class="rounded-md border border-border bg-card p-6 shadow-sm">
            <header class="mb-4 flex items-center gap-2 border-b border-border pb-2">
              <Server class="size-4 text-muted-foreground" aria-hidden="true" />
              <h3 class="text-base font-semibold text-foreground">System Backend</h3>
            </header>
            <table class="w-full text-sm">
              <tbody>
                <tr v-for="row in backendRows" :key="row.label" class="align-baseline">
                  <th class="py-1 pr-3 text-left font-medium text-muted-foreground">
                    {{ row.label }}
                  </th>
                  <td class="py-1 text-right">
                    <code class="rounded bg-muted px-2 py-0.5 text-xs text-foreground">
                      {{ row.value || '—' }}
                    </code>
                  </td>
                </tr>
              </tbody>
            </table>
          </article>

          <article class="rounded-md border border-border bg-card p-6 shadow-sm">
            <header class="mb-4 flex items-center gap-2 border-b border-border pb-2">
              <ChartLine class="size-4 text-muted-foreground" aria-hidden="true" />
              <h3 class="text-base font-semibold text-foreground">Runtime Metrics</h3>
            </header>
            <table class="w-full text-sm">
              <tbody>
                <tr v-for="row in runtimeRows" :key="row.label" class="align-baseline">
                  <th class="py-1 pr-3 text-left font-medium text-muted-foreground">
                    {{ row.label }}
                  </th>
                  <td class="py-1 text-right">
                    <code class="rounded bg-muted px-2 py-0.5 text-xs text-foreground">
                      {{ row.value || '—' }}
                    </code>
                  </td>
                </tr>
              </tbody>
            </table>
          </article>
        </div>
      </PageSection>

      <PageSection title="Settings" as="h2">
        <div class="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
          <button
            type="button"
            class="group flex items-start justify-between gap-3 rounded-md border border-border bg-card p-6 text-left shadow-sm motion-safe:transition-shadow hover:shadow-md focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
            @click="navigateToSetting('ui_config')"
          >
            <div class="min-w-0 flex-1">
              <h4 class="text-base font-semibold text-foreground">UI Settings</h4>
              <p class="mt-1 text-sm text-muted-foreground">
                Configure theme, language, and display options.
              </p>
              <dl
                v-if="!loading && systemInfo.settings.Theme"
                class="mt-3 flex flex-col gap-1 text-sm"
              >
                <div class="flex items-center justify-between">
                  <dt class="text-muted-foreground">Theme</dt>
                  <dd class="font-medium text-foreground">{{ systemInfo.settings.Theme }}</dd>
                </div>
                <div class="flex items-center justify-between">
                  <dt class="text-muted-foreground">Show Debug Info</dt>
                  <dd class="font-medium text-foreground">
                    <CheckCircle2
                      v-if="systemInfo.settings.ShowDebugInfo"
                      class="size-4 text-success"
                      aria-hidden="true"
                    />
                    <span v-else>No</span>
                  </dd>
                </div>
                <div
                  v-if="systemInfo.settings.DefaultDateFormat"
                  class="flex items-center justify-between"
                >
                  <dt class="text-muted-foreground">Date Format</dt>
                  <dd class="font-medium text-foreground">
                    {{ systemInfo.settings.DefaultDateFormat }}
                  </dd>
                </div>
              </dl>
            </div>
            <ChevronRight
              class="size-4 shrink-0 text-muted-foreground motion-safe:transition-transform group-hover:translate-x-0.5"
              aria-hidden="true"
            />
          </button>
        </div>
      </PageSection>
    </div>
  </PageContainer>
</template>

<script setup lang="ts">
/**
 * AreaListView — migrated to the design system in Phase 4 of Epic
 * #1324 (issue #1329).
 *
 * Areas are still created inline on a location's detail page (#1321),
 * so the "New" CTA points at the locations list rather than a
 * dedicated `/areas/new` route. The legacy `.area-list` and
 * `.areas-grid` / `.area-card` class anchors are preserved as no-op
 * markers so existing Playwright selectors keep resolving through the
 * strangler-fig window — see devdocs/frontend/migration-conventions.md.
 */
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Plus } from 'lucide-vue-next'

import areaService from '@/services/areaService'
import locationService from '@/services/locationService'
import { fetchAll } from '@/utils/paginationUtils'
import { useGroupStore } from '@/stores/groupStore'
import { getErrorMessage } from '@/utils/errorUtils'

import { Button } from '@design/ui/button'
import AreaCard from '@design/patterns/AreaCard.vue'
import EmptyState from '@design/patterns/EmptyState.vue'
import PageContainer from '@design/patterns/PageContainer.vue'
import PageHeader from '@design/patterns/PageHeader.vue'
import { useAppToast } from '@design/composables/useAppToast'
import { useConfirm } from '@design/composables/useConfirm'

import PaginationControls from '@/components/PaginationControls.vue'

type AnyRecord = Record<string, unknown>
type ApiResource = { id: string; attributes: AnyRecord }

const route = useRoute()
const router = useRouter()
const groupStore = useGroupStore()
const toast = useAppToast()
const { confirmDelete } = useConfirm()

const areas = ref<ApiResource[]>([])
const locations = ref<ApiResource[]>([])
const loading = ref<boolean>(true)

const currentPage = ref(1)
const pageSize = ref(50)
const totalAreas = ref(0)
const totalPages = computed(() => Math.ceil(totalAreas.value / pageSize.value))

const locationsListPath = computed(() => groupStore.groupPath('/locations'))

async function loadAreas() {
  loading.value = true
  try {
    const [areasResponse, allLocations] = await Promise.all([
      areaService.getAreas({ page: currentPage.value, per_page: pageSize.value }),
      fetchAll((params) => locationService.getLocations(params)),
    ])
    areas.value = areasResponse.data.data
    totalAreas.value = areasResponse.data.meta.areas
    locations.value = allLocations
  } catch (err) {
    toast.error(getErrorMessage(err as never, 'area', 'Failed to load areas'))
  } finally {
    loading.value = false
  }
}

onMounted(async () => {
  currentPage.value = Number(route.query.page) || 1
  await loadAreas()
})

watch(
  () => route.query.page,
  (newPage) => {
    currentPage.value = Number(newPage) || 1
    loadAreas()
  },
)

function getLocationName(locationId: string): string {
  const location = locations.value.find((l) => l.id === locationId)
  return location ? ((location.attributes as AnyRecord).name as string) : 'Unknown Location'
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
    await loadAreas()
  } catch (err) {
    toast.error(getErrorMessage(err as never, 'area', 'Failed to delete area'))
  }
}

function goToLocations() {
  router.push(locationsListPath.value)
}
</script>

<template>
  <PageContainer as="div" class="area-list">
    <PageHeader title="Areas">
      <template #actions>
        <Button class="new-area-button" @click="goToLocations">
          <Plus class="size-4" aria-hidden="true" />
          New
        </Button>
      </template>
    </PageHeader>

    <div v-if="loading" class="py-12 text-center text-sm text-muted-foreground">Loading...</div>

    <EmptyState
      v-else-if="areas.length === 0"
      title="No areas yet"
      description="No areas found. Pick a location and add your first area from there."
    >
      <template #actions>
        <Button @click="goToLocations">
          <Plus class="size-4" aria-hidden="true" />
          Create Area
        </Button>
      </template>
    </EmptyState>

    <div v-else class="flex flex-col gap-6">
      <div class="areas-grid grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-3">
        <AreaCard
          v-for="area in areas"
          :key="area.id"
          :area="(area as never)"
          :subtitle="
            (area.attributes.location_id as string)
              ? `Location: ${getLocationName(area.attributes.location_id as string)}`
              : ''
          "
          @view="viewArea"
          @edit="editArea"
          @delete="onDeleteArea"
        />
      </div>

      <PaginationControls
        :current-page="currentPage"
        :total-pages="totalPages"
        :page-size="pageSize"
        :total-items="totalAreas"
        item-label="areas"
      />
    </div>
  </PageContainer>
</template>

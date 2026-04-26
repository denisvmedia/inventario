<template>
  <div v-if="totalPages > 1" class="pagination-card">
    <div class="pagination-info">
      Showing {{ (currentPage - 1) * pageSize + 1 }} to {{ Math.min(currentPage * pageSize, totalItems) }} of
      {{ totalItems }} {{ itemLabel }}
    </div>
    <div class="pagination-controls">
      <Button
        v-if="currentPage > 1"
        as-child
        variant="outline"
        size="sm"
        class="pagination-link"
      >
        <router-link :to="getPaginationUrl(currentPage - 1)">
          <ChevronLeft class="size-4" aria-hidden="true" />
          Previous
        </router-link>
      </Button>
      <Button
        v-else
        variant="outline"
        size="sm"
        class="pagination-link"
        disabled
      >
        <ChevronLeft class="size-4" aria-hidden="true" />
        Previous
      </Button>

      <div class="page-numbers">
        <Button
          v-for="page in visiblePages"
          :key="page"
          as-child
          :variant="page === currentPage ? 'default' : 'outline'"
          size="sm"
          class="pagination-link"
        >
          <router-link :to="getPaginationUrl(page)">
            {{ page }}
          </router-link>
        </Button>
      </div>

      <Button
        v-if="currentPage < totalPages"
        as-child
        variant="outline"
        size="sm"
        class="pagination-link"
      >
        <router-link :to="getPaginationUrl(currentPage + 1)">
          Next
          <ChevronRight class="size-4" aria-hidden="true" />
        </router-link>
      </Button>
      <Button
        v-else
        variant="outline"
        size="sm"
        class="pagination-link"
        disabled
      >
        Next
        <ChevronRight class="size-4" aria-hidden="true" />
      </Button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { ChevronLeft, ChevronRight } from 'lucide-vue-next'

import { Button } from '@design/ui/button'

const props = defineProps<{
  /** The current active page number (1-based). */
  currentPage: number
  /** Total number of pages. */
  totalPages: number
  /** Number of items per page. */
  pageSize: number
  /** Total number of items across all pages. */
  totalItems: number
  /** Label for items displayed in the info line, e.g. "commodities". */
  itemLabel: string
}>()

const route = useRoute()

/** Window of page numbers centered around the current page (±2). */
const visiblePages = computed(() => {
  const pages: number[] = []
  const start = Math.max(1, props.currentPage - 2)
  const end = Math.min(props.totalPages, props.currentPage + 2)
  for (let i = start; i <= end; i++) pages.push(i)
  return pages
})

/** Builds a route location for the given page, preserving other query params. */
const getPaginationUrl = (page: number) => {
  const query = { ...route.query }
  if (page > 1) {
    query.page = page.toString()
  } else {
    delete query.page
  }
  return { path: route.path, query }
}
</script>

<style scoped>
.pagination-card {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 1rem;
  padding: 1rem;
  background: hsl(var(--card));
  border-radius: 0.375rem;
  box-shadow: 0 2px 8px rgb(0 0 0 / 10%);
}

.pagination-info {
  font-size: 0.9rem;
  color: hsl(var(--foreground));
}

.pagination-controls {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  flex-wrap: wrap;
  justify-content: center;
}

.page-numbers {
  display: flex;
  gap: 0.25rem;
}

.pagination-link {
  min-width: 2.5rem;
}
</style>

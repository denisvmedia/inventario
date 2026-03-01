<template>
  <div v-if="totalPages > 1" class="pagination-card">
    <div class="pagination-info">
      Showing {{ (currentPage - 1) * pageSize + 1 }} to {{ Math.min(currentPage * pageSize, totalItems) }} of
      {{ totalItems }} {{ itemLabel }}
    </div>
    <div class="pagination-controls">
      <router-link
        v-if="currentPage > 1"
        :to="getPaginationUrl(currentPage - 1)"
        class="btn btn-secondary pagination-link"
      >
        <font-awesome-icon icon="chevron-left" />
        Previous
      </router-link>
      <span v-else class="btn btn-secondary pagination-link disabled">
        <font-awesome-icon icon="chevron-left" />
        Previous
      </span>

      <div class="page-numbers">
        <router-link
          v-for="page in visiblePages"
          :key="page"
          :to="getPaginationUrl(page)"
          class="btn pagination-link"
          :class="{ 'btn-primary': page === currentPage, 'btn-secondary': page !== currentPage }"
        >
          {{ page }}
        </router-link>
      </div>

      <router-link
        v-if="currentPage < totalPages"
        :to="getPaginationUrl(currentPage + 1)"
        class="btn btn-secondary pagination-link"
      >
        Next
        <font-awesome-icon icon="chevron-right" />
      </router-link>
      <span v-else class="btn btn-secondary pagination-link disabled">
        Next
        <font-awesome-icon icon="chevron-right" />
      </span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'

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

<style lang="scss" scoped>
@use '@/assets/main' as *;

.pagination-card {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 1rem;
  padding: 1rem;
  background: white;
  border-radius: $default-radius;
  box-shadow: $box-shadow;
}

.pagination-info {
  font-size: 0.9rem;
  color: $text-color;
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
  text-align: center;

  &.disabled {
    opacity: 0.5;
    pointer-events: none;
  }
}
</style>


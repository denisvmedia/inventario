<script setup lang="ts">
/**
 * AppFooter — site-wide footer rendered below the router-view.
 *
 * Extracted from App.vue as part of the Phase 1 layout shell migration
 * (#1326). Pure presentational: no stores, no router. Only the
 * copyright line lives here today; later phases will add the link
 * cluster (privacy / terms) once we have copy for it.
 */
import { computed } from 'vue'

const year = computed(() => new Date().getFullYear())
</script>

<template>
  <footer data-testid="app-footer">
    <p>Inventario &copy; {{ year }}</p>
  </footer>
</template>

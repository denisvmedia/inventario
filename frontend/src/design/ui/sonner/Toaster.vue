<script setup lang="ts">
import { Toaster as Sonner, type ToasterProps } from 'vue-sonner'
import 'vue-sonner/style.css'

/**
 * Global toast host. Mounted once in App.vue alongside the legacy
 * PrimeVue <Toast /> during the strangler-fig migration.
 *
 * Defaults mirror the shadcn-vue sonner recipe: top-right placement,
 * rich colours, close button, and theming via our CSS variables so
 * dark mode flips automatically with the rest of the design tokens.
 */
const props = withDefaults(defineProps<ToasterProps>(), {
  position: 'top-right',
  richColors: true,
  closeButton: true,
})
</script>

<template>
  <Sonner
    class="toaster group"
    v-bind="props"
    :toast-options="{
      classes: {
        toast:
          'group toast group-[.toaster]:bg-background group-[.toaster]:text-foreground group-[.toaster]:border-border group-[.toaster]:shadow-lg',
        description: 'group-[.toast]:text-muted-foreground',
        actionButton: 'group-[.toast]:bg-primary group-[.toast]:text-primary-foreground',
        cancelButton: 'group-[.toast]:bg-muted group-[.toast]:text-muted-foreground',
      },
    }"
  />
</template>

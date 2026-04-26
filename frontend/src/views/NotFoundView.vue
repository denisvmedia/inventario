<template>
  <div class="not-found mx-auto max-w-3xl rounded-md border bg-card p-6 text-center shadow-sm">
    <h1 class="text-destructive text-2xl font-semibold mb-3">404 - Page Not Found</h1>
    <p>The page you are looking for does not exist.</p>
    <p class="text-sm text-muted-foreground">Current path: {{ $route.path }}</p>
    <Button class="mt-5" @click="goHome">Go Home</Button>
  </div>
</template>

<script setup lang="ts">
import { useRouter } from 'vue-router'
import { Button } from '@design/ui/button'

const router = useRouter()

function goHome() {
  router.push('/')
}
</script>

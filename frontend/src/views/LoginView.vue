<template>
  <LoginForm />
</template>

<script setup lang="ts">
import LoginForm from '../components/LoginForm.vue'
import { onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useAuthStore } from '../stores/authStore'

const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()

// Redirect if already authenticated
onMounted(() => {
  if (authStore.isAuthenticated) {
    const redirectTo = route.query.redirect as string || '/'
    console.log('Already authenticated in LoginView, redirecting to:', redirectTo)
    router.replace(redirectTo)
  }
})
</script>

<style scoped>
/* No additional styles needed - LoginForm handles all styling */
</style>

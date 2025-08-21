<template>
  <LoginForm />
</template>

<script setup lang="ts">
import LoginForm from '../components/LoginForm.vue'
import { onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '../stores/authStore'

const router = useRouter()
const authStore = useAuthStore()

// Redirect if already authenticated
onMounted(() => {
  if (authStore.isAuthenticated) {
    router.push('/')
  }
})
</script>

<style scoped>
/* No additional styles needed - LoginForm handles all styling */
</style>

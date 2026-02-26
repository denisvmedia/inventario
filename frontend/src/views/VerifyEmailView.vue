<template>
  <div class="verify-email">
    <div class="verify-card">
      <div class="verify-header">
        <h1>Inventario</h1>
        <p>Email Verification</p>
      </div>

      <div v-if="isLoading" class="status-message loading">
        <p>Verifying your email...</p>
      </div>

      <div v-else-if="success" class="status-message success">
        <p>{{ message }}</p>
        <p>
          <RouterLink to="/login" class="action-link">Sign in to your account</RouterLink>
        </p>
      </div>

      <div v-else-if="error" class="status-message error">
        <p>{{ error }}</p>
        <p>
          <RouterLink to="/login" class="action-link">Back to sign in</RouterLink>
        </p>
      </div>

      <div v-else class="status-message missing">
        <p>No verification token provided.</p>
        <p>
          <RouterLink to="/login" class="action-link">Back to sign in</RouterLink>
        </p>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { RouterLink, useRoute } from 'vue-router'
import authService from '../services/authService'

const route = useRoute()
const isLoading = ref(false)
const success = ref(false)
const message = ref('')
const error = ref<string | null>(null)

onMounted(async () => {
  const token = route.query.token as string | undefined
  if (!token) return

  isLoading.value = true
  try {
    const res = await authService.verifyEmail(token)
    message.value = res.message
    success.value = true
  } catch (err: unknown) {
    const e = err as { response?: { data?: string } }
    const data = e.response?.data
    error.value = (typeof data === 'string' && data.trim())
      ? data.trim()
      : 'Verification failed. The link may be invalid or expired.'
  } finally {
    isLoading.value = false
  }
})
</script>

<style scoped>
.verify-email {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  padding: 1rem;
}

.verify-card {
  background: white;
  border-radius: 8px;
  box-shadow: 0 10px 25px rgb(0 0 0 / 10%);
  padding: 2rem;
  width: 100%;
  max-width: 400px;
}

.verify-header {
  text-align: center;
  margin-bottom: 2rem;
}

.verify-header h1 {
  color: #333;
  margin: 0 0 0.5rem;
  font-size: 2rem;
  font-weight: 600;
}

.verify-header p {
  color: #666;
  margin: 0;
  font-size: 1rem;
}

.status-message {
  padding: 1rem;
  border-radius: 4px;
  text-align: center;
  font-size: 0.95rem;
}

.status-message.loading {
  background-color: #f0f4ff;
  color: #445;
  border: 1px solid #c8d8ff;
}

.status-message.success {
  background-color: #efe;
  color: #363;
  border: 1px solid #cfc;
}

.status-message.error {
  background-color: #fee;
  color: #c33;
  border: 1px solid #fcc;
}

.status-message.missing {
  background-color: #fff8e1;
  color: #7a5c00;
  border: 1px solid #ffe082;
}

.action-link {
  color: #667eea;
  font-weight: 500;
  text-decoration: none;
}

.action-link:hover {
  text-decoration: underline;
}
</style>


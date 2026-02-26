<template>
  <div class="login-form">
    <div class="login-card">
      <div class="login-header">
        <h1>Inventario</h1>
        <p>Sign in to your account</p>
      </div>

      <form class="login-form-content" @submit.prevent="handleSubmit">
        <div class="form-group">
          <label for="email">Email</label>
          <input
            id="email"
            v-model="form.email"
            type="email"
            required
            :disabled="isLoading"
            data-testid="email"
            placeholder="Enter your email"
          />
        </div>

        <div class="form-group">
          <label for="password">Password</label>
          <input
            id="password"
            v-model="form.password"
            type="password"
            required
            :disabled="isLoading"
            data-testid="password"
            placeholder="Enter your password"
          />
        </div>

        <div v-if="error" class="error-message">
          {{ error }}
        </div>

        <button
          type="submit"
          :disabled="isLoading || !isFormValid"
          data-testid="login-button"
          class="login-button"
        >
          <span v-if="isLoading">Signing in...</span>
          <span v-else>Sign In</span>
        </button>

        <p class="register-link">
          Don't have an account?
          <RouterLink to="/register">Create one</RouterLink>
        </p>
      </form>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import { RouterLink, useRouter, useRoute } from 'vue-router'
import { useAuthStore } from '../stores/authStore'

const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()

// Form data
const form = ref({
  email: '',
  password: ''
})

// Computed properties
const isLoading = computed(() => authStore.isLoading)
const error = computed(() => authStore.error)
const isFormValid = computed(() => {
  return form.value.email.trim() !== '' && form.value.password.trim() !== ''
})

// Methods
async function handleSubmit() {
  if (!isFormValid.value) return

  try {
    await authStore.login({
      email: form.value.email.trim(),
      password: form.value.password
    })

    // Handle redirect query parameter or default to home
    const redirectTo = route.query.redirect as string || '/'
    console.log('Login successful, redirecting to:', redirectTo)

    // Use replace instead of push to avoid login page in history
    await router.replace(redirectTo)
  } catch (error) {
    // Error is handled by the store
    console.error('Login failed:', error)
  }
}

// Auto-fill for development/testing
function fillTestCredentials() {
  form.value.email = 'admin@example.com'
  form.value.password = 'admin123'
}

// Expose for testing
defineExpose({
  fillTestCredentials
})
</script>

<style scoped>
.login-form {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  padding: 1rem;
}

.login-card {
  background: white;
  border-radius: 8px;
  box-shadow: 0 10px 25px rgb(0 0 0 / 10%);
  padding: 2rem;
  width: 100%;
  max-width: 400px;
}

.login-header {
  text-align: center;
  margin-bottom: 2rem;
}

.login-header h1 {
  color: #333;
  margin: 0 0 0.5rem;
  font-size: 2rem;
  font-weight: 600;
}

.login-header p {
  color: #666;
  margin: 0;
  font-size: 1rem;
}

.login-form-content {
  display: flex;
  flex-direction: column;
  gap: 1.5rem;
}

.form-group {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}

.form-group label {
  font-weight: 500;
  color: #333;
  font-size: 0.9rem;
}

.form-group input {
  padding: 0.75rem;
  border: 1px solid #ddd;
  border-radius: 4px;
  font-size: 1rem;
  transition: border-color 0.2s;
}

.form-group input:focus {
  outline: none;
  border-color: #667eea;
  box-shadow: 0 0 0 3px rgb(102 126 234 / 10%);
}

.form-group input:disabled {
  background-color: #f5f5f5;
  cursor: not-allowed;
}

.error-message {
  background-color: #fee;
  color: #c33;
  padding: 0.75rem;
  border-radius: 4px;
  border: 1px solid #fcc;
  font-size: 0.9rem;
}

.login-button {
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  color: white;
  border: none;
  padding: 0.75rem 1.5rem;
  border-radius: 4px;
  font-size: 1rem;
  font-weight: 500;
  cursor: pointer;
  transition: opacity 0.2s;
}

.login-button:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.login-button:hover:not(:disabled) {
  opacity: 0.9;
}

.register-link {
  text-align: center;
  color: #666;
  font-size: 0.9rem;
  margin: 0;
}
</style>

<template>
  <div class="reset-password-form">
    <div class="reset-password-card">
      <div class="reset-password-header">
        <h1>Inventario</h1>
        <p>Set a new password</p>
      </div>

      <div v-if="!token" class="error-message">
        <p>Invalid or missing reset token. Please request a new password reset link.</p>
        <p><RouterLink to="/forgot-password">Request reset link</RouterLink></p>
      </div>

      <div v-else-if="submitted" class="success-message">
        <p>{{ successMessage }}</p>
        <p><RouterLink to="/login">Sign in with your new password</RouterLink></p>
      </div>

      <form v-else class="reset-password-form-content" @submit.prevent="handleSubmit">
        <div class="form-group">
          <label for="password">New Password</label>
          <input
            id="password"
            v-model="password"
            type="password"
            required
            :disabled="isLoading"
            data-testid="password"
            placeholder="At least 8 characters"
            autocomplete="new-password"
          />
        </div>

        <div class="form-group">
          <label for="confirm-password">Confirm New Password</label>
          <input
            id="confirm-password"
            v-model="confirmPassword"
            type="password"
            required
            :disabled="isLoading"
            data-testid="confirm-password"
            placeholder="Repeat your new password"
            autocomplete="new-password"
          />
        </div>

        <div v-if="error" class="error-message">
          {{ error }}
        </div>

        <button
          type="submit"
          :disabled="isLoading || !isFormValid"
          data-testid="submit-button"
          class="submit-button"
        >
          <span v-if="isLoading">Resetting...</span>
          <span v-else>Reset Password</span>
        </button>

        <p class="login-link">
          <RouterLink to="/login">Back to sign in</RouterLink>
        </p>
      </form>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import authService from '../services/authService'

const route = useRoute()
const router = useRouter()

const token = ref('')
const password = ref('')
const confirmPassword = ref('')
const isLoading = ref(false)
const error = ref<string | null>(null)
const submitted = ref(false)
const successMessage = ref('')

const isFormValid = computed(() =>
  password.value.length >= 8 && password.value === confirmPassword.value
)

onMounted(() => {
  token.value = (route.query.token as string) || ''
})

async function handleSubmit() {
  if (!isFormValid.value) return
  if (password.value !== confirmPassword.value) {
    error.value = 'Passwords do not match.'
    return
  }
  isLoading.value = true
  error.value = null
  try {
    const res = await authService.resetPassword(token.value, password.value)
    successMessage.value = res.message
    submitted.value = true
    // Redirect to login after a short delay
    setTimeout(() => router.replace('/login'), 3000)
  } catch (err: unknown) {
    const e = err as { response?: { data?: string | { error?: string } } }
    const data = e.response?.data
    if (typeof data === 'string') {
      error.value = data.trim() || 'Failed to reset password. Please try again.'
    } else {
      error.value = 'Failed to reset password. The link may have expired. Please request a new one.'
    }
  } finally {
    isLoading.value = false
  }
}
</script>

<style scoped>
.reset-password-form {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  padding: 1rem;
}

.reset-password-card {
  background: white;
  border-radius: 8px;
  box-shadow: 0 10px 25px rgb(0 0 0 / 10%);
  padding: 2rem;
  width: 100%;
  max-width: 400px;
}

.reset-password-header {
  text-align: center;
  margin-bottom: 2rem;
}

.reset-password-header h1 {
  color: #333;
  margin: 0 0 0.5rem;
  font-size: 2rem;
  font-weight: 600;
}

.reset-password-header p {
  color: #666;
  margin: 0;
  font-size: 1rem;
}

.reset-password-form-content {
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

.success-message {
  background-color: #efe;
  color: #363;
  padding: 1rem;
  border-radius: 4px;
  border: 1px solid #cfc;
  font-size: 0.9rem;
  text-align: center;
}

.submit-button {
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

.submit-button:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.submit-button:hover:not(:disabled) {
  opacity: 0.9;
}

.login-link {
  text-align: center;
  color: #666;
  font-size: 0.9rem;
  margin: 0;
}
</style>


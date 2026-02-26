<template>
  <div class="forgot-password-form">
    <div class="forgot-password-card">
      <div class="forgot-password-header">
        <h1>Inventario</h1>
        <p>Reset your password</p>
      </div>

      <div v-if="submitted" class="success-message">
        <p>{{ successMessage }}</p>
        <p>
          <RouterLink to="/login">Back to sign in</RouterLink>
        </p>
      </div>

      <form v-else class="forgot-password-form-content" @submit.prevent="handleSubmit">
        <p class="instructions">
          Enter your email address and we'll send you a link to reset your password.
        </p>

        <div class="form-group">
          <label for="email">Email</label>
          <input
            id="email"
            v-model="email"
            type="email"
            required
            :disabled="isLoading"
            data-testid="email"
            placeholder="Enter your email"
            autocomplete="email"
          />
        </div>

        <div v-if="error" class="error-message">
          {{ error }}
        </div>

        <button
          type="submit"
          :disabled="isLoading || !email.trim()"
          data-testid="submit-button"
          class="submit-button"
        >
          <span v-if="isLoading">Sending...</span>
          <span v-else>Send Reset Link</span>
        </button>

        <p class="login-link">
          <RouterLink to="/login">Back to sign in</RouterLink>
        </p>
      </form>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { RouterLink } from 'vue-router'
import authService from '../services/authService'

const email = ref('')
const isLoading = ref(false)
const error = ref<string | null>(null)
const submitted = ref(false)
const successMessage = ref('')

async function handleSubmit() {
  if (!email.value.trim()) return
  isLoading.value = true
  error.value = null
  try {
    const res = await authService.forgotPassword(email.value.trim())
    successMessage.value = res.message
    submitted.value = true
  } catch (err: unknown) {
    const e = err as { response?: { data?: string | { error?: string } } }
    const data = e.response?.data
    if (typeof data === 'string') {
      error.value = data.trim() || 'Failed to send reset link. Please try again.'
    } else {
      error.value = 'Failed to send reset link. Please try again.'
    }
  } finally {
    isLoading.value = false
  }
}
</script>

<style scoped>
.forgot-password-form {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  padding: 1rem;
}

.forgot-password-card {
  background: white;
  border-radius: 8px;
  box-shadow: 0 10px 25px rgb(0 0 0 / 10%);
  padding: 2rem;
  width: 100%;
  max-width: 400px;
}

.forgot-password-header {
  text-align: center;
  margin-bottom: 2rem;
}

.forgot-password-header h1 {
  color: #333;
  margin: 0 0 0.5rem;
  font-size: 2rem;
  font-weight: 600;
}

.forgot-password-header p {
  color: #666;
  margin: 0;
  font-size: 1rem;
}

.forgot-password-form-content {
  display: flex;
  flex-direction: column;
  gap: 1.5rem;
}

.instructions {
  color: #555;
  font-size: 0.95rem;
  margin: 0;
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

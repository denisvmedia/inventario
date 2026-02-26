<template>
  <div class="register-form">
    <div class="register-card">
      <div class="register-header">
        <h1>Inventario</h1>
        <p>Create a new account</p>
      </div>

      <div v-if="submitted" class="success-message">
        <p>{{ successMessage }}</p>
        <p>
          <RouterLink to="/login">Back to sign in</RouterLink>
        </p>
      </div>

      <form v-else class="register-form-content" @submit.prevent="handleSubmit">
        <div class="form-group">
          <label for="name">Full Name</label>
          <input
            id="name"
            v-model="form.name"
            type="text"
            required
            :disabled="isLoading"
            data-testid="name"
            placeholder="Enter your full name"
          />
        </div>

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
            placeholder="At least 8 characters"
          />
        </div>

        <div v-if="error" class="error-message">
          {{ error }}
        </div>

        <button
          type="submit"
          :disabled="isLoading || !isFormValid"
          data-testid="register-button"
          class="register-button"
        >
          <span v-if="isLoading">Creating account...</span>
          <span v-else>Create Account</span>
        </button>

        <p class="login-link">
          Already have an account?
          <RouterLink to="/login">Sign in</RouterLink>
        </p>
      </form>
    </div>
  </div>
</template>



<script setup lang="ts">
import { ref, computed } from 'vue'
import { RouterLink } from 'vue-router'
import authService from '../services/authService'

const form = ref({ name: '', email: '', password: '' })
const isLoading = ref(false)
const error = ref<string | null>(null)
const submitted = ref(false)
const successMessage = ref('')

const isFormValid = computed(() =>
  form.value.name.trim() !== '' &&
  form.value.email.trim() !== '' &&
  form.value.password.trim() !== ''
)

async function handleSubmit() {
  if (!isFormValid.value) return
  isLoading.value = true
  error.value = null
  try {
    const res = await authService.register({
      name: form.value.name.trim(),
      email: form.value.email.trim(),
      password: form.value.password
    })
    successMessage.value = res.message
    submitted.value = true
  } catch (err: unknown) {
    const e = err as { response?: { data?: string | { error?: string } } }
    const data = e.response?.data
    if (typeof data === 'string') {
      error.value = data.trim() || 'Registration failed. Please try again.'
    } else {
      error.value = 'Registration failed. Please try again.'
    }
  } finally {
    isLoading.value = false
  }
}
</script>

<style scoped>
.register-form {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  padding: 1rem;
}

.register-card {
  background: white;
  border-radius: 8px;
  box-shadow: 0 10px 25px rgb(0 0 0 / 10%);
  padding: 2rem;
  width: 100%;
  max-width: 400px;
}

.register-header { text-align: center; margin-bottom: 2rem; }
.register-header h1 { color: #333; margin: 0 0 0.5rem; font-size: 2rem; font-weight: 600; }
.register-header p { color: #666; margin: 0; font-size: 1rem; }

.register-form-content { display: flex; flex-direction: column; gap: 1.5rem; }
.form-group { display: flex; flex-direction: column; gap: 0.5rem; }
.form-group label { font-weight: 500; color: #333; font-size: 0.9rem; }
.form-group input {
  padding: 0.75rem; border: 1px solid #ddd; border-radius: 4px;
  font-size: 1rem; transition: border-color 0.2s;
}
.form-group input:focus { outline: none; border-color: #667eea; box-shadow: 0 0 0 3px rgb(102 126 234 / 10%); }
.form-group input:disabled { background-color: #f5f5f5; cursor: not-allowed; }

.error-message {
  background-color: #fee; color: #c33; padding: 0.75rem;
  border-radius: 4px; border: 1px solid #fcc; font-size: 0.9rem;
}

.success-message {
  background-color: #efe; color: #363; padding: 1rem;
  border-radius: 4px; border: 1px solid #cfc; font-size: 0.9rem; text-align: center;
}

.register-button {
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  color: white; border: none; padding: 0.75rem 1.5rem;
  border-radius: 4px; font-size: 1rem; font-weight: 500;
  cursor: pointer; transition: opacity 0.2s;
}
.register-button:disabled { opacity: 0.6; cursor: not-allowed; }
.register-button:hover:not(:disabled) { opacity: 0.9; }

.login-link { text-align: center; color: #666; font-size: 0.9rem; margin: 0; }
</style>

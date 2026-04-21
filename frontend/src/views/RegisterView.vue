<template>
  <div class="register-form">
    <div class="register-card">
      <div class="register-header">
        <h1>Inventario</h1>
        <p>Create a new account</p>
      </div>

      <InviteBanner
        v-if="pendingInvite"
        :group-name="pendingInvite.groupName"
        prefix="You're registering to join"
      />

      <div v-if="submitted" class="success-message">
        <p>{{ successMessage }}</p>
        <p v-if="!pendingInvite">
          <RouterLink to="/login">Back to sign in</RouterLink>
        </p>
        <p v-else-if="autoAccepting">Joining the group…</p>
        <p v-else-if="autoAcceptError" class="error-message">
          {{ autoAcceptError }}
          <RouterLink :to="{ path: '/login', query: { redirect: inviteRedirect } }">Sign in manually</RouterLink>
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
import { ref, computed, onMounted } from 'vue'
import { RouterLink, useRouter } from 'vue-router'
import authService from '../services/authService'
import groupService from '../services/groupService'
import InviteBanner from '../components/InviteBanner.vue'
import { useAuthStore } from '../stores/authStore'
import { useGroupStore } from '../stores/groupStore'
import {
  consumePendingInvite,
  peekPendingInvite,
  type PendingInvite,
} from '../services/inviteHandoff'

const router = useRouter()
const authStore = useAuthStore()
const groupStore = useGroupStore()

const form = ref({ name: '', email: '', password: '' })
const isLoading = ref(false)
const error = ref<string | null>(null)
const submitted = ref(false)
const successMessage = ref('')
const pendingInvite = ref<PendingInvite | null>(null)
const autoAccepting = ref(false)
const autoAcceptError = ref<string | null>(null)
const inviteRedirect = computed(() =>
  pendingInvite.value ? `/invite/${pendingInvite.value.token}` : '/'
)

onMounted(() => {
  pendingInvite.value = peekPendingInvite()
})

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
      password: form.value.password,
      invite_token: pendingInvite.value?.token,
    })
    successMessage.value = res.message
    submitted.value = true

    if (pendingInvite.value) {
      // Invite-based registration created an active user. Sign them in
      // with the credentials they just typed and accept the invite — the
      // user should land inside the group without extra clicks.
      await completeInviteFlow(form.value.email.trim(), form.value.password)
    }
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

async function completeInviteFlow(email: string, password: string) {
  if (!pendingInvite.value) return
  autoAccepting.value = true
  autoAcceptError.value = null
  const invite = pendingInvite.value
  try {
    await authStore.login({ email, password })
    const membership = await groupService.acceptInvite(invite.token)
    // Clear the handoff only on success — if anything above throws we
    // keep the token so the user can retry manually from /invite/<token>.
    consumePendingInvite()
    pendingInvite.value = null
    await groupStore.fetchGroups()
    const joined = groupStore.groups.find((g) => g.id === membership.group_id)
    if (joined) {
      await groupStore.setCurrentGroup(joined.slug)
    }
    await router.replace('/')
  } catch (err: any) {
    autoAcceptError.value =
      err?.response?.data?.errors?.[0]?.detail ||
      'Registered, but could not automatically join the group. Please sign in manually to continue.'
  } finally {
    autoAccepting.value = false
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

.register-header {
  text-align: center;
  margin-bottom: 2rem;
}

.register-header h1 {
  color: #333;
  margin: 0 0 0.5rem;
  font-size: 2rem;
  font-weight: 600;
}

.register-header p {
  color: #666;
  margin: 0;
  font-size: 1rem;
}

.register-form-content {
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

.register-button {
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

.register-button:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.register-button:hover:not(:disabled) {
  opacity: 0.9;
}

.login-link {
  text-align: center;
  color: #666;
  font-size: 0.9rem;
  margin: 0;
}
</style>

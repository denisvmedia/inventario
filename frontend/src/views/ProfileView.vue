<template>
  <div class="profile-view">
    <h1>My Profile</h1>

    <ErrorNotificationStack :errors="errors" @dismiss="removeError" />

    <div v-if="successMessage" class="success-banner">
      <font-awesome-icon icon="check-circle" />
      {{ successMessage }}
    </div>

    <div class="profile-card">
      <form @submit.prevent="onSave">
        <div class="form-group">
          <label for="profile-name">Name</label>
          <input
            id="profile-name"
            v-model="nameField"
            type="text"
            class="form-input"
            placeholder="Your name"
            maxlength="100"
            required
          />
          <span v-if="nameError" class="field-error">{{ nameError }}</span>
        </div>

        <div class="form-group">
          <label for="profile-email">Email</label>
          <input
            id="profile-email"
            :value="authStore.userEmail"
            type="email"
            class="form-input form-input--readonly"
            readonly
            disabled
          />
          <span class="field-hint">Email cannot be changed here.</span>
        </div>

        <!-- Role field removed — roles are now per-group -->

        <div class="form-actions">
          <button type="submit" class="btn btn-primary" :disabled="saving">
            <font-awesome-icon v-if="saving" icon="spinner" spin />
            {{ saving ? 'Saving…' : 'Save Changes' }}
          </button>
        </div>
      </form>
    </div>

    <div class="profile-card password-card">
      <button class="password-toggle" type="button" @click="showPasswordSection = !showPasswordSection">
        <font-awesome-icon :icon="showPasswordSection ? 'chevron-up' : 'chevron-down'" />
        Change Password
      </button>

      <form v-if="showPasswordSection" class="password-form" @submit.prevent="onChangePassword">
        <div v-if="passwordSuccess" class="success-banner">
          <font-awesome-icon icon="check-circle" />
          {{ passwordSuccess }}
        </div>
        <div v-if="passwordError" class="error-banner">
          <font-awesome-icon icon="exclamation-circle" />
          {{ passwordError }}
        </div>

        <div class="form-group">
          <label for="current-password">Current Password</label>
          <input
            id="current-password"
            v-model="currentPassword"
            type="password"
            class="form-input"
            autocomplete="current-password"
            required
          />
        </div>

        <div class="form-group">
          <label for="new-password">New Password</label>
          <input
            id="new-password"
            v-model="newPassword"
            type="password"
            class="form-input"
            autocomplete="new-password"
            required
          />
        </div>

        <div class="form-group">
          <label for="confirm-password">Confirm New Password</label>
          <input
            id="confirm-password"
            v-model="confirmPassword"
            type="password"
            class="form-input"
            autocomplete="new-password"
            required
          />
        </div>

        <div class="form-actions">
          <button type="submit" class="btn btn-danger" :disabled="changingPassword">
            <font-awesome-icon v-if="changingPassword" icon="spinner" spin />
            {{ changingPassword ? 'Changing…' : 'Change Password' }}
          </button>
        </div>
      </form>
    </div>
  </div>
</template>



<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/authStore'
import { useErrorState } from '@/utils/errorUtils'
import ErrorNotificationStack from '@/components/ErrorNotificationStack.vue'

const router = useRouter()
const authStore = useAuthStore()
const { errors, handleError, removeError } = useErrorState()

const nameField = ref('')
const nameError = ref('')
const saving = ref(false)
const successMessage = ref('')

// Password change state
const showPasswordSection = ref(false)
const currentPassword = ref('')
const newPassword = ref('')
const confirmPassword = ref('')
const changingPassword = ref(false)
const passwordSuccess = ref('')
const passwordError = ref('')

// Logout timer handle — stored so it can be cancelled if the component unmounts
// before the 2-second delay fires (e.g. the user navigates away from /profile).
const logoutTimer = ref<ReturnType<typeof setTimeout> | null>(null)

onMounted(() => {
  nameField.value = authStore.userName ?? ''
})

onUnmounted(() => {
  if (logoutTimer.value !== null) {
    clearTimeout(logoutTimer.value)
    logoutTimer.value = null
  }
})

function validateName(): boolean {
  const trimmed = nameField.value.trim()
  if (!trimmed) {
    nameError.value = 'Name must not be blank.'
    return false
  }
  if (trimmed.length > 100) {
    nameError.value = 'Name must not exceed 100 characters.'
    return false
  }
  nameError.value = ''
  return true
}

async function onSave() {
  if (!validateName()) return

  saving.value = true
  successMessage.value = ''

  try {
    await authStore.updateProfile({ name: nameField.value.trim() })
    successMessage.value = 'Profile updated successfully.'
  } catch (err: any) {
    handleError(err, 'profile', 'Failed to update profile')
  } finally {
    saving.value = false
  }
}

async function onChangePassword() {
  passwordError.value = ''
  passwordSuccess.value = ''

  if (!currentPassword.value) {
    passwordError.value = 'Please enter your current password.'
    return
  }
  if (!newPassword.value) {
    passwordError.value = 'Please enter a new password.'
    return
  }
  if (newPassword.value === currentPassword.value) {
    passwordError.value = 'New password must differ from the current password.'
    return
  }
  if (newPassword.value !== confirmPassword.value) {
    passwordError.value = 'New password and confirmation do not match.'
    return
  }

  changingPassword.value = true
  try {
    await authStore.changePassword(currentPassword.value, newPassword.value)
    passwordSuccess.value = 'Password changed successfully. You will be logged out…'
    currentPassword.value = ''
    newPassword.value = ''
    confirmPassword.value = ''
    logoutTimer.value = setTimeout(async () => {
      await authStore.logout()
      router.push('/login')
    }, 2000)
  } catch (err: any) {
    const status = err?.response?.status
    if (status === 422) {
      passwordError.value = 'Current password is incorrect.'
    } else {
      const msg = err?.response?.data?.message || err?.response?.data || 'Failed to change password.'
      passwordError.value = typeof msg === 'string' ? msg : 'Failed to change password.'
    }
  } finally {
    changingPassword.value = false
  }
}
</script>

<style scoped>
.profile-view {
  padding: 1.5rem;
  max-width: 540px;
  margin: 0 auto;
}

.profile-view h1 {
  margin-bottom: 1.25rem;
  font-size: 1.5rem;
}

.success-banner {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  background: #dcfce7;
  color: #166534;
  border: 1px solid #bbf7d0;
  border-radius: 6px;
  padding: 0.75rem 1rem;
  margin-bottom: 1rem;
  font-size: 0.9rem;
}

.profile-card {
  background: var(--p-surface-0, #fff);
  border: 1px solid var(--p-surface-200, #e5e7eb);
  border-radius: 10px;
  padding: 1.5rem;
}

.form-group {
  margin-bottom: 1.25rem;
}

.form-group label {
  display: block;
  font-weight: 600;
  margin-bottom: 0.4rem;
  font-size: 0.9rem;
}

.form-input {
  width: 100%;
  padding: 0.5rem 0.75rem;
  border: 1px solid var(--p-surface-300, #ccc);
  border-radius: 6px;
  font-size: 0.9rem;
  background: var(--p-surface-0, #fff);
  color: var(--p-text-color, inherit);
  box-sizing: border-box;
}

.form-input--readonly {
  background: var(--p-surface-100, #f3f4f6);
  color: var(--p-text-muted-color, #6b7280);
  cursor: not-allowed;
}

.field-error {
  display: block;
  color: #dc2626;
  font-size: 0.8rem;
  margin-top: 0.25rem;
}

.field-hint {
  display: block;
  color: var(--p-text-muted-color, #6b7280);
  font-size: 0.78rem;
  margin-top: 0.25rem;
}

.form-actions {
  margin-top: 1.5rem;
}

.password-card {
  margin-top: 1.25rem;
}

.password-toggle {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  background: none;
  border: none;
  font-size: 0.95rem;
  font-weight: 600;
  color: var(--p-primary-color, #3b82f6);
  cursor: pointer;
  padding: 0;
}

.password-toggle:hover {
  text-decoration: underline;
}

.password-form {
  margin-top: 1.25rem;
}

.error-banner {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  background: #fee2e2;
  color: #991b1b;
  border: 1px solid #fca5a5;
  border-radius: 6px;
  padding: 0.75rem 1rem;
  margin-bottom: 1rem;
  font-size: 0.9rem;
}

.btn-danger {
  background: #dc2626;
  color: #fff;
  border: none;
  border-radius: 6px;
  padding: 0.5rem 1.25rem;
  font-size: 0.9rem;
  cursor: pointer;
}

.btn-danger:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.btn-danger:hover:not(:disabled) {
  background: #b91c1c;
}
</style>

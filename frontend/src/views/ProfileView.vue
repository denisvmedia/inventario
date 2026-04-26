<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { CheckCircle2, ChevronDown, ChevronUp, Loader2, TriangleAlert } from 'lucide-vue-next'

import { Button } from '@design/ui/button'
import { Input } from '@design/ui/input'
import { Label } from '@design/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@design/ui/select'
import FormGrid from '@design/patterns/FormGrid.vue'
import FormSection from '@design/patterns/FormSection.vue'
import PageContainer from '@design/patterns/PageContainer.vue'
import PageHeader from '@design/patterns/PageHeader.vue'
import PageSection from '@design/patterns/PageSection.vue'
import { useAppToast } from '@design/composables/useAppToast'
import { useTheme, type ThemePreference } from '@design/composables/useTheme'
import { useDensity, type Density } from '@design/composables/useDensity'

import { useAuthStore } from '@/stores/authStore'
import { useGroupStore } from '@/stores/groupStore'

const router = useRouter()
const authStore = useAuthStore()
const groupStore = useGroupStore()
const toast = useAppToast()

const { preference: themePreference, setTheme } = useTheme()
const { density, setDensity } = useDensity()

function onThemeChange(value: string) {
  setTheme(value as ThemePreference)
}

function onDensityChange(value: string) {
  setDensity(value as Density)
}

const nameField = ref('')
const nameError = ref('')
// #1263: empty string represents "no default" — mapped to null when sent to the
// API. Initialized from the authStore so refreshing the profile page keeps the
// currently saved preference selected.
const defaultGroupField = ref('')
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

const showGroupSelect = computed(() => groupStore.groups.length > 0)
const userEmail = computed(() => authStore.userEmail ?? '')

onMounted(async () => {
  nameField.value = authStore.userName ?? ''
  defaultGroupField.value = authStore.userDefaultGroupID ?? ''
  // Ensure the dropdown has an up-to-date list of the user's groups even when
  // the profile page is the first thing a user loads (deep-link from an email,
  // "Settings" menu, etc.). Any /groups failure is surfaced via toast instead
  // of rejecting the lifecycle hook — otherwise Vue swallows the error and
  // the user is stuck with an empty dropdown and no feedback.
  try {
    await groupStore.ensureLoaded()
  } catch (err: any) {
    toast.error(err?.message ?? 'Failed to load your groups. Please refresh the page.')
  }

  // Defensive: the stored preference might point at a group the user no longer
  // belongs to (e.g. admin removed them elsewhere without the server having
  // cleared default_group_id yet — ON DELETE SET NULL only fires when the
  // group itself is deleted, not when a membership is revoked). If we leave
  // the stale value in the select, a subsequent save would 400 on the backend
  // membership check and block unrelated profile edits. Reset silently to
  // "no default" so the user can still change their name.
  if (
    defaultGroupField.value !== '' &&
    !groupStore.groups.some((g) => g.id === defaultGroupField.value)
  ) {
    defaultGroupField.value = ''
  }
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
    await authStore.updateProfile({
      name: nameField.value.trim(),
      // Empty string → null clears the preference on the server; a group id
      // sets it. Sending the field unconditionally (not just when it changed)
      // keeps the client-side code simple and the server idempotent.
      default_group_id: defaultGroupField.value ? defaultGroupField.value : null,
    })
    successMessage.value = 'Profile updated successfully.'
  } catch (err: any) {
    toast.error(err?.response?.data?.message ?? err?.message ?? 'Failed to update profile')
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

<template>
  <PageContainer width="narrow">
    <PageHeader title="My Profile" />

    <form class="flex flex-col gap-6" @submit.prevent="onSave">
      <FormSection title="Identity">
        <FormGrid cols="1">
          <div class="flex flex-col gap-1.5">
            <Label for="profile-name">Name</Label>
            <Input
              id="profile-name"
              v-model="nameField"
              type="text"
              placeholder="Your name"
              maxlength="100"
              required
            />
            <p
              v-if="nameError"
              class="field-error text-sm text-destructive"
              role="alert"
            >
              {{ nameError }}
            </p>
          </div>

          <div class="flex flex-col gap-1.5">
            <Label for="profile-email">Email</Label>
            <Input
              id="profile-email"
              :model-value="userEmail"
              type="email"
              readonly
              disabled
            />
            <p class="text-xs text-muted-foreground">
              Email cannot be changed here.
            </p>
          </div>
        </FormGrid>
      </FormSection>

      <FormSection v-if="showGroupSelect" title="Preferences">
        <FormGrid cols="1">
          <div class="flex flex-col gap-1.5">
            <Label for="profile-default-group">Default Group</Label>
            <Select
              v-model="defaultGroupField"
              data-testid="default-group-select"
            >
              <SelectTrigger
                id="profile-default-group"
                aria-label="Default group"
              >
                <SelectValue placeholder="No default (use fallback)" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="">No default (use fallback)</SelectItem>
                <SelectItem v-for="g in groupStore.groups" :key="g.id" :value="g.id">
                  {{ g.icon ? `${g.icon} ${g.name}` : g.name }}
                </SelectItem>
              </SelectContent>
            </Select>
            <p class="text-xs text-muted-foreground">
              After login you'll land in this group. Leave as "No default" to fall
              back to the first group you created (or were invited to).
            </p>
          </div>
        </FormGrid>
      </FormSection>

      <FormSection title="Appearance">
        <FormGrid cols="2">
          <div class="flex flex-col gap-1.5">
            <Label for="profile-theme">Theme</Label>
            <Select
              :model-value="themePreference"
              data-testid="theme-select"
              @update:model-value="(v) => onThemeChange(String(v))"
            >
              <SelectTrigger id="profile-theme" aria-label="Theme">
                <SelectValue placeholder="System" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="system">System</SelectItem>
                <SelectItem value="light">Light</SelectItem>
                <SelectItem value="dark">Dark</SelectItem>
              </SelectContent>
            </Select>
            <p class="text-xs text-muted-foreground">
              "System" follows your OS preference; the choice is stored on this
              device.
            </p>
          </div>

          <div class="flex flex-col gap-1.5">
            <Label for="profile-density">Density</Label>
            <Select
              :model-value="density"
              data-testid="density-select"
              @update:model-value="(v) => onDensityChange(String(v))"
            >
              <SelectTrigger id="profile-density" aria-label="Density">
                <SelectValue placeholder="Comfortable" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="comfortable">Comfortable</SelectItem>
                <SelectItem value="compact">Compact</SelectItem>
              </SelectContent>
            </Select>
            <p class="text-xs text-muted-foreground">
              Compact tightens header controls and listings. Stored on this
              device.
            </p>
          </div>
        </FormGrid>
      </FormSection>

      <div
        v-if="successMessage"
        class="success-banner flex items-center gap-2 rounded-md border border-green-200 bg-green-50 px-3 py-2 text-sm text-green-900 dark:border-green-900/50 dark:bg-green-950/40 dark:text-green-100"
        role="status"
      >
        <CheckCircle2 class="size-4 shrink-0" aria-hidden="true" />
        {{ successMessage }}
      </div>

      <div class="flex justify-end">
        <Button type="submit" data-testid="profile-save" :disabled="saving">
          <Loader2 v-if="saving" class="size-4 animate-spin" aria-hidden="true" />
          {{ saving ? 'Saving…' : 'Save Changes' }}
        </Button>
      </div>
    </form>

    <PageSection title="Security" class="mt-8" as="h2">
      <button
        type="button"
        class="password-toggle inline-flex items-center gap-2 text-sm font-semibold text-primary hover:underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
        @click="showPasswordSection = !showPasswordSection"
      >
        <component
          :is="showPasswordSection ? ChevronUp : ChevronDown"
          class="size-4"
          aria-hidden="true"
        />
        Change Password
      </button>

      <form
        v-if="showPasswordSection"
        class="password-form mt-4 flex flex-col gap-4"
        @submit.prevent="onChangePassword"
      >
        <div
          v-if="passwordSuccess"
          class="success-banner flex items-center gap-2 rounded-md border border-green-200 bg-green-50 px-3 py-2 text-sm text-green-900 dark:border-green-900/50 dark:bg-green-950/40 dark:text-green-100"
          role="status"
        >
          <CheckCircle2 class="size-4 shrink-0" aria-hidden="true" />
          {{ passwordSuccess }}
        </div>
        <div
          v-if="passwordError"
          class="error-banner flex items-center gap-2 rounded-md border border-destructive/40 bg-destructive/10 px-3 py-2 text-sm text-destructive"
          role="alert"
        >
          <TriangleAlert class="size-4 shrink-0" aria-hidden="true" />
          {{ passwordError }}
        </div>

        <div class="flex flex-col gap-1.5">
          <Label for="current-password">Current Password</Label>
          <Input
            id="current-password"
            v-model="currentPassword"
            type="password"
            autocomplete="current-password"
            required
          />
        </div>

        <div class="flex flex-col gap-1.5">
          <Label for="new-password">New Password</Label>
          <Input
            id="new-password"
            v-model="newPassword"
            type="password"
            autocomplete="new-password"
            required
          />
        </div>

        <div class="flex flex-col gap-1.5">
          <Label for="confirm-password">Confirm New Password</Label>
          <Input
            id="confirm-password"
            v-model="confirmPassword"
            type="password"
            autocomplete="new-password"
            required
          />
        </div>

        <div class="flex justify-end">
          <Button
            type="submit"
            variant="destructive"
            data-testid="change-password-submit"
            :disabled="changingPassword"
          >
            <Loader2
              v-if="changingPassword"
              class="size-4 animate-spin"
              aria-hidden="true"
            />
            {{ changingPassword ? 'Changing…' : 'Change Password' }}
          </Button>
        </div>
      </form>
    </PageSection>
  </PageContainer>
</template>

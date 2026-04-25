<script setup lang="ts">
/**
 * RegisterView — sign-up screen rebuilt on shadcn-vue Card + Form (#1326
 * PR 1.6). Replaces the legacy hand-rolled card/form markup; all
 * Playwright anchors used by `e2e/tests/registration.spec.ts` are
 * preserved verbatim:
 *
 *   - `<h1>Inventario</h1>` (provided by AuthCard).
 *   - `input[data-testid="name|email|password"]`.
 *   - `button[data-testid="register-button"]`.
 *   - `.success-message` (post-submit confirmation block).
 *   - `.error-message` (server / mode-closed failure notice).
 *   - `form.register-form-content` (the form element itself, asserted as
 *     visible after a 403 in "Registration mode — closed").
 */
import { computed, onMounted, ref } from 'vue'
import { RouterLink, useRouter } from 'vue-router'
import { useForm } from 'vee-validate'
import { toTypedSchema } from '@vee-validate/zod'

import { Button } from '@design/ui/button'
import {
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@design/ui/form'
import { Input } from '@design/ui/input'
import AuthCard from '@design/patterns/AuthCard.vue'

import authService from '@/services/authService'
import groupService from '@/services/groupService'
import InviteBanner from '@/components/InviteBanner.vue'
import { useAuthStore } from '@/stores/authStore'
import { useGroupStore } from '@/stores/groupStore'
import {
  consumePendingInvite,
  peekPendingInvite,
  type PendingInvite,
} from '@/services/inviteHandoff'

import { registerFormSchema, type RegisterFormInput } from './RegisterView.schema'

const router = useRouter()
const authStore = useAuthStore()
const groupStore = useGroupStore()

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

const { handleSubmit, values } = useForm<RegisterFormInput>({
  validationSchema: toTypedSchema(registerFormSchema),
  initialValues: { name: '', email: '', password: '' },
})

// Match the legacy "submit disabled until all fields have content" gate
// (registration.spec.ts: "enables submit button only when all fields are
// filled"). vee-validate's meta.valid would already block on the 8-char
// password rule, so we use the same length-only predicate the legacy
// view used to keep that spec deterministic on intermediate keystrokes.
const isFormFilled = computed(
  () =>
    (values.name ?? '').trim() !== '' &&
    (values.email ?? '').trim() !== '' &&
    (values.password ?? '').trim() !== ''
)

onMounted(() => {
  pendingInvite.value = peekPendingInvite()
})

const onSubmit = handleSubmit(async (formValues) => {
  isLoading.value = true
  error.value = null
  try {
    const res = await authService.register({
      name: formValues.name.trim(),
      email: formValues.email.trim(),
      password: formValues.password,
      invite_token: pendingInvite.value?.token,
    })
    successMessage.value = res.message
    submitted.value = true

    if (pendingInvite.value) {
      await completeInviteFlow(formValues.email.trim(), formValues.password)
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
})

async function completeInviteFlow(email: string, password: string) {
  if (!pendingInvite.value) return
  autoAccepting.value = true
  autoAcceptError.value = null
  const invite = pendingInvite.value
  try {
    await authStore.login({ email, password })
    const membership = await groupService.acceptInvite(invite.token)
    consumePendingInvite()
    pendingInvite.value = null
    await groupStore.fetchGroups()
    const joined = groupStore.groups.find((g) => g.id === membership.group_id)
    if (joined) {
      await groupStore.setCurrentGroup(joined.slug)
    }
    await router.replace('/')
  } catch (err: unknown) {
    const e = err as {
      response?: { data?: { errors?: Array<{ detail?: string }> } }
    }
    autoAcceptError.value =
      e.response?.data?.errors?.[0]?.detail ||
      'Registered, but could not automatically join the group. Please sign in manually to continue.'
  } finally {
    autoAccepting.value = false
  }
}
</script>

<template>
  <AuthCard subtitle="Create a new account" test-id="register-view">
    <template #banner>
      <InviteBanner
        v-if="pendingInvite"
        :group-name="pendingInvite.groupName"
        prefix="You're registering to join"
      />
    </template>

    <div
      v-if="submitted"
      class="success-message rounded-md border border-emerald-200 bg-emerald-50 px-3 py-3 text-center text-sm text-emerald-900"
    >
      <p>{{ successMessage }}</p>
      <p v-if="!pendingInvite" class="mt-2">
        <RouterLink to="/login" class="font-medium hover:underline">Back to sign in</RouterLink>
      </p>
      <p v-else-if="autoAccepting" class="mt-2">Joining the group…</p>
      <p
        v-else-if="autoAcceptError"
        class="error-message mt-2 rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-destructive"
      >
        {{ autoAcceptError }}
        <RouterLink
          :to="{ path: '/login', query: { redirect: inviteRedirect } }"
          class="font-medium hover:underline"
        >
          Sign in manually
        </RouterLink>
      </p>
    </div>

    <form
      v-else
      class="register-form-content flex flex-col gap-4"
      @submit="onSubmit"
    >
      <FormField v-slot="{ componentField }" name="name">
        <FormItem>
          <FormLabel required>Full Name</FormLabel>
          <FormControl>
            <Input
              v-bind="componentField"
              type="text"
              autocomplete="name"
              :disabled="isLoading"
              data-testid="name"
              placeholder="Enter your full name"
            />
          </FormControl>
          <FormMessage />
        </FormItem>
      </FormField>

      <FormField v-slot="{ componentField }" name="email">
        <FormItem>
          <FormLabel required>Email</FormLabel>
          <FormControl>
            <Input
              v-bind="componentField"
              type="email"
              autocomplete="email"
              :disabled="isLoading"
              data-testid="email"
              placeholder="Enter your email"
            />
          </FormControl>
          <FormMessage />
        </FormItem>
      </FormField>

      <FormField v-slot="{ componentField }" name="password">
        <FormItem>
          <FormLabel required>Password</FormLabel>
          <FormControl>
            <Input
              v-bind="componentField"
              type="password"
              autocomplete="new-password"
              :disabled="isLoading"
              data-testid="password"
              placeholder="At least 8 characters"
            />
          </FormControl>
          <FormMessage />
        </FormItem>
      </FormField>

      <p
        v-if="error"
        class="error-message rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive"
      >
        {{ error }}
      </p>

      <Button
        type="submit"
        class="register-button w-full"
        data-testid="register-button"
        :disabled="isLoading || !isFormFilled"
      >
        {{ isLoading ? 'Creating account…' : 'Create Account' }}
      </Button>
    </form>

    <template #footer>
      <p>
        Already have an account?
        <RouterLink to="/login" class="font-medium text-primary hover:underline">
          Sign in
        </RouterLink>
      </p>
    </template>
  </AuthCard>
</template>

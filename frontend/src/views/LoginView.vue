<script setup lang="ts">
/**
 * LoginView — sign-in screen rebuilt on shadcn-vue Card + Form (#1326 PR
 * 1.6). Replaces the legacy `LoginForm.vue` component (now removed from
 * `views/`); the file owns its own template instead of being a thin
 * wrapper because the auth surface is small and the legacy split made
 * the redirect / invite-handoff flow harder to follow.
 *
 * E2E anchors preserved verbatim:
 *   - `<h1>Inventario</h1>` (provided by AuthCard) for `page.locator('h1')`.
 *   - `input[data-testid="email"]`, `input[data-testid="password"]`,
 *     `button[data-testid="login-button"]`, `a[href="/register"]`,
 *     `a[href="/forgot-password"]`.
 *   - The `.error-message` div for the inline auth error so any future
 *     login.spec assertion still hits the same node.
 */
import { computed, onMounted, ref } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'
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

import { useAuthStore } from '@/stores/authStore'
import { useGroupStore } from '@/stores/groupStore'
import groupService from '@/services/groupService'
import InviteBanner from '@/components/InviteBanner.vue'
import {
  consumePendingInvite,
  peekPendingInvite,
  type PendingInvite,
} from '@/services/inviteHandoff'

import { loginFormSchema, type LoginFormInput } from './LoginView.schema'

const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()
const groupStore = useGroupStore()

// SESSION_MESSAGES maps router-supplied `?reason=…` codes to the human
// notice rendered above the form. Keeping this in the view (not the store)
// matches the legacy LoginForm so unknown reasons fall back to the
// generic "session ended" copy.
const SESSION_MESSAGES: Record<string, string> = {
  session_expired: 'Your session has expired. Please sign in again.',
}

const sessionMessage = computed(() => {
  const reason = route.query.reason as string | undefined
  return reason
    ? SESSION_MESSAGES[reason] ?? 'Your session has ended. Please sign in again.'
    : null
})

const pendingInvite = ref<PendingInvite | null>(null)

const { handleSubmit, values } = useForm<LoginFormInput>({
  validationSchema: toTypedSchema(loginFormSchema),
  initialValues: { email: '', password: '' },
})

const isLoading = computed(() => authStore.isLoading)
const error = computed(() => authStore.error)

// Match the legacy "submit disabled until both fields have content" gate
// (#1326 PR 1.6). vee-validate's `meta.valid` is true on initial mount
// for a schema that only requires non-empty strings, so we read the
// reactive `values` directly to keep the UX identical to LoginForm.
const isFormValid = computed(
  () => (values.email ?? '').trim() !== '' && (values.password ?? '').trim() !== ''
)

onMounted(() => {
  pendingInvite.value = peekPendingInvite()
  // Bail out if the session is already restored — keeps the legacy
  // "deep-link / refresh while authenticated" UX (LoginView.vue + router
  // guard belt-and-braces).
  if (authStore.isAuthenticated) {
    const redirectTo = (route.query.redirect as string) || '/'
    void router.replace(redirectTo)
  }
})

const onSubmit = handleSubmit(async (formValues) => {
  try {
    await authStore.login({
      email: formValues.email.trim(),
      password: formValues.password,
    })

    // Post-login invite handoff (#1285): if the user came through
    // /invite/<token>, accept the invite and land them inside the group.
    // Failures fall through to the normal redirect so the user can retry
    // from /invite/<token> manually.
    if (pendingInvite.value) {
      try {
        const invite = pendingInvite.value
        const membership = await groupService.acceptInvite(invite.token)
        consumePendingInvite()
        pendingInvite.value = null
        await groupStore.fetchGroups()
        const joined = groupStore.groups.find((g) => g.id === membership.group_id)
        if (joined) {
          await groupStore.setCurrentGroup(joined.slug)
        }
        await router.replace('/')
        return
      } catch (e) {
        console.warn('Post-login invite accept failed:', e)
        // Fall through to the normal redirect below.
      }
    }

    const redirectTo = (route.query.redirect as string) || '/'
    await router.replace(redirectTo)
  } catch (err) {
    // Error surface is owned by the auth store; logged here for parity
    // with the legacy LoginForm so console traces stay consistent.
    console.error('Login failed:', err)
  }
})
</script>

<template>
  <AuthCard subtitle="Sign in to your account" test-id="login-view">
    <template #banner>
      <div
        v-if="sessionMessage"
        class="session-message rounded-md border border-amber-200 bg-amber-50 px-3 py-2 text-sm text-amber-900"
      >
        {{ sessionMessage }}
      </div>
      <InviteBanner
        v-if="pendingInvite"
        :group-name="pendingInvite.groupName"
        prefix="Sign in to accept the invitation to"
      />
    </template>

    <form class="login-form-content flex flex-col gap-4" @submit="onSubmit">
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
              autocomplete="current-password"
              :disabled="isLoading"
              data-testid="password"
              placeholder="Enter your password"
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
        class="login-button w-full"
        data-testid="login-button"
        :disabled="isLoading || !isFormValid"
      >
        {{ isLoading ? 'Signing in…' : 'Sign In' }}
      </Button>
    </form>

    <template #footer>
      <RouterLink to="/forgot-password" class="hover:text-foreground hover:underline">
        Forgot password?
      </RouterLink>
      <p>
        Don't have an account?
        <RouterLink to="/register" class="font-medium text-primary hover:underline">
          Create one
        </RouterLink>
      </p>
    </template>
  </AuthCard>
</template>

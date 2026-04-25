<script setup lang="ts">
/**
 * ResetPasswordView — set a new password from a reset link (#1326 PR
 * 1.6). Rebuilt on shadcn-vue Card + Form. Three render states:
 *   1. No / empty `?token` query param → invalid-link notice with a
 *      shortcut to /forgot-password.
 *   2. Submitted successfully → success block with a manual sign-in
 *      link plus an automatic redirect after 3s (legacy parity).
 *   3. Otherwise → password / confirm-password form whose cross-field
 *      check ("Passwords do not match") lives in the Zod schema.
 *
 * Preserves the legacy anchors:
 *   - `<h1>Inventario</h1>` (provided by AuthCard).
 *   - `input[data-testid="password|confirm-password"]`.
 *   - `button[data-testid="submit-button"]`.
 *   - `.success-message` and `.error-message` blocks.
 */
import { computed, onBeforeUnmount, ref, watchEffect } from 'vue'
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

import authService from '@/services/authService'

import {
  resetPasswordFormSchema,
  type ResetPasswordFormInput,
} from './ResetPasswordView.schema'

const route = useRoute()
const router = useRouter()

const token = ref('')
const isLoading = ref(false)
const error = ref<string | null>(null)
const submitted = ref(false)
const successMessage = ref('')

let redirectTimer: ReturnType<typeof setTimeout> | null = null
onBeforeUnmount(() => {
  if (redirectTimer !== null) clearTimeout(redirectTimer)
})

watchEffect(() => {
  token.value = (route.query.token as string) || ''
})

const { handleSubmit, values } = useForm<ResetPasswordFormInput>({
  validationSchema: toTypedSchema(resetPasswordFormSchema),
  initialValues: { password: '', confirmPassword: '' },
})

// Match the legacy submit gate (#1326 PR 1.6): both fields must have at
// least 8 chars and the values must match. Reading `values.*` keeps the
// disabled state in sync with what the user is typing without surfacing
// the schema's error chrome on every keystroke.
const isFormFilled = computed(
  () =>
    (values.password ?? '').length >= 8 &&
    values.password === values.confirmPassword
)

const onSubmit = handleSubmit(async (formValues) => {
  isLoading.value = true
  error.value = null
  try {
    const res = await authService.resetPassword(token.value, formValues.password)
    successMessage.value = res.message
    submitted.value = true
    redirectTimer = setTimeout(() => router.replace('/login'), 3000)
  } catch (err: unknown) {
    const e = err as { response?: { data?: string | { error?: string } } }
    const data = e.response?.data
    if (typeof data === 'string') {
      error.value = data.trim() || 'Failed to reset password. Please try again.'
    } else {
      error.value =
        'Failed to reset password. The link may have expired. Please request a new one.'
    }
  } finally {
    isLoading.value = false
  }
})
</script>

<template>
  <AuthCard subtitle="Set a new password" test-id="reset-password-view">
    <div
      v-if="!token"
      class="error-message rounded-md border border-destructive/30 bg-destructive/10 px-3 py-3 text-center text-sm text-destructive"
    >
      <p>Invalid or missing reset token. Please request a new password reset link.</p>
      <p class="mt-2">
        <RouterLink to="/forgot-password" class="font-medium hover:underline">
          Request reset link
        </RouterLink>
      </p>
    </div>

    <div
      v-else-if="submitted"
      class="success-message rounded-md border border-emerald-200 bg-emerald-50 px-3 py-3 text-center text-sm text-emerald-900"
    >
      <p>{{ successMessage }}</p>
      <p class="mt-2">
        <RouterLink to="/login" class="font-medium hover:underline">
          Sign in with your new password
        </RouterLink>
      </p>
    </div>

    <form
      v-else
      class="reset-password-form-content flex flex-col gap-4"
      @submit="onSubmit"
    >
      <FormField v-slot="{ componentField }" name="password">
        <FormItem>
          <FormLabel required>New Password</FormLabel>
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

      <FormField v-slot="{ componentField }" name="confirmPassword">
        <FormItem>
          <FormLabel required>Confirm New Password</FormLabel>
          <FormControl>
            <Input
              v-bind="componentField"
              type="password"
              autocomplete="new-password"
              :disabled="isLoading"
              data-testid="confirm-password"
              placeholder="Repeat your new password"
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
        class="submit-button w-full"
        data-testid="submit-button"
        :disabled="isLoading || !isFormFilled"
      >
        {{ isLoading ? 'Resetting…' : 'Reset Password' }}
      </Button>
    </form>

    <template #footer>
      <RouterLink to="/login" class="hover:text-foreground hover:underline">
        Back to sign in
      </RouterLink>
    </template>
  </AuthCard>
</template>

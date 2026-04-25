<script setup lang="ts">
/**
 * ForgotPasswordView — request a password reset link (#1326 PR 1.6).
 * Rebuilt on shadcn-vue Card + Form. The server returns a generic
 * success message regardless of whether the email is known
 * (anti-enumeration), so the only client-side concern is keeping the
 * "Send" button disabled while the field is empty.
 *
 * Preserves the legacy anchors used by future Playwright specs:
 *   - `<h1>Inventario</h1>` (provided by AuthCard).
 *   - `input[data-testid="email"]`.
 *   - `button[data-testid="submit-button"]`.
 *   - `.success-message` and `.error-message` blocks.
 */
import { computed, ref } from 'vue'
import { RouterLink } from 'vue-router'
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
  forgotPasswordFormSchema,
  type ForgotPasswordFormInput,
} from './ForgotPasswordView.schema'

const isLoading = ref(false)
const error = ref<string | null>(null)
const submitted = ref(false)
const successMessage = ref('')

const { handleSubmit, values } = useForm<ForgotPasswordFormInput>({
  validationSchema: toTypedSchema(forgotPasswordFormSchema),
  initialValues: { email: '' },
})

const isFormFilled = computed(() => (values.email ?? '').trim() !== '')

const onSubmit = handleSubmit(async (formValues) => {
  isLoading.value = true
  error.value = null
  try {
    const res = await authService.forgotPassword(formValues.email.trim())
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
})
</script>

<template>
  <AuthCard subtitle="Reset your password" test-id="forgot-password-view">
    <div
      v-if="submitted"
      class="success-message rounded-md border border-emerald-200 bg-emerald-50 px-3 py-3 text-center text-sm text-emerald-900"
    >
      <p>{{ successMessage }}</p>
      <p class="mt-2">
        <RouterLink to="/login" class="font-medium hover:underline">Back to sign in</RouterLink>
      </p>
    </div>

    <form
      v-else
      class="forgot-password-form-content flex flex-col gap-4"
      @submit="onSubmit"
    >
      <p class="text-sm text-muted-foreground">
        Enter your email address and we'll send you a link to reset your password.
      </p>

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
        {{ isLoading ? 'Sending…' : 'Send Reset Link' }}
      </Button>
    </form>

    <template #footer>
      <RouterLink to="/login" class="hover:text-foreground hover:underline">
        Back to sign in
      </RouterLink>
    </template>
  </AuthCard>
</template>

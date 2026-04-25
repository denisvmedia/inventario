<script setup lang="ts">
/**
 * VerifyEmailView — confirms a sign-up via the token in the magic link
 * (#1326 PR 1.6). Rebuilt on shadcn-vue Card. There is no form here —
 * the only input is the `?token=…` query param, and the view fires a
 * single `GET /api/v1/verify-email?token=…` request on mount.
 *
 * Render states (mutually exclusive) and the legacy class anchors that
 * `e2e/tests/registration.spec.ts` depends on:
 *   - In-flight        → `.status-message.loading`
 *   - Success          → `.status-message.success`
 *   - Server failure   → `.status-message.error`
 *   - No token in URL  → `.status-message.missing`
 */
import { onMounted, ref } from 'vue'
import { RouterLink, useRoute } from 'vue-router'

import AuthCard from '@design/patterns/AuthCard.vue'

import authService from '@/services/authService'

const route = useRoute()
const isLoading = ref(false)
const success = ref(false)
const message = ref('')
const error = ref<string | null>(null)

onMounted(async () => {
  const token = route.query.token as string | undefined
  if (!token) return

  isLoading.value = true
  try {
    const res = await authService.verifyEmail(token)
    message.value = res.message
    success.value = true
  } catch (err: unknown) {
    const e = err as { response?: { data?: string } }
    const data = e.response?.data
    error.value = (typeof data === 'string' && data.trim())
      ? data.trim()
      : 'Verification failed. The link may be invalid or expired.'
  } finally {
    isLoading.value = false
  }
})
</script>

<template>
  <AuthCard subtitle="Email Verification" test-id="verify-email-view">
    <div
      v-if="isLoading"
      class="status-message loading rounded-md border border-blue-200 bg-blue-50 px-3 py-3 text-center text-sm text-blue-900"
    >
      <p>Verifying your email…</p>
    </div>

    <div
      v-else-if="success"
      class="status-message success rounded-md border border-emerald-200 bg-emerald-50 px-3 py-3 text-center text-sm text-emerald-900"
    >
      <p>{{ message }}</p>
      <p class="mt-2">
        <RouterLink to="/login" class="font-medium hover:underline">
          Sign in to your account
        </RouterLink>
      </p>
    </div>

    <div
      v-else-if="error"
      class="status-message error rounded-md border border-destructive/30 bg-destructive/10 px-3 py-3 text-center text-sm text-destructive"
    >
      <p>{{ error }}</p>
      <p class="mt-2">
        <RouterLink to="/login" class="font-medium hover:underline">
          Back to sign in
        </RouterLink>
      </p>
    </div>

    <div
      v-else
      class="status-message missing rounded-md border border-amber-200 bg-amber-50 px-3 py-3 text-center text-sm text-amber-900"
    >
      <p>No verification token provided.</p>
      <p class="mt-2">
        <RouterLink to="/login" class="font-medium hover:underline">
          Back to sign in
        </RouterLink>
      </p>
    </div>
  </AuthCard>
</template>

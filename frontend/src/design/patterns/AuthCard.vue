<script setup lang="ts">
/**
 * AuthCard — centred card layout shared by all unauthenticated views
 * (login, register, forgot/reset password, email verification).
 *
 * Owns three things consistently for every auth screen:
 *   1. Full-viewport centring on a muted background.
 *   2. The "Inventario" brand `<h1>` that the registration / login e2e
 *      suites assert on (`page.locator('h1')` toContainText 'Inventario').
 *   3. A vertical stack inside the card with three slots: an optional
 *      `banner` (invite / session-expired notices), the `default` body
 *      (the form, success state, etc.) and an optional `footer` for
 *      "back to sign in" style links rendered in muted text below.
 *
 * Stateless — the parent owns banner visibility, form state and so on.
 */
import type { HTMLAttributes } from 'vue'
import { Card, CardContent, CardHeader } from '@design/ui/card'
import { cn } from '@design/lib/utils'

type Props = {
  /** Optional subtitle rendered under the brand heading. */
  subtitle?: string
  /** Extra classes on the outer viewport wrapper. */
  class?: HTMLAttributes['class']
  /** Extra classes on the inner Card. */
  cardClass?: HTMLAttributes['class']
  /** data-testid forwarded to the outer wrapper for e2e anchoring. */
  testId?: string
}

const props = withDefaults(defineProps<Props>(), {})

defineSlots<{
  default?: () => unknown
  /** Inline notice rendered above the body (invite, session expired, …). */
  banner?: () => unknown
  /** Muted text rendered below the body (back-to-login link, etc.). */
  footer?: () => unknown
}>()
</script>

<template>
  <div
    :class="cn('flex min-h-screen items-center justify-center bg-muted/40 p-4', props.class)"
    :data-testid="testId"
  >
    <Card :class="cn('w-full max-w-md', cardClass)">
      <CardHeader class="text-center">
        <h1 class="text-2xl font-semibold tracking-tight">Inventario</h1>
        <p v-if="subtitle" class="text-sm text-muted-foreground">{{ subtitle }}</p>
      </CardHeader>
      <CardContent class="flex flex-col gap-6">
        <slot name="banner" />
        <slot />
        <div
          v-if="$slots.footer"
          class="flex flex-col gap-2 text-center text-sm text-muted-foreground"
        >
          <slot name="footer" />
        </div>
      </CardContent>
    </Card>
  </div>
</template>

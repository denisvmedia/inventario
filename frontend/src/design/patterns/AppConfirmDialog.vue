<script setup lang="ts">
/**
 * AppConfirmDialog — thin wrapper over shadcn `AlertDialog` that
 * preserves the call-site shape of the legacy `<Confirmation>`
 * component (#1330 PR 5.7).
 *
 * Why a wrapper rather than letting views compose `AlertDialog*` parts
 * directly: every confirmation in the app needs the same six pieces
 * (title, message-as-html-or-text, confirm/cancel labels, danger
 * variant flag, two events). Repeating the AlertDialog scaffolding 12
 * times is mechanical noise; a single wrapper keeps the call-sites
 * close in shape to the old `<Confirmation>` so the migration diff is
 * about *what changes*, not how to assemble shadcn primitives.
 *
 * Event semantics (matches legacy `<Confirmation>`):
 *   - `@confirm` fires when, and only when, the user clicks the
 *     confirm button.
 *   - `@cancel` fires when the user clicks the cancel button.
 *   - Esc / outside-click / X close the dialog without emitting an
 *     event — the parent state is the dialog's `v-model:open`, which
 *     auto-syncs.
 *
 * Why no-event on Esc/outside: an earlier flag-based approach tried
 * to convert every `update:open=false` into either `confirm` or
 * `cancel`, but the order of (a) Reka's auto-close on
 * `<AlertDialogAction>` click and (b) Vue's `@click` handler on the
 * same node is not deterministic across browsers. In CI on
 * 39bf7a5 every delete cascade test timed out because `cancel` fired
 * first (clearing the parent's pending-id ref), then `confirm` fired
 * but found the ref already null and short-circuited before the
 * DELETE went out. Emitting only on the explicit button clicks
 * sidesteps the race entirely; consumers that need to clean up on
 * outside-close can watch `open` directly.
 */
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@design/ui/alert-dialog'
import { buttonVariants } from '@design/ui/button'

withDefaults(
  defineProps<{
    title?: string
    /**
     * Message rendered inside the dialog. Can contain limited inline
     * HTML — that is consistent with the legacy `<Confirmation>` API,
     * which several call-sites rely on (e.g. file delete with bold
     * filename).
     */
    message?: string
    confirmLabel?: string
    cancelLabel?: string
    /**
     * `'danger'` paints the confirm button red; everything else uses
     * the default primary look. Matches the legacy
     * `confirm-button-class` enum at the call-site level.
     */
    variant?: 'default' | 'danger'
    confirmDisabled?: boolean
  }>(),
  {
    title: 'Confirm Action',
    message: 'Are you sure you want to proceed?',
    confirmLabel: 'Confirm',
    cancelLabel: 'Cancel',
    variant: 'default',
    confirmDisabled: false,
  },
)

const open = defineModel<boolean>('open', { default: false })

const emit = defineEmits<{
  (_e: 'confirm'): void
  (_e: 'cancel'): void
}>()

function onConfirmClick() {
  emit('confirm')
}

function onCancelClick() {
  emit('cancel')
}
</script>

<template>
  <AlertDialog v-model:open="open">
    <!-- `.confirmation-modal` is preserved for back-compat with the
         e2e helpers in `e2e/tests/includes/{areas,commodities,locations,exports,uploads}.ts`
         which target the legacy class to wait for the dialog and
         click the confirm button by text. -->
    <AlertDialogContent
      data-testid="app-confirm-dialog"
      class="confirmation-modal"
    >
      <AlertDialogHeader>
        <AlertDialogTitle>{{ title }}</AlertDialogTitle>
        <AlertDialogDescription as="div">
          <!-- eslint-disable-next-line vue/no-v-html -- legacy callers
               pass small inline HTML strings (bold filenames, line
               breaks). Sanitisation is the caller's job, same contract
               as the legacy <Confirmation>. -->
          <div v-html="message" />
        </AlertDialogDescription>
      </AlertDialogHeader>
      <AlertDialogFooter>
        <AlertDialogCancel @click="onCancelClick">
          {{ cancelLabel }}
        </AlertDialogCancel>
        <AlertDialogAction
          :class="
            variant === 'danger'
              ? buttonVariants({ variant: 'destructive' })
              : buttonVariants()
          "
          :disabled="confirmDisabled"
          @click="onConfirmClick"
        >
          {{ confirmLabel }}
        </AlertDialogAction>
      </AlertDialogFooter>
    </AlertDialogContent>
  </AlertDialog>
</template>

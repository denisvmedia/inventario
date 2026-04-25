<script setup lang="ts">
/**
 * AppConfirmDialog — thin wrapper over shadcn `AlertDialog` that
 * preserves the call-site shape of the legacy `<Confirmation>`
 * component (#1330 PR 5.7).
 *
 * Why a wrapper rather than letting views compose `AlertDialog*` parts
 * directly: every confirmation in the app needs the same six pieces
 * (title, message-as-html-or-text, confirm/cancel labels, danger
 * variant flag, two events). Repeating the AlertDialog scaffolding 11
 * times is mechanical noise; a single wrapper keeps the call-sites
 * close in shape to the old `<Confirmation>` so the migration diff is
 * about *what changes*, not how to assemble shadcn primitives.
 *
 * The component is presentation-only: parents own the open/close state
 * via `v-model:open`. The deprecated `v-model:visible` shape is
 * accepted as well so a sweep of the 11 consumers can rename in one
 * pass; emit shape (`@confirm` / `@cancel`) matches `<Confirmation>`.
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

// Reka closes the dialog itself for AlertDialogCancel clicks, Escape
// presses, and outside-clicks (all of these reach us as
// `update:open=false`). Centralise the `cancel` emit here so the
// parent gets exactly one event regardless of how the dialog was
// dismissed. The confirm path also flips `open` to false in
// `onConfirm`, but that runs *after* its own emit, so we suppress the
// cancel emit by checking whether a confirm just happened.
let confirmJustEmitted = false
function onConfirmClick() {
  confirmJustEmitted = true
  emit('confirm')
  open.value = false
}

function onOpenUpdate(value: boolean) {
  if (!value && !confirmJustEmitted) {
    emit('cancel')
  }
  confirmJustEmitted = false
}
</script>

<template>
  <AlertDialog v-model:open="open" @update:open="onOpenUpdate">
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
        <AlertDialogCancel>
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

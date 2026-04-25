<script setup lang="ts">
/**
 * IconButton — accessible icon-only button.
 *
 * A thin wrapper around the shadcn-vue `Button` primitive with two
 * non-negotiable contracts:
 *
 *   1. `ariaLabel` is required at the type level. Icon-only buttons
 *      have no visible text, so without an aria-label they are
 *      invisible to assistive tech (see devdocs/frontend/icons.md
 *      and devdocs/frontend/accessibility.md).
 *   2. The default `type` is `"button"`. Bare `<button>` defaults to
 *      `submit`, which silently submits the nearest enclosing form
 *      when the user clicks an icon-only "Close" / "Edit" affordance.
 *
 * Defaults to `variant="ghost"` and `size="icon"` because that's the
 * shape almost every consumer wants (toolbar buttons, dropdown
 * triggers, inline row actions). Both are overridable via props.
 *
 * The icon is supplied via the default slot — typically a Lucide
 * component sized to the surrounding text. The shadcn `Button`
 * variants already apply `size-4` to nested SVGs that don't carry a
 * `size-*` class, matching the icon scale documented in
 * devdocs/frontend/icons.md.
 */
import type { HTMLAttributes, ButtonHTMLAttributes } from 'vue'
import { Button, type ButtonVariants } from '@design/ui/button'

type Props = {
  /**
   * Accessible name announced by screen readers. Required because the
   * icon itself is `aria-hidden` — without this, the control has no
   * accessible name.
   */
  ariaLabel: string
  variant?: ButtonVariants['variant']
  size?: ButtonVariants['size']
  /** Native button type. Defaults to 'button' to avoid accidental form submits. */
  type?: ButtonHTMLAttributes['type']
  disabled?: boolean
  class?: HTMLAttributes['class']
  testId?: string
}

const props = withDefaults(defineProps<Props>(), {
  variant: 'ghost',
  size: 'icon',
  type: 'button',
  disabled: false,
})

type Emits = {
  click: [event: MouseEvent]
}
const emit = defineEmits<Emits>()

defineSlots<{
  /** The icon to render. Typically a Lucide component (`<X />`, `<Pencil />`, …). */
  default: () => unknown
}>()

function onClick(event: MouseEvent) {
  emit('click', event)
}
</script>

<template>
  <Button
    :variant="props.variant"
    :size="props.size"
    :type="props.type"
    :disabled="props.disabled"
    :aria-label="props.ariaLabel"
    :data-testid="props.testId"
    :class="props.class"
    @click="onClick"
  >
    <slot />
  </Button>
</template>

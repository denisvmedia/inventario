<script setup lang="ts">
/**
 * Banner — inline status / call-to-action strip rendered above content.
 *
 * Replaces the legacy `NotificationBanner.vue` (info / warning / error /
 * success notifications) and `InviteBanner.vue` (single-purpose invite
 * notice) with one consolidated pattern. Existing consumers stay on the
 * legacy components until their owning views migrate in later phases —
 * see migration-conventions.md.
 *
 * The default slot is the message body; an optional `actions` slot
 * renders trailing buttons. `dismissible` adds a close button that emits
 * `dismiss`; the parent owns the visibility state to keep the pattern
 * stateless.
 */
import { computed, type FunctionalComponent, type HTMLAttributes } from 'vue'
import { cva, type VariantProps } from 'class-variance-authority'
import {
  AlertCircle,
  AlertTriangle,
  CheckCircle2,
  Info,
  X,
  type LucideProps,
} from 'lucide-vue-next'
import { cn } from '@design/lib/utils'

const bannerVariants = cva(
  'flex items-start gap-3 rounded-md border px-4 py-3 text-sm shadow-sm',
  {
    variants: {
      variant: {
        info: 'border-blue-200 bg-blue-50 text-blue-900 dark:border-blue-900/50 dark:bg-blue-950/40 dark:text-blue-100',
        success:
          'border-green-200 bg-green-50 text-green-900 dark:border-green-900/50 dark:bg-green-950/40 dark:text-green-100',
        warning:
          'border-amber-200 bg-amber-50 text-amber-900 dark:border-amber-900/50 dark:bg-amber-950/40 dark:text-amber-100',
        error:
          'border-red-200 bg-red-50 text-red-900 dark:border-red-900/50 dark:bg-red-950/40 dark:text-red-100',
      },
    },
    defaultVariants: { variant: 'info' },
  },
)

type BannerVariants = VariantProps<typeof bannerVariants>
type BannerVariant = NonNullable<BannerVariants['variant']>

type Props = {
  variant?: BannerVariant
  dismissible?: boolean
  /** Override the default per-variant icon. Pass `null` to render no icon. */
  icon?: FunctionalComponent<LucideProps> | null
  class?: HTMLAttributes['class']
  testId?: string
}

const props = withDefaults(defineProps<Props>(), {
  variant: 'info',
  dismissible: false,
  icon: undefined,
})

type Emits = {
  dismiss: []
}
const emit = defineEmits<Emits>()

defineSlots<{
  default?: () => unknown
  /** Trailing action buttons (e.g. "Retry"). */
  actions?: () => unknown
}>()

const defaultIconByVariant: Record<BannerVariant, FunctionalComponent<LucideProps>> = {
  info: Info,
  success: CheckCircle2,
  warning: AlertTriangle,
  error: AlertCircle,
}

const resolvedIcon = computed(() => {
  if (props.icon === null) return null
  return props.icon ?? defaultIconByVariant[props.variant]
})
</script>

<template>
  <div
    role="status"
    :class="cn(bannerVariants({ variant }), props.class)"
    :data-testid="testId"
    :data-variant="variant"
  >
    <component :is="resolvedIcon" v-if="resolvedIcon" class="size-5 shrink-0 mt-0.5" aria-hidden="true" />
    <div class="flex-1 min-w-0">
      <slot />
    </div>
    <div v-if="$slots.actions" class="flex items-center gap-2 shrink-0">
      <slot name="actions" />
    </div>
    <button
      v-if="dismissible"
      type="button"
      class="shrink-0 rounded-sm p-1 opacity-70 hover:opacity-100 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
      aria-label="Dismiss"
      @click="emit('dismiss')"
    >
      <X class="size-4" aria-hidden="true" />
    </button>
  </div>
</template>

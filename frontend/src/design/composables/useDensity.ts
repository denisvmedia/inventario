import { computed, ref, watch } from 'vue'

export type Density = 'comfortable' | 'compact'

const STORAGE_KEY = 'inventario:density'
const VALID: readonly Density[] = ['comfortable', 'compact'] as const

const density = ref<Density>(readInitial())

function readInitial(): Density {
  if (typeof window === 'undefined') return 'comfortable'
  const stored = window.localStorage.getItem(STORAGE_KEY)
  if (stored && (VALID as readonly string[]).includes(stored)) {
    return stored as Density
  }
  return 'comfortable'
}

function applyToDocument(value: Density) {
  if (typeof document === 'undefined') return
  const root = document.documentElement
  if (value === 'compact') {
    root.dataset.density = 'compact'
  } else {
    delete root.dataset.density
  }
}

watch(
  density,
  (next) => {
    applyToDocument(next)
    if (typeof window !== 'undefined') {
      window.localStorage.setItem(STORAGE_KEY, next)
    }
  },
  { immediate: true },
)

/**
 * Reactive access to the app density preference. Persisted to localStorage
 * and reflected on `<html data-density="...">`. Components that opt in read
 * the corresponding tokens from `tokens/spacing.css`.
 */
export function useDensity() {
  return {
    density: computed(() => density.value),
    setDensity(next: Density) {
      if ((VALID as readonly string[]).includes(next)) {
        density.value = next
      }
    },
  }
}

export function initDensityOnBoot() {
  applyToDocument(density.value)
}

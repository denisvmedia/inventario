<template>
  <div class="app">
    <!-- vue-sonner Toaster — the only toast host now that PR 5.6
         (#1330) removed the PrimeVue Toast stack. Every former
         `useToast` call-site has been migrated to `useAppToast`. -->
    <Toaster />

    <!-- Layout shell (#1326 PR 1.6). Auth views own their own full-bleed
         layout via @design/patterns/AuthCard, so the global header,
         footer and centred .container wrapper are suppressed on those
         routes. Print routes keep their existing minimal shell. -->
    <AppHeader v-if="!isPrintRoute && !isAuthRoute" />

    <main
      :class="{
        container: !isPrintRoute && !isAuthRoute,
        'print-container': isPrintRoute,
      }"
    >
      <router-view />
    </main>

    <AppFooter v-if="!isPrintRoute && !isAuthRoute" />

    <!-- Global Cmd+K / Ctrl+K command palette (#1330 PR 5.4). Mounted
         only on authenticated, group-scoped routes — the search endpoint
         is mounted under `/g/{slug}/search` so the palette is useless
         (and the request 404s) without a slug on the current route. -->
    <CommandPalette
      v-if="!isPrintRoute && !isAuthRoute && authStore.isAuthenticated && hasGroupSlug"
      v-model:open="commandPaletteOpen"
    />

    <!-- Keyboard shortcut cheatsheet (#1331 PR 6.6). Bound to `?`. -->
    <KeyboardShortcutsCheatsheet
      v-if="!isPrintRoute && !isAuthRoute"
      v-model="cheatsheetOpen"
    />

    <!-- Global confirmation host bound to `confirmationStore`. The
         strangler-fig `useConfirm` composable (and the legacy
         `confirmationUtil.confirm`) both call `store.show()` and await
         a resolution promise; without a host component bound to the
         store the dialog never renders, the promise never resolves,
         and Delete actions in views still using `useConfirm`
         (Area, Location, Commodity detail) hang. PR 5.7 (#1330)
         deleted the legacy `<Confirmation>` mount but several Phase 4
         migrated views still rely on the promise-returning facade,
         so we re-host using the new `AppConfirmDialog`. -->
    <AppConfirmDialog
      v-model:open="confirmationStore.isVisible"
      :title="confirmationStore.title"
      :message="confirmationStore.message"
      :confirm-label="confirmationStore.confirmLabel"
      :cancel-label="confirmationStore.cancelLabel"
      :variant="confirmationStore.confirmButtonClass === 'danger' ? 'danger' : 'default'"
      @confirm="confirmationStore.confirm"
      @cancel="confirmationStore.cancel"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useSettingsStore } from '@/stores/settingsStore'
import { useAuthStore } from '@/stores/authStore'
import { useGroupStore } from '@/stores/groupStore'
import { useConfirmationStore } from '@/stores/confirmationStore'
import { Toaster } from '@design/ui/sonner'
import AppHeader from '@design/patterns/AppHeader.vue'
import AppFooter from '@design/patterns/AppFooter.vue'
import AppConfirmDialog from '@design/patterns/AppConfirmDialog.vue'
import CommandPalette from '@design/patterns/CommandPalette.vue'
import KeyboardShortcutsCheatsheet from '@design/patterns/KeyboardShortcutsCheatsheet.vue'
import { useKeyboardShortcuts } from '@design/composables/useKeyboardShortcuts'

const route = useRoute()
const router = useRouter()
const settingsStore = useSettingsStore()
const authStore = useAuthStore()
const groupStore = useGroupStore()
const confirmationStore = useConfirmationStore()

// Routes whose views own their full-bleed layout via @design/patterns/AuthCard
// (#1326 PR 1.6). Listed by route name so the gate stays robust against
// future path renames; `meta.requiresAuth === false` is intentionally
// not used as the predicate because /invite/:token is also public but
// keeps the global header/footer.
const AUTH_ROUTE_NAMES = new Set([
  'login',
  'register',
  'forgot-password',
  'reset-password',
  'verify-email',
])

const isPrintRoute = computed(() => route.path.includes('/print'))
const isAuthRoute = computed(() => {
  const name = typeof route.name === 'string' ? route.name : ''
  return AUTH_ROUTE_NAMES.has(name)
})

// Cmd+K / Ctrl+K opens the global CommandPalette. Bound here (App.vue)
// instead of inside the pattern so the hotkey works even before the
// palette dialog has mounted its own listeners. The palette queries
// `/api/v1/search`, which is mounted server-side only inside
// `/g/{slug}/`; the axios interceptor declines to rewrite the URL when
// the current route has no `:groupSlug` param. Gate the shortcut on a
// group-scoped route so Cmd+K from `/`, `/profile`, `/no-group` etc.
// is a no-op rather than firing a request that 404s.
const commandPaletteOpen = ref(false)
const cheatsheetOpen = ref(false)
const hasGroupSlug = computed(
  () => typeof route.params.groupSlug === 'string' && route.params.groupSlug !== '',
)

// Two-key sequence buffer for `g _` navigation shortcuts (#1331 PR 6.6).
// The `g` keystroke arms the buffer for a short window; the next key either
// completes a navigation shortcut (h / l / c / f) or clears the buffer.
// Any *other* key (or a click / route change / focus into a text field)
// also clears it so a stale `g` press can't unexpectedly turn a later
// `h/l/c/f` keystroke into a navigation. Without this guard, "g … type
// 'l' inside a search input … blur input … press 'l'" could navigate.
const G_BUFFER_TIMEOUT_MS = 1200
const G_NAVIGATION_KEYS = new Set(['g', 'h', 'l', 'c', 'f'])
const gBufferActive = ref(false)
let gBufferTimer: ReturnType<typeof setTimeout> | null = null

function clearGBuffer() {
  gBufferActive.value = false
  if (gBufferTimer !== null) {
    clearTimeout(gBufferTimer)
    gBufferTimer = null
  }
}

function armGBuffer() {
  gBufferActive.value = true
  if (gBufferTimer !== null) clearTimeout(gBufferTimer)
  gBufferTimer = setTimeout(clearGBuffer, G_BUFFER_TIMEOUT_MS)
}

function isInsideTextField(target: EventTarget | null): boolean {
  if (!(target instanceof HTMLElement)) return false
  const tag = target.tagName
  if (tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT') return true
  return target.isContentEditable
}

// Cancel any armed `g` buffer when the next keystroke isn't a recognised
// completion (`h/l/c/f`) or a re-arming `g`. Runs alongside
// useKeyboardShortcuts on the same `keydown` event; ordering doesn't matter
// because navigation handlers consume the buffer before the listener fires.
function onAnyKeydown(event: KeyboardEvent) {
  if (!gBufferActive.value) return
  if (isInsideTextField(event.target)) {
    clearGBuffer()
    return
  }
  if (!G_NAVIGATION_KEYS.has(event.key.toLowerCase())) {
    clearGBuffer()
  }
}

onMounted(() => {
  window.addEventListener('keydown', onAnyKeydown)
})
onBeforeUnmount(() => {
  window.removeEventListener('keydown', onAnyKeydown)
})

function navigateWithinGroup(target: string) {
  if (!hasGroupSlug.value) return
  const slug = route.params.groupSlug as string
  router.push(`/g/${slug}${target}`)
}

useKeyboardShortcuts([
  {
    key: 'k',
    modifiers: ['mod'],
    handler: (event) => {
      if (isPrintRoute.value || isAuthRoute.value) return
      if (!authStore.isAuthenticated) return
      if (!hasGroupSlug.value) return
      event.preventDefault()
      commandPaletteOpen.value = true
    },
  },
  {
    key: '?',
    modifiers: ['shift'],
    ignoreInInput: true,
    handler: (event) => {
      if (isPrintRoute.value || isAuthRoute.value) return
      event.preventDefault()
      cheatsheetOpen.value = true
    },
  },
  {
    key: '/',
    ignoreInInput: true,
    handler: (event) => {
      if (isPrintRoute.value || isAuthRoute.value) return
      if (!authStore.isAuthenticated) return
      if (!hasGroupSlug.value) return
      event.preventDefault()
      commandPaletteOpen.value = true
    },
  },
  {
    key: 'g',
    ignoreInInput: true,
    handler: (event) => {
      if (isPrintRoute.value || isAuthRoute.value) return
      if (!authStore.isAuthenticated) return
      // Prevent default so browser type-ahead-find / focus-on-body
      // behaviour doesn't fire alongside the buffer being armed.
      event.preventDefault()
      armGBuffer()
    },
  },
  {
    key: 'h',
    ignoreInInput: true,
    handler: (event) => {
      if (!gBufferActive.value) return
      event.preventDefault()
      clearGBuffer()
      navigateWithinGroup('/')
    },
  },
  {
    key: 'l',
    ignoreInInput: true,
    handler: (event) => {
      if (!gBufferActive.value) return
      event.preventDefault()
      clearGBuffer()
      navigateWithinGroup('/locations')
    },
  },
  {
    key: 'c',
    ignoreInInput: true,
    handler: (event) => {
      if (!gBufferActive.value) return
      event.preventDefault()
      clearGBuffer()
      navigateWithinGroup('/commodities')
    },
  },
  {
    key: 'f',
    ignoreInInput: true,
    handler: (event) => {
      if (!gBufferActive.value) return
      event.preventDefault()
      clearGBuffer()
      navigateWithinGroup('/files')
    },
  },
])

// bootstrapForAuthenticatedUser loads the data the SPA needs the moment the
// user becomes authenticated: main currency shim (no-op now, kept for back-
// compat) and the group list. ensureLoaded is single-flight — the router
// guard also calls it on the first navigation, but only one /api/v1/groups
// request actually hits the wire. The zero-group redirect lives in the
// router guard (#1261) so every protected route is covered, not just '/'.
async function bootstrapForAuthenticatedUser(): Promise<void> {
  await settingsStore.fetchMainCurrency()
  try {
    await groupStore.ensureLoaded()
  } catch (err) {
    console.warn('Failed to initialize groups:', err)
  }
}

// Initialize global settings when the app starts.
// Two entry points matter:
//   1. The user was already authenticated at mount time (page reload, deep
//      link that includes a valid JWT in localStorage). Handled by
//      onMounted.
//   2. The user logs in after the page is already mounted (the e2e flow:
//      fresh context → / redirects to /login → form submit → SPA restores
//      session without re-mounting App.vue). Handled by the watch on
//      authStore.isAuthenticated.
// Before this watch existed, case (2) never bootstrapped the group list,
// so `.group-selector` stayed hidden and every post-login UI assertion
// depending on a populated groupStore raced or failed.
onMounted(async () => {
  if (authStore.isAuthenticated) {
    await bootstrapForAuthenticatedUser()
  }
})

watch(
  () => authStore.isAuthenticated,
  async (isAuth, wasAuth) => {
    if (isAuth && !wasAuth) {
      await bootstrapForAuthenticatedUser()
    }
    // On explicit sign-out, drop any group state so the next login starts
    // from a clean slate (otherwise stale groups[] could briefly render).
    if (!isAuth && wasAuth) {
      groupStore.clearAll()
    }
  }
)
</script>

<style lang="scss">
@use './assets/variables' as *;

.print-container {
  max-width: 100%;
  margin: 0;
  padding: 0;
}

.group-role-cluster {
  display: inline-flex;
  align-items: center;
  gap: $header-control-gap;
}

// Role indicator sits next to the GroupSelector trigger and mirrors its
// visual language (border, padding, font-size, radius) so the pair reads
// as one unit. It's intentionally non-interactive — selecting a different
// role isn't a thing; the role follows the active group.
.role-indicator {
  display: inline-flex;
  align-items: center;
  padding: $header-control-padding-y $header-control-padding-x;
  border: 1px solid $header-control-border-color;
  border-radius: $header-control-radius;
  font-size: $header-control-font-size;
  line-height: 1.2;
  color: inherit;
  background: none;
  text-transform: capitalize;
  letter-spacing: 0.02em;

  &--admin {
    border-color: rgb(76 175 80 / 70%);
    background: rgb(76 175 80 / 15%);
  }

  &--user {
    border-color: rgb(108 117 125 / 70%);
    background: rgb(108 117 125 / 18%);
  }
}

.header-content {
  display: flex;
  align-items: center;
  max-width: 1200px;
  margin: 0 auto;
  padding: 0 1rem;
  justify-content: space-between;
}

.logo-container {
  margin-right: 2rem;
}

.logo {
  height: 40px;
  width: auto;
  vertical-align: middle;
  transition: transform 0.2s ease;

  &:hover {
    transform: scale(1.05);
  }
}

.user-info {
  display: flex;
  align-items: center;
  margin-left: auto;
  position: relative;
}

.user-menu-trigger {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  color: white;
  font-size: 0.9rem;
  padding: 0.5rem 1rem;
  background-color: rgb(255 255 255 / 10%);
  border-radius: 4px;
  border: 1px solid rgb(255 255 255 / 20%);
  cursor: pointer;
  transition: background-color 0.2s ease;

  &:hover {
    background-color: rgb(255 255 255 / 20%);
  }

  .menu-chevron {
    font-size: 0.75rem;
    opacity: 0.8;
  }
}

.user-dropdown {
  position: absolute;
  top: calc(100% + 0.4rem);
  right: 0;
  min-width: 160px;
  background: white;
  border: 1px solid rgb(0 0 0 / 12%);
  border-radius: 6px;
  box-shadow: 0 4px 16px rgb(0 0 0 / 15%);
  z-index: 1000;
  overflow: hidden;
}

.dropdown-item {
  display: flex;
  align-items: center;
  gap: 0.6rem;
  width: 100%;
  padding: 0.65rem 1rem;
  font-size: 0.9rem;
  color: #333;
  text-decoration: none;
  background: none;
  border: none;
  cursor: pointer;
  text-align: left;
  transition: background-color 0.15s ease;

  &:hover {
    background-color: #f5f5f5;
  }

  &--logout {
    color: #c0392b;

    &:hover {
      background-color: #fff5f5;
    }
  }
}

@media (width <= 768px) {
  .header-content {
    flex-direction: column;
    align-items: center;
  }

  .logo-container {
    margin-right: 0;
    margin-bottom: 1rem;
  }

  .logo {
    height: 35px;
  }

  .user-info {
    margin-left: 0;
    margin-top: 1rem;
  }
}

@media print {
  .app {
    padding: 0;
    margin: 0;
  }
}
</style>

<template>
  <div class="app">
    <!-- Global Toast component -->
    <Toast />
    <!-- vue-sonner Toaster — new toast stack (Epic #1324). Co-exists
         with the PrimeVue <Toast /> above during the strangler-fig
         migration; both hosts stay until every call-site has been
         switched from useToast to useAppToast. -->
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
         only on authenticated, non-auth routes — the dialog needs a
         signed-in API session and a populated groupStore for its
         search results to be meaningful. -->
    <CommandPalette
      v-if="!isPrintRoute && !isAuthRoute && authStore.isAuthenticated"
      v-model:open="commandPaletteOpen"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useSettingsStore } from '@/stores/settingsStore'
import { useAuthStore } from '@/stores/authStore'
import { useGroupStore } from '@/stores/groupStore'
// eslint-disable-next-line @typescript-eslint/no-restricted-imports -- removed in #1330
import Toast from 'primevue/toast'
import { Toaster } from '@design/ui/sonner'
import AppHeader from '@design/patterns/AppHeader.vue'
import AppFooter from '@design/patterns/AppFooter.vue'
import CommandPalette from '@design/patterns/CommandPalette.vue'
import { useKeyboardShortcuts } from '@design/composables/useKeyboardShortcuts'

const route = useRoute()
const settingsStore = useSettingsStore()
const authStore = useAuthStore()
const groupStore = useGroupStore()

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
// palette dialog has mounted its own listeners.
const commandPaletteOpen = ref(false)
useKeyboardShortcuts([
  {
    key: 'k',
    modifiers: ['mod'],
    handler: (event) => {
      if (isPrintRoute.value || isAuthRoute.value) return
      if (!authStore.isAuthenticated) return
      event.preventDefault()
      commandPaletteOpen.value = true
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

<template>
  <div class="app">
    <!-- Global Toast component -->
    <Toast />
    <!-- vue-sonner Toaster — new toast stack (Epic #1324). Co-exists
         with the PrimeVue <Toast /> above during the strangler-fig
         migration; both hosts stay until every call-site has been
         switched from useToast to useAppToast. -->
    <Toaster />

    <!-- Global confirmation dialog component -->
    <header v-if="!isPrintRoute">
      <div class="header-content">
        <div class="logo-container">
          <router-link to="/">
            <img src="/favicon.png" alt="Inventario Logo" class="logo" />
          </router-link>
        </div>
        <nav>
          <router-link to="/" :class="{ 'custom-active': isHomeActive }">Home</router-link> |
          <router-link :to="groupPath('/locations')" :class="{ 'custom-active': isLocationsActive }">Locations</router-link> |
          <router-link :to="groupPath('/commodities')" :class="{ 'custom-active': isCommoditiesActive }">Commodities</router-link> |
          <router-link :to="groupPath('/files')" :class="{ 'custom-active': isFilesActive }">Files</router-link> |
          <router-link :to="groupPath('/exports')" :class="{ 'custom-active': isExportsActive }">Exports</router-link> |
          <router-link :to="groupPath('/system')" :class="{ 'custom-active': isSystemActive }">System</router-link>
        </nav>
        <!-- Group selector + current role badge are two facets of the same
             "my identity in this context" display (#1258). They live in a
             flex cluster so the pair reads as one unit and stays together
             when the header wraps on narrow viewports. -->
        <div
          v-if="authStore.isAuthenticated && groupStore.hasGroups"
          class="group-role-cluster"
        >
          <GroupSelector />
          <span
            v-if="groupStore.currentRole"
            class="role-indicator"
            :class="`role-indicator--${groupStore.currentRole}`"
            data-testid="current-role"
            :title="groupStore.currentRole === 'admin'
              ? 'You are an admin of the current group'
              : 'You are a member of the current group'"
          >
            {{ groupStore.currentRole }}
          </span>
        </div>
        <div v-if="authStore.isAuthenticated" ref="userMenuRef" class="user-info">
          <button
            class="user-menu-trigger"
            data-testid="user-menu"
            :aria-expanded="isMenuOpen"
            aria-haspopup="true"
            @click.stop="isMenuOpen = !isMenuOpen"
          >
            <span data-testid="current-user">{{ authStore.userName || authStore.userEmail }}</span>
            <font-awesome-icon :icon="isMenuOpen ? 'chevron-up' : 'chevron-down'" class="menu-chevron" />
          </button>
          <div v-if="isMenuOpen" class="user-dropdown">
            <router-link to="/profile" class="dropdown-item" @click="isMenuOpen = false">
              <font-awesome-icon icon="user" /> Profile
            </router-link>
            <button class="dropdown-item dropdown-item--logout" @click="handleLogout">
              <font-awesome-icon icon="right-from-bracket" /> Logout
            </button>
          </div>
        </div>
      </div>
    </header>

    <!-- Debug information -->
    <div v-if="false" class="debug-info">
      <p>Current route: {{ $route.path }}</p>
    </div>

    <main :class="{ 'container': !isPrintRoute, 'print-container': isPrintRoute }">
      <router-view />
    </main>

    <footer v-if="!isPrintRoute">
      <p>Inventario &copy; {{ new Date().getFullYear() }}</p>
    </footer>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useSettingsStore } from '@/stores/settingsStore'
import { useAuthStore } from '@/stores/authStore'
import { useGroupStore } from '@/stores/groupStore'
import GroupSelector from '@/components/GroupSelector.vue'
// eslint-disable-next-line @typescript-eslint/no-restricted-imports -- removed in #1330
import Toast from 'primevue/toast'
import { Toaster } from '@design/ui/sonner'

const route = useRoute()
const router = useRouter()
const settingsStore = useSettingsStore()
const authStore = useAuthStore()
const groupStore = useGroupStore()

// User dropdown menu state
const isMenuOpen = ref(false)
const userMenuRef = ref<HTMLElement | null>(null)

function handleClickOutside(event: MouseEvent) {
  if (userMenuRef.value && !userMenuRef.value.contains(event.target as Node)) {
    isMenuOpen.value = false
  }
}

async function handleLogout() {
  isMenuOpen.value = false
  await authStore.logout()
  await router.push('/login')
}

// Check if current route is a print route
const isPrintRoute = computed(() => {
  return route.path.includes('/print')
})

// groupPath is re-exported from the store so template bindings like
// :to="groupPath('/locations')" keep working. The store owns the single
// source of truth for /g/<slug>/ URL construction (see groupStore.ts).
const groupPath = groupStore.groupPath

// Each nav link highlights when the URL (stripped of the optional
// /g/<slug> prefix) starts with its section root. Matching against the
// group-prefixed and flat forms keeps the highlight correct for both the
// new canonical URLs and any legacy bookmarks still in the wild.
function sectionPathMatches(...prefixes: string[]): boolean {
  const raw = route.path
  const slug = typeof route.params.groupSlug === 'string' ? route.params.groupSlug : ''
  const stripped = slug ? raw.replace(`/g/${encodeURIComponent(slug)}`, '') : raw
  return prefixes.some((p) => stripped.startsWith(p))
}

// Computed properties to determine active navigation sections
const isHomeActive = computed(() => route.path === '/')
const isLocationsActive = computed(() => sectionPathMatches('/locations', '/areas'))
const isCommoditiesActive = computed(() => sectionPathMatches('/commodities'))
const isFilesActive = computed(() => sectionPathMatches('/files'))
const isExportsActive = computed(() => sectionPathMatches('/exports'))
const isSystemActive = computed(() => sectionPathMatches('/system'))

// Admin active state removed — user management is now per-group

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
  document.addEventListener('click', handleClickOutside)
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

onUnmounted(() => {
  document.removeEventListener('click', handleClickOutside)
})
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

<script setup lang="ts">
/**
 * AppHeader — site-wide header rendered above the router-view.
 *
 * Extracted from App.vue as part of the Phase 1 layout shell migration
 * (#1326). DOM, classes and `data-testid` hooks are preserved verbatim
 * so the existing Playwright suites (profile.spec, groups.spec,
 * includes/auth.ts, includes/multi-user-auth.ts,
 * includes/user-isolation-auth.ts) keep matching:
 *
 *   - [data-testid="user-menu"]
 *   - [data-testid="current-user"]
 *   - [data-testid="current-role"]
 *   - .user-dropdown
 *   - .group-role-cluster
 *   - .group-selector__trigger / .group-selector__name (owned by GroupSelector)
 *
 * Icon swap (FA → Lucide via @design/lib/icons) is deferred to PR 1.5
 * so this commit stays a pure structural move.
 */
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/authStore'
import { useGroupStore } from '@/stores/groupStore'
import GroupSelector from '@/components/GroupSelector.vue'

const route = useRoute()
const router = useRouter()
const authStore = useAuthStore()
const groupStore = useGroupStore()

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

// groupPath is re-exported from the store so template bindings like
// :to="groupPath('/locations')" keep working. The store owns the single
// source of truth for /g/<slug>/ URL construction (see groupStore.ts).
const groupPath = groupStore.groupPath

// Each nav link highlights when the URL (stripped of the optional
// /g/<slug> prefix) starts with its section root.
function sectionPathMatches(...prefixes: string[]): boolean {
  const raw = route.path
  const slug = typeof route.params.groupSlug === 'string' ? route.params.groupSlug : ''
  const stripped = slug ? raw.replace(`/g/${encodeURIComponent(slug)}`, '') : raw
  return prefixes.some((p) => stripped.startsWith(p))
}

const isHomeActive = computed(() => route.path === '/')
const isLocationsActive = computed(() => sectionPathMatches('/locations', '/areas'))
const isCommoditiesActive = computed(() => sectionPathMatches('/commodities'))
const isFilesActive = computed(() => sectionPathMatches('/files'))
const isExportsActive = computed(() => sectionPathMatches('/exports'))
const isSystemActive = computed(() => sectionPathMatches('/system'))

onMounted(() => {
  document.addEventListener('click', handleClickOutside)
})

onUnmounted(() => {
  document.removeEventListener('click', handleClickOutside)
})
</script>

<template>
  <header data-testid="app-header">
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
</template>

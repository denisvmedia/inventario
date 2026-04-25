import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { createRouter, createWebHistory } from 'vue-router'

// Mocks have to be declared before the component import so vi.mock
// hoisting picks them up.
const mockAuthStore = {
  isAuthenticated: false,
  userName: null as string | null,
  userEmail: null as string | null,
  logout: vi.fn().mockResolvedValue(undefined),
}

const mockGroupStore = {
  hasGroups: false,
  currentRole: null as 'admin' | 'user' | null,
  currentGroupSlug: null as string | null,
  groupPath(subpath: string): string {
    const slug = mockGroupStore.currentGroupSlug
    if (!slug) return '/no-group'
    const normalized = subpath.startsWith('/') ? subpath : `/${subpath}`
    return `/g/${encodeURIComponent(slug)}${normalized}`
  },
}

vi.mock('@/stores/authStore', () => ({
  useAuthStore: () => mockAuthStore,
}))

vi.mock('@/stores/groupStore', () => ({
  useGroupStore: () => mockGroupStore,
}))

import AppHeader from '../AppHeader.vue'

const SLUG = 'home'

function buildRouter() {
  return createRouter({
    history: createWebHistory(),
    routes: [
      { path: '/', component: { template: '<div />' } },
      { path: '/no-group', component: { template: '<div />' } },
      { path: '/login', component: { template: '<div />' } },
      { path: '/profile', component: { template: '<div />' } },
      {
        path: '/g/:groupSlug',
        component: { template: '<router-view />' },
        children: [
          { path: 'locations', component: { template: '<div />' } },
          { path: 'commodities', component: { template: '<div />' } },
          { path: 'files', component: { template: '<div />' } },
          { path: 'exports', component: { template: '<div />' } },
          { path: 'system', component: { template: '<div />' } },
        ],
      },
    ],
  })
}

async function mountHeader(initialPath = '/') {
  const router = buildRouter()
  await router.push(initialPath)
  const wrapper = mount(AppHeader, {
    global: {
      plugins: [router],
      stubs: { GroupSelector: true },
    },
  })
  await wrapper.vm.$nextTick()
  return wrapper
}

describe('AppHeader', () => {
  beforeEach(() => {
    mockAuthStore.isAuthenticated = false
    mockAuthStore.userName = null
    mockAuthStore.userEmail = null
    mockGroupStore.hasGroups = false
    mockGroupStore.currentRole = null
    mockGroupStore.currentGroupSlug = null
  })

  afterEach(() => {
    vi.clearAllMocks()
  })

  it('renders inside a <header> with the app-header testid', async () => {
    const wrapper = await mountHeader()
    expect(wrapper.element.tagName).toBe('HEADER')
    expect(wrapper.attributes('data-testid')).toBe('app-header')
  })

  it('hides the user menu and group cluster when unauthenticated', async () => {
    const wrapper = await mountHeader()

    expect(wrapper.find('[data-testid="user-menu"]').exists()).toBe(false)
    expect(wrapper.find('.group-role-cluster').exists()).toBe(false)
  })

  it('renders user-menu and current-user when authenticated', async () => {
    mockAuthStore.isAuthenticated = true
    mockAuthStore.userName = 'Alice'
    const wrapper = await mountHeader()

    const trigger = wrapper.get('[data-testid="user-menu"]')
    expect(trigger.attributes('aria-expanded')).toBe('false')
    expect(wrapper.get('[data-testid="current-user"]').text()).toBe('Alice')
  })

  it('falls back to userEmail when userName is missing', async () => {
    mockAuthStore.isAuthenticated = true
    mockAuthStore.userEmail = 'alice@example.com'
    const wrapper = await mountHeader()

    expect(wrapper.get('[data-testid="current-user"]').text()).toBe('alice@example.com')
  })

  it('opens the dropdown when the trigger is clicked', async () => {
    mockAuthStore.isAuthenticated = true
    mockAuthStore.userName = 'Alice'
    const wrapper = await mountHeader()

    expect(wrapper.find('.user-dropdown').exists()).toBe(false)
    await wrapper.get('[data-testid="user-menu"]').trigger('click')
    expect(wrapper.find('.user-dropdown').exists()).toBe(true)
    expect(wrapper.get('[data-testid="user-menu"]').attributes('aria-expanded')).toBe('true')
  })

  it('renders Lucide icons (svg) inside the trigger and dropdown items', async () => {
    mockAuthStore.isAuthenticated = true
    mockAuthStore.userName = 'Alice'
    const wrapper = await mountHeader()

    // Chevron lives on the trigger — present in both states; verify it is
    // an inline SVG (Lucide), not a font-awesome-icon stub.
    expect(wrapper.get('.menu-chevron').element.tagName.toLowerCase()).toBe('svg')

    await wrapper.get('[data-testid="user-menu"]').trigger('click')
    const profile = wrapper.get('a[href="/profile"]')
    const logout = wrapper.get('.dropdown-item--logout')
    expect(profile.find('svg').exists()).toBe(true)
    expect(logout.find('svg').exists()).toBe(true)
    // Decorative icons must not be exposed to assistive tech.
    expect(profile.get('svg').attributes('aria-hidden')).toBe('true')
    expect(logout.get('svg').attributes('aria-hidden')).toBe('true')
  })

  it('renders the role indicator inside the cluster when role is present', async () => {
    mockAuthStore.isAuthenticated = true
    mockGroupStore.hasGroups = true
    mockGroupStore.currentRole = 'admin'
    const wrapper = await mountHeader()

    const cluster = wrapper.get('.group-role-cluster')
    const badge = cluster.get('[data-testid="current-role"]')
    expect(badge.text()).toBe('admin')
    expect(badge.classes()).toContain('role-indicator--admin')
  })

  it('omits the cluster when the user has no groups', async () => {
    mockAuthStore.isAuthenticated = true
    mockGroupStore.hasGroups = false
    mockGroupStore.currentRole = 'admin'
    const wrapper = await mountHeader()

    expect(wrapper.find('.group-role-cluster').exists()).toBe(false)
  })

  it('highlights the active section based on the current route', async () => {
    mockGroupStore.currentGroupSlug = SLUG
    const wrapper = await mountHeader(`/g/${SLUG}/locations`)

    const link = wrapper.get(`nav a[href="/g/${SLUG}/locations"]`)
    expect(link.classes()).toContain('custom-active')
  })
})

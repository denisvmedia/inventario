import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { createRouter, createWebHistory } from 'vue-router'
import App from '@/App.vue'

// Mock the settings store
const mockSettingsStore = {
  fetchMainCurrency: vi.fn().mockResolvedValue(undefined),
  mainCurrency: { id: 'usd', code: 'USD', name: 'US Dollar' }
}

const mockAuthStore = {
  isAuthenticated: false,
  userName: null,
  userEmail: null
}

vi.mock('@/stores/settingsStore', () => ({
  useSettingsStore: () => mockSettingsStore
}))

vi.mock('@/stores/authStore', () => ({
  useAuthStore: () => mockAuthStore
}))

const mockGroupStore = {
  groups: [],
  currentGroup: null,
  currentMembership: null,
  hasGroups: false,
  currentGroupSlug: null as string | null,
  currentGroupName: null,
  currentGroupIcon: null,
  currentRole: null as 'admin' | 'user' | null,
  isGroupAdmin: false,
  isGroupUser: false,
  fetchGroups: vi.fn().mockResolvedValue(undefined),
  ensureLoaded: vi.fn().mockResolvedValue(undefined),
  restoreFromPreference: vi.fn().mockResolvedValue(undefined),
  clearAll: vi.fn(),
  // groupPath mirrors the real store (#1321): when a slug is active it
  // produces /g/<slug>/<subpath>, otherwise it returns /no-group so nav
  // links rendered before a group resolves don't end up on the 404 route.
  groupPath(subpath: string): string {
    const slug = mockGroupStore.currentGroupSlug
    if (!slug) return '/no-group'
    const normalized = subpath.startsWith('/') ? subpath : `/${subpath}`
    return `/g/${encodeURIComponent(slug)}${normalized}`
  },
}

vi.mock('@/stores/groupStore', () => ({
  useGroupStore: () => mockGroupStore
}))

// App.vue mounts a global <Confirmation /> bound to confirmationStore so
// useConfirm() callers (Area/Commodity/Location/File detail views) get a
// host for the dialog. The shape mirrors the Pinia setup store.
const mockConfirmationStore = {
  isVisible: false,
  title: 'Confirm Action',
  message: 'Are you sure?',
  confirmLabel: 'Confirm',
  cancelLabel: 'Cancel',
  confirmButtonClass: 'primary',
  show: vi.fn().mockResolvedValue(false),
  hide: vi.fn(),
  confirm: vi.fn(),
  cancel: vi.fn(),
}

vi.mock('@/stores/confirmationStore', () => ({
  useConfirmationStore: () => mockConfirmationStore
}))

describe('App.vue Navigation', () => {
  // All data routes are group-scoped under /g/:groupSlug/... (#1321).
  // The nav renders scoped hrefs via groupStore.groupPath(), and the
  // active-class logic in sectionPathMatches strips the /g/<slug> prefix
  // from route.path before comparing against the section root.
  const SLUG = 'home'

  const createRouterForTest = () => {
    return createRouter({
      history: createWebHistory(),
      routes: [
        { path: '/', component: { template: '<div>Home</div>' } },
        // /no-group is the fallback groupStore.groupPath() returns when no
        // slug is active; register it here so router-link resolution during
        // the "no groups" / unauthenticated tests doesn't warn.
        { path: '/no-group', component: { template: '<div>No Group</div>' } },
        {
          path: '/g/:groupSlug',
          component: { template: '<router-view />' },
          children: [
            { path: 'locations', component: { template: '<div>Locations</div>' } },
            { path: 'locations/:id', component: { template: '<div>Location Detail</div>' } },
            { path: 'areas/:id', component: { template: '<div>Area Detail</div>' } },
            { path: 'commodities', component: { template: '<div>Commodities</div>' } },
            { path: 'commodities/new', component: { template: '<div>Commodity Create</div>' } },
            { path: 'commodities/:id', component: { template: '<div>Commodity Detail</div>' } },
            { path: 'files', component: { template: '<div>Files</div>' } },
            { path: 'exports', component: { template: '<div>Exports</div>' } },
            { path: 'exports/:id', component: { template: '<div>Export Detail</div>' } },
            { path: 'exports/new', component: { template: '<div>Export Create</div>' } },
            { path: 'system', component: { template: '<div>System</div>' } },
            { path: 'system/settings/:id', component: { template: '<div>System Setting Detail</div>' } },
          ],
        },
      ],
    })
  }

  beforeEach(() => {
    mockGroupStore.currentGroupSlug = SLUG
  })

  afterEach(() => {
    mockGroupStore.currentGroupSlug = null
  })

  it('highlights Home menu when on home page', async () => {
    const router = createRouterForTest()
    await router.push('/')

    const wrapper = mount(App, {
      global: {
        plugins: [router]
      }
    })

    await wrapper.vm.$nextTick()

    const homeLink = wrapper.find('nav a[href="/"]')
    expect(homeLink.classes()).toContain('custom-active')
  })

  it('highlights Locations menu when on locations list page', async () => {
    const router = createRouterForTest()
    await router.push(`/g/${SLUG}/locations`)

    const wrapper = mount(App, {
      global: {
        plugins: [router]
      }
    })

    await wrapper.vm.$nextTick()

    const locationsLink = wrapper.find(`nav a[href="/g/${SLUG}/locations"]`)
    expect(locationsLink.classes()).toContain('custom-active')
  })

  it('highlights Locations menu when on location detail page', async () => {
    const router = createRouterForTest()
    await router.push(`/g/${SLUG}/locations/123`)

    const wrapper = mount(App, {
      global: {
        plugins: [router]
      }
    })

    await wrapper.vm.$nextTick()

    const locationsLink = wrapper.find(`nav a[href="/g/${SLUG}/locations"]`)
    expect(locationsLink.classes()).toContain('custom-active')
  })

  it('highlights Locations menu when on area detail page', async () => {
    const router = createRouterForTest()
    await router.push(`/g/${SLUG}/areas/456`)

    const wrapper = mount(App, {
      global: {
        plugins: [router]
      }
    })

    await wrapper.vm.$nextTick()

    const locationsLink = wrapper.find(`nav a[href="/g/${SLUG}/locations"]`)
    expect(locationsLink.classes()).toContain('custom-active')
  })

  it('highlights Commodities menu when on commodities list page', async () => {
    const router = createRouterForTest()
    await router.push(`/g/${SLUG}/commodities`)

    const wrapper = mount(App, {
      global: {
        plugins: [router]
      }
    })

    await wrapper.vm.$nextTick()

    const commoditiesLink = wrapper.find(`nav a[href="/g/${SLUG}/commodities"]`)
    expect(commoditiesLink.classes()).toContain('custom-active')
  })

  it('highlights Commodities menu when on commodity create page', async () => {
    const router = createRouterForTest()
    await router.push(`/g/${SLUG}/commodities/new`)

    const wrapper = mount(App, {
      global: {
        plugins: [router]
      }
    })

    await wrapper.vm.$nextTick()

    const commoditiesLink = wrapper.find(`nav a[href="/g/${SLUG}/commodities"]`)
    expect(commoditiesLink.classes()).toContain('custom-active')
  })

  it('highlights Commodities menu when on commodity detail page', async () => {
    const router = createRouterForTest()
    await router.push(`/g/${SLUG}/commodities/789`)

    const wrapper = mount(App, {
      global: {
        plugins: [router]
      }
    })

    await wrapper.vm.$nextTick()

    const commoditiesLink = wrapper.find(`nav a[href="/g/${SLUG}/commodities"]`)
    expect(commoditiesLink.classes()).toContain('custom-active')
  })

  it('highlights Exports menu when on exports detail page', async () => {
    const router = createRouterForTest()
    await router.push(`/g/${SLUG}/exports/123`)

    const wrapper = mount(App, {
      global: {
        plugins: [router]
      }
    })

    await wrapper.vm.$nextTick()

    const exportsLink = wrapper.find(`nav a[href="/g/${SLUG}/exports"]`)
    expect(exportsLink.classes()).toContain('custom-active')
  })

  it('highlights System menu when on system setting detail page', async () => {
    const router = createRouterForTest()
    await router.push(`/g/${SLUG}/system/settings/456`)

    const wrapper = mount(App, {
      global: {
        plugins: [router]
      }
    })

    await wrapper.vm.$nextTick()

    const systemLink = wrapper.find(`nav a[href="/g/${SLUG}/system"]`)
    expect(systemLink.classes()).toContain('custom-active')
  })

  it('does not highlight other menus when on commodities page', async () => {
    const router = createRouterForTest()
    await router.push(`/g/${SLUG}/commodities`)

    const wrapper = mount(App, {
      global: {
        plugins: [router]
      }
    })

    await wrapper.vm.$nextTick()

    const homeLink = wrapper.find('nav a[href="/"]')
    const locationsLink = wrapper.find(`nav a[href="/g/${SLUG}/locations"]`)
    const exportsLink = wrapper.find(`nav a[href="/g/${SLUG}/exports"]`)
    const systemLink = wrapper.find(`nav a[href="/g/${SLUG}/system"]`)

    expect(homeLink.classes()).not.toContain('custom-active')
    expect(locationsLink.classes()).not.toContain('custom-active')
    expect(exportsLink.classes()).not.toContain('custom-active')
    expect(systemLink.classes()).not.toContain('custom-active')
  })
})

describe('App.vue header — group role indicator (#1258)', () => {
  // Locks in the acceptance criteria from issue #1258:
  //   * Role indicator lives in the header cluster next to the group
  //     selector, not in a separate or missing part of the UI.
  //   * The badge reflects groupStore.currentRole reactively, so switching
  //     groups (which triggers loadCurrentMembership -> updates role) flows
  //     through to what the user sees.
  //   * When membership is unknown (null), nothing is rendered rather than
  //     showing a misleading "admin" placeholder.

  const buildRouter = () =>
    createRouter({
      history: createWebHistory(),
      // /no-group is where groupStore.groupPath() now points nav links when
      // no slug is active (#1321); register it so router-link resolution
      // during the no-groups / unauthenticated cases doesn't warn.
      routes: [
        { path: '/', component: { template: '<div>Home</div>' } },
        { path: '/no-group', component: { template: '<div>No Group</div>' } },
      ],
    })

  const mountApp = async () => {
    const router = buildRouter()
    await router.push('/')
    const wrapper = mount(App, {
      global: {
        plugins: [router],
        // Stubbing GroupSelector keeps the test focused on the role badge
        // wiring — the selector is covered independently and pulling it in
        // would drag along its own store/router expectations.
        stubs: { GroupSelector: true },
      },
    })
    await wrapper.vm.$nextTick()
    return wrapper
  }

  beforeEach(() => {
    mockAuthStore.isAuthenticated = true
    mockGroupStore.hasGroups = true
  })

  afterEach(() => {
    mockAuthStore.isAuthenticated = false
    mockGroupStore.hasGroups = false
    mockGroupStore.currentRole = null
  })

  it('renders the role indicator next to GroupSelector when role is admin', async () => {
    mockGroupStore.currentRole = 'admin'
    const wrapper = await mountApp()

    const cluster = wrapper.find('.group-role-cluster')
    expect(cluster.exists()).toBe(true)

    const badge = cluster.find('[data-testid="current-role"]')
    expect(badge.exists()).toBe(true)
    expect(badge.text()).toBe('admin')
    expect(badge.classes()).toContain('role-indicator')
    expect(badge.classes()).toContain('role-indicator--admin')
  })

  it('renders the role indicator with the user modifier when role is user', async () => {
    mockGroupStore.currentRole = 'user'
    const wrapper = await mountApp()

    const badge = wrapper.find('[data-testid="current-role"]')
    expect(badge.exists()).toBe(true)
    expect(badge.text()).toBe('user')
    expect(badge.classes()).toContain('role-indicator--user')
  })

  it('omits the role indicator when currentRole is null (membership not loaded)', async () => {
    mockGroupStore.currentRole = null
    const wrapper = await mountApp()

    // Cluster itself still renders (group selector lives there), but the
    // role badge must stay out of the DOM until we actually know the role.
    expect(wrapper.find('.group-role-cluster').exists()).toBe(true)
    expect(wrapper.find('[data-testid="current-role"]').exists()).toBe(false)
  })

  it('omits the cluster entirely when the user has no groups', async () => {
    mockGroupStore.hasGroups = false
    mockGroupStore.currentRole = 'admin'
    const wrapper = await mountApp()

    expect(wrapper.find('.group-role-cluster').exists()).toBe(false)
    expect(wrapper.find('[data-testid="current-role"]').exists()).toBe(false)
  })

  it('omits the cluster when the user is not authenticated', async () => {
    mockAuthStore.isAuthenticated = false
    mockGroupStore.currentRole = 'admin'
    const wrapper = await mountApp()

    expect(wrapper.find('.group-role-cluster').exists()).toBe(false)
  })
})

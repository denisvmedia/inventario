import { describe, it, expect, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { createRouter, createWebHistory } from 'vue-router'
import App from '@/App.vue'

// Mock the settings store
const mockSettingsStore = {
  fetchMainCurrency: vi.fn().mockResolvedValue(undefined),
  mainCurrency: { id: 'usd', code: 'USD', name: 'US Dollar' }
}

vi.mock('@/stores/settingsStore', () => ({
  useSettingsStore: () => mockSettingsStore
}))

describe('App.vue Navigation', () => {
  const createRouterForTest = (initialPath = '/') => {
    return createRouter({
      history: createWebHistory(),
      routes: [
        { path: '/', component: { template: '<div>Home</div>' } },
        { path: '/locations', component: { template: '<div>Locations</div>' } },
        { path: '/locations/:id', component: { template: '<div>Location Detail</div>' } },
        { path: '/areas/:id', component: { template: '<div>Area Detail</div>' } },
        { path: '/commodities', component: { template: '<div>Commodities</div>' } },
        { path: '/commodities/new', component: { template: '<div>Commodity Create</div>' } },
        { path: '/commodities/:id', component: { template: '<div>Commodity Detail</div>' } },
        { path: '/exports', component: { template: '<div>Exports</div>' } },
        { path: '/exports/new', component: { template: '<div>Export Create</div>' } },
        { path: '/settings', component: { template: '<div>Settings</div>' } },
        { path: '/settings/:id', component: { template: '<div>Setting Detail</div>' } }
      ]
    })
  }

  it('highlights Home menu when on home page', async () => {
    const router = createRouterForTest('/')
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
    const router = createRouterForTest('/locations')
    await router.push('/locations')
    
    const wrapper = mount(App, {
      global: {
        plugins: [router]
      }
    })

    await wrapper.vm.$nextTick()
    
    const locationsLink = wrapper.find('nav a[href="/locations"]')
    expect(locationsLink.classes()).toContain('custom-active')
  })

  it('highlights Locations menu when on location detail page', async () => {
    const router = createRouterForTest('/locations/123')
    await router.push('/locations/123')
    
    const wrapper = mount(App, {
      global: {
        plugins: [router]
      }
    })

    await wrapper.vm.$nextTick()
    
    const locationsLink = wrapper.find('nav a[href="/locations"]')
    expect(locationsLink.classes()).toContain('custom-active')
  })

  it('highlights Locations menu when on area detail page', async () => {
    const router = createRouterForTest('/areas/456')
    await router.push('/areas/456')
    
    const wrapper = mount(App, {
      global: {
        plugins: [router]
      }
    })

    await wrapper.vm.$nextTick()
    
    const locationsLink = wrapper.find('nav a[href="/locations"]')
    expect(locationsLink.classes()).toContain('custom-active')
  })

  it('highlights Commodities menu when on commodities list page', async () => {
    const router = createRouterForTest('/commodities')
    await router.push('/commodities')
    
    const wrapper = mount(App, {
      global: {
        plugins: [router]
      }
    })

    await wrapper.vm.$nextTick()
    
    const commoditiesLink = wrapper.find('nav a[href="/commodities"]')
    expect(commoditiesLink.classes()).toContain('custom-active')
  })

  it('highlights Commodities menu when on commodity create page', async () => {
    const router = createRouterForTest('/commodities/new')
    await router.push('/commodities/new')
    
    const wrapper = mount(App, {
      global: {
        plugins: [router]
      }
    })

    await wrapper.vm.$nextTick()
    
    const commoditiesLink = wrapper.find('nav a[href="/commodities"]')
    expect(commoditiesLink.classes()).toContain('custom-active')
  })

  it('highlights Commodities menu when on commodity detail page', async () => {
    const router = createRouterForTest('/commodities/789')
    await router.push('/commodities/789')
    
    const wrapper = mount(App, {
      global: {
        plugins: [router]
      }
    })

    await wrapper.vm.$nextTick()
    
    const commoditiesLink = wrapper.find('nav a[href="/commodities"]')
    expect(commoditiesLink.classes()).toContain('custom-active')
  })

  it('highlights Exports menu when on exports detail page', async () => {
    const router = createRouterForTest('/exports/123')
    await router.push('/exports/123')
    
    const wrapper = mount(App, {
      global: {
        plugins: [router]
      }
    })

    await wrapper.vm.$nextTick()
    
    const exportsLink = wrapper.find('nav a[href="/exports"]')
    expect(exportsLink.classes()).toContain('custom-active')
  })

  it('highlights Settings menu when on setting detail page', async () => {
    const router = createRouterForTest('/settings/456')
    await router.push('/settings/456')
    
    const wrapper = mount(App, {
      global: {
        plugins: [router]
      }
    })

    await wrapper.vm.$nextTick()
    
    const settingsLink = wrapper.find('nav a[href="/settings"]')
    expect(settingsLink.classes()).toContain('custom-active')
  })

  it('does not highlight other menus when on commodities page', async () => {
    const router = createRouterForTest('/commodities')
    await router.push('/commodities')
    
    const wrapper = mount(App, {
      global: {
        plugins: [router]
      }
    })

    await wrapper.vm.$nextTick()
    
    const homeLink = wrapper.find('nav a[href="/"]')
    const locationsLink = wrapper.find('nav a[href="/locations"]')
    const exportsLink = wrapper.find('nav a[href="/exports"]')
    const settingsLink = wrapper.find('nav a[href="/settings"]')
    
    expect(homeLink.classes()).not.toContain('custom-active')
    expect(locationsLink.classes()).not.toContain('custom-active')
    expect(exportsLink.classes()).not.toContain('custom-active')
    expect(settingsLink.classes()).not.toContain('custom-active')
  })
})

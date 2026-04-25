import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import { defineComponent, h } from 'vue'

import {
  COMMODITY_STATUSES,
  COMMODITY_STATUS_LABELS,
  CommodityStatusPill,
  EXPORT_STATUSES,
  EXPORT_STATUS_LABELS,
  ExportStatusPill,
  StatusBadge,
  commodityStatusPillVariants,
  exportStatusPillVariants,
} from '@design/patterns'
import type { CommodityStatus, ExportStatus } from '@design/patterns'

/**
 * PR 2.4 — status pills.
 *
 * Two pills + a migration alias:
 *   - `CommodityStatusPill` — 6 statuses, status tokens.
 *   - `ExportStatusPill` — 5 statuses, semantic tokens (success /
 *     destructive / muted / primary).
 *   - `StatusBadge` — re-export of `CommodityStatusPill` for views
 *     that adopt the legacy name during the migration window.
 *
 * Per the issue contract:
 *   - typed prop `status: CommodityStatus`/`ExportStatus`,
 *   - icon is `aria-hidden="true"` so screen readers only consume
 *     the visible text label,
 *   - one assertion per status (text + icon + variant class wired),
 *   - one snapshot frame per pill that renders every status side by
 *     side as a visual contract for downstream views.
 */
describe('CommodityStatusPill (PR 2.4)', () => {
  it.each(COMMODITY_STATUSES)(
    'renders the %s status with its label, icon, and variant tokens',
    (status: CommodityStatus) => {
      const wrapper = mount(CommodityStatusPill, { props: { status } })

      const root = wrapper.find('[data-slot="commodity-status-pill"]')
      expect(root.exists()).toBe(true)
      expect(root.attributes('data-status')).toBe(status)
      expect(root.text()).toBe(COMMODITY_STATUS_LABELS[status])

      const svg = root.find('svg')
      expect(svg.exists()).toBe(true)
      expect(svg.attributes('aria-hidden')).toBe('true')

      // The variant class set is reflected on the root.
      const variantClasses = commodityStatusPillVariants({ status })
      const tokenClass = variantClasses
        .split(/\s+/)
        .find((cls) => cls.startsWith('text-status-'))
      expect(tokenClass).toBeTruthy()
      expect(root.classes()).toContain(tokenClass!)
    },
  )

  it('honours the label override prop for i18n', () => {
    const wrapper = mount(CommodityStatusPill, {
      props: { status: 'in_use', label: 'En usage' },
    })

    expect(wrapper.text()).toBe('En usage')
  })

  it('renders all 6 statuses side by side (snapshot)', () => {
    const Frame = defineComponent({
      render: () =>
        h(
          'div',
          { 'data-testid': 'frame' },
          COMMODITY_STATUSES.map((status) =>
            h(CommodityStatusPill, { status, key: status }),
          ),
        ),
    })

    const wrapper = mount(Frame)
    expect(wrapper.find('[data-testid="frame"]').html()).toMatchSnapshot()
  })
})

describe('ExportStatusPill (PR 2.4)', () => {
  it.each(EXPORT_STATUSES)(
    'renders the %s status with its label, icon, and variant tokens',
    (status: ExportStatus) => {
      const wrapper = mount(ExportStatusPill, { props: { status } })

      const root = wrapper.find('[data-slot="export-status-pill"]')
      expect(root.exists()).toBe(true)
      expect(root.attributes('data-status')).toBe(status)
      expect(root.text()).toBe(EXPORT_STATUS_LABELS[status])

      const svg = root.find('svg')
      expect(svg.exists()).toBe(true)
      expect(svg.attributes('aria-hidden')).toBe('true')

      const variantClasses = exportStatusPillVariants({ status })
      // Spot-check that the variant class set landed on the root.
      const firstClass = variantClasses
        .split(/\s+/)
        .find((cls) => cls.startsWith('border-'))
      expect(firstClass).toBeTruthy()
      expect(root.classes()).toContain(firstClass!)
    },
  )

  it('animates the in_progress icon under motion-safe', () => {
    const wrapper = mount(ExportStatusPill, {
      props: { status: 'in_progress' },
    })
    const svg = wrapper.find('svg')
    expect(svg.classes()).toContain('motion-safe:animate-spin')
  })

  it('does not animate icons for non-progress statuses', () => {
    const wrapper = mount(ExportStatusPill, { props: { status: 'completed' } })
    expect(wrapper.find('svg').classes()).not.toContain('motion-safe:animate-spin')
  })

  it('honours the label override prop for i18n', () => {
    const wrapper = mount(ExportStatusPill, {
      props: { status: 'failed', label: 'Échoué' },
    })

    expect(wrapper.text()).toBe('Échoué')
  })

  it('renders all 5 statuses side by side (snapshot)', () => {
    const Frame = defineComponent({
      render: () =>
        h(
          'div',
          { 'data-testid': 'frame' },
          EXPORT_STATUSES.map((status) =>
            h(ExportStatusPill, { status, key: status }),
          ),
        ),
    })

    const wrapper = mount(Frame)
    expect(wrapper.find('[data-testid="frame"]').html()).toMatchSnapshot()
  })
})

describe('StatusBadge migration alias (PR 2.4)', () => {
  it('is a thin re-export of CommodityStatusPill', () => {
    expect(StatusBadge).toBe(CommodityStatusPill)
  })

  it('renders identically when used under the legacy name', () => {
    const a = mount(StatusBadge, { props: { status: 'sold' } })
    const b = mount(CommodityStatusPill, { props: { status: 'sold' } })
    expect(a.html()).toBe(b.html())
  })
})

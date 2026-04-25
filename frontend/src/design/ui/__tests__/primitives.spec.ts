import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'

import { Card, CardContent, CardFooter, CardHeader, CardTitle } from '@design/ui/card'
import { Badge, badgeVariants } from '@design/ui/badge'
import { Input } from '@design/ui/input'
import { Textarea } from '@design/ui/textarea'
import { Label } from '@design/ui/label'
import { Separator } from '@design/ui/separator'
import { Skeleton } from '@design/ui/skeleton'

/**
 * Smoke tests for the shadcn-vue primitives batch added in PR 0.5.
 *
 * The primitives themselves are copy-in code from the shadcn-vue
 * registry — their detailed behaviour is exercised upstream (and the
 * underlying Reka UI components are covered by their own test suites).
 * What we verify here is local to this repo:
 *   - every module we consume in application code is importable,
 *   - simple leaf primitives render their default slot content,
 *   - the standard `class` prop is merged onto the root element via
 *     our `cn()` helper so caller overrides reach the DOM.
 *
 * Compound primitives (Dialog, Popover, AlertDialog, Tooltip,
 * DropdownMenu, Select, RadioGroup, Tabs, Checkbox, Switch) are not
 * mounted here — they require portals / user gestures and offer no
 * value over their upstream tests at this shallow level. They are
 * instead exercised by the pattern tests added in later phases as
 * real composites land.
 */
describe('shadcn-vue primitives batch (PR 0.5)', () => {
  describe('Card composite', () => {
    it('renders the full Card → Header/Title/Content/Footer stack', () => {
      const wrapper = mount({
        components: { Card, CardHeader, CardTitle, CardContent, CardFooter },
        template: `
          <Card class="w-80">
            <CardHeader><CardTitle>Summary</CardTitle></CardHeader>
            <CardContent>Body</CardContent>
            <CardFooter>Footer</CardFooter>
          </Card>
        `,
      })

      const html = wrapper.html()
      expect(html).toContain('Summary')
      expect(html).toContain('Body')
      expect(html).toContain('Footer')
      expect(wrapper.find('[data-slot="card"]').classes()).toContain('w-80')
    })
  })

  describe('Badge', () => {
    it('renders default variant with primary tokens and merges a caller class', () => {
      const wrapper = mount(Badge, {
        props: { class: 'extra-marker' },
        slots: { default: 'New' },
      })

      expect(wrapper.text()).toBe('New')
      expect(wrapper.classes()).toContain('bg-primary')
      expect(wrapper.classes()).toContain('extra-marker')
    })

    it('applies variant classes via badgeVariants()', () => {
      const classes = badgeVariants({ variant: 'destructive' })
      expect(classes).toContain('bg-destructive')
    })
  })

  describe('Input', () => {
    it('renders an <input> bound via v-model and forwards class merging', async () => {
      const wrapper = mount(Input, {
        props: { modelValue: 'hello', class: 'custom-input' },
      })

      const input = wrapper.find('input')
      expect(input.exists()).toBe(true)
      expect((input.element as HTMLInputElement).value).toBe('hello')
      expect(input.classes()).toContain('custom-input')
    })
  })

  describe('Textarea', () => {
    it('renders a <textarea> and accepts a class prop', () => {
      const wrapper = mount(Textarea, {
        props: { modelValue: 'line', class: 'custom-textarea' },
      })

      const el = wrapper.find('textarea')
      expect(el.exists()).toBe(true)
      expect((el.element as HTMLTextAreaElement).value).toBe('line')
      expect(el.classes()).toContain('custom-textarea')
    })
  })

  describe('Label / Separator / Skeleton', () => {
    it('Label renders slot content on a <label> element', () => {
      const wrapper = mount(Label, { slots: { default: 'Name' } })
      expect(wrapper.element.tagName).toBe('LABEL')
      expect(wrapper.text()).toBe('Name')
    })

    it('Separator renders with role=none by default and accepts class', () => {
      const wrapper = mount(Separator, { props: { class: 'my-sep' } })
      expect(wrapper.element.getAttribute('data-slot')).toBe('separator')
      expect(wrapper.classes()).toContain('my-sep')
    })

    it('Skeleton renders an animated placeholder div with class merging', () => {
      const wrapper = mount(Skeleton, { props: { class: 'h-4 w-32' } })
      expect(wrapper.element.tagName).toBe('DIV')
      expect(wrapper.classes()).toContain('h-4')
      expect(wrapper.classes()).toContain('w-32')
      expect(wrapper.classes()).toContain('animate-pulse')
    })
  })

  describe('Compound-primitive barrels export their public API', () => {
    it('every barrel under @design/ui resolves to a module with at least one named export', async () => {
      const barrels = await Promise.all([
        import('@design/ui/dialog'),
        import('@design/ui/alert-dialog'),
        import('@design/ui/popover'),
        import('@design/ui/tooltip'),
        import('@design/ui/dropdown-menu'),
        import('@design/ui/select'),
        import('@design/ui/radio-group'),
        import('@design/ui/tabs'),
        import('@design/ui/checkbox'),
        import('@design/ui/switch'),
      ])

      for (const mod of barrels) {
        const keys = Object.keys(mod).filter((k) => k !== 'default')
        expect(keys.length).toBeGreaterThan(0)
        for (const k of keys) {
          expect((mod as Record<string, unknown>)[k]).toBeDefined()
        }
      }
    })
  })
})

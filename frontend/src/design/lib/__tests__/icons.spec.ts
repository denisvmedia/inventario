import { describe, expect, it } from 'vitest'
import * as bridge from '../icons'
import * as lucide from 'lucide-vue-next'

/**
 * The FA→Lucide bridge is a pure re-export — the behaviour we care about
 * is that every alias resolves to a Lucide component, that a few of the
 * well-known mappings from devdocs/frontend/icons.md line up as
 * documented, and that no entry silently points at `undefined` because
 * of a typo at re-export time.
 */
describe('FA→Lucide bridge', () => {
  const aliases = Object.keys(bridge) as Array<keyof typeof bridge>

  it('exports at least 40 aliases covering the legacy icon set', () => {
    expect(aliases.length).toBeGreaterThanOrEqual(40)
  })

  it('resolves every alias to a defined Lucide component', () => {
    for (const name of aliases) {
      expect(bridge[name], `alias ${String(name)} is not defined`).toBeDefined()
    }
  })

  it.each([
    ['FaBox', 'Box'],
    ['FaTrash', 'Trash2'],
    ['FaEdit', 'Pencil'],
    ['FaTimes', 'X'],
    ['FaMapMarkerAlt', 'MapPin'],
    ['FaRightFromBracket', 'LogOut'],
    ['FaSpinner', 'Loader2'],
    ['FaCheckCircle', 'CheckCircle2'],
    ['FaExclamationTriangle', 'AlertTriangle'],
    ['FaExclamationCircle', 'AlertCircle'],
    ['FaCloudUploadAlt', 'UploadCloud'],
    ['FaChevronUp', 'ChevronUp'],
    ['FaChevronDown', 'ChevronDown'],
  ] as const)('aliases %s → Lucide %s', (faName, lucideName) => {
    const bridgeRef = (bridge as Record<string, unknown>)[faName]
    const lucideRef = (lucide as Record<string, unknown>)[lucideName]
    expect(bridgeRef).toBe(lucideRef)
  })
})

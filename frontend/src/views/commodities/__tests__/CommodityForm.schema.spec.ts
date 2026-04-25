import { describe, expect, it } from 'vitest'

import {
  commodityFormSchema,
  defaultCommodityFormValues,
} from '../CommodityForm.schema'

const validBase = () =>
  defaultCommodityFormValues({
    name: 'Coffee Maker',
    shortName: 'coffee',
    areaId: 'area-1',
  })

describe('commodityFormSchema', () => {
  describe('defaults', () => {
    it('produces a payload that satisfies the schema', () => {
      const result = commodityFormSchema.safeParse(validBase())
      expect(result.success).toBe(true)
    })

    it('honours overrides', () => {
      const values = defaultCommodityFormValues({ name: 'X', shortName: 'x', areaId: 'a' })
      expect(values.name).toBe('X')
      expect(values.shortName).toBe('x')
      expect(values.areaId).toBe('a')
    })
  })

  describe('required fields', () => {
    it('rejects an empty name', () => {
      const result = commodityFormSchema.safeParse({ ...validBase(), name: '' })
      expect(result.success).toBe(false)
      if (!result.success) {
        expect(result.error.issues.some((i) => i.path[0] === 'name')).toBe(true)
      }
    })

    it('rejects an empty short name', () => {
      const result = commodityFormSchema.safeParse({ ...validBase(), shortName: '' })
      expect(result.success).toBe(false)
    })

    it('rejects an empty area id', () => {
      const result = commodityFormSchema.safeParse({ ...validBase(), areaId: '' })
      expect(result.success).toBe(false)
    })

    it('rejects an empty purchase date', () => {
      const result = commodityFormSchema.safeParse({ ...validBase(), purchaseDate: '' })
      expect(result.success).toBe(false)
    })
  })

  describe('max lengths', () => {
    it('caps short name at 20 characters', () => {
      const result = commodityFormSchema.safeParse({
        ...validBase(),
        shortName: 'a'.repeat(21),
      })
      expect(result.success).toBe(false)
      if (!result.success) {
        expect(result.error.issues.some((i) => i.path[0] === 'shortName')).toBe(true)
      }
    })

    it('caps comments at 1000 characters', () => {
      const result = commodityFormSchema.safeParse({
        ...validBase(),
        comments: 'x'.repeat(1001),
      })
      expect(result.success).toBe(false)
      if (!result.success) {
        expect(result.error.issues.some((i) => i.path[0] === 'comments')).toBe(true)
      }
    })
  })

  describe('enum refinements', () => {
    it('rejects an unknown type id', () => {
      const result = commodityFormSchema.safeParse({ ...validBase(), type: 'not-a-type' })
      expect(result.success).toBe(false)
    })

    it('rejects an unknown status id', () => {
      const result = commodityFormSchema.safeParse({ ...validBase(), status: 'not-a-status' })
      expect(result.success).toBe(false)
    })
  })

  describe('numeric coercions', () => {
    it('coerces stringified numbers for prices and count', () => {
      const result = commodityFormSchema.safeParse({
        ...validBase(),
        count: '3' as unknown as number,
        originalPrice: '10' as unknown as number,
        convertedOriginalPrice: '10' as unknown as number,
        currentPrice: '5' as unknown as number,
      })
      expect(result.success).toBe(true)
      if (result.success) {
        expect(result.data.count).toBe(3)
        expect(result.data.originalPrice).toBe(10)
        expect(result.data.currentPrice).toBe(5)
      }
    })

    it('rejects non-integer counts', () => {
      const result = commodityFormSchema.safeParse({ ...validBase(), count: 1.5 })
      expect(result.success).toBe(false)
    })

    it('rejects negative prices', () => {
      const result = commodityFormSchema.safeParse({ ...validBase(), originalPrice: -1 })
      expect(result.success).toBe(false)
    })

    it('rejects count below 1', () => {
      const result = commodityFormSchema.safeParse({ ...validBase(), count: 0 })
      expect(result.success).toBe(false)
    })
  })

  describe('purchase date refinement', () => {
    it('accepts a past purchase date', () => {
      const result = commodityFormSchema.safeParse({
        ...validBase(),
        purchaseDate: '2000-01-01',
      })
      expect(result.success).toBe(true)
    })

    it('rejects a future purchase date with the expected message', () => {
      const future = new Date()
      future.setFullYear(future.getFullYear() + 1)
      const futureIso = future.toISOString().split('T')[0]
      const result = commodityFormSchema.safeParse({
        ...validBase(),
        purchaseDate: futureIso,
      })
      expect(result.success).toBe(false)
      if (!result.success) {
        const issue = result.error.issues.find((i) => i.path[0] === 'purchaseDate')
        expect(issue?.message).toBe('Purchase Date cannot be in the future')
      }
    })
  })
})

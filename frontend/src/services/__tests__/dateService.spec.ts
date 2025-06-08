import { describe, it, expect } from 'vitest'
import { formatDate } from '../dateService'

describe('dateService', () => {
  describe('formatDate', () => {
    const testDate = '2023-12-25' // Christmas 2023
    
    it('formats date in DD/MM/YYYY format', () => {
      const result = formatDate(testDate, 'DD/MM/YYYY')
      expect(result).toBe('25/12/2023')
    })
    
    it('formats date in MM/DD/YYYY format', () => {
      const result = formatDate(testDate, 'MM/DD/YYYY')
      expect(result).toBe('12/25/2023')
    })
    
    it('formats date in YYYY-MM-DD format', () => {
      const result = formatDate(testDate, 'YYYY-MM-DD')
      expect(result).toBe('2023-12-25')
    })
    
    it('formats date in YYYY/MM/DD format', () => {
      const result = formatDate(testDate, 'YYYY/MM/DD')
      expect(result).toBe('2023/12/25')
    })
    
    it('returns empty string for empty date', () => {
      const result = formatDate('', 'DD/MM/YYYY')
      expect(result).toBe('')
    })
    
    it('returns original string for invalid date', () => {
      const result = formatDate('invalid-date', 'DD/MM/YYYY')
      expect(result).toBe('invalid-date')
    })
    
    it('uses default format when no format specified', () => {
      const result = formatDate(testDate)
      expect(result).toBe('25/12/2023') // Default is DD/MM/YYYY
    })
    
    it('falls back to DD/MM/YYYY for unknown format', () => {
      const result = formatDate(testDate, 'UNKNOWN_FORMAT')
      expect(result).toBe('25/12/2023')
    })
  })
})

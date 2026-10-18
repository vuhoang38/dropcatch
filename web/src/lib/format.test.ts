import { describe, expect, it } from 'vitest'
import { formatCents, formatVsTypical } from '@/lib/format'

describe('formatCents', () => {
  it('formats cents as SGD with thousands separators', () => {
    expect(formatCents(129900)).toBe('$1,299.00')
    expect(formatCents(2750)).toBe('$27.50')
  })
})

describe('formatVsTypical', () => {
  it('describes how far a price sits from typical', () => {
    expect(formatVsTypical(-17.92)).toBe('18% under typical')
    expect(formatVsTypical(4.2)).toBe('4% over typical')
    expect(formatVsTypical(0.3)).toBe('at typical')
    expect(formatVsTypical(null)).toBe('no typical price yet')
  })
})

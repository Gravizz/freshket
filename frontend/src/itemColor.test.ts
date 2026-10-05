import { describe, expect, it } from 'vitest'
import { codeFromColor, displayCode, honeycomb, itemColor } from './itemColor'

describe('itemColor', () => {
  it('turns a picked colour into an item code and back', () => {
    expect(codeFromColor('#1a2b3c')).toBe('1A2B3C')
    expect(itemColor('1A2B3C')).toBe('#1A2B3C')
    expect(displayCode('1A2B3C')).toBe('#1A2B3C')
  })

  it('keeps the seeded colour names', () => {
    expect(itemColor('RED')).toBe('#ef4444')
    expect(displayCode('RED')).toBe('RED')
  })

  it('gives any other code a stable hue', () => {
    expect(itemColor('BLACK_SET')).toBe(itemColor('BLACK_SET'))
    expect(itemColor('BLACK_SET')).toMatch(/^hsl\(/)
  })
})

describe('honeycomb', () => {
  it('offers 37 distinct colours, each a valid item code', () => {
    const colors = honeycomb.flat()
    expect(honeycomb.map((row) => row.length)).toEqual([4, 5, 6, 7, 6, 5, 4])
    expect(new Set(colors).size).toBe(37)
    for (const color of colors) expect(codeFromColor(color)).toMatch(/^[0-9A-F]{6}$/)
  })
})

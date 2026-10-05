import { describe, expect, it } from 'vitest'
import { MAX_PRICE, MAX_QUANTITY, parsePrice, parseWholeNumber, validateItem, validateRule } from './validation'

describe('parsePrice', () => {
  it.each([
    ['45', 4500],
    ['45.5', 4550],
    ['45.50', 4550],
    ['0.01', 1],
    ['  12.34  ', 1234],
    ['1.10', 110], // floats would give 109.99999999999999
    ['1000000', MAX_PRICE],
    ['007', 700],
  ])('reads %j as %d satang', (text, satang) => {
    expect(parsePrice(text)).toEqual({ value: satang })
  })

  it.each([
    ['', 'Enter a price.'],
    ['   ', 'Enter a price.'],
    ['abc', 'digits only'],
    ['12abc', 'digits only'],
    ['1e3', 'digits only'],
    ['1,50', 'digits only'],
    ['-5', 'digits only'],
    ['+5', 'digits only'],
    ['.5', 'digits only'],
    ['5.', 'digits only'],
    ['1.005', 'at most 2 decimals'],
    ['0', 'at least'],
    ['0.00', 'at least'],
    ['1000000.01', 'at most'],
    ['9'.repeat(400), 'at most'],
  ])('rejects %j', (text, message) => {
    expect(parsePrice(text).error).toContain(message)
  })
})

describe('parseWholeNumber', () => {
  it('reads digits within the range', () => {
    expect(parseWholeNumber(' 12 ', 'the percent', 1, 100)).toEqual({ value: 12 })
    expect(parseWholeNumber('100', 'the percent', 1, 100)).toEqual({ value: 100 })
  })

  it.each([
    ['', 'Enter the percent.'],
    ['5.5', 'The percent must be a whole number.'],
    ['5%', 'The percent must be a whole number.'],
    ['-1', 'The percent must be a whole number.'],
    ['0', 'The percent must be between 1 and 100.'],
    ['101', 'The percent must be between 1 and 100.'],
  ])('rejects %j', (text, message) => {
    expect(parseWholeNumber(text, 'the percent', 1, 100).error).toBe(message)
  })
})

describe('validateItem', () => {
  const ok = { code: '00CE7C', name: ' Black set ', price: '45.50' }

  it('returns the item with a trimmed name and the price in satang', () => {
    expect(validateItem(ok, new Set())).toEqual({
      ok: true,
      value: { code: '00CE7C', name: 'Black set', price: 4550, active: true },
    })
  })

  it('reports every problem at once', () => {
    const result = validateItem({ code: '00CE7C', name: '   ', price: 'x' }, new Set(['00CE7C']))
    expect(result).toEqual({
      ok: false,
      errors: {
        code: 'This colour is already used by another item.',
        name: 'Enter a name.',
        price: expect.stringContaining('digits only'),
      },
    })
  })

  it('rejects a name over 60 characters but accepts exactly 60', () => {
    expect(validateItem({ ...ok, name: 'n'.repeat(60) }, new Set()).ok).toBe(true)
    const tooLong = validateItem({ ...ok, name: 'n'.repeat(61) }, new Set())
    expect(tooLong.ok === false && tooLong.errors.name).toContain('60 characters')
  })

  it('counts characters, not UTF-16 units', () => {
    expect(validateItem({ ...ok, name: '🍜'.repeat(60) }, new Set()).ok).toBe(true)
  })

  it('reports when no colour is left', () => {
    const result = validateItem({ ...ok, code: undefined }, new Set())
    expect(result).toEqual({ ok: false, errors: { code: 'Every colour is already in use.' } })
  })
})

describe('validateRule', () => {
  const ok = { name: ' Bundle A ', rows: [{ itemCode: 'GREEN', qty: '2' }], percent: '12', memberOnly: false }

  it('returns the rule with numbers parsed and the name trimmed', () => {
    expect(validateRule(ok)).toEqual({
      ok: true,
      value: { name: 'Bundle A', bundle: [{ itemCode: 'GREEN', qty: 2 }], percent: 12, memberOnly: false, active: true },
    })
  })

  it('treats no rows as a whole-order rule', () => {
    const result = validateRule({ ...ok, rows: [] })
    expect(result.ok && result.value.bundle).toEqual([])
  })

  it('flags a blank name and a bad percent', () => {
    const result = validateRule({ ...ok, name: ' ', percent: '5.5' })
    expect(result.ok).toBe(false)
    if (!result.ok) expect(result.errors).toMatchObject({ name: 'Enter a name.', percent: 'The percent must be a whole number.' })
  })

  it('flags a row without an item, a bad quantity and a repeated item, row by row', () => {
    const result = validateRule({
      ...ok,
      rows: [
        { itemCode: 'GREEN', qty: '2' },
        { itemCode: '', qty: '1' },
        { itemCode: 'GREEN', qty: '0' },
        { itemCode: 'RED', qty: String(MAX_QUANTITY + 1) },
      ],
    })
    expect(result.ok).toBe(false)
    if (!result.ok) {
      expect(result.errors.rows).toEqual([
        {},
        { item: 'Choose an item.' },
        { item: 'This item is already in the bundle.', qty: 'A quantity must be between 1 and 10000.' },
        { qty: 'A quantity must be between 1 and 10000.' },
      ])
    }
  })
})

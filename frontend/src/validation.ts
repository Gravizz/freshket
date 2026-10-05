// Checks the admin forms before anything is sent, so a slip shows up next to the
// field it came from. The limits mirror internal/pricing; the server stays the
// authority and re-checks every request.

export const MAX_PRICE = 100_000_000 // satang (THB 1,000,000)
export const MAX_QUANTITY = 10_000
export const MAX_NAME = 60

export type Parsed = { value: number; error?: undefined } | { value?: undefined; error: string }

// parsePrice reads a baht amount such as "45" or "45.50" into satang. It works
// on the digits, not on floats, so "1.005" is rejected instead of rounded.
export function parsePrice(text: string): Parsed {
  const t = text.trim()
  if (t === '') return { error: 'Enter a price.' }
  const match = /^(\d+)(?:\.(\d{1,2}))?$/.exec(t)
  if (!match) return { error: 'Use digits only, such as 45 or 45.50 (at most 2 decimals).' }
  const satang = Number(match[1]) * 100 + Number((match[2] ?? '').padEnd(2, '0'))
  if (satang < 1) return { error: 'The price must be at least ฿0.01.' }
  if (satang > MAX_PRICE) return { error: `The price can be at most ฿${MAX_PRICE / 100}.` }
  return { value: satang }
}

// parseWholeNumber reads digits such as "12" into an integer within [min, max].
export function parseWholeNumber(text: string, label: string, min: number, max: number): Parsed {
  const t = text.trim()
  if (t === '') return { error: `Enter ${label}.` }
  if (!/^\d+$/.test(t)) return { error: `${capitalize(label)} must be a whole number.` }
  const n = Number(t)
  if (n < min || n > max) return { error: `${capitalize(label)} must be between ${min} and ${max}.` }
  return { value: n }
}

function capitalize(s: string): string {
  return s.charAt(0).toUpperCase() + s.slice(1)
}

function nameError(name: string): string | undefined {
  const trimmed = name.trim()
  if (trimmed === '') return 'Enter a name.'
  if ([...trimmed].length > MAX_NAME) return `Keep the name to ${MAX_NAME} characters or fewer.`
}

export type Validation<V, E> = { ok: true; value: V } | { ok: false; errors: E }

export type ItemErrors = { code?: string; name?: string; price?: string }

export type NewItem = { code: string; name: string; price: number; active: true }

// validateItem checks the add-item form. code is undefined when every colour
// in the tray is already used by another item.
export function validateItem(
  form: { code: string | undefined; name: string; price: string },
  takenCodes: ReadonlySet<string>,
): Validation<NewItem, ItemErrors> {
  const errors: ItemErrors = {}
  if (form.code === undefined) errors.code = 'Every colour is already in use.'
  else if (takenCodes.has(form.code)) errors.code = 'This colour is already used by another item.'
  const name = nameError(form.name)
  if (name) errors.name = name
  const price = parsePrice(form.price)
  if (price.error !== undefined) errors.price = price.error

  if (form.code === undefined || price.error !== undefined || Object.keys(errors).length > 0) {
    return { ok: false, errors }
  }
  return { ok: true, value: { code: form.code, name: form.name.trim(), price: price.value, active: true } }
}

export type BundleRowInput = { itemCode: string; qty: string }
export type RowErrors = { item?: string; qty?: string }
export type RuleErrors = { name?: string; percent?: string; rows: RowErrors[] }

export type NewRule = {
  name: string
  bundle: { itemCode: string; qty: number }[]
  percent: number
  memberOnly: boolean
  active: true
}

// validateRule checks the add-rule form. No rows means a whole-order rule.
export function validateRule(form: {
  name: string
  rows: BundleRowInput[]
  percent: string
  memberOnly: boolean
}): Validation<NewRule, RuleErrors> {
  const percent = parseWholeNumber(form.percent, 'the percent', 1, 100)
  const seen = new Set<string>()
  const parsedQty: Parsed[] = []
  const rows = form.rows.map((row): RowErrors => {
    const qty = parseWholeNumber(row.qty, 'a quantity', 1, MAX_QUANTITY)
    parsedQty.push(qty)
    let item: string | undefined
    if (row.itemCode === '') item = 'Choose an item.'
    else if (seen.has(row.itemCode)) item = 'This item is already in the bundle.'
    seen.add(row.itemCode)
    return { item, qty: qty.error }
  })

  const errors: RuleErrors = { name: nameError(form.name), percent: percent.error, rows }
  const failed = errors.name || errors.percent || rows.some((r) => r.item || r.qty)
  if (failed || percent.error !== undefined) return { ok: false, errors }
  return {
    ok: true,
    value: {
      name: form.name.trim(),
      bundle: form.rows.map((row, i) => ({ itemCode: row.itemCode, qty: parsedQty[i].value as number })),
      percent: percent.value,
      memberOnly: form.memberOnly,
      active: true,
    },
  }
}

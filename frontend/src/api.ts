// All amounts are integer satang (1 THB = 100 satang).

export type MenuItem = {
  code: string
  name: string
  price: number
}

export type OrderLine = {
  code: string
  qty: number
}

export type Breakdown = {
  subtotal: number
  discounts: { label: string; amount: number }[]
  total: number
}

export type AdminItem = MenuItem & { active: boolean }

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  let res: Response
  try {
    res = await fetch(path, init)
  } catch {
    throw new Error('Cannot reach the server. Check your connection and try again.')
  }
  if (!res.ok) throw new Error(await errorMessage(res))
  return res.json() as Promise<T>
}

// errorMessage prefers the API's own plain-text message and falls back to the
// status when the body is empty, or HTML from a proxy that cannot reach the API.
async function errorMessage(res: Response): Promise<string> {
  const body = (await res.text()).trim()
  const readable = body !== '' && body.length <= 300 && !body.startsWith('<')
  return readable ? body : `Request failed: ${res.status}`
}

export function fetchMenu(): Promise<MenuItem[]> {
  return request('/api/menu')
}

export function calculate(items: OrderLine[], member: boolean): Promise<Breakdown> {
  return request('/api/orders/calculate', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ items, member }),
  })
}

// A bundle is the items (and how many of each) a promotion needs; an empty
// bundle means the rule discounts the whole order.
export type BundleItem = {
  itemCode: string
  qty: number
}

export type Rule = {
  id: number
  name: string
  bundle: BundleItem[]
  percent: number
  memberOnly: boolean
  active: boolean
}

const json = { 'Content-Type': 'application/json' }

export function fetchAdminMenu(): Promise<AdminItem[]> {
  return request('/api/admin/menu')
}

export function createItem(item: AdminItem): Promise<AdminItem> {
  return request('/api/admin/menu', { method: 'POST', headers: json, body: JSON.stringify(item) })
}

export function updateItem(item: AdminItem): Promise<AdminItem> {
  return request(`/api/admin/menu/${encodeURIComponent(item.code)}`, {
    method: 'PUT',
    headers: json,
    body: JSON.stringify(item),
  })
}

export function fetchRules(): Promise<Rule[]> {
  return request('/api/admin/rules')
}

export function createRule(rule: Omit<Rule, 'id'>): Promise<Rule> {
  return request('/api/admin/rules', { method: 'POST', headers: json, body: JSON.stringify(rule) })
}

export function updateRule(rule: Rule): Promise<Rule> {
  return request(`/api/admin/rules/${rule.id}`, { method: 'PUT', headers: json, body: JSON.stringify(rule) })
}

export function formatTHB(satang: number): string {
  return `฿${(satang / 100).toFixed(2)}`
}

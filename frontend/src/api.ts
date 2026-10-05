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
  const res = await fetch(path, init)
  if (!res.ok) {
    const body = await res.text()
    throw new Error(body || `Request failed: ${res.status}`)
  }
  return res.json() as Promise<T>
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

export type Rule = {
  id: number
  name: string
  itemCode: string
  groupSize: number
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

export function fetchRules(): Promise<Rule[]> {
  return request('/api/admin/rules')
}

export function createRule(rule: Omit<Rule, 'id'>): Promise<Rule> {
  return request('/api/admin/rules', { method: 'POST', headers: json, body: JSON.stringify(rule) })
}

export function formatTHB(satang: number): string {
  return `฿${(satang / 100).toFixed(2)}`
}

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

export function formatTHB(satang: number): string {
  return `฿${(satang / 100).toFixed(2)}`
}

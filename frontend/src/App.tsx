import { useEffect, useState } from 'react'
import { calculate, fetchMenu, formatTHB, type Breakdown, type MenuItem } from './api'

export default function App() {
  const [menu, setMenu] = useState<MenuItem[]>([])
  const [quantities, setQuantities] = useState<Record<string, number>>({})
  const [member, setMember] = useState(false)
  const [breakdown, setBreakdown] = useState<Breakdown | null>(null)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    fetchMenu()
      .then(setMenu)
      .catch((e: Error) => setError(e.message))
  }, [])

  useEffect(() => {
    let ignore = false
    const items = Object.entries(quantities)
      .filter(([, qty]) => qty > 0)
      .map(([code, qty]) => ({ code, qty }))

    calculate(items, member)
      .then((b) => {
        if (!ignore) {
          setBreakdown(b)
          setError(null)
        }
      })
      .catch((e: Error) => {
        if (!ignore) setError(e.message)
      })
    return () => {
      ignore = true
    }
  }, [quantities, member])

  const setQty = (code: string, qty: number) =>
    setQuantities((q) => ({ ...q, [code]: Math.max(0, qty) }))

  return (
    <main className="mx-auto max-w-xl px-4 py-10 text-slate-800">
      <h1 className="mb-6 text-2xl font-semibold">Food Store Calculator</h1>

      <ul className="divide-y divide-slate-200 rounded-lg border border-slate-200">
        {menu.map((item) => {
          const qty = quantities[item.code] ?? 0
          return (
            <li key={item.code} className="flex items-center justify-between px-4 py-3">
              <div>
                <p className="font-medium">{item.name}</p>
                <p className="text-sm text-slate-500">{formatTHB(item.price)}</p>
              </div>
              <div className="flex items-center gap-2">
                <button
                  type="button"
                  aria-label={`Remove ${item.name}`}
                  onClick={() => setQty(item.code, qty - 1)}
                  className="size-8 rounded-md border border-slate-300 hover:bg-slate-100"
                >
                  −
                </button>
                <span className="w-6 text-center tabular-nums">{qty}</span>
                <button
                  type="button"
                  aria-label={`Add ${item.name}`}
                  onClick={() => setQty(item.code, qty + 1)}
                  className="size-8 rounded-md border border-slate-300 hover:bg-slate-100"
                >
                  +
                </button>
              </div>
            </li>
          )
        })}
      </ul>

      <label className="mt-4 flex items-center gap-2">
        <input type="checkbox" checked={member} onChange={(e) => setMember(e.target.checked)} />
        Member card
      </label>

      {error && <p className="mt-4 rounded-md bg-red-50 px-4 py-3 text-red-700">{error}</p>}

      {breakdown && (
        <dl className="mt-6 space-y-2 rounded-lg bg-slate-50 p-4">
          <div className="flex justify-between">
            <dt>Subtotal</dt>
            <dd className="tabular-nums">{formatTHB(breakdown.subtotal)}</dd>
          </div>
          {breakdown.discounts.map((d) => (
            <div key={d.label} className="flex justify-between text-emerald-700">
              <dt>{d.label}</dt>
              <dd className="tabular-nums">−{formatTHB(d.amount)}</dd>
            </div>
          ))}
          <div className="flex justify-between border-t border-slate-200 pt-2 text-lg font-semibold">
            <dt>Total</dt>
            <dd className="tabular-nums">{formatTHB(breakdown.total)}</dd>
          </div>
        </dl>
      )}
    </main>
  )
}

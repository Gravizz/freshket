import { useEffect, useState, type FormEvent } from 'react'
import {
  createItem,
  createRule,
  fetchAdminMenu,
  fetchRules,
  formatTHB,
  type AdminItem,
  type Rule,
} from './api'

const inputClass = 'rounded-md border border-slate-300 px-2 py-1'

export default function Admin() {
  const [items, setItems] = useState<AdminItem[]>([])
  const [code, setCode] = useState('')
  const [name, setName] = useState('')
  const [price, setPrice] = useState('')
  const [rules, setRules] = useState<Rule[]>([])
  const [ruleName, setRuleName] = useState('')
  const [ruleItem, setRuleItem] = useState('')
  const [groupSize, setGroupSize] = useState('')
  const [percent, setPercent] = useState('')
  const [memberOnly, setMemberOnly] = useState(false)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    fetchAdminMenu().then(setItems)
    fetchRules().then(setRules)
  }, [])

  const addRule = async (e: FormEvent) => {
    e.preventDefault()
    setError(null)
    try {
      const created = await createRule({
        name: ruleName,
        itemCode: ruleItem,
        groupSize: ruleItem ? parseInt(groupSize, 10) : 0,
        percent: parseInt(percent, 10),
        memberOnly,
        active: true,
      })
      setRules((list) => [...list, created])
      setRuleName('')
      setRuleItem('')
      setGroupSize('')
      setPercent('')
      setMemberOnly(false)
    } catch (err) {
      setError((err as Error).message)
    }
  }

  const addItem = async (e: FormEvent) => {
    e.preventDefault()
    setError(null)
    try {
      const created = await createItem({
        code,
        name,
        price: Math.round(parseFloat(price) * 100),
        active: true,
      })
      setItems((list) => [...list, created])
      setCode('')
      setName('')
      setPrice('')
    } catch (err) {
      setError((err as Error).message)
    }
  }

  return (
    <main className="mx-auto max-w-2xl px-4 py-10 text-slate-800">
      <h1 className="mb-6 text-2xl font-semibold">Admin</h1>
      {error && (
        <p role="alert" className="mb-4 rounded-md bg-red-50 px-4 py-3 text-red-700">
          {error}
        </p>
      )}

      <section>
        <h2 className="mb-2 text-lg font-medium">Menu items</h2>
        <ul className="divide-y divide-slate-200 rounded-lg border border-slate-200">
          {items.map((item) => (
            <li key={item.code} className="flex justify-between px-4 py-2">
              <span>
                <span className="font-mono text-sm text-slate-500">{item.code}</span>{' '}
                <span>{item.name}</span>
              </span>
              <span className="tabular-nums">{formatTHB(item.price)}</span>
            </li>
          ))}
        </ul>

        <form onSubmit={addItem} className="mt-3 flex flex-wrap items-end gap-2">
          <label className="flex flex-col text-sm">
            Item code
            <input className={inputClass} value={code} onChange={(e) => setCode(e.target.value)} />
          </label>
          <label className="flex flex-col text-sm">
            Item name
            <input className={inputClass} value={name} onChange={(e) => setName(e.target.value)} />
          </label>
          <label className="flex flex-col text-sm">
            Price (THB)
            <input className={inputClass} value={price} onChange={(e) => setPrice(e.target.value)} />
          </label>
          <button type="submit" className="rounded-md bg-slate-800 px-3 py-1 text-white">
            Add item
          </button>
        </form>
      </section>

      <section className="mt-10">
        <h2 className="mb-2 text-lg font-medium">Discount rules</h2>
        <ul className="divide-y divide-slate-200 rounded-lg border border-slate-200">
          {rules.map((rule) => (
            <li key={rule.id} className="flex justify-between px-4 py-2">
              <span>{rule.name}</span>
              <span className="text-sm text-slate-500">
                {rule.percent}% {rule.itemCode ? `· every ${rule.groupSize} × ${rule.itemCode}` : '· whole order'}
              </span>
            </li>
          ))}
        </ul>

        <form onSubmit={addRule} className="mt-3 flex flex-wrap items-end gap-2">
          <label className="flex flex-col text-sm">
            Rule name
            <input className={inputClass} value={ruleName} onChange={(e) => setRuleName(e.target.value)} />
          </label>
          <label className="flex flex-col text-sm">
            Rule item
            <select className={inputClass} value={ruleItem} onChange={(e) => setRuleItem(e.target.value)}>
              <option value="">Whole order</option>
              {items.map((item) => (
                <option key={item.code} value={item.code}>
                  {item.name}
                </option>
              ))}
            </select>
          </label>
          {ruleItem && (
            <label className="flex flex-col text-sm">
              Group size
              <input className={inputClass} value={groupSize} onChange={(e) => setGroupSize(e.target.value)} />
            </label>
          )}
          <label className="flex flex-col text-sm">
            Percent
            <input className={inputClass} value={percent} onChange={(e) => setPercent(e.target.value)} />
          </label>
          <label className="flex items-center gap-1 text-sm">
            <input type="checkbox" checked={memberOnly} onChange={(e) => setMemberOnly(e.target.checked)} />
            Members only
          </label>
          <button type="submit" className="rounded-md bg-slate-800 px-3 py-1 text-white">
            Add rule
          </button>
        </form>
      </section>
    </main>
  )
}

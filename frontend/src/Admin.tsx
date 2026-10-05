import { useEffect, useId, useState, type FormEvent, type ReactNode } from 'react'
import {
  createItem,
  createRule,
  fetchAdminMenu,
  fetchRules,
  formatTHB,
  updateItem,
  updateRule,
  type AdminItem,
  type Rule,
} from './api'
import { itemColor } from './itemColor'
import Plate from './Plate'

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
  const [notice, setNotice] = useState<string | null>(null)

  useEffect(() => {
    Promise.all([fetchAdminMenu(), fetchRules()])
      .then(([menuItems, discountRules]) => {
        setItems(menuItems)
        setRules(discountRules)
      })
      .catch((err: Error) => setError(err.message))
  }, [])

  useEffect(() => {
    if (!notice) return
    const timer = setTimeout(() => setNotice(null), 3000)
    return () => clearTimeout(timer)
  }, [notice])

  // run clears the banners, performs one admin write, and surfaces its outcome.
  const run = async (action: () => Promise<string>) => {
    setError(null)
    setNotice(null)
    try {
      setNotice(await action())
    } catch (err) {
      setError((err as Error).message)
    }
  }

  const addRule = (e: FormEvent) => {
    e.preventDefault()
    return run(async () => {
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
      return `Rule “${created.name}” is live`
    })
  }

  const addItem = (e: FormEvent) => {
    e.preventDefault()
    return run(async () => {
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
      return `“${created.name}” added to the menu`
    })
  }

  const toggleItem = (item: AdminItem) =>
    run(async () => {
      const saved = await updateItem({ ...item, active: !item.active })
      setItems((list) => list.map((it) => (it.code === saved.code ? saved : it)))
      return `“${saved.name}” ${saved.active ? 'is back on' : 'taken off'} the menu`
    })

  const toggleRule = (rule: Rule) =>
    run(async () => {
      const saved = await updateRule({ ...rule, active: !rule.active })
      setRules((list) => list.map((r) => (r.id === saved.id ? saved : r)))
      return `Rule “${saved.name}” ${saved.active ? 'activated' : 'paused'}`
    })

  const itemName = (itemCode: string) => items.find((it) => it.code === itemCode)?.name ?? itemCode
  const preview = describeRule(ruleItem ? itemName(ruleItem) : '', groupSize, percent, memberOnly)

  return (
    <main className="mx-auto max-w-6xl px-4 pb-20 sm:px-6">
      <section className="flex animate-rise flex-wrap items-end justify-between gap-6 pt-6 pb-8 sm:pt-10">
        <div>
          <p className="inline-flex items-center gap-2 rounded-full bg-white/80 px-3 py-1 text-xs font-medium text-fk-700 ring-1 ring-fk-200">
            <span className="size-1.5 rounded-full bg-sun" />
            Back office
          </p>
          <h1 className="mt-4 font-display text-4xl font-semibold tracking-tight text-fk-900 sm:text-5xl">
            Menu & promotions<span className="text-fk-400">.</span>
          </h1>
          <p className="mt-3 max-w-xl text-ink/70">
            Changes apply to the very next order. No restart, no deploy.
          </p>
        </div>
        <div className="flex gap-3">
          <Stat label="Active sets" value={items.filter((it) => it.active).length} />
          <Stat label="Live promotions" value={rules.filter((r) => r.active).length} />
        </div>
      </section>

      <div
        aria-live="polite"
        className="pointer-events-none fixed inset-x-0 bottom-6 z-30 flex flex-col items-center gap-2 px-4"
      >
        {error && (
          <p
            role="alert"
            className="pointer-events-auto flex max-w-md animate-rise items-start gap-3 rounded-xl bg-white px-4 py-3 text-sm text-red-700 shadow-lg ring-1 ring-red-200"
          >
            <span aria-hidden className="grid size-5 shrink-0 place-items-center rounded-full bg-red-600 text-xs text-white">
              !
            </span>
            <span className="flex-1">{error}</span>
            <button
              type="button"
              aria-label="Dismiss error"
              onClick={() => setError(null)}
              className="text-ink/40 hover:text-ink"
            >
              ×
            </button>
          </p>
        )}
        {notice && (
          <p
            role="status"
            className="pointer-events-auto flex max-w-md animate-rise items-center gap-2 rounded-xl bg-fk-900 px-4 py-3 text-sm font-medium text-white shadow-lg"
          >
            <span aria-hidden className="grid size-5 place-items-center rounded-full bg-fk-400 text-xs text-fk-900">
              ✓
            </span>
            {notice}
          </p>
        )}
      </div>

      <div className="grid gap-6 lg:grid-cols-2 lg:items-start">
        <Panel title="Menu items" subtitle="Inactive sets are hidden from the store." delay={60}>
          <ul className="divide-y divide-black/5">
            {items.map((item) => (
              <li
                key={item.code}
                className={`flex items-center gap-3 px-5 py-3 transition ${item.active ? '' : 'opacity-45'}`}
              >
                <Plate code={item.code} size="sm" />
                <div className="min-w-0 flex-1">
                  <p className="truncate font-medium">{item.name}</p>
                  <p className="font-mono text-[11px] tracking-wider text-ink/45">{item.code}</p>
                </div>
                <span className="font-mono text-sm tabular-nums">{formatTHB(item.price)}</span>
                <Switch label={`${item.name} active`} checked={item.active} onChange={() => toggleItem(item)} />
              </li>
            ))}
          </ul>

          <form onSubmit={addItem} className="grid gap-3 border-t border-black/5 bg-mint/60 p-5 sm:grid-cols-[1fr_1.4fr_1fr]">
            <TextField label="Item code" value={code} onChange={(v) => setCode(v.toUpperCase())} placeholder="BLACK" mono />
            <TextField label="Item name" value={name} onChange={setName} placeholder="Black set" />
            <TextField label="Price (THB)" value={price} onChange={setPrice} placeholder="45.00" prefix="฿" inputMode="decimal" mono />
            <SubmitButton className="sm:col-span-3">Add item</SubmitButton>
          </form>
        </Panel>

        <Panel title="Discount rules" subtitle="Item rules apply first, then whole-order rules." delay={120}>
          <ul className="divide-y divide-black/5">
            {rules.map((rule) => (
              <li
                key={rule.id}
                className={`flex items-center gap-3 px-5 py-3 transition ${rule.active ? '' : 'opacity-45'}`}
              >
                <span
                  aria-hidden
                  className="grid size-9 shrink-0 place-items-center rounded-xl font-mono text-xs font-semibold text-white"
                  style={{ background: rule.itemCode ? itemColor(rule.itemCode) : 'var(--color-fk-600)' }}
                >
                  {rule.percent}%
                </span>
                <div className="min-w-0 flex-1">
                  <p className="truncate font-medium">{rule.name}</p>
                  <p className="text-xs text-ink/55">
                    {rule.itemCode ? `Every ${rule.groupSize} × ${itemName(rule.itemCode)}` : 'Whole order'}
                  </p>
                </div>
                {rule.memberOnly && (
                  <span className="rounded-full bg-sun/25 px-2 py-0.5 text-[11px] font-medium text-amber-900">
                    Members
                  </span>
                )}
                <Switch
                  label={rule.itemCode ? `${rule.name} (${itemName(rule.itemCode)}) active` : `${rule.name} active`}
                  checked={rule.active}
                  onChange={() => toggleRule(rule)}
                />
              </li>
            ))}
          </ul>

          <form onSubmit={addRule} className="grid gap-3 border-t border-black/5 bg-mint/60 p-5 sm:grid-cols-2">
            <TextField label="Rule name" value={ruleName} onChange={setRuleName} placeholder="Buy 3 save 10%" />
            <SelectField label="Rule item" value={ruleItem} onChange={setRuleItem}>
              <option value="">Whole order</option>
              {items.map((item) => (
                <option key={item.code} value={item.code}>
                  {item.name}
                </option>
              ))}
            </SelectField>
            {ruleItem && (
              <TextField label="Group size" value={groupSize} onChange={setGroupSize} placeholder="2" suffix="sets" inputMode="numeric" mono />
            )}
            <TextField label="Percent" value={percent} onChange={setPercent} placeholder="5" suffix="%" inputMode="numeric" mono />
            <label className="flex cursor-pointer items-center gap-2 self-end rounded-lg px-1 py-2 text-sm font-medium text-ink/75">
              <input
                type="checkbox"
                checked={memberOnly}
                onChange={(e) => setMemberOnly(e.target.checked)}
                className="size-4 accent-fk-600"
              />
              Members only
            </label>
            <p className="rounded-lg bg-white/70 px-3 py-2 text-xs text-ink/60 ring-1 ring-black/5 sm:col-span-2">
              <span className="font-medium text-fk-700">Preview · </span>
              {preview}
            </p>
            <SubmitButton className="sm:col-span-2">Add rule</SubmitButton>
          </form>
        </Panel>
      </div>
    </main>
  )
}

// describeRule turns the half-filled rule form into a plain sentence, so an
// admin can read back the promotion before saving it.
function describeRule(item: string, groupSize: string, percent: string, memberOnly: boolean): string {
  const pct = percent || '…'
  const who = memberOnly ? ' for members' : ''
  if (!item) return `${pct}% off the whole order${who}.`
  return `Every ${groupSize || '…'} × ${item} get ${pct}% off${who}; leftovers pay full price.`
}

function Panel({ title, subtitle, delay, children }: { title: string; subtitle: string; delay: number; children: ReactNode }) {
  return (
    <section
      style={{ animationDelay: `${delay}ms` }}
      className="animate-rise overflow-hidden rounded-2xl bg-white shadow-[0_18px_40px_-24px_rgb(0_77_61/0.35)] ring-1 ring-black/5"
    >
      <header className="px-5 pt-5 pb-3">
        <h2 className="font-display text-lg font-semibold text-fk-900">{title}</h2>
        <p className="text-xs text-ink/50">{subtitle}</p>
      </header>
      {children}
    </section>
  )
}

function Stat({ label, value }: { label: string; value: number }) {
  return (
    <div className="min-w-28 rounded-2xl bg-white px-4 py-3 shadow-sm ring-1 ring-black/5">
      <p className="font-mono text-2xl font-semibold text-fk-700 tabular-nums">{value}</p>
      <p className="text-xs text-ink/50">{label}</p>
    </div>
  )
}

const fieldClass =
  'w-full rounded-lg border border-black/10 bg-white px-3 py-2 text-sm shadow-xs transition placeholder:text-ink/30 focus:border-fk-400 focus:ring-3 focus:ring-fk-400/20 focus:outline-none'

type TextFieldProps = {
  label: string
  value: string
  onChange: (value: string) => void
  placeholder?: string
  prefix?: string
  suffix?: string
  inputMode?: 'decimal' | 'numeric'
  mono?: boolean
}

function TextField({ label, value, onChange, placeholder, prefix, suffix, inputMode, mono }: TextFieldProps) {
  const id = useId()
  return (
    <div className="flex flex-col gap-1">
      <label htmlFor={id} className="text-xs font-medium text-ink/70">
        {label}
      </label>
      <div className="relative">
        {prefix && <Adornment side="left">{prefix}</Adornment>}
        <input
          id={id}
          value={value}
          onChange={(e) => onChange(e.target.value)}
          placeholder={placeholder}
          inputMode={inputMode}
          className={`${fieldClass} ${mono ? 'font-mono' : ''} ${prefix ? 'pl-7' : ''} ${suffix ? 'pr-12' : ''}`}
        />
        {suffix && <Adornment side="right">{suffix}</Adornment>}
      </div>
    </div>
  )
}

function Adornment({ side, children }: { side: 'left' | 'right'; children: string }) {
  return (
    <span
      aria-hidden
      className={`pointer-events-none absolute inset-y-0 flex items-center text-xs text-ink/40 ${
        side === 'left' ? 'left-3' : 'right-3'
      }`}
    >
      {children}
    </span>
  )
}

type SelectFieldProps = { label: string; value: string; onChange: (value: string) => void; children: ReactNode }

function SelectField({ label, value, onChange, children }: SelectFieldProps) {
  const id = useId()
  return (
    <div className="flex flex-col gap-1">
      <label htmlFor={id} className="text-xs font-medium text-ink/70">
        {label}
      </label>
      <select id={id} value={value} onChange={(e) => onChange(e.target.value)} className={fieldClass}>
        {children}
      </select>
    </div>
  )
}

function SubmitButton({ children, className = '' }: { children: string; className?: string }) {
  return (
    <button
      type="submit"
      className={`rounded-lg bg-fk-600 px-4 py-2.5 text-sm font-medium text-white shadow-sm transition hover:bg-fk-700 focus-visible:ring-2 focus-visible:ring-fk-400 focus-visible:ring-offset-2 focus-visible:outline-none active:scale-[0.99] ${className}`}
    >
      {children}
    </button>
  )
}

function Switch({ label, checked, onChange }: { label: string; checked: boolean; onChange: () => void }) {
  return (
    <button
      type="button"
      role="switch"
      aria-checked={checked}
      aria-label={label}
      onClick={onChange}
      className={`relative h-6 w-11 shrink-0 rounded-full transition focus-visible:ring-2 focus-visible:ring-fk-400 focus-visible:ring-offset-2 focus-visible:outline-none ${
        checked ? 'bg-fk-500' : 'bg-ink/15'
      }`}
    >
      <span
        className={`absolute top-0.5 left-0.5 size-5 rounded-full bg-white shadow transition-transform duration-300 ${
          checked ? 'translate-x-5' : ''
        }`}
      />
    </button>
  )
}

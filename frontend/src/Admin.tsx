import { useEffect, useId, useMemo, useRef, useState, type FormEvent, type ReactNode } from 'react'
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
import { codeFromColor, displayCode, firstFreeColor, honeycomb, itemColor } from './itemColor'
import Plate from './Plate'
import {
  MAX_NAME,
  MAX_QUANTITY,
  validateItem,
  validateRule,
  type ItemErrors,
  type RuleErrors,
  type RowErrors,
} from './validation'

export default function Admin() {
  const [items, setItems] = useState<AdminItem[]>([])
  const [chosenColor, setChosenColor] = useState<string | null>(null)
  const [name, setName] = useState('')
  const [price, setPrice] = useState('')
  const [rules, setRules] = useState<Rule[]>([])
  const [ruleName, setRuleName] = useState('')
  const [bundleRows, setBundleRows] = useState<BundleRow[]>([])
  const [percent, setPercent] = useState('')
  const [memberOnly, setMemberOnly] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [notice, setNotice] = useState<string | null>(null)
  // busy blocks a second write while one is in flight (a double click would
  // otherwise add the same rule twice); the ref also covers clicks in the same tick.
  const [busy, setBusy] = useState(false)
  const writing = useRef(false)
  // Field errors appear once a form has been submitted, then follow every edit.
  const [itemTried, setItemTried] = useState(false)
  const [ruleTried, setRuleTried] = useState(false)

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
    if (writing.current) return
    writing.current = true
    setBusy(true)
    setError(null)
    setNotice(null)
    try {
      setNotice(await action())
    } catch (err) {
      setError((err as Error).message)
    } finally {
      writing.current = false
      setBusy(false)
    }
  }

  const takenCodes = useMemo(() => new Set(items.map((it) => it.code)), [items])
  const color = chosenColor ?? firstFreeColor(takenCodes)
  const itemCheck = validateItem({ code: color && codeFromColor(color), name, price }, takenCodes)
  const ruleCheck = validateRule({ name: ruleName, rows: bundleRows, percent, memberOnly })
  const itemErrors: ItemErrors = itemCheck.ok ? {} : itemTried ? itemCheck.errors : { code: itemCheck.errors.code }
  const ruleErrors: RuleErrors | undefined = !ruleCheck.ok && ruleTried ? ruleCheck.errors : undefined

  const addRule = (e: FormEvent) => {
    e.preventDefault()
    setRuleTried(true)
    if (!ruleCheck.ok) return
    return run(async () => {
      const created = await createRule(ruleCheck.value)
      setRules((list) => [...list, created])
      setRuleName('')
      setBundleRows([])
      setPercent('')
      setMemberOnly(false)
      setRuleTried(false)
      return `Rule “${created.name}” is live`
    })
  }

  const addItem = (e: FormEvent) => {
    e.preventDefault()
    setItemTried(true)
    if (!itemCheck.ok) return
    return run(async () => {
      const created = await createItem(itemCheck.value)
      setItems((list) => [...list, created])
      setChosenColor(null)
      setName('')
      setPrice('')
      setItemTried(false)
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
  const preview = describeRule(bundleRows, itemName, percent, memberOnly)

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
                  <p className="font-mono text-[11px] tracking-wider text-ink/45">{displayCode(item.code)}</p>
                </div>
                <span className="font-mono text-sm tabular-nums">{formatTHB(item.price)}</span>
                <Switch label={`${item.name} active`} checked={item.active} disabled={busy} onChange={() => toggleItem(item)} />
              </li>
            ))}
          </ul>

          <form noValidate autoComplete="off" onSubmit={addItem} className="grid gap-3 rounded-b-2xl border-t border-black/5 bg-mint/60 p-5 sm:grid-cols-[1fr_1.4fr_1fr]">
            <ColorField label="Item code" value={color ?? ''} taken={takenCodes} error={itemErrors.code} onChange={setChosenColor} />
            <TextField label="Item name" value={name} onChange={setName} placeholder="Black set" maxLength={MAX_NAME} error={itemErrors.name} />
            <TextField label="Price (THB)" value={price} onChange={setPrice} placeholder="45.00" prefix="฿" inputMode="decimal" maxLength={12} mono error={itemErrors.price} />
            <SubmitButton busy={busy} className="sm:col-span-3">Add item</SubmitButton>
          </form>
        </Panel>

        <Panel title="Discount rules" subtitle="Bundles apply first, biggest first, then whole-order rules." delay={120}>
          <ul className="divide-y divide-black/5">
            {rules.map((rule) => (
              <li
                key={rule.id}
                className={`flex items-center gap-3 px-5 py-3 transition ${rule.active ? '' : 'opacity-45'}`}
              >
                <span
                  aria-hidden
                  className="grid size-9 shrink-0 place-items-center rounded-xl font-mono text-xs font-semibold text-white"
                  style={{ background: rule.bundle.length ? itemColor(rule.bundle[0].itemCode) : 'var(--color-fk-600)' }}
                >
                  {rule.percent}%
                </span>
                <div className="min-w-0 flex-1">
                  <p className="truncate font-medium">{rule.name}</p>
                  <p className="text-xs text-ink/55">
                    {rule.bundle.length ? describeBundle(rule.bundle, itemName) : 'Whole order'}
                  </p>
                </div>
                {rule.memberOnly && (
                  <span className="rounded-full bg-sun/25 px-2 py-0.5 text-[11px] font-medium text-amber-900">
                    Members
                  </span>
                )}
                <Switch
                  label={`${rule.name} active`}
                  checked={rule.active}
                  disabled={busy}
                  onChange={() => toggleRule(rule)}
                />
              </li>
            ))}
          </ul>

          <form noValidate autoComplete="off" onSubmit={addRule} className="grid gap-3 rounded-b-2xl border-t border-black/5 bg-mint/60 p-5 sm:grid-cols-2">
            <TextField label="Rule name" value={ruleName} onChange={setRuleName} placeholder="Buy 3 save 10%" maxLength={MAX_NAME} error={ruleErrors?.name} />
            <BundleEditor items={items} rows={bundleRows} errors={ruleErrors?.rows} onChange={setBundleRows} />
            <TextField label="Percent" value={percent} onChange={setPercent} placeholder="5" suffix="%" inputMode="numeric" maxLength={3} mono error={ruleErrors?.percent} />
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
            <SubmitButton busy={busy} className="sm:col-span-2">Add rule</SubmitButton>
          </form>
        </Panel>
      </div>
    </main>
  )
}

type BundleRow = { itemCode: string; qty: string }

// describeBundle reads a bundle back as "2 × Green set + 1 × Red set".
function describeBundle(bundle: { itemCode: string; qty: number | string }[], itemName: (code: string) => string): string {
  return bundle.map((c) => `${c.qty} × ${itemName(c.itemCode)}`).join(' + ')
}

// describeRule turns the half-filled rule form into a plain sentence, so an
// admin can read back the promotion before saving it.
function describeRule(rows: BundleRow[], itemName: (code: string) => string, percent: string, memberOnly: boolean): string {
  const pct = percent || '…'
  const who = memberOnly ? ' for members' : ''
  if (rows.length === 0) return `${pct}% off the whole order${who}.`
  const bundle = describeBundle(
    rows.map((row) => ({ itemCode: row.itemCode, qty: row.qty || '…' })),
    (code) => (code ? itemName(code) : '…'),
  )
  return `Every bundle of ${bundle} gets ${pct}% off${who}; sets outside a complete bundle pay full price.`
}

// BundleEditor edits the rows of a bundle; no rows means a whole-order rule.
function BundleEditor({
  items,
  rows,
  errors = [],
  onChange,
}: {
  items: AdminItem[]
  rows: BundleRow[]
  errors?: RowErrors[]
  onChange: (rows: BundleRow[]) => void
}) {
  const setRow = (i: number, patch: Partial<BundleRow>) =>
    onChange(rows.map((row, j) => (j === i ? { ...row, ...patch } : row)))
  return (
    <div className="flex flex-col gap-2 sm:col-span-2">
      <p className="text-xs font-medium text-ink/70">Bundle items {rows.length === 0 && <span className="font-normal text-ink/45">(none: applies to the whole order)</span>}</p>
      {rows.map((row, i) => {
        const itemError = errors[i]?.item
        const qtyError = errors[i]?.qty
        return (
          <div key={i} className="grid grid-cols-[1fr_6rem_auto] items-start gap-2">
            <div className="flex flex-col gap-1">
              <select
                aria-label={`Bundle item ${i + 1}`}
                aria-invalid={itemError ? true : undefined}
                value={row.itemCode}
                onChange={(e) => setRow(i, { itemCode: e.target.value })}
                className={`${fieldClass} ${itemError ? invalidClass : ''}`}
              >
                <option value="">Choose item</option>
                {items.map((item) => (
                  <option
                    key={item.code}
                    value={item.code}
                    disabled={rows.some((other, j) => j !== i && other.itemCode === item.code)}
                  >
                    {item.active ? item.name : `${item.name} (off the menu)`}
                  </option>
                ))}
              </select>
              <FieldError>{itemError}</FieldError>
            </div>
            <div className="flex flex-col gap-1">
              <input
                aria-label={`Bundle quantity ${i + 1}`}
                aria-invalid={qtyError ? true : undefined}
                value={row.qty}
                onChange={(e) => setRow(i, { qty: e.target.value })}
                placeholder="2"
                inputMode="numeric"
                maxLength={String(MAX_QUANTITY).length}
                className={`${fieldClass} font-mono ${qtyError ? invalidClass : ''}`}
              />
              <FieldError>{qtyError}</FieldError>
            </div>
            <button
              type="button"
              aria-label={`Remove bundle item ${i + 1}`}
              onClick={() => onChange(rows.filter((_, j) => j !== i))}
              className="rounded-lg px-2 py-2 text-ink/40 transition hover:bg-black/5 hover:text-ink"
            >
              ×
            </button>
          </div>
        )
      })}
      <button
        type="button"
        onClick={() => onChange([...rows, { itemCode: '', qty: '' }])}
        className="self-start rounded-lg px-2 py-1.5 text-sm font-medium text-fk-700 transition hover:bg-fk-50"
      >
        <span aria-hidden>+ </span>Add item to bundle
      </button>
    </div>
  )
}

function Panel({ title, subtitle, delay, children }: { title: string; subtitle: string; delay: number; children: ReactNode }) {
  return (
    <section
      style={{ animationDelay: `${delay}ms` }}
      className="relative animate-rise rounded-2xl bg-white has-[[aria-expanded=true]]:z-20 shadow-[0_18px_40px_-24px_rgb(0_77_61/0.35)] ring-1 ring-black/5"
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

const invalidClass = 'border-red-400 focus:border-red-500 focus:ring-red-400/20'

// FieldError shows the message under a field; it renders nothing without one.
function FieldError({ id, children }: { id?: string; children?: string }) {
  if (!children) return null
  return (
    <p id={id} role="alert" className="text-xs text-red-700">
      {children}
    </p>
  )
}

type TextFieldProps = {
  label: string
  value: string
  onChange: (value: string) => void
  placeholder?: string
  prefix?: string
  suffix?: string
  inputMode?: 'decimal' | 'numeric'
  maxLength?: number
  mono?: boolean
  error?: string
}

function TextField({ label, value, onChange, placeholder, prefix, suffix, inputMode, maxLength, mono, error }: TextFieldProps) {
  const id = useId()
  const errorId = `${id}-error`
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
          maxLength={maxLength}
          aria-invalid={error ? true : undefined}
          aria-describedby={error ? errorId : undefined}
          className={`${fieldClass} ${mono ? 'font-mono' : ''} ${prefix ? 'pl-7' : ''} ${suffix ? 'pr-12' : ''} ${error ? invalidClass : ''}`}
        />
        {suffix && <Adornment side="right">{suffix}</Adornment>}
      </div>
      <FieldError id={errorId}>{error}</FieldError>
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

// ColorField picks an item's colour from the honeycomb tray; its hex digits
// become the item code.
function ColorField({
  label,
  value,
  taken,
  error,
  onChange,
}: {
  label: string
  value: string
  taken: ReadonlySet<string>
  error?: string
  onChange: (value: string) => void
}) {
  const id = useId()
  const errorId = `${id}-error`
  const [open, setOpen] = useState(false)
  const pick = (color: string) => {
    onChange(color)
    setOpen(false)
  }
  return (
    <div className="relative flex flex-col gap-1">
      <label htmlFor={id} className="text-xs font-medium text-ink/70">
        {label}
      </label>
      <button
        id={id}
        type="button"
        aria-expanded={open}
        aria-invalid={error ? true : undefined}
        aria-describedby={error ? errorId : undefined}
        onClick={() => setOpen((o) => !o)}
        onKeyDown={(e) => e.key === 'Escape' && setOpen(false)}
        className={`${fieldClass} flex items-center gap-2 py-1.5 text-left ${error ? invalidClass : ''}`}
      >
        <span className="size-6 shrink-0 rounded-md ring-1 ring-black/10" style={{ background: value }} />
        <span className="flex-1 font-mono">{value ? displayCode(codeFromColor(value)) : 'None free'}</span>
        <span aria-hidden className="text-ink/35">▾</span>
      </button>
      <FieldError id={errorId}>{error}</FieldError>

      {open && (
        <>
          <div aria-hidden className="fixed inset-0 z-30" onClick={() => setOpen(false)} />
          <div
            role="radiogroup"
            aria-label="Item colour"
            onKeyDown={(e) => e.key === 'Escape' && setOpen(false)}
            className="absolute top-full left-0 z-40 mt-2 animate-rise rounded-2xl bg-white p-4 shadow-xl ring-1 ring-black/10"
          >
            {honeycomb.map((row, i) => (
              <div key={i} className="-mt-[5px] flex justify-center gap-[3px] first:mt-0">
                {row.map((color) => {
                  const used = taken.has(codeFromColor(color))
                  return (
                    <button
                      key={color}
                      type="button"
                      role="radio"
                      aria-checked={color === value.toUpperCase()}
                      aria-label={color}
                      title={used ? `${color} (in use)` : color}
                      disabled={used}
                      onClick={() => pick(color)}
                      className={`h-[27px] w-6 transition [clip-path:polygon(50%_0,100%_25%,100%_75%,50%_100%,0_75%,0_25%)] focus-visible:outline-none enabled:hover:scale-125 enabled:focus-visible:scale-125 disabled:cursor-not-allowed disabled:opacity-20 ${
                        color === value.toUpperCase() ? 'scale-125' : ''
                      }`}
                      style={{ background: color }}
                    />
                  )
                })}
              </div>
            ))}
          </div>
        </>
      )}
    </div>
  )
}

function SubmitButton({ children, busy, className = '' }: { children: string; busy: boolean; className?: string }) {
  return (
    <button
      type="submit"
      disabled={busy}
      className={`rounded-lg bg-fk-600 px-4 py-2.5 text-sm font-medium text-white shadow-sm transition hover:bg-fk-700 focus-visible:ring-2 focus-visible:ring-fk-400 focus-visible:ring-offset-2 focus-visible:outline-none active:scale-[0.99] disabled:cursor-wait disabled:opacity-60 ${className}`}
    >
      {busy ? 'Saving…' : children}
    </button>
  )
}

function Switch({ label, checked, disabled, onChange }: { label: string; checked: boolean; disabled: boolean; onChange: () => void }) {
  return (
    <button
      type="button"
      role="switch"
      aria-checked={checked}
      aria-label={label}
      disabled={disabled}
      onClick={onChange}
      className={`relative h-6 w-11 shrink-0 rounded-full transition disabled:cursor-wait disabled:opacity-60 focus-visible:ring-2 focus-visible:ring-fk-400 focus-visible:ring-offset-2 focus-visible:outline-none ${
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

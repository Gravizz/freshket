import { useEffect, useState } from 'react'
import { calculate, fetchMenu, formatTHB, type Breakdown, type MenuItem } from './api'
import { displayCode } from './itemColor'
import Plate from './Plate'

export default function App() {
  const [menu, setMenu] = useState<MenuItem[] | null>(null)
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

  const lines = (menu ?? [])
    .filter((item) => (quantities[item.code] ?? 0) > 0)
    .map((item) => ({ item, qty: quantities[item.code] }))

  const setCount = lines.reduce((n, line) => n + line.qty, 0)

  return (
    <main className="mx-auto max-w-6xl px-4 pb-20 sm:px-6">
      <section className="flex animate-rise flex-wrap items-end justify-between gap-8 pt-6 pb-8 sm:pt-10">
        <div>
          <p className="inline-flex items-center gap-2 rounded-full bg-white/80 px-3 py-1 text-xs font-medium text-fk-700 ring-1 ring-fk-200">
            <span className="size-1.5 rounded-full bg-fk-400" />
            Food store calculator
          </p>
          <h1 className="mt-4 font-display text-4xl font-semibold tracking-tight text-fk-900 sm:text-5xl">
            Build today’s order<span className="text-fk-400">.</span>
          </h1>
          <p className="mt-3 max-w-xl text-base text-ink/70">
            Pick your sets and show a member card. Every discount is priced by the server, to the satang.
            <span className="mt-1 block text-sm text-ink/50">เลือกชุดอาหาร แล้วระบบคำนวณส่วนลดให้อัตโนมัติ</span>
          </p>
        </div>

        <ol className="hidden gap-2 text-xs md:flex" aria-label="How your total is built">
          {[
            ['Subtotal', 'price × sets'],
            ['Set promos', 'per complete group'],
            ['Order promos', 'on the running total'],
          ].map(([title, hint], i) => (
            <li key={title} className="flex items-center gap-2">
              {i > 0 && <span aria-hidden className="text-fk-400">→</span>}
              <span className="rounded-xl bg-white/80 px-3 py-2 ring-1 ring-fk-200">
                <span className="mr-1.5 font-mono text-fk-400">0{i + 1}</span>
                <span className="font-medium text-fk-900">{title}</span>
                <span className="block text-ink/45">{hint}</span>
              </span>
            </li>
          ))}
        </ol>
      </section>

      <div className="grid gap-8 lg:grid-cols-[minmax(0,1fr)_380px] lg:items-start">
        <section aria-labelledby="menu-heading">
          <div className="mb-4 flex items-baseline justify-between">
            <h2 id="menu-heading" className="font-display text-xl font-semibold text-fk-900">
              Menu
            </h2>
            {menu && <span className="text-sm text-ink/50">{menu.length} sets available</span>}
          </div>

          <ul className="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
            {menu === null && !error
              ? Array.from({ length: 6 }, (_, i) => (
                  <li key={i} className="h-44 animate-pulse rounded-2xl bg-white/60 ring-1 ring-black/5" />
                ))
              : menu?.map((item, i) => (
                  <MenuCard
                    key={item.code}
                    item={item}
                    qty={quantities[item.code] ?? 0}
                    onChange={(qty) => setQty(item.code, qty)}
                    delay={i * 50}
                  />
                ))}
          </ul>
        </section>

        <aside className="space-y-5 lg:sticky lg:top-24">
          <MemberCard checked={member} onChange={setMember} />

          {error && (
            <p role="alert" className="rounded-xl bg-red-50 px-4 py-3 text-sm text-red-700 ring-1 ring-red-200">
              {error}
            </p>
          )}

          <div id="order-summary" className="scroll-mt-24 drop-shadow-[0_18px_30px_rgb(0_77_61/0.12)]">
            <div className="receipt-tear rounded-t-2xl bg-white px-6 pt-6 pb-10">
              <div className="flex items-center justify-between">
                <h2 className="font-display text-lg font-semibold text-fk-900">Order summary</h2>
                {lines.length > 0 && (
                  <button
                    type="button"
                    onClick={() => setQuantities({})}
                    className="rounded-md px-2 py-1 text-xs font-medium text-ink/50 transition hover:bg-mint hover:text-fk-700"
                  >
                    Clear
                  </button>
                )}
              </div>

              {lines.length === 0 ? (
                <div className="mt-5 flex flex-col items-center rounded-xl border border-dashed border-fk-200 bg-mint/60 px-4 py-6 text-center">
                  <div className="flex -space-x-3">
                    <Plate code="ORANGE" size="sm" />
                    <Plate code="GREEN" size="sm" />
                    <Plate code="PINK" size="sm" />
                  </div>
                  <p className="mt-3 text-sm font-medium text-ink/70">Your basket is empty</p>
                  <p className="text-xs text-ink/45">Add a set from the menu to see the price.</p>
                </div>
              ) : (
                <ul className="mt-4 space-y-3">
                  {lines.map(({ item, qty }) => (
                    <li key={item.code} className="flex items-center gap-3 text-sm">
                      <Plate code={item.code} size="sm" />
                      <span className="flex-1 font-medium">{item.name}</span>
                      <span className="font-mono text-xs text-ink/55">
                        {qty} × {formatTHB(item.price)}
                      </span>
                    </li>
                  ))}
                </ul>
              )}

              {breakdown && (
                <>
                  <dl className="mt-5 space-y-2 border-t border-dashed border-ink/15 pt-4 text-sm">
                    <div className="flex justify-between">
                      <dt className="text-ink/60">Subtotal</dt>
                      <dd className="font-mono tabular-nums">{formatTHB(breakdown.subtotal)}</dd>
                    </div>
                    {breakdown.discounts.map((d, i) => (
                      <div key={i} className="flex animate-rise justify-between gap-3 text-fk-700">
                        <dt className="flex items-start gap-1.5">
                          <TagIcon />
                          <span>{d.label}</span>
                        </dt>
                        <dd className="font-mono whitespace-nowrap tabular-nums">−{formatTHB(d.amount)}</dd>
                      </div>
                    ))}
                  </dl>

                  <dl className="mt-4 flex items-end justify-between border-t border-ink/10 pt-4">
                    <dt className="pb-1 font-display font-medium text-fk-900">Total</dt>
                    <dd
                      key={breakdown.total}
                      aria-live="polite"
                      className="animate-pop font-mono text-3xl font-semibold tracking-tight text-fk-900 tabular-nums"
                    >
                      {formatTHB(breakdown.total)}
                    </dd>
                  </dl>

                  {breakdown.subtotal > breakdown.total && (
                    <p className="mt-4 flex animate-rise items-center justify-center gap-2 rounded-full bg-sun/20 px-3 py-1.5 text-sm font-medium text-amber-900">
                      <span aria-hidden>🎉</span>
                      You save {formatTHB(breakdown.subtotal - breakdown.total)}
                    </p>
                  )}
                </>
              )}
            </div>
          </div>
        </aside>
      </div>

      {setCount > 0 && (
        <button
          type="button"
          onClick={() => document.getElementById('order-summary')?.scrollIntoView({ behavior: 'smooth' })}
          className="fixed inset-x-4 bottom-4 z-30 flex animate-rise items-center justify-between rounded-full bg-fk-900 py-2 pr-2 pl-5 text-sm font-medium text-white shadow-xl lg:hidden"
        >
          <span>
            {setCount} {setCount === 1 ? 'set' : 'sets'} in your basket
          </span>
          <span className="rounded-full bg-fk-400 px-4 py-2 text-fk-900">View order ↓</span>
        </button>
      )}
    </main>
  )
}

type MenuCardProps = {
  item: MenuItem
  qty: number
  onChange: (qty: number) => void
  delay: number
}

function MenuCard({ item, qty, onChange, delay }: MenuCardProps) {
  const selected = qty > 0
  return (
    <li
      style={{ animationDelay: `${delay}ms` }}
      className={`group relative grid animate-rise grid-cols-[auto_1fr_auto] items-center gap-x-4 rounded-2xl bg-white p-3 transition duration-300 sm:flex sm:flex-col sm:items-stretch sm:p-4 ${
        selected
          ? 'shadow-[0_14px_30px_-12px_rgb(0_128_101/0.45)] ring-2 ring-fk-400'
          : 'shadow-sm ring-1 ring-black/5 hover:-translate-y-0.5 hover:shadow-md'
      }`}
    >
      <span className="absolute top-3 right-4 hidden font-mono text-[10px] tracking-wider text-ink/35 uppercase sm:block">
        {displayCode(item.code)}
      </span>
      <div className="sm:self-start">
        <div className="relative">
          <Plate code={item.code} className="transition duration-500 group-hover:rotate-12 max-sm:size-14" />
          {selected && (
            <span
              key={qty}
              aria-hidden
              className="absolute -top-1 -right-1 grid h-6 min-w-6 animate-pop place-items-center rounded-full bg-fk-600 px-1.5 font-mono text-xs font-semibold text-white ring-2 ring-white"
            >
              {qty}
            </span>
          )}
        </div>
      </div>

      <div className="min-w-0 sm:mt-3">
        <p className="truncate font-display text-lg font-medium text-ink">{item.name}</p>
        <p className="text-sm text-ink/55">
          <span className="font-mono text-ink/80">{formatTHB(item.price)}</span> / set
        </p>
      </div>

      <div className="w-28 sm:mt-4 sm:w-auto">
        {selected ? (
          <div className="flex items-center justify-between rounded-full bg-mint p-1 ring-1 ring-fk-200">
            <StepButton label={`Remove ${item.name}`} onClick={() => onChange(qty - 1)}>
              −
            </StepButton>
            <span className="font-mono font-semibold text-fk-900 tabular-nums">{qty}</span>
            <StepButton label={`Add ${item.name}`} onClick={() => onChange(qty + 1)}>
              +
            </StepButton>
          </div>
        ) : (
          <button
            type="button"
            aria-label={`Add ${item.name}`}
            onClick={() => onChange(1)}
            className="w-full rounded-full bg-fk-600 px-4 py-2 text-sm font-medium text-white transition hover:bg-fk-700 focus-visible:ring-2 focus-visible:ring-fk-400 focus-visible:ring-offset-2 focus-visible:outline-none active:scale-[0.98]"
          >
            + Add<span className="hidden sm:inline"> to order</span>
          </button>
        )}
      </div>
    </li>
  )
}

function StepButton({ label, onClick, children }: { label: string; onClick: () => void; children: string }) {
  return (
    <button
      type="button"
      aria-label={label}
      onClick={onClick}
      className="grid size-8 place-items-center rounded-full bg-white text-lg leading-none text-fk-700 shadow-sm ring-1 ring-fk-200 transition hover:bg-fk-600 hover:text-white focus-visible:ring-2 focus-visible:ring-fk-400 focus-visible:outline-none active:scale-90"
    >
      {children}
    </button>
  )
}

function MemberCard({ checked, onChange }: { checked: boolean; onChange: (v: boolean) => void }) {
  return (
    <label
      className={`relative block cursor-pointer overflow-hidden rounded-2xl p-5 transition duration-500 has-[:focus-visible]:ring-2 has-[:focus-visible]:ring-fk-400 has-[:focus-visible]:ring-offset-2 ${
        checked
          ? 'bg-linear-to-br from-fk-600 via-fk-700 to-fk-900 text-white shadow-[0_18px_40px_-14px_rgb(0_102_80/0.7)]'
          : 'bg-white text-ink ring-1 ring-black/5 hover:ring-fk-200'
      }`}
    >
      <input
        type="checkbox"
        aria-label="Member card"
        checked={checked}
        onChange={(e) => onChange(e.target.checked)}
        className="sr-only"
      />
      {checked && (
        <span
          aria-hidden
          className="pointer-events-none absolute inset-y-0 left-0 w-1/3 animate-shine bg-linear-to-r from-transparent via-white/25 to-transparent"
        />
      )}
      <span
        aria-hidden
        className={`pointer-events-none absolute -right-10 -bottom-14 size-40 rounded-full ${
          checked ? 'bg-fk-400/30' : 'bg-fk-100'
        }`}
      />

      <span className="relative flex items-start justify-between">
        <span className="font-display text-lg font-bold tracking-tight">
          <span className={checked ? 'text-white' : 'text-fk-600'}>fresh</span>
          <span className="text-fk-400">ket</span>
          <span className={`ml-2 text-[10px] font-medium tracking-[0.25em] ${checked ? 'text-white/70' : 'text-ink/40'}`}>
            MEMBER
          </span>
        </span>
        <span
          className={`relative h-6 w-11 rounded-full transition ${checked ? 'bg-fk-400' : 'bg-ink/15'}`}
          aria-hidden
        >
          <span
            className={`absolute top-0.5 left-0.5 size-5 rounded-full bg-white shadow transition-transform duration-300 ${
              checked ? 'translate-x-5' : ''
            }`}
          />
        </span>
      </span>

      <span className="relative mt-6 flex items-end justify-between">
        <span>
          <span className={`block text-xs ${checked ? 'text-white/70' : 'text-ink/50'}`}>
            {checked ? 'Member pricing applied' : 'Have a member card?'}
          </span>
          <span className="font-display text-base font-medium">
            {checked ? 'Card on file ✓' : 'Tap to apply member pricing'}
          </span>
        </span>
        <span
          aria-hidden
          className={`h-7 w-9 rounded-md ${
            checked ? 'bg-linear-to-br from-sun to-amber-500' : 'bg-linear-to-br from-ink/10 to-ink/20'
          }`}
        />
      </span>
    </label>
  )
}

function TagIcon() {
  return (
    <svg aria-hidden viewBox="0 0 16 16" className="mt-0.5 size-3.5 shrink-0" fill="currentColor">
      <path d="M1 2.5A1.5 1.5 0 0 1 2.5 1h4.38a1.5 1.5 0 0 1 1.06.44l6.62 6.62a1.5 1.5 0 0 1 0 2.12l-4.38 4.38a1.5 1.5 0 0 1-2.12 0L1.44 7.94A1.5 1.5 0 0 1 1 6.88V2.5Zm3.5 3a1 1 0 1 0 0-2 1 1 0 0 0 0 2Z" />
    </svg>
  )
}

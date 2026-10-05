import { useEffect, useState } from 'react'
import Admin from './Admin'
import App from './App'

export default function Root() {
  const [hash, setHash] = useState(location.hash)

  useEffect(() => {
    const onChange = () => {
      setHash(location.hash)
      window.scrollTo(0, 0)
    }
    window.addEventListener('hashchange', onChange)
    return () => window.removeEventListener('hashchange', onChange)
  }, [])

  const isAdmin = hash === '#/admin'

  return (
    <>
      <header className="sticky top-0 z-20 border-b border-fk-200/60 bg-mint/80 backdrop-blur-md">
        <div className="mx-auto flex h-16 max-w-6xl items-center justify-between gap-4 px-4 sm:px-6">
          <a href="#/" className="flex items-center gap-3" aria-label="freshket home">
            <span className="font-display text-2xl leading-none font-bold tracking-tight">
              <span className="text-fk-600">fresh</span>
              <span className="text-fk-400">ket</span>
            </span>
            <span className="hidden h-7 w-px bg-fk-600/25 sm:block" />
            <span className="hidden font-display text-[11px] leading-tight font-medium text-fk-600 sm:block">
              สดจริง ส่งไว
              <br />
              ใส่ใจร้านอาหาร
            </span>
          </a>

          <nav className="flex rounded-full bg-white p-1 text-sm font-medium shadow-sm ring-1 ring-black/5">
            <NavLink href="#/" active={!isAdmin}>
              Store
            </NavLink>
            <NavLink href="#/admin" active={isAdmin}>
              Admin
            </NavLink>
          </nav>
        </div>
      </header>

      {isAdmin ? <Admin /> : <App />}

      <footer className="mx-auto max-w-6xl px-4 pb-10 text-xs text-ink/40 sm:px-6">
        Prices are calculated by the Go API in integer satang · React + Tailwind front end
      </footer>
    </>
  )
}

function NavLink({ href, active, children }: { href: string; active: boolean; children: string }) {
  return (
    <a
      href={href}
      aria-current={active ? 'page' : undefined}
      className={`rounded-full px-4 py-1.5 transition ${
        active ? 'bg-fk-600 text-white shadow-sm' : 'text-ink/60 hover:text-fk-700'
      }`}
    >
      {children}
    </a>
  )
}

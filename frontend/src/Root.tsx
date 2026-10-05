import { useEffect, useState } from 'react'
import Admin from './Admin'
import App from './App'

export default function Root() {
  const [hash, setHash] = useState(location.hash)

  useEffect(() => {
    const onChange = () => setHash(location.hash)
    window.addEventListener('hashchange', onChange)
    return () => window.removeEventListener('hashchange', onChange)
  }, [])

  const isAdmin = hash === '#/admin'

  return (
    <>
      <header className="flex justify-end px-4 pt-4">
        <a
          href={isAdmin ? '#/' : '#/admin'}
          className="rounded-md border border-slate-300 px-3 py-1 text-sm text-slate-700 hover:bg-slate-100"
        >
          {isAdmin ? 'Store' : 'Admin'}
        </a>
      </header>
      {isAdmin ? <Admin /> : <App />}
    </>
  )
}

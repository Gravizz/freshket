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

  return hash === '#/admin' ? <Admin /> : <App />
}

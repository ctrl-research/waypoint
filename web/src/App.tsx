import { useEffect, useState } from 'react'
import './App.css'

function App() {
  const [apiStatus, setApiStatus] = useState<'checking' | 'ok' | 'unreachable'>('checking')

  useEffect(() => {
    fetch('/api/v1/ping')
      .then((res) => setApiStatus(res.ok ? 'ok' : 'unreachable'))
      .catch(() => setApiStatus('unreachable'))
  }, [])

  return (
    <main>
      <h1>Waypoint</h1>
      <p>Self-hosted travel planner, logger, and tracker.</p>
      <p>
        API:{' '}
        {apiStatus === 'checking' ? 'checking…' : apiStatus === 'ok' ? 'connected' : 'unreachable'}
      </p>
    </main>
  )
}

export default App

import { useQuery } from '@tanstack/react-query'
import { Navigate } from '@tanstack/react-router'
import { fetchMe } from '../api'

export function HomePage() {
  const { data: me, isLoading } = useQuery({ queryKey: ['me'], queryFn: fetchMe })

  if (isLoading) return null
  if (!me) return <Navigate to="/login" />

  return (
    <div className="mx-auto mt-12 w-full max-w-3xl px-4">
      <h1 className="text-2xl font-semibold text-slate-900">Welcome, {me.displayName || me.email}</h1>
      <p className="mt-2 text-slate-500">
        Your trips will live here — trip planning arrives with milestone M2.
      </p>
    </div>
  )
}

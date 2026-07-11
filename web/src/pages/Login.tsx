import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Navigate, useNavigate } from '@tanstack/react-router'
import { ApiError, fetchMe, fetchProviders, login } from '../api'

export function LoginPage() {
  const { data: me, isLoading: meLoading } = useQuery({ queryKey: ['me'], queryFn: fetchMe })
  const { data: providers } = useQuery({ queryKey: ['providers'], queryFn: fetchProviders })

  if (meLoading) return null
  if (me) return <Navigate to="/" />

  return (
    <div className="mx-auto mt-24 w-full max-w-sm rounded-xl border border-slate-200 bg-white p-8 shadow-sm">
      <h1 className="text-2xl font-semibold text-slate-900">Sign in to Waypoint</h1>
      <p className="mt-1 text-sm text-slate-500">Plan, log, and track your travels.</p>

      <div className="mt-6 space-y-4">
        {providers?.includes('google') && (
          <a
            href="/auth/google"
            className="flex w-full items-center justify-center gap-2 rounded-lg border border-slate-300 px-4 py-2 text-sm font-medium text-slate-700 hover:bg-slate-50"
          >
            <GoogleLogo />
            Sign in with Google
          </a>
        )}

        {providers?.includes('google') && providers?.includes('local') && (
          <div className="flex items-center gap-3 text-xs text-slate-400">
            <div className="h-px flex-1 bg-slate-200" />
            or
            <div className="h-px flex-1 bg-slate-200" />
          </div>
        )}

        {providers?.includes('local') && <LocalLoginForm />}

        {providers && providers.length === 0 && (
          <p className="text-sm text-amber-700">
            No sign-in methods are configured. Set <code>WAYPOINT_GOOGLE_CLIENT_ID</code>/
            <code>SECRET</code> or <code>WAYPOINT_LOCAL_AUTH=true</code> on the server.
          </p>
        )}
      </div>
    </div>
  )
}

function LocalLoginForm() {
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const queryClient = useQueryClient()
  const navigate = useNavigate()

  const mutation = useMutation({
    mutationFn: () => login(email, password),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ['me'] })
      navigate({ to: '/' })
    },
  })

  return (
    <form
      className="space-y-3"
      onSubmit={(e) => {
        e.preventDefault()
        mutation.mutate()
      }}
    >
      <label className="block">
        <span className="text-sm font-medium text-slate-700">Email</span>
        <input
          type="email"
          required
          autoComplete="email"
          value={email}
          onChange={(e) => setEmail(e.target.value)}
          className="mt-1 w-full rounded-lg border border-slate-300 px-3 py-2 text-sm focus:border-slate-500 focus:outline-none"
        />
      </label>
      <label className="block">
        <span className="text-sm font-medium text-slate-700">Password</span>
        <input
          type="password"
          required
          autoComplete="current-password"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          className="mt-1 w-full rounded-lg border border-slate-300 px-3 py-2 text-sm focus:border-slate-500 focus:outline-none"
        />
      </label>
      {mutation.error && (
        <p className="text-sm text-red-600">
          {mutation.error instanceof ApiError ? mutation.error.message : 'Sign-in failed'}
        </p>
      )}
      <button
        type="submit"
        disabled={mutation.isPending}
        className="w-full rounded-lg bg-slate-900 px-4 py-2 text-sm font-medium text-white hover:bg-slate-700 disabled:opacity-50"
      >
        {mutation.isPending ? 'Signing in…' : 'Sign in'}
      </button>
    </form>
  )
}

function GoogleLogo() {
  return (
    <svg width="18" height="18" viewBox="0 0 48 48" aria-hidden="true">
      <path
        fill="#EA4335"
        d="M24 9.5c3.54 0 6.71 1.22 9.21 3.6l6.85-6.85C35.9 2.38 30.47 0 24 0 14.62 0 6.51 5.38 2.56 13.22l7.98 6.19C12.43 13.72 17.74 9.5 24 9.5z"
      />
      <path
        fill="#4285F4"
        d="M46.98 24.55c0-1.57-.15-3.09-.38-4.55H24v9.02h12.94c-.58 2.96-2.26 5.48-4.78 7.18l7.73 6c4.51-4.18 7.09-10.36 7.09-17.65z"
      />
      <path
        fill="#FBBC05"
        d="M10.53 28.59c-.48-1.45-.76-2.99-.76-4.59s.27-3.14.76-4.59l-7.98-6.19C.92 16.46 0 20.12 0 24c0 3.88.92 7.54 2.56 10.78l7.97-6.19z"
      />
      <path
        fill="#34A853"
        d="M24 48c6.48 0 11.93-2.13 15.89-5.81l-7.73-6c-2.15 1.45-4.92 2.3-8.16 2.3-6.26 0-11.57-4.22-13.47-9.91l-7.98 6.19C6.51 42.62 14.62 48 24 48z"
      />
    </svg>
  )
}

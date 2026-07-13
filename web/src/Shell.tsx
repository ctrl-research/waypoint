import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link, Outlet, useNavigate } from '@tanstack/react-router'
import { fetchMe, logout } from './api'

export function Shell() {
  return (
    <div className="min-h-screen bg-slate-50">
      <header className="border-b border-slate-200 bg-white">
        <div className="mx-auto flex h-14 w-full max-w-5xl items-center justify-between px-4">
          <div className="flex items-center gap-5">
            <Link to="/" className="text-lg font-semibold tracking-tight text-slate-900">
              🧭 Waypoint
            </Link>
            <NavLinks />
          </div>
          <UserMenu />
        </div>
      </header>
      <main>
        <Outlet />
      </main>
    </div>
  )
}

function NavLinks() {
  const { data: me } = useQuery({ queryKey: ['me'], queryFn: fetchMe })
  if (!me) return null
  return (
    <Link
      to="/stats"
      className="text-sm text-slate-500 hover:text-slate-900 [&.active]:font-medium [&.active]:text-slate-900"
    >
      Stats
    </Link>
  )
}

function UserMenu() {
  const { data: me } = useQuery({ queryKey: ['me'], queryFn: fetchMe })
  const queryClient = useQueryClient()
  const navigate = useNavigate()

  const signOut = useMutation({
    mutationFn: logout,
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ['me'] })
      navigate({ to: '/login' })
    },
  })

  if (!me) return null

  return (
    <div className="flex items-center gap-3">
      {me.avatarUrl ? (
        <img src={me.avatarUrl} alt="" className="h-8 w-8 rounded-full" referrerPolicy="no-referrer" />
      ) : (
        <div className="flex h-8 w-8 items-center justify-center rounded-full bg-slate-200 text-sm font-medium text-slate-600">
          {(me.displayName || me.email).charAt(0).toUpperCase()}
        </div>
      )}
      <span className="text-sm text-slate-700">{me.displayName || me.email}</span>
      <button
        type="button"
        onClick={() => signOut.mutate()}
        disabled={signOut.isPending}
        className="rounded-lg border border-slate-300 px-3 py-1.5 text-sm text-slate-600 hover:bg-slate-50 disabled:opacity-50"
      >
        Sign out
      </button>
    </div>
  )
}

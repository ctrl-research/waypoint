import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link, Outlet, useNavigate } from '@tanstack/react-router'
import { fetchMe, logout } from './api'
import { CompassLogo } from './CompassLogo'
// Side effect: applies the stored theme and follows OS changes app-wide;
// the picker itself lives on the Settings page.
import './theme'

export function Shell() {
  return (
    <div className="min-h-screen bg-slate-50 dark:bg-slate-950">
      <header className="border-b border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 print:hidden">
        <div className="mx-auto flex h-14 w-full max-w-5xl items-center justify-between px-4">
          <div className="flex items-center gap-5">
            <Link
              to="/"
              className="flex items-center gap-2 text-lg font-semibold tracking-tight text-slate-900 dark:text-slate-100"
            >
              <CompassLogo size={32} /> Waypoint
            </Link>
            <NavLinks />
          </div>
          <div className="flex items-center gap-3">
            <UserMenu />
          </div>
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
  const link =
    'text-sm text-slate-500 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-100 [&.active]:font-medium [&.active]:text-slate-900 dark:[&.active]:text-slate-100'
  return (
    <>
      <Link to="/" activeOptions={{ exact: true }} className={link}>
        Trips
      </Link>
      <Link to="/calendar" className={link}>
        Calendar
      </Link>
      <Link to="/stats" className={link}>
        Stats
      </Link>
      <Link to="/settings" className={link}>
        Settings
      </Link>
    </>
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
        <div className="flex h-8 w-8 items-center justify-center rounded-full bg-slate-200 dark:bg-slate-700 text-sm font-medium text-slate-600 dark:text-slate-400">
          {(me.displayName || me.email).charAt(0).toUpperCase()}
        </div>
      )}
      <span className="text-sm text-slate-700 dark:text-slate-300">{me.displayName || me.email}</span>
      <button
        type="button"
        onClick={() => signOut.mutate()}
        disabled={signOut.isPending}
        className="rounded-lg border border-slate-300 dark:border-slate-600 px-3 py-1.5 text-sm text-slate-600 dark:text-slate-400 hover:bg-slate-50 dark:hover:bg-slate-800 disabled:opacity-50"
      >
        Sign out
      </button>
    </div>
  )
}

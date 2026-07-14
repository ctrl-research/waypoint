import { useEffect, useState, useSyncExternalStore } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Navigate } from '@tanstack/react-router'
import { ApiError, createHome, deleteHome, fetchMe, geocode, listHomes } from '../api'
import { getTheme, setTheme, subscribeTheme, type Theme } from '../theme'

const field =
  'rounded-lg border border-slate-300 dark:border-slate-600 px-3 py-2 text-sm focus:border-slate-500 focus:outline-none'

export function SettingsPage() {
  const { data: me, isLoading } = useQuery({ queryKey: ['me'], queryFn: fetchMe })

  if (isLoading) return null
  if (!me) return <Navigate to="/login" />

  return (
    <div className="mx-auto mt-8 w-full max-w-2xl px-4 pb-24">
      <h1 className="text-2xl font-semibold text-slate-900 dark:text-slate-100">Settings</h1>

      <section className="mt-6 rounded-xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 p-5">
        <h2 className="font-medium text-slate-900 dark:text-slate-100">Appearance</h2>
        <p className="mt-1 text-sm text-slate-500 dark:text-slate-400">
          System follows your device’s light/dark preference.
        </p>
        <ThemePicker />
      </section>

      <section className="mt-6 rounded-xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 p-5">
        <h2 className="font-medium text-slate-900 dark:text-slate-100">Home cities</h2>
        <p className="mt-1 text-sm text-slate-500 dark:text-slate-400">
          The cities your travels start and end from — add more than one if life is split between
          places. Flights and trains can use any of them, shown as “(home) City”, as departure or
          arrival, and they anchor the distance stats.
        </p>
        <HomesEditor />
      </section>
    </div>
  )
}

const THEME_OPTIONS: [Theme, string, string][] = [
  ['light', '☀️', 'Light'],
  ['dark', '🌙', 'Dark'],
  ['system', '💻', 'System'],
]

function ThemePicker() {
  const theme = useSyncExternalStore(subscribeTheme, getTheme)
  return (
    <div className="mt-3 flex rounded-lg border border-slate-300 dark:border-slate-600 p-0.5 w-fit">
      {THEME_OPTIONS.map(([value, icon, label]) => (
        <button
          key={value}
          type="button"
          onClick={() => setTheme(value)}
          aria-pressed={theme === value}
          className={`rounded-md px-3 py-1.5 text-sm ${
            theme === value
              ? 'bg-slate-900 dark:bg-slate-100 text-white dark:text-slate-900'
              : 'text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-100'
          }`}
        >
          {icon} {label}
        </button>
      ))}
    </div>
  )
}

function HomesEditor() {
  const queryClient = useQueryClient()
  const homes = useQuery({ queryKey: ['homes'], queryFn: listHomes })
  const [query, setQuery] = useState('')
  const [debounced, setDebounced] = useState('')
  const [open, setOpen] = useState(false)
  const invalidate = () => queryClient.invalidateQueries({ queryKey: ['homes'] })

  useEffect(() => {
    const id = window.setTimeout(() => setDebounced(query.trim()), 400)
    return () => window.clearTimeout(id)
  }, [query])

  const results = useQuery({
    queryKey: ['geocode', 'city', debounced],
    queryFn: () => geocode(debounced, true),
    enabled: debounced.length >= 2,
    staleTime: 5 * 60 * 1000,
  })

  const add = useMutation({
    mutationFn: ({ name, lat, lon }: { name: string; lat: number; lon: number }) =>
      createHome(name, lat, lon),
    onSuccess: async () => {
      setQuery('')
      setOpen(false)
      await invalidate()
    },
  })
  const remove = useMutation({ mutationFn: deleteHome, onSuccess: invalidate })

  return (
    <div className="mt-4">
      <div className="space-y-2">
        {homes.data?.length === 0 && <p className="text-sm text-slate-400 dark:text-slate-500">No homes set yet.</p>}
        {homes.data?.map((home) => (
          <div
            key={home.id}
            className="flex items-center justify-between rounded-lg border border-slate-200 dark:border-slate-800 px-4 py-2.5"
          >
            <span className="text-sm text-slate-700 dark:text-slate-300">🏠 (home) {home.name}</span>
            <button
              type="button"
              onClick={() => remove.mutate(home.id)}
              className="text-sm text-slate-400 dark:text-slate-500 hover:text-red-600 dark:hover:text-red-400"
            >
              Remove
            </button>
          </div>
        ))}
      </div>

      <div className="relative mt-3">
        <input
          value={query}
          onChange={(e) => {
            setQuery(e.target.value)
            setOpen(true)
          }}
          onFocus={() => setOpen(true)}
          onBlur={() => window.setTimeout(() => setOpen(false), 150)}
          placeholder="Search for a city to add as home…"
          className={`${field} w-full`}
        />
        {open && debounced.length >= 2 && (
          <div className="absolute z-10 mt-1 w-full overflow-hidden rounded-lg border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 shadow-lg">
            {results.isLoading && <p className="px-4 py-2 text-sm text-slate-400 dark:text-slate-500">Searching…</p>}
            {results.data?.map((r) => (
              <button
                key={`${r.lat},${r.lon}`}
                type="button"
                onMouseDown={() => add.mutate({ name: shortName(r.name), lat: r.lat, lon: r.lon })}
                className="block w-full truncate px-4 py-2 text-left text-sm text-slate-700 dark:text-slate-300 hover:bg-slate-50 dark:hover:bg-slate-800"
              >
                📍 {r.name}
              </button>
            ))}
            {results.data?.length === 0 && (
              <p className="px-4 py-2 text-sm text-slate-400 dark:text-slate-500">No cities found.</p>
            )}
          </div>
        )}
      </div>
      {(add.error || remove.error) && (
        <p className="mt-2 text-sm text-red-600 dark:text-red-400">
          {add.error instanceof ApiError ? add.error.message : 'Could not save'}
        </p>
      )}
    </div>
  )
}

function shortName(displayName: string): string {
  const parts = displayName.split(', ')
  return parts.length <= 2 ? displayName : `${parts[0]}, ${parts[parts.length - 1]}`
}

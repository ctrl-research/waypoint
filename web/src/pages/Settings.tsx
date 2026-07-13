import { useEffect, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Navigate } from '@tanstack/react-router'
import { ApiError, createHome, deleteHome, fetchMe, geocode, listHomes } from '../api'

const field =
  'rounded-lg border border-slate-300 px-3 py-2 text-sm focus:border-slate-500 focus:outline-none'

export function SettingsPage() {
  const { data: me, isLoading } = useQuery({ queryKey: ['me'], queryFn: fetchMe })

  if (isLoading) return null
  if (!me) return <Navigate to="/login" />

  return (
    <div className="mx-auto mt-8 w-full max-w-2xl px-4 pb-24">
      <h1 className="text-2xl font-semibold text-slate-900">Settings</h1>

      <section className="mt-6 rounded-xl border border-slate-200 bg-white p-5">
        <h2 className="font-medium text-slate-900">Homes</h2>
        <p className="mt-1 text-sm text-slate-500">
          Where your travels start and end — add more than one if life is split between places.
          Flights and trains can use any of them, shown as “(home) Name”, as departure or arrival,
          and they anchor the distance stats.
        </p>
        <HomesEditor />
      </section>
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
    queryKey: ['geocode', debounced],
    queryFn: () => geocode(debounced),
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
        {homes.data?.length === 0 && <p className="text-sm text-slate-400">No homes set yet.</p>}
        {homes.data?.map((home) => (
          <div
            key={home.id}
            className="flex items-center justify-between rounded-lg border border-slate-200 px-4 py-2.5"
          >
            <span className="text-sm text-slate-700">🏠 (home) {home.name}</span>
            <button
              type="button"
              onClick={() => remove.mutate(home.id)}
              className="text-sm text-slate-400 hover:text-red-600"
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
          placeholder="Search to add a home…"
          className={`${field} w-full`}
        />
        {open && debounced.length >= 2 && (
          <div className="absolute z-10 mt-1 w-full overflow-hidden rounded-lg border border-slate-200 bg-white shadow-lg">
            {results.isLoading && <p className="px-4 py-2 text-sm text-slate-400">Searching…</p>}
            {results.data?.map((r) => (
              <button
                key={`${r.lat},${r.lon}`}
                type="button"
                onMouseDown={() => add.mutate({ name: shortName(r.name), lat: r.lat, lon: r.lon })}
                className="block w-full truncate px-4 py-2 text-left text-sm text-slate-700 hover:bg-slate-50"
              >
                📍 {r.name}
              </button>
            ))}
            {results.data?.length === 0 && (
              <p className="px-4 py-2 text-sm text-slate-400">No places found.</p>
            )}
          </div>
        )}
      </div>
      {(add.error || remove.error) && (
        <p className="mt-2 text-sm text-red-600">
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

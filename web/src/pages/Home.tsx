import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link, Navigate, useNavigate } from '@tanstack/react-router'
import { ApiError, createTrip, fetchMe, listTrips, type Trip, type TripStatus } from '../api'

export const statusStyles: Record<TripStatus, string> = {
  planning: 'bg-amber-100 dark:bg-amber-950 text-amber-800 dark:text-amber-300',
  active: 'bg-emerald-100 dark:bg-emerald-950 text-emerald-800 dark:text-emerald-300',
  completed: 'bg-slate-200 dark:bg-slate-700 text-slate-600 dark:text-slate-400',
}

export function HomePage() {
  const { data: me, isLoading } = useQuery({ queryKey: ['me'], queryFn: fetchMe })
  const trips = useQuery({ queryKey: ['trips'], queryFn: listTrips, enabled: !!me })
  const [creating, setCreating] = useState(false)

  if (isLoading) return null
  if (!me) return <Navigate to="/login" />

  return (
    <div className="mx-auto mt-8 w-full max-w-5xl px-4 pb-16">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-semibold text-slate-900 dark:text-slate-100">Your trips</h1>
        <button
          type="button"
          onClick={() => setCreating((v) => !v)}
          className="rounded-lg bg-slate-900 dark:bg-slate-100 px-4 py-2 text-sm font-medium text-white dark:text-slate-900 hover:bg-slate-700 dark:hover:bg-slate-300"
        >
          {creating ? 'Cancel' : 'New trip'}
        </button>
      </div>

      {creating && <NewTripForm onDone={() => setCreating(false)} />}

      {trips.data && trips.data.length === 0 && !creating && (
        <p className="mt-12 text-center text-slate-500 dark:text-slate-400">
          No trips yet — plan your first one with “New trip”.
        </p>
      )}

      {trips.data && <GroupedTrips trips={trips.data} />}
    </div>
  )
}

/** Splits trips into happening-now / upcoming (soonest first, undated last) /
 * past (most recent first). Completed trips are always past (#49). */
function GroupedTrips({ trips }: { trips: Trip[] }) {
  const today = new Date().toISOString().slice(0, 10)

  const now = trips.filter(
    (t) =>
      t.status !== 'completed' &&
      t.startDate !== null &&
      t.startDate <= today &&
      (t.endDate === null || t.endDate >= today),
  )
  const isPast = (t: Trip) =>
    t.status === 'completed' || (t.endDate !== null && t.endDate < today)
  const past = trips
    .filter(isPast)
    .sort((a, b) => (b.endDate ?? b.startDate ?? '').localeCompare(a.endDate ?? a.startDate ?? ''))
  const upcoming = trips
    .filter((t) => !now.includes(t) && !isPast(t))
    .sort((a, b) => (a.startDate ?? '9999').localeCompare(b.startDate ?? '9999'))

  const sections: [string, Trip[]][] = [
    ['Happening now', now],
    ['Upcoming', upcoming],
    ['Past trips', past],
  ]

  return (
    <>
      {sections.map(
        ([title, group]) =>
          group.length > 0 && (
            <section key={title} className="mt-8">
              <h2 className="text-sm font-semibold uppercase tracking-wide text-slate-400 dark:text-slate-500">
                {title}
              </h2>
              <div className="mt-3 grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
                {group.map((trip) => (
                  <TripCard key={trip.id} trip={trip} />
                ))}
              </div>
            </section>
          ),
      )}
    </>
  )
}

function TripCard({ trip }: { trip: Trip }) {
  return (
    <Link
      to="/trips/$tripId"
      params={{ tripId: trip.id }}
      className="rounded-xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 p-5 shadow-sm transition hover:border-slate-300 dark:hover:border-slate-600 hover:shadow"
    >
      <div className="flex items-start justify-between gap-2">
        <h2 className="font-medium text-slate-900 dark:text-slate-100">{trip.title}</h2>
        <div className="flex shrink-0 gap-1">
          {trip.role !== 'owner' && (
            <span className="rounded-full bg-indigo-100 dark:bg-indigo-950 px-2 py-0.5 text-xs font-medium text-indigo-700 dark:text-indigo-300">
              shared
            </span>
          )}
          <span className={`rounded-full px-2 py-0.5 text-xs font-medium ${statusStyles[trip.status]}`}>
            {trip.status}
          </span>
        </div>
      </div>
      <p className="mt-1 text-sm text-slate-500 dark:text-slate-400">{formatRange(trip.startDate, trip.endDate)}</p>
      {trip.description && (
        <p className="mt-2 line-clamp-2 text-sm text-slate-600 dark:text-slate-400">{trip.description}</p>
      )}
    </Link>
  )
}

function NewTripForm({ onDone }: { onDone: () => void }) {
  const [title, setTitle] = useState('')
  const [startDate, setStartDate] = useState('')
  const [endDate, setEndDate] = useState('')
  const [description, setDescription] = useState('')
  const queryClient = useQueryClient()
  const navigate = useNavigate()

  const mutation = useMutation({
    mutationFn: () => createTrip({ title, startDate, endDate, description }),
    onSuccess: async (trip) => {
      await queryClient.invalidateQueries({ queryKey: ['trips'] })
      onDone()
      navigate({ to: '/trips/$tripId', params: { tripId: trip.id } })
    },
  })

  return (
    <form
      className="mt-6 space-y-3 rounded-xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 p-5 shadow-sm"
      onSubmit={(e) => {
        e.preventDefault()
        mutation.mutate()
      }}
    >
      <label className="block">
        <span className="text-sm font-medium text-slate-700 dark:text-slate-300">Title</span>
        <input
          required
          value={title}
          onChange={(e) => setTitle(e.target.value)}
          placeholder="Japan, spring 2027"
          className="mt-1 w-full rounded-lg border border-slate-300 dark:border-slate-600 px-3 py-2 text-sm focus:border-slate-500 focus:outline-none"
        />
      </label>
      <div className="grid grid-cols-2 gap-3">
        <label className="block">
          <span className="text-sm font-medium text-slate-700 dark:text-slate-300">Start date</span>
          <input
            type="date"
            value={startDate}
            onChange={(e) => setStartDate(e.target.value)}
            className="mt-1 w-full rounded-lg border border-slate-300 dark:border-slate-600 px-3 py-2 text-sm focus:border-slate-500 focus:outline-none"
          />
        </label>
        <label className="block">
          <span className="text-sm font-medium text-slate-700 dark:text-slate-300">End date</span>
          <input
            type="date"
            value={endDate}
            onChange={(e) => setEndDate(e.target.value)}
            className="mt-1 w-full rounded-lg border border-slate-300 dark:border-slate-600 px-3 py-2 text-sm focus:border-slate-500 focus:outline-none"
          />
        </label>
      </div>
      <label className="block">
        <span className="text-sm font-medium text-slate-700 dark:text-slate-300">Description</span>
        <textarea
          value={description}
          onChange={(e) => setDescription(e.target.value)}
          rows={2}
          className="mt-1 w-full rounded-lg border border-slate-300 dark:border-slate-600 px-3 py-2 text-sm focus:border-slate-500 focus:outline-none"
        />
      </label>
      {mutation.error && (
        <p className="text-sm text-red-600 dark:text-red-400">
          {mutation.error instanceof ApiError ? mutation.error.message : 'Could not create trip'}
        </p>
      )}
      <button
        type="submit"
        disabled={mutation.isPending}
        className="rounded-lg bg-slate-900 dark:bg-slate-100 px-4 py-2 text-sm font-medium text-white dark:text-slate-900 hover:bg-slate-700 dark:hover:bg-slate-300 disabled:opacity-50"
      >
        {mutation.isPending ? 'Creating…' : 'Create trip'}
      </button>
    </form>
  )
}

export function formatRange(start: string | null, end: string | null): string {
  if (!start && !end) return 'Dates TBD'
  const fmt = (d: string) =>
    new Date(d + 'T00:00:00').toLocaleDateString(undefined, {
      month: 'short',
      day: 'numeric',
      year: 'numeric',
    })
  if (start && end) return `${fmt(start)} – ${fmt(end)}`
  return fmt((start ?? end)!)
}

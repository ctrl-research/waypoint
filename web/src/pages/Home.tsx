import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link, Navigate, useNavigate } from '@tanstack/react-router'
import { ApiError, createTrip, fetchMe, listTrips, type Trip, type TripStatus } from '../api'

export const statusStyles: Record<TripStatus, string> = {
  planning: 'bg-amber-100 text-amber-800',
  active: 'bg-emerald-100 text-emerald-800',
  completed: 'bg-slate-200 text-slate-600',
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
        <h1 className="text-2xl font-semibold text-slate-900">Your trips</h1>
        <button
          type="button"
          onClick={() => setCreating((v) => !v)}
          className="rounded-lg bg-slate-900 px-4 py-2 text-sm font-medium text-white hover:bg-slate-700"
        >
          {creating ? 'Cancel' : 'New trip'}
        </button>
      </div>

      {creating && <NewTripForm onDone={() => setCreating(false)} />}

      {trips.data && trips.data.length === 0 && !creating && (
        <p className="mt-12 text-center text-slate-500">
          No trips yet — plan your first one with “New trip”.
        </p>
      )}

      <div className="mt-6 grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
        {trips.data?.map((trip) => <TripCard key={trip.id} trip={trip} />)}
      </div>
    </div>
  )
}

function TripCard({ trip }: { trip: Trip }) {
  return (
    <Link
      to="/trips/$tripId"
      params={{ tripId: trip.id }}
      className="rounded-xl border border-slate-200 bg-white p-5 shadow-sm transition hover:border-slate-300 hover:shadow"
    >
      <div className="flex items-start justify-between gap-2">
        <h2 className="font-medium text-slate-900">{trip.title}</h2>
        <span className={`rounded-full px-2 py-0.5 text-xs font-medium ${statusStyles[trip.status]}`}>
          {trip.status}
        </span>
      </div>
      <p className="mt-1 text-sm text-slate-500">{formatRange(trip.startDate, trip.endDate)}</p>
      {trip.description && (
        <p className="mt-2 line-clamp-2 text-sm text-slate-600">{trip.description}</p>
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
      className="mt-6 space-y-3 rounded-xl border border-slate-200 bg-white p-5 shadow-sm"
      onSubmit={(e) => {
        e.preventDefault()
        mutation.mutate()
      }}
    >
      <label className="block">
        <span className="text-sm font-medium text-slate-700">Title</span>
        <input
          required
          value={title}
          onChange={(e) => setTitle(e.target.value)}
          placeholder="Japan, spring 2027"
          className="mt-1 w-full rounded-lg border border-slate-300 px-3 py-2 text-sm focus:border-slate-500 focus:outline-none"
        />
      </label>
      <div className="grid grid-cols-2 gap-3">
        <label className="block">
          <span className="text-sm font-medium text-slate-700">Start date</span>
          <input
            type="date"
            value={startDate}
            onChange={(e) => setStartDate(e.target.value)}
            className="mt-1 w-full rounded-lg border border-slate-300 px-3 py-2 text-sm focus:border-slate-500 focus:outline-none"
          />
        </label>
        <label className="block">
          <span className="text-sm font-medium text-slate-700">End date</span>
          <input
            type="date"
            value={endDate}
            onChange={(e) => setEndDate(e.target.value)}
            className="mt-1 w-full rounded-lg border border-slate-300 px-3 py-2 text-sm focus:border-slate-500 focus:outline-none"
          />
        </label>
      </div>
      <label className="block">
        <span className="text-sm font-medium text-slate-700">Description</span>
        <textarea
          value={description}
          onChange={(e) => setDescription(e.target.value)}
          rows={2}
          className="mt-1 w-full rounded-lg border border-slate-300 px-3 py-2 text-sm focus:border-slate-500 focus:outline-none"
        />
      </label>
      {mutation.error && (
        <p className="text-sm text-red-600">
          {mutation.error instanceof ApiError ? mutation.error.message : 'Could not create trip'}
        </p>
      )}
      <button
        type="submit"
        disabled={mutation.isPending}
        className="rounded-lg bg-slate-900 px-4 py-2 text-sm font-medium text-white hover:bg-slate-700 disabled:opacity-50"
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

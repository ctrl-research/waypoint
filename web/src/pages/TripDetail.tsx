import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link, Navigate, useNavigate, useParams } from '@tanstack/react-router'
import {
  ApiError,
  createItem,
  createStop,
  deleteItem,
  deleteStop,
  deleteTrip,
  fetchMe,
  getTrip,
  updateTrip,
  type ItineraryCategory,
  type ItineraryItem,
  type Stop,
  type TripStatus,
} from '../api'
import { formatRange, statusStyles } from './Home'

const categoryIcons: Record<ItineraryCategory, string> = {
  activity: '🎟️',
  food: '🍜',
  lodging: '🛏️',
  transport: '🚆',
  other: '📌',
}

export function TripDetailPage() {
  const { tripId } = useParams({ from: '/trips/$tripId' })
  const { data: me, isLoading: meLoading } = useQuery({ queryKey: ['me'], queryFn: fetchMe })
  const detail = useQuery({
    queryKey: ['trip', tripId],
    queryFn: () => getTrip(tripId),
    enabled: !!me,
  })

  if (meLoading) return null
  if (!me) return <Navigate to="/login" />
  if (detail.error) {
    return (
      <div className="mx-auto mt-16 max-w-md text-center text-slate-500">
        <p>Trip not found.</p>
        <Link to="/" className="mt-2 inline-block text-slate-900 underline">
          Back to your trips
        </Link>
      </div>
    )
  }
  if (!detail.data) return null

  const { trip, stops, items } = detail.data

  return (
    <div className="mx-auto mt-8 w-full max-w-5xl px-4 pb-24">
      <TripHeader tripId={tripId} />

      <div className="mt-8 grid grid-cols-1 gap-8 lg:grid-cols-2">
        <section>
          <h2 className="text-lg font-semibold text-slate-900">Stops</h2>
          <p className="text-sm text-slate-500">The places this trip visits, in order.</p>
          <StopsSection tripId={trip.id} stops={stops} />
        </section>

        <section>
          <h2 className="text-lg font-semibold text-slate-900">Itinerary</h2>
          <p className="text-sm text-slate-500">Day by day. Reordering arrives with the board (#10).</p>
          <ItinerarySection tripId={trip.id} items={items} stops={stops} />
        </section>
      </div>
    </div>
  )
}

function TripHeader({ tripId }: { tripId: string }) {
  const queryClient = useQueryClient()
  const navigate = useNavigate()
  const { data } = useQuery({ queryKey: ['trip', tripId], queryFn: () => getTrip(tripId) })
  const [editing, setEditing] = useState(false)

  const remove = useMutation({
    mutationFn: () => deleteTrip(tripId),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ['trips'] })
      navigate({ to: '/' })
    },
  })

  if (!data) return null
  const { trip } = data

  return (
    <div>
      <Link to="/" className="text-sm text-slate-500 hover:text-slate-900">
        ← Your trips
      </Link>
      {editing ? (
        <EditTripForm
          tripId={tripId}
          initial={{
            title: trip.title,
            description: trip.description,
            status: trip.status,
            startDate: trip.startDate ?? '',
            endDate: trip.endDate ?? '',
          }}
          onDone={() => setEditing(false)}
        />
      ) : (
        <div className="mt-2 flex flex-wrap items-start justify-between gap-4">
          <div>
            <div className="flex items-center gap-3">
              <h1 className="text-2xl font-semibold text-slate-900">{trip.title}</h1>
              <span className={`rounded-full px-2 py-0.5 text-xs font-medium ${statusStyles[trip.status]}`}>
                {trip.status}
              </span>
            </div>
            <p className="mt-1 text-sm text-slate-500">{formatRange(trip.startDate, trip.endDate)}</p>
            {trip.description && <p className="mt-2 max-w-2xl text-slate-600">{trip.description}</p>}
          </div>
          <div className="flex gap-2">
            <button
              type="button"
              onClick={() => setEditing(true)}
              className="rounded-lg border border-slate-300 px-3 py-1.5 text-sm text-slate-600 hover:bg-slate-50"
            >
              Edit
            </button>
            <button
              type="button"
              onClick={() => {
                if (window.confirm(`Delete “${trip.title}” and everything in it?`)) remove.mutate()
              }}
              className="rounded-lg border border-red-200 px-3 py-1.5 text-sm text-red-600 hover:bg-red-50"
            >
              Delete
            </button>
          </div>
        </div>
      )}
    </div>
  )
}

function EditTripForm({
  tripId,
  initial,
  onDone,
}: {
  tripId: string
  initial: { title: string; description: string; status: TripStatus; startDate: string; endDate: string }
  onDone: () => void
}) {
  const [form, setForm] = useState(initial)
  const queryClient = useQueryClient()

  const mutation = useMutation({
    mutationFn: () => updateTrip(tripId, form),
    onSuccess: async () => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['trip', tripId] }),
        queryClient.invalidateQueries({ queryKey: ['trips'] }),
      ])
      onDone()
    },
  })

  const field = 'mt-1 w-full rounded-lg border border-slate-300 px-3 py-2 text-sm focus:border-slate-500 focus:outline-none'

  return (
    <form
      className="mt-3 space-y-3 rounded-xl border border-slate-200 bg-white p-5 shadow-sm"
      onSubmit={(e) => {
        e.preventDefault()
        mutation.mutate()
      }}
    >
      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
        <label className="block sm:col-span-2">
          <span className="text-sm font-medium text-slate-700">Title</span>
          <input
            required
            value={form.title}
            onChange={(e) => setForm({ ...form, title: e.target.value })}
            className={field}
          />
        </label>
        <label className="block">
          <span className="text-sm font-medium text-slate-700">Status</span>
          <select
            value={form.status}
            onChange={(e) => setForm({ ...form, status: e.target.value as TripStatus })}
            className={field}
          >
            <option value="planning">planning</option>
            <option value="active">active</option>
            <option value="completed">completed</option>
          </select>
        </label>
        <div className="grid grid-cols-2 gap-3">
          <label className="block">
            <span className="text-sm font-medium text-slate-700">Start</span>
            <input
              type="date"
              value={form.startDate}
              onChange={(e) => setForm({ ...form, startDate: e.target.value })}
              className={field}
            />
          </label>
          <label className="block">
            <span className="text-sm font-medium text-slate-700">End</span>
            <input
              type="date"
              value={form.endDate}
              onChange={(e) => setForm({ ...form, endDate: e.target.value })}
              className={field}
            />
          </label>
        </div>
        <label className="block sm:col-span-2">
          <span className="text-sm font-medium text-slate-700">Description</span>
          <textarea
            rows={2}
            value={form.description}
            onChange={(e) => setForm({ ...form, description: e.target.value })}
            className={field}
          />
        </label>
      </div>
      {mutation.error && (
        <p className="text-sm text-red-600">
          {mutation.error instanceof ApiError ? mutation.error.message : 'Could not save'}
        </p>
      )}
      <div className="flex gap-2">
        <button
          type="submit"
          disabled={mutation.isPending}
          className="rounded-lg bg-slate-900 px-4 py-2 text-sm font-medium text-white hover:bg-slate-700 disabled:opacity-50"
        >
          {mutation.isPending ? 'Saving…' : 'Save'}
        </button>
        <button
          type="button"
          onClick={onDone}
          className="rounded-lg border border-slate-300 px-4 py-2 text-sm text-slate-600 hover:bg-slate-50"
        >
          Cancel
        </button>
      </div>
    </form>
  )
}

function StopsSection({ tripId, stops }: { tripId: string; stops: Stop[] }) {
  const queryClient = useQueryClient()
  const [name, setName] = useState('')
  const invalidate = () => queryClient.invalidateQueries({ queryKey: ['trip', tripId] })

  const add = useMutation({
    mutationFn: () => createStop(tripId, { name }),
    onSuccess: async () => {
      setName('')
      await invalidate()
    },
  })
  const remove = useMutation({
    mutationFn: (stopId: string) => deleteStop(tripId, stopId),
    onSuccess: invalidate,
  })

  return (
    <div className="mt-4 space-y-2">
      {stops.map((stop, i) => (
        <div
          key={stop.id}
          className="flex items-center justify-between rounded-lg border border-slate-200 bg-white px-4 py-3"
        >
          <div className="flex items-center gap-3">
            <span className="flex h-6 w-6 items-center justify-center rounded-full bg-slate-100 text-xs font-medium text-slate-600">
              {i + 1}
            </span>
            <div>
              <p className="text-sm font-medium text-slate-900">{stop.name}</p>
              {(stop.arrivalDate || stop.departureDate) && (
                <p className="text-xs text-slate-500">{formatRange(stop.arrivalDate, stop.departureDate)}</p>
              )}
            </div>
          </div>
          <button
            type="button"
            onClick={() => remove.mutate(stop.id)}
            className="text-sm text-slate-400 hover:text-red-600"
            aria-label={`Remove ${stop.name}`}
          >
            ✕
          </button>
        </div>
      ))}

      <form
        className="flex gap-2"
        onSubmit={(e) => {
          e.preventDefault()
          if (name.trim()) add.mutate()
        }}
      >
        <input
          value={name}
          onChange={(e) => setName(e.target.value)}
          placeholder="Add a stop (e.g. Kyoto)"
          className="flex-1 rounded-lg border border-slate-300 px-3 py-2 text-sm focus:border-slate-500 focus:outline-none"
        />
        <button
          type="submit"
          disabled={add.isPending || !name.trim()}
          className="rounded-lg bg-slate-900 px-4 py-2 text-sm font-medium text-white hover:bg-slate-700 disabled:opacity-50"
        >
          Add
        </button>
      </form>
      {add.error && (
        <p className="text-sm text-red-600">
          {add.error instanceof ApiError ? add.error.message : 'Could not add stop'}
        </p>
      )}
    </div>
  )
}

function ItinerarySection({
  tripId,
  items,
  stops,
}: {
  tripId: string
  items: ItineraryItem[]
  stops: Stop[]
}) {
  const queryClient = useQueryClient()
  const stopName = (id: string | null) => stops.find((s) => s.id === id)?.name

  const byDay = new Map<string, ItineraryItem[]>()
  for (const item of items) {
    byDay.set(item.day, [...(byDay.get(item.day) ?? []), item])
  }

  const remove = useMutation({
    mutationFn: (itemId: string) => deleteItem(tripId, itemId),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['trip', tripId] }),
  })

  return (
    <div className="mt-4 space-y-4">
      {[...byDay.entries()].map(([day, dayItems]) => (
        <div key={day}>
          <h3 className="text-sm font-semibold text-slate-700">
            {new Date(day + 'T00:00:00').toLocaleDateString(undefined, {
              weekday: 'short',
              month: 'short',
              day: 'numeric',
            })}
          </h3>
          <div className="mt-1 space-y-1">
            {dayItems.map((item) => (
              <div
                key={item.id}
                className="flex items-center justify-between rounded-lg border border-slate-200 bg-white px-4 py-2"
              >
                <div className="flex items-center gap-2 text-sm">
                  <span>{categoryIcons[item.category]}</span>
                  {item.startTime && <span className="tabular-nums text-slate-500">{item.startTime}</span>}
                  <span className="font-medium text-slate-900">{item.title}</span>
                  {stopName(item.stopId) && (
                    <span className="text-xs text-slate-400">@ {stopName(item.stopId)}</span>
                  )}
                </div>
                <button
                  type="button"
                  onClick={() => remove.mutate(item.id)}
                  className="text-sm text-slate-400 hover:text-red-600"
                  aria-label={`Remove ${item.title}`}
                >
                  ✕
                </button>
              </div>
            ))}
          </div>
        </div>
      ))}

      <NewItemForm tripId={tripId} stops={stops} />
    </div>
  )
}

function NewItemForm({ tripId, stops }: { tripId: string; stops: Stop[] }) {
  const queryClient = useQueryClient()
  const [title, setTitle] = useState('')
  const [day, setDay] = useState('')
  const [startTime, setStartTime] = useState('')
  const [category, setCategory] = useState<ItineraryCategory>('activity')
  const [stopId, setStopId] = useState('')

  const add = useMutation({
    mutationFn: () =>
      createItem(tripId, {
        title,
        day,
        category,
        ...(startTime ? { startTime } : {}),
        ...(stopId ? { stopId } : {}),
      }),
    onSuccess: async () => {
      setTitle('')
      await queryClient.invalidateQueries({ queryKey: ['trip', tripId] })
    },
  })

  const field = 'rounded-lg border border-slate-300 px-3 py-2 text-sm focus:border-slate-500 focus:outline-none'

  return (
    <form
      className="space-y-2 rounded-lg border border-dashed border-slate-300 p-3"
      onSubmit={(e) => {
        e.preventDefault()
        if (title.trim() && day) add.mutate()
      }}
    >
      <div className="flex flex-wrap gap-2">
        <input
          value={title}
          onChange={(e) => setTitle(e.target.value)}
          placeholder="Add an activity"
          className={`${field} min-w-40 flex-1`}
        />
        <input type="date" required value={day} onChange={(e) => setDay(e.target.value)} className={field} />
        <input
          type="time"
          value={startTime}
          onChange={(e) => setStartTime(e.target.value)}
          className={field}
        />
      </div>
      <div className="flex flex-wrap gap-2">
        <select
          value={category}
          onChange={(e) => setCategory(e.target.value as ItineraryCategory)}
          className={field}
        >
          {(Object.keys(categoryIcons) as ItineraryCategory[]).map((c) => (
            <option key={c} value={c}>
              {categoryIcons[c]} {c}
            </option>
          ))}
        </select>
        <select value={stopId} onChange={(e) => setStopId(e.target.value)} className={field}>
          <option value="">no stop</option>
          {stops.map((s) => (
            <option key={s.id} value={s.id}>
              {s.name}
            </option>
          ))}
        </select>
        <button
          type="submit"
          disabled={add.isPending || !title.trim() || !day}
          className="rounded-lg bg-slate-900 px-4 py-2 text-sm font-medium text-white hover:bg-slate-700 disabled:opacity-50"
        >
          Add
        </button>
      </div>
      {add.error && (
        <p className="text-sm text-red-600">
          {add.error instanceof ApiError ? add.error.message : 'Could not add item'}
        </p>
      )}
    </form>
  )
}

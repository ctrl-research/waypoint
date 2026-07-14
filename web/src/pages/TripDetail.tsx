import { Suspense, lazy, useEffect, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link, Navigate, useNavigate, useParams } from '@tanstack/react-router'
import {
  DndContext,
  PointerSensor,
  closestCorners,
  useSensor,
  useSensors,
  type DragEndEvent,
} from '@dnd-kit/core'
import { SortableContext, arrayMove, useSortable, verticalListSortingStrategy } from '@dnd-kit/sortable'
import { CSS } from '@dnd-kit/utilities'
import {
  ApiError,
  createItem,
  createStop,
  deleteItem,
  deleteStop,
  deleteTrip,
  fetchMe,
  geocode,
  getTrip,
  listHomes,
  reorderStops,
  updateItem,
  updateTrip,
  type ItemInput,
  type ItineraryCategory,
  type ItineraryItem,
  type ItineraryLayer,
  type Stop,
  type StopInput,
  type Trip,
  type TripStatus,
} from '../api'
import { defaultTripDay, formatRange, statusStyles } from './Home'
import { ItineraryBoard, categoryIcons } from './ItineraryBoard'
import { JournalTimeline } from './Journal'
import { MembersSection, ShareSection } from './Members'

// MapLibre is ~1MB minified; load it only when a trip page renders.
const TripMap = lazy(() => import('../TripMap').then((m) => ({ default: m.TripMap })))
type MarkerKey = `stop:${string}` | `item:${string}`


export function TripDetailPage() {
  const { tripId } = useParams({ from: '/trips/$tripId' })
  const { data: me, isLoading: meLoading } = useQuery({ queryKey: ['me'], queryFn: fetchMe })
  const detail = useQuery({
    queryKey: ['trip', tripId],
    queryFn: () => getTrip(tripId),
    enabled: !!me,
  })

  // List row under the cursor, mirrored on the map (#71).
  const [highlightKey, setHighlightKey] = useState<MarkerKey | null>(null)

  if (meLoading) return null
  if (!me) return <Navigate to="/login" />
  if (detail.error) {
    return (
      <div className="mx-auto mt-16 max-w-md text-center text-slate-500 dark:text-slate-400">
        <p>Trip not found.</p>
        <Link to="/" className="mt-2 inline-block text-slate-900 dark:text-slate-100 underline">
          Back to your trips
        </Link>
      </div>
    )
  }
  if (!detail.data) return null

  const { trip, stops, items, layers } = detail.data
  const canEdit = trip.role !== 'viewer'
  // The itinerary is the merge of visible layers (#73).
  const hiddenLayers = new Set(layers.filter((l) => !l.visible).map((l) => l.id))
  const planItems = items.filter((i) => !hiddenLayers.has(i.layerId))

  return (
    <div className="mx-auto mt-8 w-full max-w-5xl px-4 pb-24">
      <TripHeader tripId={tripId} />

      <div className="mt-6">
        <Suspense fallback={<div className="h-80 w-full rounded-xl border border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-950" />}>
          <TripMap stops={stops} items={planItems} highlightKey={highlightKey} />
        </Suspense>
      </div>

      <section className="mt-8">
        <h2 className="text-lg font-semibold text-slate-900 dark:text-slate-100">Stops</h2>
          <p className="text-sm text-slate-500 dark:text-slate-400">
            The places this trip visits, in order.
            {canEdit && ' Drag to change the route.'}
          </p>
        <StopsSection tripId={trip.id} stops={stops} canEdit={canEdit} onHover={setHighlightKey} />
      </section>

      <section className="mt-10">
        <div className="flex items-center justify-between">
          <div>
            <h2 className="text-lg font-semibold text-slate-900 dark:text-slate-100">Itinerary</h2>
            <p className="text-sm text-slate-500 dark:text-slate-400">Everything on the visible layers, day by day.</p>
          </div>
          {canEdit && (
            <Link
              to="/trips/$tripId/itinerary"
              params={{ tripId: trip.id }}
              className="rounded-lg border border-slate-300 dark:border-slate-600 px-4 py-2 text-sm text-slate-600 dark:text-slate-400 hover:bg-slate-50 dark:hover:bg-slate-800"
            >
              ✏️ Edit itinerary
            </Link>
          )}
        </div>
        <ItineraryBoard
          trip={trip}
          items={planItems}
          readOnly
          combined
          layers={layers}
          onHover={setHighlightKey}
        />
      </section>

      <JournalTimeline trip={trip} items={items} stops={stops} canEdit={canEdit} />
      <MembersSection tripId={trip.id} role={trip.role} />
      {trip.role === 'owner' && <ShareSection tripId={trip.id} />}

      <section className="mt-8">
        <h2 className="text-lg font-semibold text-slate-900 dark:text-slate-100">Export</h2>
        <p className="text-sm text-slate-500 dark:text-slate-400">Take your data with you.</p>
        <div className="mt-3 flex flex-wrap gap-2">
          {(
            [
              ['gpx', 'GPX', 'stops + route for GPS apps'],
              ['geojson', 'GeoJSON', 'stops, route, and journal points'],
              ['markdown', 'Markdown', 'trip.md + photos as a zip'],
            ] as const
          ).map(([format, label, hint]) => (
            <a
              key={format}
              href={`/api/v1/trips/${trip.id}/export/${format}`}
              download
              title={hint}
              className="rounded-lg border border-slate-300 dark:border-slate-600 px-4 py-2 text-sm text-slate-600 dark:text-slate-400 hover:bg-slate-50 dark:hover:bg-slate-800"
            >
              ⬇ {label}
            </a>
          ))}
          <Link
            to="/trips/$tripId/print"
            params={{ tripId: trip.id }}
            title="print-friendly document — save as PDF from the print dialog"
            className="rounded-lg border border-slate-300 dark:border-slate-600 px-4 py-2 text-sm text-slate-600 dark:text-slate-400 hover:bg-slate-50 dark:hover:bg-slate-800"
          >
            🖨 Print / PDF
          </Link>
        </div>
      </section>
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
      <Link to="/" className="text-sm text-slate-500 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-100">
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
              <h1 className="text-2xl font-semibold text-slate-900 dark:text-slate-100">{trip.title}</h1>
              <span className={`rounded-full px-2 py-0.5 text-xs font-medium ${statusStyles[trip.effectiveStatus]}`}>
                {trip.effectiveStatus}
              </span>
              {trip.role !== 'owner' && (
                <span className="rounded-full bg-indigo-100 dark:bg-indigo-950 px-2 py-0.5 text-xs font-medium text-indigo-700 dark:text-indigo-300">
                  shared · {trip.role}
                </span>
              )}
            </div>
            <p className="mt-1 text-sm text-slate-500 dark:text-slate-400">{formatRange(trip.startDate, trip.endDate)}</p>
            {trip.description && <p className="mt-2 max-w-2xl text-slate-600 dark:text-slate-400">{trip.description}</p>}
          </div>
          <div className="flex gap-2">
            {trip.role !== 'viewer' && (
              <button
                type="button"
                onClick={() => setEditing(true)}
                className="rounded-lg border border-slate-300 dark:border-slate-600 px-3 py-1.5 text-sm text-slate-600 dark:text-slate-400 hover:bg-slate-50 dark:hover:bg-slate-800"
              >
                Edit
              </button>
            )}
            {trip.role === 'owner' && (
              <button
                type="button"
                onClick={() => {
                  if (window.confirm(`Delete “${trip.title}” and everything in it?`)) remove.mutate()
                }}
                className="rounded-lg border border-red-200 dark:border-red-900 px-3 py-1.5 text-sm text-red-600 dark:text-red-400 hover:bg-red-50 dark:hover:bg-red-950"
              >
                Delete
              </button>
            )}
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

  const field = 'mt-1 w-full rounded-lg border border-slate-300 dark:border-slate-600 px-3 py-2 text-sm focus:border-slate-500 focus:outline-none'

  return (
    <form
      className="mt-3 space-y-3 rounded-xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 p-5 shadow-sm"
      onSubmit={(e) => {
        e.preventDefault()
        mutation.mutate()
      }}
    >
      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
        <label className="block sm:col-span-2">
          <span className="text-sm font-medium text-slate-700 dark:text-slate-300">Title</span>
          <input
            required
            value={form.title}
            onChange={(e) => setForm({ ...form, title: e.target.value })}
            className={field}
          />
        </label>
        <label className="block">
          <span className="text-sm font-medium text-slate-700 dark:text-slate-300">Status</span>
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
            <span className="text-sm font-medium text-slate-700 dark:text-slate-300">Start</span>
            <input
              type="date"
              value={form.startDate}
              max={form.endDate || undefined}
              onChange={(e) => setForm({ ...form, startDate: e.target.value })}
              className={field}
            />
          </label>
          <label className="block">
            <span className="text-sm font-medium text-slate-700 dark:text-slate-300">End</span>
            <input
              type="date"
              value={form.endDate}
              min={form.startDate || undefined}
              onChange={(e) => setForm({ ...form, endDate: e.target.value })}
              className={field}
            />
          </label>
        </div>
        <label className="block sm:col-span-2">
          <span className="text-sm font-medium text-slate-700 dark:text-slate-300">Description</span>
          <textarea
            rows={2}
            value={form.description}
            onChange={(e) => setForm({ ...form, description: e.target.value })}
            className={field}
          />
        </label>
      </div>
      {mutation.error && (
        <p className="text-sm text-red-600 dark:text-red-400">
          {mutation.error instanceof ApiError ? mutation.error.message : 'Could not save'}
        </p>
      )}
      <div className="flex gap-2">
        <button
          type="submit"
          disabled={mutation.isPending}
          className="rounded-lg bg-slate-900 dark:bg-slate-100 px-4 py-2 text-sm font-medium text-white dark:text-slate-900 hover:bg-slate-700 dark:hover:bg-slate-300 disabled:opacity-50"
        >
          {mutation.isPending ? 'Saving…' : 'Save'}
        </button>
        <button
          type="button"
          onClick={onDone}
          className="rounded-lg border border-slate-300 dark:border-slate-600 px-4 py-2 text-sm text-slate-600 dark:text-slate-400 hover:bg-slate-50 dark:hover:bg-slate-800"
        >
          Cancel
        </button>
      </div>
    </form>
  )
}

function StopsSection({
  tripId,
  stops,
  canEdit,
  onHover,
}: {
  tripId: string
  stops: Stop[]
  canEdit: boolean
  onHover: (key: MarkerKey | null) => void
}) {
  const queryClient = useQueryClient()
  const invalidate = () => queryClient.invalidateQueries({ queryKey: ['trip', tripId] })

  // Local copy so drags feel instant; server state re-syncs on refetch.
  const [local, setLocal] = useState(stops)
  useEffect(() => setLocal(stops), [stops])

  const add = useMutation({
    mutationFn: (input: StopInput) => createStop(tripId, input),
    onSuccess: invalidate,
  })
  const remove = useMutation({
    mutationFn: (stopId: string) => deleteStop(tripId, stopId),
    onSuccess: invalidate,
  })
  const reorder = useMutation({
    mutationFn: (ids: string[]) => reorderStops(tripId, ids),
    onSettled: invalidate,
  })

  const sensors = useSensors(useSensor(PointerSensor, { activationConstraint: { distance: 5 } }))

  function onDragEnd({ active, over }: DragEndEvent) {
    if (!over || active.id === over.id) return
    const from = local.findIndex((s) => s.id === active.id)
    const to = local.findIndex((s) => s.id === over.id)
    if (from < 0 || to < 0) return
    const next = arrayMove(local, from, to)
    setLocal(next)
    reorder.mutate(next.map((s) => s.id))
  }

  return (
    <div className="mt-4 space-y-2">
      <DndContext sensors={sensors} collisionDetection={closestCorners} onDragEnd={onDragEnd}>
        <SortableContext items={local.map((s) => s.id)} strategy={verticalListSortingStrategy}>
          {local.map((stop, i) => (
            <StopRow
              key={stop.id}
              stop={stop}
              index={i}
              canEdit={canEdit}
              onHover={onHover}
              onDelete={(id) => remove.mutate(id)}
            />
          ))}
        </SortableContext>
      </DndContext>

      {canEdit && <StopSearch pending={add.isPending} onAdd={(input) => add.mutate(input)} />}
      {add.error && (
        <p className="text-sm text-red-600 dark:text-red-400">
          {add.error instanceof ApiError ? add.error.message : 'Could not add stop'}
        </p>
      )}
    </div>
  )
}

function StopRow({
  stop,
  index,
  canEdit,
  onHover,
  onDelete,
}: {
  stop: Stop
  index: number
  canEdit: boolean
  onHover: (key: MarkerKey | null) => void
  onDelete: (id: string) => void
}) {
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({
    id: stop.id,
    disabled: !canEdit,
  })
  return (
    <div
      ref={setNodeRef}
      style={{ transform: CSS.Transform.toString(transform), transition }}
      onMouseEnter={() => onHover(`stop:${stop.id}`)}
      onMouseLeave={() => onHover(null)}
      className={`flex items-center justify-between rounded-lg border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 px-4 py-3 ${
        isDragging ? 'z-10 opacity-70 shadow-lg' : ''
      }`}
    >
      <div className="flex items-center gap-3">
        {canEdit && (
          <button
            type="button"
            {...attributes}
            {...listeners}
            className="cursor-grab touch-none px-1 text-slate-300 dark:text-slate-600 hover:text-slate-500 dark:hover:text-slate-400 active:cursor-grabbing"
            aria-label={`Drag ${stop.name}`}
          >
            ⠿
          </button>
        )}
        <span className="flex h-6 w-6 items-center justify-center rounded-full bg-slate-100 dark:bg-slate-800 text-xs font-medium text-slate-600 dark:text-slate-400">
          {index + 1}
        </span>
        <div>
          <p className="text-sm font-medium text-slate-900 dark:text-slate-100">{stop.name}</p>
          {(stop.arrivalDate || stop.departureDate) && (
            <p className="text-xs text-slate-500 dark:text-slate-400">{formatRange(stop.arrivalDate, stop.departureDate)}</p>
          )}
        </div>
      </div>
      {canEdit && (
        <button
          type="button"
          onClick={() => onDelete(stop.id)}
          className="px-1 text-sm text-slate-400 dark:text-slate-500 hover:text-red-600 dark:hover:text-red-400"
          aria-label={`Remove ${stop.name}`}
        >
          ✕
        </button>
      )}
    </div>
  )
}

// VenueSearch attaches an address (and coordinates when picked from the
// geocoder) to an itinerary item, so its map pin sits at the actual venue.
function VenueSearch({
  onPick,
}: {
  onPick: (venue: { address: string; lat?: number; lon?: number }) => void
}) {
  const [query, setQuery] = useState('')
  const [debounced, setDebounced] = useState('')
  const [open, setOpen] = useState(false)

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

  return (
    <div className="relative">
      <input
        value={query}
        onChange={(e) => {
          setQuery(e.target.value)
          setOpen(true)
        }}
        onFocus={() => setOpen(true)}
        onBlur={() => {
          window.setTimeout(() => setOpen(false), 150)
          if (query.trim()) onPick({ address: query.trim() })
        }}
        placeholder="Address / venue (optional)"
        className="w-full rounded-lg border border-slate-300 dark:border-slate-600 px-3 py-2 text-sm focus:border-slate-500 focus:outline-none"
      />
      {open && debounced.length >= 2 && (
        <div className="absolute z-10 mt-1 w-full overflow-hidden rounded-lg border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 shadow-lg">
          {results.isLoading && (
            <p className="px-4 py-2 text-sm text-slate-400 dark:text-slate-500">Searching…</p>
          )}
          {results.data?.map((r) => (
            <button
              key={`${r.lat},${r.lon}`}
              type="button"
              onMouseDown={() => {
                onPick({ address: r.name, lat: r.lat, lon: r.lon })
                setQuery('')
                setOpen(false)
              }}
              className="block w-full truncate px-4 py-2 text-left text-sm text-slate-700 dark:text-slate-300 hover:bg-slate-50 dark:hover:bg-slate-800"
            >
              📍 {r.name}
            </button>
          ))}
          {results.data?.length === 0 && (
            <p className="px-4 py-2 text-sm text-slate-400 dark:text-slate-500">
              No places found — blur keeps it as a plain address.
            </p>
          )}
        </div>
      )}
    </div>
  )
}

// StopSearch is a geocoding autocomplete: picking a result adds the stop
// with coordinates; submitting free text adds it without (pick-on-map is #14).
function StopSearch({ pending, onAdd }: { pending: boolean; onAdd: (input: StopInput) => void }) {
  const [query, setQuery] = useState('')
  const [debounced, setDebounced] = useState('')
  const [open, setOpen] = useState(false)

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

  const pick = (input: StopInput) => {
    onAdd(input)
    setQuery('')
    setOpen(false)
  }

  return (
    <div className="relative">
      <form
        className="flex gap-2"
        onSubmit={(e) => {
          e.preventDefault()
          if (query.trim()) pick({ name: query.trim() })
        }}
      >
        <input
          value={query}
          onChange={(e) => {
            setQuery(e.target.value)
            setOpen(true)
          }}
          onFocus={() => setOpen(true)}
          onBlur={() => window.setTimeout(() => setOpen(false), 150)}
          placeholder="Search for a place (e.g. Kyoto)"
          className="flex-1 rounded-lg border border-slate-300 dark:border-slate-600 px-3 py-2 text-sm focus:border-slate-500 focus:outline-none"
        />
        <button
          type="submit"
          disabled={pending || !query.trim()}
          className="rounded-lg bg-slate-900 dark:bg-slate-100 px-4 py-2 text-sm font-medium text-white dark:text-slate-900 hover:bg-slate-700 dark:hover:bg-slate-300 disabled:opacity-50"
        >
          Add
        </button>
      </form>

      {open && debounced.length >= 2 && (
        <div className="absolute z-10 mt-1 w-full overflow-hidden rounded-lg border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 shadow-lg">
          {results.isLoading && <p className="px-4 py-2 text-sm text-slate-400 dark:text-slate-500">Searching…</p>}
          {results.data?.map((r) => (
            <button
              key={`${r.lat},${r.lon}`}
              type="button"
              // onMouseDown so the click wins over the input's onBlur
              onMouseDown={() => pick({ name: shortName(r.name), lat: r.lat, lon: r.lon })}
              className="block w-full truncate px-4 py-2 text-left text-sm text-slate-700 dark:text-slate-300 hover:bg-slate-50 dark:hover:bg-slate-800"
            >
              📍 {r.name}
            </button>
          ))}
          {results.data?.length === 0 && (
            <p className="px-4 py-2 text-sm text-slate-400 dark:text-slate-500">
              No places found — “Add” saves it without coordinates.
            </p>
          )}
        </div>
      )}
    </div>
  )
}

// shortName trims a Nominatim display_name ("Kyoto, Kyoto Prefecture, Japan")
// to its two most significant segments.
function shortName(displayName: string): string {
  const parts = displayName.split(', ')
  return parts.length <= 2 ? displayName : `${parts[0]}, ${parts[parts.length - 1]}`
}


/** Select value encoding one leg endpoint: '' | stopId | 'home:<id>'. */
function endpointValue(stopId: string | null, homeId: string | null): string {
  return homeId ? `home:${homeId}` : (stopId ?? '')
}

/**
 * Create/edit form for itinerary items. With `item` set it edits in place —
 * every field is sent explicitly (empty = clear) so removals stick — and
 * only changed leg endpoints are touched, keeping other members' homes.
 */
export function NewItemForm({
  trip,
  stops,
  item,
  layers,
  onDone,
}: {
  trip: Trip
  stops: Stop[]
  item?: ItineraryItem
  /** Layers the caller may write; new items land on the selected one and
   * edit mode offers moving between them. An empty id means the default
   * Main layer (it may not exist yet). */
  layers?: ItineraryLayer[]
  onDone?: () => void
}) {
  const tripId = trip.id
  const queryClient = useQueryClient()
  const [title, setTitle] = useState(item?.title ?? '')
  const [day, setDay] = useState(() => item?.day ?? defaultTripDay(trip.startDate, trip.endDate))
  const [startTime, setStartTime] = useState(item?.startTime ?? '')
  const [endTime, setEndTime] = useState(item?.endTime ?? '')
  const [category, setCategory] = useState<ItineraryCategory>(item?.category ?? 'activity')
  const [stopId, setStopId] = useState(item ? endpointValue(item.stopId, item.originHomeId) : '')
  const [destinationStopId, setDestinationStopId] = useState(
    item ? endpointValue(item.destinationStopId, item.destinationHomeId) : '',
  )
  const [venue, setVenue] = useState<{ address: string; lat?: number; lon?: number } | null>(
    item?.address
      ? { address: item.address, lat: item.lat ?? undefined, lon: item.lon ?? undefined }
      : null,
  )
  const [itemLayerId, setItemLayerId] = useState(item?.layerId ?? layers?.[0]?.id ?? '')
  const isLeg = category === 'flight' || category === 'train'
  const myHomes = useQuery({ queryKey: ['homes'], queryFn: listHomes, enabled: isLeg })

  const add = useMutation({
    mutationFn: () => {
      const origin: ItemInput = stopId.startsWith('home:')
        ? { originHomeId: stopId.slice(5), clearStop: true }
        : stopId
          ? { stopId, clearOriginHome: true }
          : { clearStop: true, clearOriginHome: true }
      const destination: ItemInput =
        isLeg && destinationStopId.startsWith('home:')
          ? { destinationHomeId: destinationStopId.slice(5), clearDestination: true }
          : isLeg && destinationStopId
            ? { destinationStopId, clearDestinationHome: true }
            : { clearDestination: true, clearDestinationHome: true }
      const venueFields: ItemInput = venue
        ? {
            address: venue.address,
            ...(venue.lat !== undefined && venue.lon !== undefined
              ? { lat: venue.lat, lon: venue.lon }
              : { clearLatLon: true }),
          }
        : { address: '', clearLatLon: true }

      if (item) {
        return updateItem(tripId, item.id, {
          title,
          day,
          category,
          startTime,
          endTime,
          ...(itemLayerId && itemLayerId !== item.layerId ? { layerId: itemLayerId } : {}),
          ...venueFields,
          // Untouched endpoints stay as they are — they may reference
          // another member's home, which we could not re-submit.
          ...(stopId !== endpointValue(item.stopId, item.originHomeId) ? origin : {}),
          ...(destinationStopId !== endpointValue(item.destinationStopId, item.destinationHomeId)
            ? destination
            : {}),
        })
      }
      return createItem(tripId, {
        title,
        day,
        category,
        ...(itemLayerId ? { layerId: itemLayerId } : {}),
        ...(startTime ? { startTime } : {}),
        ...(endTime ? { endTime } : {}),
        ...(venue
          ? {
              address: venue.address,
              ...(venue.lat !== undefined && venue.lon !== undefined
                ? { lat: venue.lat, lon: venue.lon }
                : {}),
            }
          : {}),
        ...(stopId.startsWith('home:')
          ? { originHomeId: stopId.slice(5) }
          : stopId
            ? { stopId }
            : {}),
        ...(isLeg && destinationStopId.startsWith('home:')
          ? { destinationHomeId: destinationStopId.slice(5) }
          : isLeg && destinationStopId
            ? { destinationStopId }
            : {}),
      })
    },
    onSuccess: async () => {
      if (!item) {
        setTitle('')
        setVenue(null)
      }
      await queryClient.invalidateQueries({ queryKey: ['trip', tripId] })
      onDone?.()
    },
  })
  const remove = useMutation({
    mutationFn: () => deleteItem(tripId, item!.id),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ['trip', tripId] })
      onDone?.()
    },
  })

  const field = 'rounded-lg border border-slate-300 dark:border-slate-600 px-3 py-2 text-sm focus:border-slate-500 focus:outline-none'

  return (
    <form
      className="space-y-2 rounded-lg border border-dashed border-slate-300 dark:border-slate-600 p-3"
      onSubmit={(e) => {
        e.preventDefault()
        if (title.trim() && day) add.mutate()
      }}
    >
      <div className="flex flex-wrap gap-2">
        <input
          value={title}
          onChange={(e) => setTitle(e.target.value)}
          placeholder={isLeg ? 'Flight/train number or name' : 'Add an activity'}
          className={`${field} min-w-40 flex-1`}
        />
        <input
          type="date"
          required
          value={day}
          min={trip.startDate ?? undefined}
          max={trip.endDate ?? undefined}
          onChange={(e) => setDay(e.target.value)}
          className={field}
        />
        <input
          type="time"
          value={startTime}
          onChange={(e) => setStartTime(e.target.value)}
          title={isLeg ? 'Departure time' : 'Start time'}
          className={field}
        />
        <input
          type="time"
          value={endTime}
          onChange={(e) => setEndTime(e.target.value)}
          title={isLeg ? 'Arrival time (local)' : 'End time'}
          className={field}
        />
      </div>
      {venue ? (
        <div className="flex items-center gap-2 text-sm">
          <span className="truncate rounded-lg bg-slate-100 dark:bg-slate-800 px-3 py-1.5 text-slate-700 dark:text-slate-300">
            📍 {venue.address}
          </span>
          <button
            type="button"
            onClick={() => setVenue(null)}
            className="text-slate-400 dark:text-slate-500 hover:text-red-600 dark:hover:text-red-400"
            aria-label="Remove venue"
          >
            ✕
          </button>
        </div>
      ) : (
        <VenueSearch onPick={setVenue} />
      )}
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
        {layers && layers.length > 1 && (
          <select
            value={itemLayerId}
            onChange={(e) => setItemLayerId(e.target.value)}
            aria-label="Layer"
            title="Layer this item belongs to"
            className={field}
          >
            {layers.map((l) => (
              <option key={l.id} value={l.id}>
                ⬤ {l.name}
              </option>
            ))}
          </select>
        )}
        <select value={stopId} onChange={(e) => setStopId(e.target.value)} className={field}>
          <option value="">{isLeg ? 'from' : 'no stop'}</option>
          {isLeg &&
            myHomes.data?.map((h) => (
              <option key={h.id} value={`home:${h.id}`}>
                (home) {h.name}
              </option>
            ))}
          {stops.map((s) => (
            <option key={s.id} value={s.id}>
              {s.name}
            </option>
          ))}
        </select>
        {isLeg && (
          <select
            value={destinationStopId}
            onChange={(e) => setDestinationStopId(e.target.value)}
            className={field}
          >
            <option value="">to</option>
            {myHomes.data?.map((h) => (
              <option key={h.id} value={`home:${h.id}`}>
                (home) {h.name}
              </option>
            ))}
            {stops.map((s) => (
              <option key={s.id} value={s.id}>
                {s.name}
              </option>
            ))}
          </select>
        )}
        <button
          type="submit"
          disabled={add.isPending || !title.trim() || !day}
          className="rounded-lg bg-slate-900 dark:bg-slate-100 px-4 py-2 text-sm font-medium text-white dark:text-slate-900 hover:bg-slate-700 dark:hover:bg-slate-300 disabled:opacity-50"
        >
          {item ? 'Save' : 'Add'}
        </button>
        {item && onDone && (
          <button
            type="button"
            onClick={onDone}
            className="rounded-lg px-3 py-2 text-sm text-slate-500 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-100"
          >
            Cancel
          </button>
        )}
        {item && (
          <button
            type="button"
            onClick={() => {
              if (window.confirm(`Delete "${item.title}"?`)) remove.mutate()
            }}
            disabled={remove.isPending}
            className="ml-auto rounded-lg px-3 py-2 text-sm text-red-600 dark:text-red-400 hover:bg-red-50 dark:hover:bg-red-950 disabled:opacity-50"
          >
            Delete item
          </button>
        )}
      </div>
      {add.error && (
        <p className="text-sm text-red-600 dark:text-red-400">
          {add.error instanceof ApiError ? add.error.message : item ? 'Could not save item' : 'Could not add item'}
        </p>
      )}
    </form>
  )
}

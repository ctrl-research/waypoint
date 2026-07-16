import { Suspense, lazy, useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Link, Navigate, useParams } from '@tanstack/react-router'
import { fetchMe, getTrip, type ItineraryItem, type ItineraryLayer } from '../api'
import { ItineraryBoard } from './ItineraryBoard'
import { LayersPanel } from './LayersPanel'
import { NewItemForm, StopsSection } from './TripDetail'

const TripMap = lazy(() => import('../TripMap').then((m) => ({ default: m.TripMap })))
type MarkerKey = `stop:${string}` | `item:${string}`

/**
 * Dedicated itinerary editor (#73). Members organize items on any number
 * of named layers; the itinerary is simply the merge of all visible
 * layers, everywhere — the map, this list, the trip page, and exports.
 */
export function ItineraryEditorPage() {
  const { tripId } = useParams({ from: '/trips/$tripId/itinerary' })
  const { data: me, isLoading } = useQuery({ queryKey: ['me'], queryFn: fetchMe })
  const detail = useQuery({ queryKey: ['trip', tripId], queryFn: () => getTrip(tripId), enabled: !!me })
  const [highlightKey, setHighlightKey] = useState<MarkerKey | null>(null)
  const [editing, setEditing] = useState<ItineraryItem | null>(null)
  const [focusStop, setFocusStop] = useState<{ id: string; nonce: number } | null>(null)

  const layers = detail.data?.layers ?? []
  const layerColors = useMemo(
    () => Object.fromEntries(layers.map((l) => [l.id, l.color])),
    [layers],
  )

  if (isLoading) return null
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

  const { trip, stops, items } = detail.data
  const canEditMain = trip.role !== 'viewer'

  // The itinerary is the merge of visible layers.
  const hidden = new Set(layers.filter((l) => !l.visible).map((l) => l.id))
  const visibleItems = items.filter((i) => !hidden.has(i.layerId))

  // Mirrors the server's rules (layerEditable / layerManageable).
  const canEditLayer = (layer: ItineraryLayer) => canEditMain || layer.ownerId === me.id
  const canEditItem = (item: ItineraryItem) => {
    const layer = layers.find((l) => l.id === item.layerId)
    return canEditMain || layer?.ownerId === me.id
  }

  // Layers offered by the add/edit forms. Until the Main layer exists (it
  // is created lazily with the first item) editors get a stand-in option.
  const editableLayers = layers.filter(canEditLayer)
  const formLayers =
    canEditMain && !layers.some((l) => l.ownerId === null)
      ? [{ id: '', name: 'Main', color: '#2a78d6', ownerId: null, visible: true }, ...editableLayers]
      : editableLayers
  const canAddItems = formLayers.length > 0

  return (
    <div className="mx-auto mt-8 w-full max-w-5xl px-4 pb-24">
      <Link
        to="/trips/$tripId"
        params={{ tripId }}
        className="text-sm text-slate-500 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-100"
      >
        ← {trip.title}
      </Link>
      <div className="mt-2">
        <h1 className="text-2xl font-semibold text-slate-900 dark:text-slate-100">
          Itinerary editor
        </h1>
        <p className="text-sm text-slate-500 dark:text-slate-400">
          {canAddItems
            ? 'The itinerary is everything on the visible layers, ordered by start time. Changes save as you go.'
            : 'You have view-only access — add a layer of your own to start suggesting items.'}
        </p>
      </div>

      <div className="mt-6">
        <Suspense
          fallback={<div className="h-80 w-full rounded-xl border border-slate-200 dark:border-slate-700 bg-slate-50 dark:bg-slate-950" />}
        >
          <TripMap
            stops={stops}
            items={visibleItems}
            layerColors={layerColors}
            highlightKey={highlightKey}
            focusStop={focusStop}
          />
        </Suspense>
      </div>

      <section className="mt-6">
        <h2 className="text-sm font-semibold text-slate-700 dark:text-slate-300">Areas</h2>
        <p className="text-sm text-slate-500 dark:text-slate-400">
          The route's countries, cities, or regions. Click to focus the map; drag to reorder.
        </p>
        <StopsSection
          tripId={trip.id}
          stops={stops}
          items={items}
          canEdit={canEditMain}
          onHover={setHighlightKey}
          onFocus={(id) => setFocusStop((cur) => ({ id, nonce: (cur?.nonce ?? 0) + 1 }))}
        />
      </section>

      <section className="mt-6" data-tour="layers-panel">
        <LayersPanel tripId={trip.id} role={trip.role} meId={me.id} layers={layers} items={items} manage />
      </section>

      <div className="mt-2">
        <ItineraryBoard
          trip={trip}
          items={visibleItems}
          combined
          layers={layers}
          canEditItem={canEditItem}
          onEdit={(item) => setEditing(item)}
          onHover={setHighlightKey}
        />
        {editing ? (
          <div className="mt-4">
            <h3 className="mb-1 text-sm font-semibold text-slate-700 dark:text-slate-300">
              Editing “{editing.title}”
            </h3>
            <NewItemForm
              key={editing.id}
              trip={trip}
              stops={stops}
              item={editing}
              layers={editableLayers}
              onDone={() => setEditing(null)}
            />
          </div>
        ) : (
          canAddItems && (
            <div className="mt-4" data-tour="item-form">
              <NewItemForm trip={trip} stops={stops} layers={formLayers} />
            </div>
          )
        )}
      </div>
    </div>
  )
}

import { Suspense, lazy, useMemo, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link, Navigate, useParams } from '@tanstack/react-router'
import {
  deleteLayer,
  ensureMyLayer,
  fetchMe,
  getTrip,
  updateItem,
  type ItineraryLayer,
} from '../api'
import { EyeIcon } from '../icons'
import { ItineraryBoard } from './ItineraryBoard'
import { NewItemForm } from './TripDetail'

const TripMap = lazy(() => import('../TripMap').then((m) => ({ default: m.TripMap })))
type MarkerKey = `stop:${string}` | `item:${string}`

/**
 * Dedicated itinerary editor (#73). One layer is active on the board at a
 * time — Final or a member's proposal; the map overlays every visible layer
 * in its color. Promotion moves an item's layer_id, never copies.
 */
export function ItineraryEditorPage() {
  const { tripId } = useParams({ from: '/trips/$tripId/itinerary' })
  const { data: me, isLoading } = useQuery({ queryKey: ['me'], queryFn: fetchMe })
  const detail = useQuery({ queryKey: ['trip', tripId], queryFn: () => getTrip(tripId), enabled: !!me })
  const queryClient = useQueryClient()
  const [highlightKey, setHighlightKey] = useState<MarkerKey | null>(null)
  const [activeLayerId, setActiveLayerId] = useState<string | null>(null)
  const [hiddenLayers, setHiddenLayers] = useState<Set<string>>(new Set())

  const invalidate = () => queryClient.invalidateQueries({ queryKey: ['trip', tripId] })
  const propose = useMutation({
    mutationFn: () => ensureMyLayer(tripId),
    onSuccess: (layer) => {
      setActiveLayerId(layer.id)
      invalidate()
    },
  })
  const removeLayer = useMutation({
    mutationFn: (layerId: string) => deleteLayer(tripId, layerId),
    onSuccess: () => {
      setActiveLayerId(null)
      invalidate()
    },
  })
  const promote = useMutation({
    mutationFn: ({ itemId, layerId }: { itemId: string; layerId: string }) =>
      updateItem(tripId, itemId, { layerId }),
    onSettled: invalidate,
  })

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

  const { trip, stops, items, homes } = detail.data
  const finalLayer = layers.find((l) => l.ownerId === null)
  const myLayer = layers.find((l) => l.ownerId === me.id)
  const canEditFinal = trip.role !== 'viewer'

  // The board edits one layer at a time; Final is the default. A trip that
  // predates its Final layer (no items yet) behaves as Final until the
  // first item creates it.
  const activeLayer: ItineraryLayer | null =
    layers.find((l) => l.id === activeLayerId) ?? finalLayer ?? null
  const activeIsFinal = !activeLayer || activeLayer.ownerId === null
  const canEditActive = activeIsFinal ? canEditFinal : activeLayer?.ownerId === me.id || canEditFinal
  const boardItems = activeLayer ? items.filter((i) => i.layerId === activeLayer.id) : items
  const mapItems = items.filter((i) => !hiddenLayers.has(i.layerId))

  // Promotion targets: proposals send items to Final (editor+); Final sends
  // them back to your own proposal layer.
  const promoteTarget =
    !activeIsFinal && canEditFinal && finalLayer
      ? { layerId: finalLayer.id, label: '→ Final' }
      : activeIsFinal && myLayer
        ? { layerId: myLayer.id, label: `→ ${myLayer.name}` }
        : null

  const toggleLayerVisibility = (layerId: string) =>
    setHiddenLayers((cur) => {
      const next = new Set(cur)
      if (next.has(layerId)) next.delete(layerId)
      else next.add(layerId)
      return next
    })

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
          {canEditActive
            ? 'Changes save as you go. Use the layer chips to switch between the final plan and proposals.'
            : 'You have view-only access to this layer — propose changes on your own layer.'}
        </p>
      </div>

      <div className="mt-6">
        <Suspense
          fallback={<div className="h-80 w-full rounded-xl border border-slate-200 dark:border-slate-700 bg-slate-50 dark:bg-slate-950" />}
        >
          <TripMap
            stops={stops}
            items={mapItems}
            layerColors={layerColors}
            highlightKey={highlightKey}
            picking={false}
            onPick={() => {}}
          />
        </Suspense>
      </div>

      {/* Layer switcher: click a chip to edit that layer; the 👁 keeps other
          layers overlaid on the map for comparison. */}
      <div className="mt-6 flex flex-wrap items-center gap-2">
        {(layers.length > 0 ? layers : []).map((layer) => {
          const active = activeLayer?.id === layer.id
          return (
            <div
              key={layer.id}
              className={`flex items-center overflow-hidden rounded-full border text-sm ${
                active
                  ? 'border-slate-900 dark:border-slate-100 bg-slate-900 dark:bg-slate-100 text-white dark:text-slate-900'
                  : 'border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900 text-slate-700 dark:text-slate-300'
              }`}
            >
              <button
                type="button"
                onClick={() => setActiveLayerId(layer.id)}
                className="flex items-center gap-2 py-1 pl-3 pr-1"
              >
                <span
                  className="h-2.5 w-2.5 rounded-full"
                  style={{ backgroundColor: layer.color }}
                />
                {layer.name}
              </button>
              <button
                type="button"
                onClick={() => toggleLayerVisibility(layer.id)}
                className={`py-1 pl-1 pr-2.5 ${hiddenLayers.has(layer.id) ? 'opacity-40' : ''}`}
                title={hiddenLayers.has(layer.id) ? 'Show on map' : 'Hide from map'}
                aria-label={`Toggle ${layer.name} on the map`}
              >
                <EyeIcon open={!hiddenLayers.has(layer.id)} />
              </button>
            </div>
          )
        })}
        {layers.length === 0 && (
          <span className="rounded-full border border-slate-200 dark:border-slate-700 px-3 py-1 text-sm text-slate-500 dark:text-slate-400">
            Final
          </span>
        )}
        {!myLayer && (
          <button
            type="button"
            onClick={() => propose.mutate()}
            disabled={propose.isPending}
            className="rounded-full border border-dashed border-slate-300 dark:border-slate-600 px-3 py-1 text-sm text-slate-500 dark:text-slate-400 hover:border-slate-900 dark:hover:border-slate-100 hover:text-slate-900 dark:hover:text-slate-100"
          >
            ＋ Propose changes
          </button>
        )}
        {activeLayer && !activeIsFinal && (activeLayer.ownerId === me.id || trip.role === 'owner') && (
          <button
            type="button"
            onClick={() => {
              if (window.confirm(`Delete "${activeLayer.name}" and its ${boardItems.length} item(s)?`)) {
                removeLayer.mutate(activeLayer.id)
              }
            }}
            className="ml-auto text-xs text-slate-400 dark:text-slate-500 hover:text-red-600 dark:hover:text-red-400"
          >
            Delete layer
          </button>
        )}
      </div>

      <div className="mt-2">
        <ItineraryBoard
          trip={trip}
          items={boardItems}
          stops={stops}
          homes={homes}
          readOnly={!canEditActive}
          onHover={setHighlightKey}
          layerId={activeLayer?.id}
          promoteLabel={canEditActive && promoteTarget ? promoteTarget.label : undefined}
          onPromote={
            canEditActive && promoteTarget
              ? (itemId) => promote.mutate({ itemId, layerId: promoteTarget.layerId })
              : undefined
          }
        />
        {canEditActive && (
          <div className="mt-4">
            <NewItemForm trip={trip} stops={stops} layerId={activeLayer?.id} />
          </div>
        )}
      </div>
    </div>
  )
}

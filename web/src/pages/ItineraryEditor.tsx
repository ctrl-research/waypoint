import { Suspense, lazy, useMemo, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link, Navigate, useParams } from '@tanstack/react-router'
import {
  createLayer,
  deleteLayer,
  fetchMe,
  getTrip,
  updateLayer,
  type ItineraryItem,
  type ItineraryLayer,
} from '../api'
import { EyeIcon } from '../icons'
import { ItineraryBoard } from './ItineraryBoard'
import { NewItemForm } from './TripDetail'

const TripMap = lazy(() => import('../TripMap').then((m) => ({ default: m.TripMap })))
type MarkerKey = `stop:${string}` | `item:${string}`

/** Swatches offered in layer settings — the server's palette plus Final blue. */
const LAYER_SWATCHES = ['#2a78d6', '#d97706', '#059669', '#7c3aed', '#db2777', '#0891b2', '#65a30d']

/**
 * Dedicated itinerary editor (#73). Members organize items on any number
 * of named layers and compile them into the shared Plan layer — the one
 * the trip page shows. The list and map overlay every visible layer in
 * its color; promotion moves an item's layer_id, never copies.
 */
export function ItineraryEditorPage() {
  const { tripId } = useParams({ from: '/trips/$tripId/itinerary' })
  const { data: me, isLoading } = useQuery({ queryKey: ['me'], queryFn: fetchMe })
  const detail = useQuery({ queryKey: ['trip', tripId], queryFn: () => getTrip(tripId), enabled: !!me })
  const queryClient = useQueryClient()
  const [highlightKey, setHighlightKey] = useState<MarkerKey | null>(null)
  const [activeLayerId, setActiveLayerId] = useState<string | null>(null)
  const [hiddenLayers, setHiddenLayers] = useState<Set<string>>(new Set())
  const [settingsOpen, setSettingsOpen] = useState(false)
  const [layerName, setLayerName] = useState('')
  const [editing, setEditing] = useState<ItineraryItem | null>(null)
  const [newLayerOpen, setNewLayerOpen] = useState(false)
  const [newLayerName, setNewLayerName] = useState('')

  const invalidate = () => queryClient.invalidateQueries({ queryKey: ['trip', tripId] })
  const addLayer = useMutation({
    mutationFn: (name: string) => createLayer(tripId, name),
    onSuccess: (layer) => {
      setActiveLayerId(layer.id)
      setNewLayerOpen(false)
      setNewLayerName('')
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
  const customize = useMutation({
    mutationFn: ({ layerId, name, color }: { layerId: string; name?: string; color?: string }) =>
      updateLayer(tripId, layerId, { ...(name ? { name } : {}), ...(color ? { color } : {}) }),
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
  const planLayer = layers.find((l) => l.ownerId === null)
  const canEditPlan = trip.role !== 'viewer'

  // New items land on the active layer; Plan is the default. A trip that
  // predates its Plan layer (no items yet) behaves as Plan until the
  // first item creates it.
  const activeLayer: ItineraryLayer | null =
    layers.find((l) => l.id === activeLayerId) ?? planLayer ?? null
  const activeIsPlan = !activeLayer || activeLayer.ownerId === null
  const canEditActive = activeIsPlan ? canEditPlan : activeLayer?.ownerId === me.id || canEditPlan

  // The list and map show every visible layer combined; the active chip
  // only decides where new items land and which layer Customize targets.
  const visibleItems = items.filter((i) => !hiddenLayers.has(i.layerId))
  const activeCount = activeLayer ? items.filter((i) => i.layerId === activeLayer.id).length : 0

  // Mirrors the server's rules: Plan needs editor+, a member layer belongs
  // to its owner (the trip owner can moderate).
  const canManageActive = activeLayer
    ? activeIsPlan
      ? canEditPlan
      : activeLayer.ownerId === me.id || trip.role === 'owner'
    : false

  const canEditItem = (item: ItineraryItem) => {
    const layer = layers.find((l) => l.id === item.layerId)
    return canEditPlan || layer?.ownerId === me.id
  }
  const editableLayers = layers.filter((l) =>
    l.ownerId === null ? canEditPlan : canEditPlan || l.ownerId === me.id,
  )

  // Per-row quick action: compile an item into the Plan (editor+). Moving
  // between member layers happens in the edit form's layer select.
  const promoteFor = (item: ItineraryItem) => {
    const onPlan = planLayer && item.layerId === planLayer.id
    if (!onPlan && planLayer && canEditPlan && canEditItem(item)) {
      return { layerId: planLayer.id, label: '→ Plan' }
    }
    return null
  }

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
            ? 'Changes save as you go. All visible layers are listed together, ordered by start time; new items land on the selected layer.'
            : 'You have view-only access to this layer — propose changes on your own layer.'}
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
            picking={false}
            onPick={() => {}}
          />
        </Suspense>
      </div>

      {/* Layer switcher: the selected chip is where new items land; the eye
          shows/hides that layer's items on the map and in the list. */}
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
            Plan
          </span>
        )}
        {newLayerOpen ? (
          <form
            className="flex items-center gap-1"
            onSubmit={(e) => {
              e.preventDefault()
              if (newLayerName.trim()) addLayer.mutate(newLayerName.trim())
            }}
          >
            <input
              autoFocus
              value={newLayerName}
              onChange={(e) => setNewLayerName(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === 'Escape') setNewLayerOpen(false)
              }}
              placeholder="Layer name"
              aria-label="New layer name"
              className="w-36 rounded-full border border-slate-300 dark:border-slate-600 bg-transparent px-3 py-1 text-sm text-slate-900 dark:text-slate-100 focus:border-slate-500 focus:outline-none"
            />
            <button
              type="submit"
              disabled={addLayer.isPending || !newLayerName.trim()}
              className="rounded-full bg-slate-900 dark:bg-slate-100 px-3 py-1 text-sm text-white dark:text-slate-900 disabled:opacity-50"
            >
              Add
            </button>
            <button
              type="button"
              onClick={() => setNewLayerOpen(false)}
              className="px-1 text-sm text-slate-400 dark:text-slate-500 hover:text-slate-900 dark:hover:text-slate-100"
              aria-label="Cancel new layer"
            >
              ✕
            </button>
          </form>
        ) : (
          <button
            type="button"
            onClick={() => setNewLayerOpen(true)}
            className="rounded-full border border-dashed border-slate-300 dark:border-slate-600 px-3 py-1 text-sm text-slate-500 dark:text-slate-400 hover:border-slate-900 dark:hover:border-slate-100 hover:text-slate-900 dark:hover:text-slate-100"
          >
            ＋ New layer
          </button>
        )}
        {activeLayer && canManageActive && (
          <button
            type="button"
            onClick={() => {
              setLayerName(activeLayer.name)
              setSettingsOpen((open) => !open)
            }}
            className="text-xs text-slate-400 dark:text-slate-500 hover:text-slate-900 dark:hover:text-slate-100"
            title={`Customize ${activeLayer.name}`}
          >
            ✎ Customize
          </button>
        )}
        {activeLayer && !activeIsPlan && canManageActive && (
          <button
            type="button"
            onClick={() => {
              if (window.confirm(`Delete "${activeLayer.name}" and its ${activeCount} item(s)?`)) {
                removeLayer.mutate(activeLayer.id)
              }
            }}
            className="ml-auto text-xs text-slate-400 dark:text-slate-500 hover:text-red-600 dark:hover:text-red-400"
          >
            Delete layer
          </button>
        )}
      </div>

      {settingsOpen && activeLayer && canManageActive && (
        <div className="mt-3 flex flex-wrap items-center gap-3 rounded-xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900 px-3 py-2">
          <input
            value={layerName}
            onChange={(e) => setLayerName(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Enter' && layerName.trim()) {
                customize.mutate({ layerId: activeLayer.id, name: layerName.trim() })
              }
            }}
            onBlur={() => {
              if (layerName.trim() && layerName.trim() !== activeLayer.name) {
                customize.mutate({ layerId: activeLayer.id, name: layerName.trim() })
              }
            }}
            aria-label="Layer name"
            className="w-40 rounded-lg border border-slate-200 dark:border-slate-700 bg-transparent px-2 py-1 text-sm text-slate-900 dark:text-slate-100"
          />
          <div className="flex items-center gap-1.5">
            {LAYER_SWATCHES.map((color) => (
              <button
                key={color}
                type="button"
                onClick={() => customize.mutate({ layerId: activeLayer.id, color })}
                className={`h-5 w-5 rounded-full ${
                  activeLayer.color === color ? 'ring-2 ring-slate-900 dark:ring-slate-100 ring-offset-1' : ''
                }`}
                style={{ backgroundColor: color }}
                aria-label={`Set color ${color}`}
              />
            ))}
            <input
              type="color"
              value={activeLayer.color}
              onChange={(e) => customize.mutate({ layerId: activeLayer.id, color: e.target.value })}
              aria-label="Custom color"
              className="h-6 w-8 cursor-pointer rounded border border-slate-200 dark:border-slate-700 bg-transparent"
            />
          </div>
          <button
            type="button"
            onClick={() => setSettingsOpen(false)}
            className="ml-auto text-xs text-slate-400 dark:text-slate-500 hover:text-slate-900 dark:hover:text-slate-100"
          >
            Done
          </button>
        </div>
      )}

      <div className="mt-2">
        <ItineraryBoard
          trip={trip}
          items={visibleItems}
          stops={stops}
          homes={homes}
          combined
          layers={layers}
          canEditItem={canEditItem}
          promoteFor={promoteFor}
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
          canEditActive && (
            <div className="mt-4">
              <NewItemForm trip={trip} stops={stops} layerId={activeLayer?.id} />
            </div>
          )
        )}
      </div>
    </div>
  )
}

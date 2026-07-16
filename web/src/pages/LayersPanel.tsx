import { useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import {
  createLayer,
  deleteLayer,
  updateLayer,
  type ItineraryItem,
  type ItineraryLayer,
  type TripRole,
} from '../api'
import { EyeIcon, PencilIcon } from '../icons'

/** Swatches offered in layer settings — the server's palette plus Main blue. */
const LAYER_SWATCHES = ['#2a78d6', '#d97706', '#059669', '#7c3aed', '#db2777', '#0891b2', '#65a30d']

/**
 * The trip's layers, one row each with an include/hide eye — the itinerary
 * everywhere is the merge of visible layers (#73). The same panel shows on
 * the trip page and the editor for consistency; `manage` adds creating,
 * renaming/recoloring, and deleting, which live in the editor.
 */
export function LayersPanel({
  tripId,
  role,
  meId,
  layers,
  items,
  manage = false,
  hint,
}: {
  tripId: string
  role: TripRole
  meId: string
  layers: ItineraryLayer[]
  items: ItineraryItem[]
  manage?: boolean
  hint?: string
}) {
  const queryClient = useQueryClient()
  const [newLayerOpen, setNewLayerOpen] = useState(false)
  const [newLayerName, setNewLayerName] = useState('')
  const [editingLayerId, setEditingLayerId] = useState<string | null>(null)

  const invalidate = () => queryClient.invalidateQueries({ queryKey: ['trip', tripId] })
  const addLayer = useMutation({
    mutationFn: (name: string) => createLayer(tripId, name),
    onSuccess: () => {
      setNewLayerOpen(false)
      setNewLayerName('')
      invalidate()
    },
  })
  const removeLayer = useMutation({
    mutationFn: (layerId: string) => deleteLayer(tripId, layerId),
    onSuccess: invalidate,
  })
  const customize = useMutation({
    mutationFn: ({ layerId, ...input }: { layerId: string; name?: string; color?: string; visible?: boolean }) =>
      updateLayer(tripId, layerId, input),
    onSettled: invalidate,
  })

  // Mirrors the server's rules (layerEditable / layerManageable).
  const canEditMain = role !== 'viewer'
  const canEditLayer = (layer: ItineraryLayer) => canEditMain || layer.ownerId === meId
  const canManageLayer = (layer: ItineraryLayer) =>
    layer.ownerId === null ? canEditMain : layer.ownerId === meId || role === 'owner'
  const countFor = (layerId: string) => items.filter((i) => i.layerId === layerId).length

  return (
    <div>
      <div className="flex items-center justify-between">
        <h3 className="text-sm font-semibold text-slate-700 dark:text-slate-300">Layers</h3>
        {manage &&
          (newLayerOpen ? (
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
                className="w-40 rounded-lg border border-slate-300 dark:border-slate-600 bg-transparent px-3 py-1 text-sm text-slate-900 dark:text-slate-100 focus:border-slate-500 focus:outline-none"
              />
              <button
                type="submit"
                disabled={addLayer.isPending || !newLayerName.trim()}
                className="rounded-lg bg-slate-900 dark:bg-slate-100 px-3 py-1 text-sm text-white dark:text-slate-900 disabled:opacity-50"
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
              className="text-sm text-slate-500 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-100"
            >
              ＋ New layer
            </button>
          ))}
      </div>
      {hint && <p className="text-sm text-slate-500 dark:text-slate-400">{hint}</p>}

      <div className="mt-2 divide-y divide-slate-100 dark:divide-slate-800 rounded-xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900">
        {layers.length === 0 && (
          <p className="px-4 py-3 text-sm text-slate-400 dark:text-slate-500">
            Items live on the Main layer{manage ? '; add layers to organize ideas separately' : ''}.
          </p>
        )}
        {layers.map((layer) =>
          manage && editingLayerId === layer.id ? (
            <LayerSettingsRow
              key={layer.id}
              layer={layer}
              onSave={(input) => {
                customize.mutate({ layerId: layer.id, ...input })
                setEditingLayerId(null)
              }}
              onCancel={() => setEditingLayerId(null)}
            />
          ) : (
            <div
              key={layer.id}
              className={`flex items-center gap-3 px-4 py-2.5 ${layer.visible ? '' : 'opacity-50'}`}
            >
              <span
                className="h-3 w-3 shrink-0 rounded-full"
                style={{ backgroundColor: layer.color }}
                aria-hidden="true"
              />
              <span className="min-w-0 truncate text-sm font-medium text-slate-900 dark:text-slate-100">
                {layer.name}
              </span>
              <span className="text-xs text-slate-400 dark:text-slate-500">
                {countFor(layer.id)} item{countFor(layer.id) === 1 ? '' : 's'}
              </span>
              <div className="ml-auto flex items-center gap-1">
                {canEditLayer(layer) && (
                  <button
                    type="button"
                    onClick={() => customize.mutate({ layerId: layer.id, visible: !layer.visible })}
                    className="rounded-md p-1.5 text-slate-500 dark:text-slate-400 hover:bg-slate-100 dark:hover:bg-slate-800 hover:text-slate-900 dark:hover:text-slate-100"
                    title={layer.visible ? 'Hide from the itinerary' : 'Include in the itinerary'}
                    aria-pressed={layer.visible}
                    aria-label={`${layer.visible ? 'Hide' : 'Show'} ${layer.name}`}
                  >
                    <EyeIcon open={layer.visible} className="h-4 w-4" />
                  </button>
                )}
                {manage && canManageLayer(layer) && (
                  <button
                    type="button"
                    onClick={() => setEditingLayerId(layer.id)}
                    className="rounded-md p-1.5 text-slate-500 dark:text-slate-400 hover:bg-slate-100 dark:hover:bg-slate-800 hover:text-slate-900 dark:hover:text-slate-100"
                    title={`Rename or recolor ${layer.name}`}
                    aria-label={`Edit ${layer.name}`}
                  >
                    <PencilIcon className="h-4 w-4" />
                  </button>
                )}
                {manage && layer.ownerId !== null && canManageLayer(layer) && (
                  <button
                    type="button"
                    onClick={() => {
                      if (window.confirm(`Delete "${layer.name}" and its ${countFor(layer.id)} item(s)?`)) {
                        removeLayer.mutate(layer.id)
                      }
                    }}
                    className="rounded-md p-1.5 text-sm leading-none text-slate-400 dark:text-slate-500 hover:bg-slate-100 dark:hover:bg-slate-800 hover:text-red-600 dark:hover:text-red-400"
                    title={`Delete ${layer.name}`}
                    aria-label={`Delete ${layer.name}`}
                  >
                    ✕
                  </button>
                )}
              </div>
            </div>
          ),
        )}
      </div>
    </div>
  )
}

/** Inline rename + recolor, in place of the layer's row. */
function LayerSettingsRow({
  layer,
  onSave,
  onCancel,
}: {
  layer: ItineraryLayer
  onSave: (input: { name?: string; color?: string }) => void
  onCancel: () => void
}) {
  const [name, setName] = useState(layer.name)
  const [color, setColor] = useState(layer.color)

  const save = () => {
    const input: { name?: string; color?: string } = {}
    if (name.trim() && name.trim() !== layer.name) input.name = name.trim()
    if (color !== layer.color) input.color = color
    if (input.name || input.color) onSave(input)
    else onCancel()
  }

  return (
    <form
      className="flex flex-wrap items-center gap-3 px-4 py-2.5"
      onSubmit={(e) => {
        e.preventDefault()
        save()
      }}
    >
      <input
        autoFocus
        value={name}
        onChange={(e) => setName(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === 'Escape') onCancel()
        }}
        aria-label="Layer name"
        className="w-40 rounded-lg border border-slate-300 dark:border-slate-600 bg-transparent px-2 py-1 text-sm text-slate-900 dark:text-slate-100 focus:border-slate-500 focus:outline-none"
      />
      <div className="flex items-center gap-1.5">
        {LAYER_SWATCHES.map((swatch) => (
          <button
            key={swatch}
            type="button"
            onClick={() => setColor(swatch)}
            className={`h-5 w-5 rounded-full ${
              color === swatch ? 'ring-2 ring-slate-900 dark:ring-slate-100 ring-offset-1' : ''
            }`}
            style={{ backgroundColor: swatch }}
            aria-label={`Set color ${swatch}`}
          />
        ))}
        <input
          type="color"
          value={color}
          onChange={(e) => setColor(e.target.value)}
          aria-label="Custom color"
          className="h-6 w-8 cursor-pointer rounded border border-slate-200 dark:border-slate-700 bg-transparent"
        />
      </div>
      <div className="ml-auto flex items-center gap-2">
        <button
          type="submit"
          className="rounded-lg bg-slate-900 dark:bg-slate-100 px-3 py-1 text-sm text-white dark:text-slate-900"
        >
          Save
        </button>
        <button
          type="button"
          onClick={onCancel}
          className="text-sm text-slate-400 dark:text-slate-500 hover:text-slate-900 dark:hover:text-slate-100"
        >
          Cancel
        </button>
      </div>
    </form>
  )
}

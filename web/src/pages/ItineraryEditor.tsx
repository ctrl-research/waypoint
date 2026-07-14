import { Suspense, lazy, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Link, Navigate, useParams } from '@tanstack/react-router'
import { fetchMe, getTrip } from '../api'
import { ItineraryBoard } from './ItineraryBoard'
import { NewItemForm } from './TripDetail'

const TripMap = lazy(() => import('../TripMap').then((m) => ({ default: m.TripMap })))
type MarkerKey = `stop:${string}` | `item:${string}`

/**
 * Dedicated itinerary editor (#73, slice 1): the editable board and item
 * form live here; the trip page shows the read-only Final-layer overview.
 * Proposal layers and promotion arrive in slice 2.
 */
export function ItineraryEditorPage() {
  const { tripId } = useParams({ from: '/trips/$tripId/itinerary' })
  const { data: me, isLoading } = useQuery({ queryKey: ['me'], queryFn: fetchMe })
  const detail = useQuery({ queryKey: ['trip', tripId], queryFn: () => getTrip(tripId), enabled: !!me })
  const [highlightKey, setHighlightKey] = useState<MarkerKey | null>(null)

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
  const canEdit = trip.role !== 'viewer'

  return (
    <div className="mx-auto mt-8 w-full max-w-5xl px-4 pb-24">
      <Link
        to="/trips/$tripId"
        params={{ tripId }}
        className="text-sm text-slate-500 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-100"
      >
        ← {trip.title}
      </Link>
      <div className="mt-2 flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-semibold text-slate-900 dark:text-slate-100">
            Itinerary editor
          </h1>
          <p className="text-sm text-slate-500 dark:text-slate-400">
            {canEdit
              ? 'Drag to reorder or move days; changes publish straight to the trip.'
              : 'You have view-only access to this trip.'}
          </p>
        </div>
      </div>

      <div className="mt-6">
        <Suspense
          fallback={<div className="h-80 w-full rounded-xl border border-slate-200 dark:border-slate-700 bg-slate-50 dark:bg-slate-950" />}
        >
          <TripMap
            stops={stops}
            items={items}
            highlightKey={highlightKey}
            picking={false}
            onPick={() => {}}
          />
        </Suspense>
      </div>

      <div className="mt-6">
        <ItineraryBoard
          trip={trip}
          items={items}
          stops={stops}
          homes={homes}
          readOnly={!canEdit}
          onHover={setHighlightKey}
        />
        {canEdit && (
          <div className="mt-4">
            <NewItemForm trip={trip} stops={stops} />
          </div>
        )}
      </div>
    </div>
  )
}

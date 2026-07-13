import { useEffect } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Navigate, useParams } from '@tanstack/react-router'
import Markdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import {
  fetchMe,
  getTrip,
  listJournal,
  type ItineraryItem,
  type Stop,
  type TripHome,
} from '../api'
import { formatRange } from './Home'
import { categoryIcons } from './ItineraryBoard'

/**
 * Print-optimized trip document (#50): the plan and journal as one clean
 * page for the browser's Print → Save as PDF. Everything the browser can
 * render (CJK names, photos) prints faithfully — no server-side PDF pipeline.
 */
export function PrintTripPage() {
  const { tripId } = useParams({ from: '/trips/$tripId/print' })
  const { data: me, isLoading } = useQuery({ queryKey: ['me'], queryFn: fetchMe })
  const detail = useQuery({ queryKey: ['trip', tripId], queryFn: () => getTrip(tripId), enabled: !!me })
  const journal = useQuery({
    queryKey: ['journal', tripId],
    queryFn: () => listJournal(tripId),
    enabled: !!me,
  })

  // Print in light theme regardless of the app setting.
  useEffect(() => {
    const wasDark = document.documentElement.classList.contains('dark')
    document.documentElement.classList.remove('dark')
    return () => {
      if (wasDark) document.documentElement.classList.add('dark')
    }
  }, [])

  if (isLoading) return null
  if (!me) return <Navigate to="/login" />
  if (!detail.data || !journal.data) return null

  const { trip, stops, items, homes } = detail.data
  const entries = journal.data
  const days = [...new Set([...items.map((i) => i.day), ...entries.map((e) => e.entryDate)])].sort()

  return (
    <div className="mx-auto w-full max-w-3xl px-6 py-8 text-slate-900">
      <div className="mb-6 flex items-center justify-between print:hidden">
        <p className="text-sm text-slate-500">
          Use your browser’s print dialog to save this as a PDF for offline use.
        </p>
        <button
          type="button"
          onClick={() => window.print()}
          className="rounded-lg bg-slate-900 px-4 py-2 text-sm font-medium text-white hover:bg-slate-700"
        >
          🖨 Print / save PDF
        </button>
      </div>

      <header>
        <h1 className="text-3xl font-semibold">{trip.title}</h1>
        <p className="mt-1 text-slate-600">
          {formatRange(trip.startDate, trip.endDate)} · {trip.status}
        </p>
        {trip.description && <p className="mt-2 text-slate-700">{trip.description}</p>}
      </header>

      {stops.length > 0 && (
        <section className="mt-8 break-inside-avoid">
          <h2 className="border-b border-slate-300 pb-1 text-lg font-semibold">Stops</h2>
          <ol className="mt-3 space-y-1">
            {stops.map((stop, i) => (
              <li key={stop.id} className="text-sm">
                <span className="mr-2 font-medium">{i + 1}.</span>
                {stop.name}
                {(stop.arrivalDate || stop.departureDate) && (
                  <span className="text-slate-500"> — {formatRange(stop.arrivalDate, stop.departureDate)}</span>
                )}
                {stop.notes && <span className="text-slate-500"> · {stop.notes}</span>}
              </li>
            ))}
          </ol>
        </section>
      )}

      {days.length > 0 && (
        <section className="mt-8">
          <h2 className="border-b border-slate-300 pb-1 text-lg font-semibold">Day by day</h2>
          {days.map((day) => {
            const dayItems = items.filter((i) => i.day === day)
            const dayEntries = entries.filter((e) => e.entryDate === day)
            return (
              <div key={day} className="mt-4 break-inside-avoid">
                <h3 className="font-medium">
                  {new Date(day + 'T00:00:00').toLocaleDateString(undefined, {
                    weekday: 'long',
                    month: 'long',
                    day: 'numeric',
                    year: 'numeric',
                  })}
                </h3>
                {dayItems.length > 0 && (
                  <ul className="mt-1 space-y-0.5">
                    {dayItems.map((item) => (
                      <li key={item.id} className="text-sm text-slate-700">
                        {categoryIcons[item.category]}{' '}
                        {item.startTime && (
                          <span className="tabular-nums">
                            {item.startTime}
                            {item.endTime && `–${item.endTime}`}{' '}
                          </span>
                        )}
                        <span className="font-medium text-slate-900">{item.title}</span>
                        {printRoute(item, stops, homes) && (
                          <span className="text-slate-500"> · {printRoute(item, stops, homes)}</span>
                        )}
                        {item.notes && <span className="text-slate-500"> — {item.notes}</span>}
                      </li>
                    ))}
                  </ul>
                )}
                {dayEntries.map((entry) => (
                  <article key={entry.id} className="mt-3 rounded-lg border border-slate-200 p-4">
                    {entry.title && <h4 className="font-medium">{entry.title}</h4>}
                    {entry.body && (
                      <div className="mt-1 text-sm text-slate-700 [&_li]:ml-4 [&_li]:list-disc [&_p]:mt-1 first:[&_p]:mt-0">
                        <Markdown remarkPlugins={[remarkGfm]}>{entry.body}</Markdown>
                      </div>
                    )}
                    {entry.photos.length > 0 && (
                      <div className="mt-2 flex flex-wrap gap-2">
                        {entry.photos.map((photo) => (
                          <img
                            key={photo.id}
                            src={photo.url}
                            alt={photo.caption}
                            className="h-28 w-28 rounded object-cover"
                          />
                        ))}
                      </div>
                    )}
                  </article>
                ))}
              </div>
            )
          })}
        </section>
      )}

      <p className="mt-10 text-center text-xs text-slate-400">
        {trip.title} · exported from 🧭 Waypoint
      </p>
    </div>
  )
}

function printRoute(item: ItineraryItem, stops: Stop[], homes: TripHome[]): string | undefined {
  const stopName = (id: string | null) => stops.find((s) => s.id === id)?.name
  const homeName = (id: string | null) => {
    const h = id && homes.find((h) => h.id === id)
    return h ? `(home) ${h.name}` : undefined
  }
  const from = homeName(item.originHomeId) ?? stopName(item.stopId)
  if (item.category !== 'flight' && item.category !== 'train') {
    return from ? `@ ${from}` : undefined
  }
  const to = homeName(item.destinationHomeId) ?? stopName(item.destinationStopId)
  if (from && to) return `${from} → ${to}`
  return from ?? (to && `→ ${to}`) ?? undefined
}

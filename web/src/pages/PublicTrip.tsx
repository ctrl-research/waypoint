import { Suspense, lazy } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useParams } from '@tanstack/react-router'
import Markdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import { fetchPublicTrip, type ItineraryItem } from '../api'
import { formatRange, statusStyles } from './Home'
import { categoryIcons } from './ItineraryBoard'

const TripMap = lazy(() => import('../TripMap').then((m) => ({ default: m.TripMap })))

/** Read-only public trip view behind a share token (#24). No session. */
export function PublicTripPage() {
  const { token } = useParams({ from: '/share/$token' })
  const { data, error } = useQuery({
    queryKey: ['public', token],
    queryFn: () => fetchPublicTrip(token),
    retry: 1,
  })

  if (error) {
    return (
      <div className="mx-auto mt-24 max-w-md text-center text-slate-500 dark:text-slate-400">
        <p className="text-4xl">🧭</p>
        <p className="mt-3">This share link doesn’t exist or has been revoked.</p>
      </div>
    )
  }
  if (!data) return null

  const { trip, stops, items, entries, tileUrl } = data
  const stopName = (id: string | null) => stops.find((s) => s.id === id)?.name
  const itemsByDay = new Map<string, ItineraryItem[]>()
  for (const item of items) {
    itemsByDay.set(item.day, [...(itemsByDay.get(item.day) ?? []), item])
  }
  const days = [...new Set([...items.map((i) => i.day), ...entries.map((e) => e.entryDate)])].sort()

  return (
    <div className="mx-auto mt-8 w-full max-w-4xl px-4 pb-24">
      <div className="flex items-center gap-3">
        <h1 className="text-2xl font-semibold text-slate-900 dark:text-slate-100">{trip.title}</h1>
        <span className={`rounded-full px-2 py-0.5 text-xs font-medium ${statusStyles[trip.status]}`}>
          {trip.status}
        </span>
      </div>
      <p className="mt-1 text-sm text-slate-500 dark:text-slate-400">{formatRange(trip.startDate, trip.endDate)}</p>
      {trip.description && <p className="mt-2 text-slate-600 dark:text-slate-400">{trip.description}</p>}

      <div className="mt-6">
        <Suspense fallback={<div className="h-80 w-full rounded-xl border border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-950" />}>
          <TripMap stops={stops} picking={false} onPick={() => {}} tileUrl={tileUrl} />
        </Suspense>
      </div>

      {stops.length > 0 && (
        <section className="mt-8">
          <h2 className="text-lg font-semibold text-slate-900 dark:text-slate-100">Stops</h2>
          <ol className="mt-3 space-y-1">
            {stops.map((stop, i) => (
              <li key={stop.id} className="flex items-center gap-3 text-sm text-slate-700 dark:text-slate-300">
                <span className="flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-slate-100 dark:bg-slate-800 text-xs font-medium text-slate-600 dark:text-slate-400">
                  {i + 1}
                </span>
                {stop.name}
                {(stop.arrivalDate || stop.departureDate) && (
                  <span className="text-xs text-slate-400 dark:text-slate-500">
                    {formatRange(stop.arrivalDate, stop.departureDate)}
                  </span>
                )}
              </li>
            ))}
          </ol>
        </section>
      )}

      {days.length > 0 && (
        <section className="mt-8 space-y-6">
          <h2 className="text-lg font-semibold text-slate-900 dark:text-slate-100">Day by day</h2>
          {days.map((day) => {
            const dayItems = itemsByDay.get(day) ?? []
            const dayEntries = entries.filter((e) => e.entryDate === day)
            return (
              <div key={day} className="relative border-l-2 border-slate-200 dark:border-slate-800 pl-6">
                <div className="absolute -left-[7px] top-1 h-3 w-3 rounded-full bg-slate-400 dark:bg-slate-500" />
                <h3 className="text-sm font-semibold text-slate-900 dark:text-slate-100">
                  {new Date(day + 'T00:00:00').toLocaleDateString(undefined, {
                    weekday: 'long',
                    month: 'long',
                    day: 'numeric',
                    year: 'numeric',
                  })}
                </h3>
                {dayItems.length > 0 && (
                  <ul className="mt-2 space-y-0.5">
                    {dayItems.map((item) => (
                      <li key={item.id} className="flex items-center gap-2 text-sm text-slate-500 dark:text-slate-400">
                        <span>{categoryIcons[item.category]}</span>
                        {item.startTime && <span className="tabular-nums">{item.startTime}</span>}
                        <span>{item.title}</span>
                        {stopName(item.stopId) && (
                          <span className="text-xs">@ {stopName(item.stopId)}</span>
                        )}
                      </li>
                    ))}
                  </ul>
                )}
                <div className="mt-3 space-y-4">
                  {dayEntries.map((entry) => (
                    <article key={entry.id} className="rounded-xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 p-5 shadow-sm">
                      {entry.title && <h4 className="font-medium text-slate-900 dark:text-slate-100">{entry.title}</h4>}
                      {entry.body && (
                        <div className="mt-2 max-w-none text-sm text-slate-700 dark:text-slate-300 [&_a]:underline [&_img]:max-h-80 [&_img]:rounded-lg [&_li]:ml-4 [&_li]:list-disc [&_p]:mt-2 first:[&_p]:mt-0">
                          <Markdown remarkPlugins={[remarkGfm]}>{entry.body}</Markdown>
                        </div>
                      )}
                      {entry.photos.length > 0 && (
                        <div className="mt-3 flex flex-wrap gap-2">
                          {entry.photos.map((photo) => (
                            <a key={photo.id} href={photo.url} target="_blank" rel="noreferrer">
                              <img
                                src={photo.url}
                                alt={photo.caption}
                                className="h-32 w-32 rounded-lg border border-slate-200 dark:border-slate-800 object-cover"
                                loading="lazy"
                              />
                            </a>
                          ))}
                        </div>
                      )}
                    </article>
                  ))}
                </div>
              </div>
            )
          })}
        </section>
      )}

      <p className="mt-12 text-center text-xs text-slate-400 dark:text-slate-500">
        Shared read-only from a self-hosted 🧭 Waypoint
      </p>
    </div>
  )
}

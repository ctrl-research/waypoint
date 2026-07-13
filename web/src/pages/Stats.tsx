import { Suspense, lazy, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Navigate } from '@tanstack/react-router'
import { fetchMe, fetchStats } from '../api'
import type { StatsMapMode, StatsMapProjection, VisitedPlaces } from '../StatsMap'

const StatsMap = lazy(() => import('../StatsMap').then((m) => ({ default: m.StatsMap })))

// Same validated hue as the map fill (dataviz palette slot 1).
const DATA_HUE = '#2a78d6'

export function StatsPage() {
  const { data: me, isLoading } = useQuery({ queryKey: ['me'], queryFn: fetchMe })
  const stats = useQuery({ queryKey: ['stats'], queryFn: fetchStats, enabled: !!me })
  const [mode, setMode] = useState<StatsMapMode>('countries')
  const [projection, setProjection] = useState<StatsMapProjection>('mercator')
  const [visited, setVisited] = useState<VisitedPlaces>({ countries: [], continents: [], countryTotal: 0 })

  if (isLoading) return null
  if (!me) return <Navigate to="/login" />
  if (!stats.data) return null

  const { totals, flights, trains, tripsPerYear, stops } = stats.data
  const maxYearCount = Math.max(1, ...tripsPerYear.map((y) => y.count))

  return (
    <div className="mx-auto mt-8 w-full max-w-5xl px-4 pb-24">
      <h1 className="text-2xl font-semibold text-slate-900 dark:text-slate-100">Your travels</h1>
      <p className="text-sm text-slate-500 dark:text-slate-400">Across every trip you own or share.</p>

      <div className="mt-6 grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-5">
        <StatTile label="Trips" value={totals.trips} hint={tileHint(totals)} />
        <StatTile label="Continents" value={visited.continents.length} hint="of 7 continents" />
        <StatTile
          label="Countries"
          value={visited.countries.length}
          hint={`of ${visited.countryTotal || '…'} countries`}
        />
        <StatTile label="Cities" value={totals.cities} hint="distinct stops" />
        <StatTile label="Days on the road" value={totals.daysOnRoad} hint="dated trips" />
        <StatTile
          label="Planned distance"
          value={`${totals.plannedDistanceKm.toLocaleString()} km`}
          hint="between stops, as the crow flies"
        />
        <StatTile label="Flights" value={flights.count} hint="itinerary ✈️ legs" />
        <StatTile
          label="Flight distance"
          value={`${flights.distanceKm.toLocaleString()} km`}
          hint="great-circle"
        />
        <StatTile label="Time in the air" value={formatMinutes(flights.minutes)} hint="departure to arrival" />
        <StatTile label="Trains" value={trains.count} hint="itinerary 🚆 legs" />
        <StatTile
          label="Train distance"
          value={`${trains.distanceKm.toLocaleString()} km`}
          hint="great-circle"
        />
        <StatTile label="Time on rails" value={formatMinutes(trains.minutes)} hint="departure to arrival" />
      </div>

      {tripsPerYear.length > 0 && (
        <section className="mt-8">
          <h2 className="text-lg font-semibold text-slate-900 dark:text-slate-100">Trips per year</h2>
          <div className="mt-3 space-y-1.5">
            {tripsPerYear.map(({ year, count }) => (
              <div key={year} className="flex items-center gap-3 text-sm">
                <span className="w-12 shrink-0 tabular-nums text-slate-500 dark:text-slate-400">{year}</span>
                <div className="h-4 flex-1">
                  <div
                    className="flex h-4 items-center rounded-r"
                    style={{ width: `${(count / maxYearCount) * 100}%`, backgroundColor: DATA_HUE }}
                    title={`${count} trip${count === 1 ? '' : 's'} in ${year}`}
                  />
                </div>
                <span className="w-6 shrink-0 tabular-nums text-slate-700 dark:text-slate-300">{count}</span>
              </div>
            ))}
          </div>
        </section>
      )}

      <section className="mt-8">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <h2 className="text-lg font-semibold text-slate-900 dark:text-slate-100">Where you’ve been</h2>
          <div className="flex gap-2">
            <Segmented
              value={mode}
              options={[
                ['continents', 'Continents'],
                ['countries', 'Countries'],
                ['cities', 'Cities'],
              ]}
              onChange={(v) => setMode(v as StatsMapMode)}
            />
            <Segmented
              value={projection}
              options={[
                ['mercator', '2D'],
                ['globe', 'Globe'],
              ]}
              onChange={(v) => setProjection(v as StatsMapProjection)}
            />
          </div>
        </div>
        <div className="mt-3">
          <Suspense fallback={<div className="h-[28rem] w-full rounded-xl border border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-950" />}>
            <StatsMap stops={stops} mode={mode} projection={projection} onVisited={setVisited} />
          </Suspense>
        </div>
        {mode === 'countries' && visited.countries.length > 0 && (
          <p className="mt-2 text-sm text-slate-500 dark:text-slate-400">
            <span className="font-medium text-slate-700 dark:text-slate-300">{visited.countries.length} countries:</span>{' '}
            {visited.countries.join(', ')}
          </p>
        )}
        {mode === 'continents' && visited.continents.length > 0 && (
          <p className="mt-2 text-sm text-slate-500 dark:text-slate-400">
            <span className="font-medium text-slate-700 dark:text-slate-300">
              {visited.continents.length} of 7 continents:
            </span>{' '}
            {visited.continents.join(', ')}
          </p>
        )}
        {stops.length === 0 && (
          <p className="mt-2 text-sm text-slate-400 dark:text-slate-500">
            Add located stops to your trips and they’ll light up here.
          </p>
        )}
      </section>
    </div>
  )
}

function formatMinutes(minutes: number): string {
  if (minutes === 0) return '0h'
  const h = Math.floor(minutes / 60)
  const m = minutes % 60
  return m ? `${h}h ${m}m` : `${h}h`
}

function tileHint(totals: { planning: number; active: number; completed: number }): string {
  const parts = []
  if (totals.active) parts.push(`${totals.active} active`)
  if (totals.planning) parts.push(`${totals.planning} planning`)
  if (totals.completed) parts.push(`${totals.completed} done`)
  return parts.join(' · ') || '—'
}

function StatTile({ label, value, hint }: { label: string; value: number | string; hint: string }) {
  return (
    <div className="rounded-xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 p-4">
      <p className="text-xs font-medium uppercase tracking-wide text-slate-400 dark:text-slate-500">{label}</p>
      <p className="mt-1 text-2xl font-semibold tabular-nums text-slate-900 dark:text-slate-100">{value}</p>
      <p className="mt-0.5 truncate text-xs text-slate-400 dark:text-slate-500" title={hint}>
        {hint}
      </p>
    </div>
  )
}

function Segmented({
  value,
  options,
  onChange,
}: {
  value: string
  options: readonly (readonly [string, string])[]
  onChange: (v: string) => void
}) {
  return (
    <div className="flex rounded-lg border border-slate-300 dark:border-slate-600 p-0.5">
      {options.map(([v, label]) => (
        <button
          key={v}
          type="button"
          onClick={() => onChange(v)}
          className={`rounded-md px-3 py-1 text-sm ${
            value === v ? 'bg-slate-900 dark:bg-slate-100 text-white dark:text-slate-900' : 'text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-100'
          }`}
        >
          {label}
        </button>
      ))}
    </div>
  )
}

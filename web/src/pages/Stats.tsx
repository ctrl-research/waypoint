import { Suspense, lazy, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Navigate } from '@tanstack/react-router'
import { fetchMe, fetchStats } from '../api'
import type { StatsMapMode, StatsMapProjection } from '../StatsMap'

const StatsMap = lazy(() => import('../StatsMap').then((m) => ({ default: m.StatsMap })))

// Same validated hue as the map fill (dataviz palette slot 1).
const DATA_HUE = '#2a78d6'

export function StatsPage() {
  const { data: me, isLoading } = useQuery({ queryKey: ['me'], queryFn: fetchMe })
  const stats = useQuery({ queryKey: ['stats'], queryFn: fetchStats, enabled: !!me })
  const [mode, setMode] = useState<StatsMapMode>('countries')
  const [projection, setProjection] = useState<StatsMapProjection>('mercator')
  const [visitedCountries, setVisitedCountries] = useState<string[]>([])

  if (isLoading) return null
  if (!me) return <Navigate to="/login" />
  if (!stats.data) return null

  const { totals, tripsPerYear, stops } = stats.data
  const maxYearCount = Math.max(1, ...tripsPerYear.map((y) => y.count))

  return (
    <div className="mx-auto mt-8 w-full max-w-5xl px-4 pb-24">
      <h1 className="text-2xl font-semibold text-slate-900">Your travels</h1>
      <p className="text-sm text-slate-500">Across every trip you own or share.</p>

      <div className="mt-6 grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-5">
        <StatTile label="Trips" value={totals.trips} hint={tileHint(totals)} />
        <StatTile label="Countries" value={visitedCountries.length} hint="from stop locations" />
        <StatTile label="Cities" value={totals.cities} hint="distinct stops" />
        <StatTile label="Days on the road" value={totals.daysOnRoad} hint="dated trips" />
        <StatTile
          label="Planned distance"
          value={`${totals.plannedDistanceKm.toLocaleString()} km`}
          hint="between stops, as the crow flies"
        />
      </div>

      {tripsPerYear.length > 0 && (
        <section className="mt-8">
          <h2 className="text-lg font-semibold text-slate-900">Trips per year</h2>
          <div className="mt-3 space-y-1.5">
            {tripsPerYear.map(({ year, count }) => (
              <div key={year} className="flex items-center gap-3 text-sm">
                <span className="w-12 shrink-0 tabular-nums text-slate-500">{year}</span>
                <div className="h-4 flex-1">
                  <div
                    className="flex h-4 items-center rounded-r"
                    style={{ width: `${(count / maxYearCount) * 100}%`, backgroundColor: DATA_HUE }}
                    title={`${count} trip${count === 1 ? '' : 's'} in ${year}`}
                  />
                </div>
                <span className="w-6 shrink-0 tabular-nums text-slate-700">{count}</span>
              </div>
            ))}
          </div>
        </section>
      )}

      <section className="mt-8">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <h2 className="text-lg font-semibold text-slate-900">Where you’ve been</h2>
          <div className="flex gap-2">
            <Segmented
              value={mode}
              options={[
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
          <Suspense fallback={<div className="h-[28rem] w-full rounded-xl border border-slate-200 bg-slate-50" />}>
            <StatsMap
              stops={stops}
              mode={mode}
              projection={projection}
              onVisitedCountries={setVisitedCountries}
            />
          </Suspense>
        </div>
        {mode === 'countries' && visitedCountries.length > 0 && (
          <p className="mt-2 text-sm text-slate-500">
            <span className="font-medium text-slate-700">{visitedCountries.length} countries:</span>{' '}
            {visitedCountries.join(', ')}
          </p>
        )}
        {stops.length === 0 && (
          <p className="mt-2 text-sm text-slate-400">
            Add located stops to your trips and they’ll light up here.
          </p>
        )}
      </section>
    </div>
  )
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
    <div className="rounded-xl border border-slate-200 bg-white p-4">
      <p className="text-xs font-medium uppercase tracking-wide text-slate-400">{label}</p>
      <p className="mt-1 text-2xl font-semibold tabular-nums text-slate-900">{value}</p>
      <p className="mt-0.5 truncate text-xs text-slate-400" title={hint}>
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
    <div className="flex rounded-lg border border-slate-300 p-0.5">
      {options.map(([v, label]) => (
        <button
          key={v}
          type="button"
          onClick={() => onChange(v)}
          className={`rounded-md px-3 py-1 text-sm ${
            value === v ? 'bg-slate-900 text-white' : 'text-slate-600 hover:text-slate-900'
          }`}
        >
          {label}
        </button>
      ))}
    </div>
  )
}

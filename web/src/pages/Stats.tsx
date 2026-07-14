import { Suspense, lazy, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Navigate } from '@tanstack/react-router'
import { fetchMe, fetchStats } from '../api'
import type { StatsMapMode, StatsMapProjection, VisitedPlaces } from '../StatsMap'

const StatsMap = lazy(() => import('../StatsMap').then((m) => ({ default: m.StatsMap })))

// Travelled/planned hues (#53) — validated as a pair on the light surface.
const TRAVELLED = '#059669'
const PLANNED = '#d97706'

export function StatsPage() {
  const { data: me, isLoading } = useQuery({ queryKey: ['me'], queryFn: fetchMe })
  const stats = useQuery({ queryKey: ['stats'], queryFn: fetchStats, enabled: !!me })
  const [mode, setMode] = useState<StatsMapMode>('countries')
  const [projection, setProjection] = useState<StatsMapProjection>('mercator')
  const [visited, setVisited] = useState<VisitedPlaces>({
    countries: [],
    plannedCountries: [],
    continents: [],
    plannedContinents: [],
    countryTotal: 0,
  })

  if (isLoading) return null
  if (!me) return <Navigate to="/login" />
  if (!stats.data) return null

  const { totals, flights, trains, tripsPerYear, stops } = stats.data

  return (
    <div className="mx-auto mt-8 w-full max-w-5xl px-4 pb-24">
      <h1 className="text-2xl font-semibold text-slate-900 dark:text-slate-100">Your travels</h1>
      <p className="text-sm text-slate-500 dark:text-slate-400">
        Across every trip you own or share. <span style={{ color: TRAVELLED }}>Green</span> is where
        you’ve been; <span style={{ color: PLANNED }}>+N</span> is still ahead.
      </p>

      <section className="mt-6">
        <h2 className="text-sm font-semibold uppercase tracking-wide text-slate-500 dark:text-slate-400">Trips</h2>
        <div className="mt-2 grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-6">
          <StatTile label="Trips" value={totals.trips} hint={tileHint(totals)} />
          <StatTile
            label="Continents"
            value={visited.continents.length}
            plus={visited.plannedContinents.length}
            hint="of 7 continents"
            travelled
          />
          <StatTile
            label="Countries"
            value={visited.countries.length}
            plus={visited.plannedCountries.length}
            hint={`of ${visited.countryTotal || '…'} countries`}
            travelled
          />
          <StatTile label="Cities" value={totals.cities} plus={totals.citiesPlanned} travelled />
          <StatTile
            label="Days on the road"
            value={totals.daysOnRoad}
            plus={totals.daysOnRoadPlanned}
            travelled
          />
          <StatTile
            label="Traveled distance"
            value={`${totals.traveledDistanceKm.toLocaleString()} km`}
            plus={totals.plannedDistanceKm ? `${totals.plannedDistanceKm.toLocaleString()} km` : 0}
            travelled
          />
        </div>
      </section>

      <section className="mt-6">
        <h2 className="text-sm font-semibold uppercase tracking-wide text-slate-500 dark:text-slate-400">Transportation</h2>
        <div className="mt-2 grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-6">
          <StatTile label="Flights" value={flights.count} />
          <StatTile label="Flight distance" value={`${flights.distanceKm.toLocaleString()} km`} />
          <StatTile label="Time in the air" value={formatMinutes(flights.minutes)} />
          <StatTile label="Trains" value={trains.count} />
          <StatTile label="Train distance" value={`${trains.distanceKm.toLocaleString()} km`} />
          <StatTile label="Time on rails" value={formatMinutes(trains.minutes)} />
        </div>
      </section>

      {tripsPerYear.length > 0 && <TripsPerYear data={tripsPerYear} />}

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
        {mode === 'countries' && (visited.countries.length > 0 || visited.plannedCountries.length > 0) && (
          <p className="mt-2 text-sm text-slate-500 dark:text-slate-400">
            {visited.countries.length > 0 && (
              <>
                <span className="font-medium" style={{ color: TRAVELLED }}>
                  {visited.countries.length} visited:
                </span>{' '}
                {visited.countries.join(', ')}
              </>
            )}
            {visited.plannedCountries.length > 0 && (
              <>
                {visited.countries.length > 0 && ' · '}
                <span className="font-medium" style={{ color: PLANNED }}>
                  {visited.plannedCountries.length} planned:
                </span>{' '}
                {visited.plannedCountries.join(', ')}
              </>
            )}
          </p>
        )}
        {mode === 'continents' && (visited.continents.length > 0 || visited.plannedContinents.length > 0) && (
          <p className="mt-2 text-sm text-slate-500 dark:text-slate-400">
            {visited.continents.length > 0 && (
              <>
                <span className="font-medium" style={{ color: TRAVELLED }}>
                  {visited.continents.length} of 7 continents:
                </span>{' '}
                {visited.continents.join(', ')}
              </>
            )}
            {visited.plannedContinents.length > 0 && (
              <>
                {visited.continents.length > 0 && ' · '}
                <span className="font-medium" style={{ color: PLANNED }}>
                  {visited.plannedContinents.length} planned:
                </span>{' '}
                {visited.plannedContinents.join(', ')}
              </>
            )}
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

/** Vertical trips-per-year bars: travelled stacks below planned (#64). */
function TripsPerYear({ data }: { data: { year: number; travelled: number; planned: number }[] }) {
  const max = Math.max(1, ...data.map((y) => y.travelled + y.planned))
  return (
    <section className="mt-8">
      <h2 className="text-sm font-semibold uppercase tracking-wide text-slate-500 dark:text-slate-400">Trips per year</h2>
      <div className="mt-3 flex items-end gap-3 overflow-x-auto pb-1">
        {data.map(({ year, travelled, planned }) => {
          const total = travelled + planned
          return (
            <div key={year} className="flex w-10 shrink-0 flex-col items-center gap-1">
              <span className="text-xs tabular-nums text-slate-700 dark:text-slate-300">{total}</span>
              <div
                className="flex h-28 w-6 flex-col-reverse gap-0.5"
                title={`${year}: ${travelled} travelled${planned ? `, ${planned} planned` : ''}`}
              >
                {travelled > 0 && (
                  <div
                    className="w-full rounded-t"
                    style={{ height: `${(travelled / max) * 100}%`, backgroundColor: TRAVELLED }}
                  />
                )}
                {planned > 0 && (
                  <div
                    className="w-full rounded-t"
                    style={{ height: `${(planned / max) * 100}%`, backgroundColor: PLANNED }}
                  />
                )}
              </div>
              <span className="text-xs tabular-nums text-slate-500 dark:text-slate-400">{year}</span>
            </div>
          )
        })}
      </div>
    </section>
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

function StatTile({
  label,
  value,
  plus,
  hint,
  travelled = false,
}: {
  label: string
  value: number | string
  /** Still-planned remainder, shown as an amber "+N" (#53). */
  plus?: number | string
  hint?: string
  travelled?: boolean
}) {
  const showPlus = typeof plus === 'string' ? true : (plus ?? 0) > 0
  return (
    <div className="rounded-xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 p-4">
      <p className="text-xs font-medium uppercase tracking-wide text-slate-400 dark:text-slate-500">{label}</p>
      <p className="mt-1 text-2xl font-semibold tabular-nums" style={travelled ? { color: TRAVELLED } : undefined}>
        <span className={travelled ? '' : 'text-slate-900 dark:text-slate-100'}>{value}</span>
        {showPlus && (
          <span className="ml-1 text-sm font-semibold" style={{ color: PLANNED }}>
            +{plus}
          </span>
        )}
      </p>
      {hint && (
        <p className="mt-0.5 truncate text-xs text-slate-400 dark:text-slate-500" title={hint}>
          {hint}
        </p>
      )}
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

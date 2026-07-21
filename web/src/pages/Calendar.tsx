import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Link, Navigate, useNavigate } from '@tanstack/react-router'
import { fetchMe, listTrips, type Trip } from '../api'
import { statusStyles } from './Home'

/** Lanes per week row; busier weeks overflow into a "+N more" note. */
const MAX_LANES = 3

/**
 * Consolidated calendar (#52): every dated trip as a span bar on a month
 * grid, colored by its effective status. Weeks start on Monday.
 */
export function CalendarPage() {
  const { data: me, isLoading } = useQuery({ queryKey: ['me'], queryFn: fetchMe })
  const trips = useQuery({ queryKey: ['trips'], queryFn: listTrips, enabled: !!me })
  const [month, setMonth] = useState(() => {
    const d = new Date()
    return new Date(d.getFullYear(), d.getMonth(), 1)
  })

  if (isLoading) return null
  if (!me) return <Navigate to="/login" />

  const dated = (trips.data ?? []).filter((t) => t.startDate)
  const undated = (trips.data ?? []).length - dated.length
  const weeks = monthWeeks(month)
  const today = localISO(new Date())

  return (
    <div className="mx-auto mt-8 w-full max-w-5xl px-4 pb-24">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h1 className="text-2xl font-semibold text-slate-900 dark:text-slate-100">Calendar</h1>
        <div className="flex items-center gap-2">
          <button type="button" onClick={() => setMonth(addMonths(month, -1))} aria-label="Previous month" className={navBtn}>
            ←
          </button>
          <button
            type="button"
            onClick={() => {
              const d = new Date()
              setMonth(new Date(d.getFullYear(), d.getMonth(), 1))
            }}
            className={navBtn}
          >
            Today
          </button>
          <button type="button" onClick={() => setMonth(addMonths(month, 1))} aria-label="Next month" className={navBtn}>
            →
          </button>
        </div>
      </div>
      <p className="mt-1 text-lg font-medium text-slate-700 dark:text-slate-300">
        {month.toLocaleDateString(undefined, { month: 'long', year: 'numeric' })}
      </p>

      <div className="mt-4 overflow-x-auto" data-tour="calendar-grid">
        <div className="min-w-[40rem] overflow-hidden rounded-xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900">
        <div className="grid grid-cols-7 border-b border-slate-200 dark:border-slate-800 text-center text-xs font-medium text-slate-400 dark:text-slate-500">
          {['Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat', 'Sun'].map((d) => (
            <div key={d} className="py-1.5">
              {d}
            </div>
          ))}
        </div>
        {weeks.map((week) => (
          <WeekRow key={week[0]} days={week} month={month} trips={dated} today={today} />
        ))}
        </div>
      </div>

      {undated > 0 && (
        <p className="mt-2 text-sm text-slate-400 dark:text-slate-500">
          {undated} undated trip{undated === 1 ? '' : 's'} not shown —{' '}
          <Link to="/" className="underline">
            see the trip list
          </Link>
          .
        </p>
      )}
    </div>
  )
}

const navBtn =
  'rounded-lg border border-slate-300 dark:border-slate-600 px-3 py-1.5 text-sm text-slate-600 dark:text-slate-400 hover:bg-slate-50 dark:hover:bg-slate-800'

function WeekRow({
  days,
  month,
  trips,
  today,
}: {
  days: string[]
  month: Date
  trips: Trip[]
  today: string
}) {
  const navigate = useNavigate()
  const weekStart = days[0]
  const weekEnd = days[6]

  // Trips overlapping this week, earliest first, greedily packed into lanes.
  const segments: { trip: Trip; startCol: number; span: number; lane: number }[] = []
  const laneEnds: string[] = []
  const overflow = new Map<string, number>()
  for (const trip of [...trips].sort((a, b) => a.startDate!.localeCompare(b.startDate!))) {
    const start = trip.startDate!
    const end = trip.endDate && trip.endDate >= start ? trip.endDate : start
    if (end < weekStart || start > weekEnd) continue
    const from = start < weekStart ? 0 : days.indexOf(start)
    const to = end > weekEnd ? 6 : days.indexOf(end)
    let lane = laneEnds.findIndex((l) => l < days[from])
    if (lane === -1) lane = laneEnds.length
    if (lane >= MAX_LANES) {
      for (let i = from; i <= to; i++) overflow.set(days[i], (overflow.get(days[i]) ?? 0) + 1)
      continue
    }
    laneEnds[lane] = days[to]
    segments.push({ trip, startCol: from, span: to - from + 1, lane })
  }

  return (
    <div className="relative grid grid-cols-7 border-b border-slate-100 dark:border-slate-800 last:border-b-0">
      {days.map((day) => {
        const inMonth = new Date(day + 'T00:00:00').getMonth() === month.getMonth()
        return (
          <div key={day} className="h-28 border-r border-slate-100 dark:border-slate-800 p-1 last:border-r-0">
            <span
              className={`inline-flex h-6 w-6 items-center justify-center rounded-full text-xs ${
                day === today
                  ? 'bg-slate-900 dark:bg-slate-100 font-semibold text-white dark:text-slate-900'
                  : inMonth
                    ? 'text-slate-700 dark:text-slate-300'
                    : 'text-slate-300 dark:text-slate-600'
              }`}
            >
              {Number(day.slice(8))}
            </span>
            {overflow.has(day) && (
              <p className="mt-auto text-[10px] text-slate-400 dark:text-slate-500">+{overflow.get(day)} more</p>
            )}
          </div>
        )
      })}
      {segments.map(({ trip, startCol, span, lane }) => (
        <button
          key={trip.id + weekStart}
          type="button"
          onClick={() => navigate({ to: '/trips/$tripId', params: { tripId: trip.id } })}
          title={trip.title}
          className={`absolute h-5 cursor-pointer truncate rounded px-1.5 text-left text-[11px] font-medium leading-5 hover:opacity-80 ${statusStyles[trip.effectiveStatus]}`}
          style={{
            left: `calc(${(startCol / 7) * 100}% + 2px)`,
            width: `calc(${(span / 7) * 100}% - 4px)`,
            top: `${32 + lane * 22}px`,
          }}
        >
          {trip.title}
        </button>
      ))}
    </div>
  )
}

// ---- date grid helpers (local time, "YYYY-MM-DD" keys) -----------------------

function localISO(d: Date): string {
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
}

function addMonths(d: Date, n: number): Date {
  return new Date(d.getFullYear(), d.getMonth() + n, 1)
}

/** The month's weeks as rows of 7 ISO dates, Monday-first, padded to full weeks. */
function monthWeeks(month: Date): string[][] {
  const first = new Date(month.getFullYear(), month.getMonth(), 1)
  const start = new Date(first)
  start.setDate(first.getDate() - ((first.getDay() + 6) % 7)) // back to Monday
  const weeks: string[][] = []
  const cur = new Date(start)
  do {
    const week: string[] = []
    for (let i = 0; i < 7; i++) {
      week.push(localISO(cur))
      cur.setDate(cur.getDate() + 1)
    }
    weeks.push(week)
  } while (cur.getMonth() === month.getMonth())
  return weeks
}

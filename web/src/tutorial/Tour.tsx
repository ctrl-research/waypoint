import { useEffect, useRef, useState, useSyncExternalStore } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { DEMO_TRIP_ID } from './fixtures'
import {
  endTutorial,
  setTutorialStep,
  subscribeTutorial,
  tutorialActive,
  tutorialStep,
} from './state'

type TourStep = {
  route: string
  /** data-tour attribute of the highlighted element; null centers the popup. */
  target: string | null
  title: string
  body: string
}

const STEPS: TourStep[] = [
  {
    route: '/',
    target: null,
    title: 'Welcome to Waypoint 🧭',
    body: 'A quick tour of planning, logging, and reliving your trips. Everything you see during the tour is demo data — nothing you look at here is saved.',
  },
  {
    route: '/',
    target: 'new-trip',
    title: 'Trips start here',
    body: 'Create a trip with a title and dates. Trips group everything else: the route, the day-by-day plan, journal entries, and stats.',
  },
  {
    route: '/',
    target: 'trip-search',
    title: 'Find any trip',
    body: 'Search by name, city, or even country — plus status and year filters. Handy once the list grows.',
  },
  {
    route: `/trips/${DEMO_TRIP_ID}`,
    target: 'trip-map',
    title: 'The trip map',
    body: 'Areas and itinerary items live on the map. Open ▶ Layers to toggle what is shown, use the crosshair to re-frame the route, and hit ▶ Replay to watch the trip play out.',
  },
  {
    route: `/trips/${DEMO_TRIP_ID}`,
    target: 'itinerary-overview',
    title: 'Areas and the day-by-day plan',
    body: 'Areas are the countries, cities, or regions of the route — click one to focus the map. Below them, the itinerary shows every visible layer merged, ordered by time.',
  },
  {
    route: `/trips/${DEMO_TRIP_ID}/itinerary`,
    target: 'layers-panel',
    title: 'Layers for planning together',
    body: 'Everyone can keep their own idea layers ("Food ideas") next to the shared Main layer. The eye includes or hides a layer everywhere — the itinerary is whatever is visible.',
  },
  {
    route: `/trips/${DEMO_TRIP_ID}/itinerary`,
    target: 'item-form',
    title: 'Build the plan',
    body: 'Add activities, food, lodging, and travel legs. Flights and trains take departure and arrival stations with times — the map and replay use them to draw the journey.',
  },
  {
    route: '/calendar',
    target: 'calendar-grid',
    title: 'All trips on a calendar',
    body: 'Every dated trip as a bar on the month grid. You can also subscribe your personal calendar to all your trips from Settings.',
  },
  {
    route: '/stats',
    target: 'stat-tiles',
    title: 'Your travel stats',
    body: 'Where you have been and what is still planned — the amber +N. The map below lights up visited countries, and flights and trains get their own tallies.',
  },
  {
    route: '/',
    target: null,
    title: 'That’s the tour!',
    body: 'You are back on your own data. Replay the tour anytime from Settings → Tour.',
  },
]

/**
 * The guided tour (#96): a full-screen overlay that spotlights one element
 * per step with a popup, driving the router through demo-data screens.
 * Interaction with the app is blocked while it runs, keeping it stateless.
 */
export function Tour() {
  const active = useSyncExternalStore(subscribeTutorial, tutorialActive)
  const step = useSyncExternalStore(subscribeTutorial, tutorialStep)
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const [rect, setRect] = useState<DOMRect | null>(null)
  const prevActive = useRef(false)

  // Entering/leaving the tour swaps the whole data layer. resetQueries
  // (not clear) forces mounted queries to refetch immediately, so real
  // data replaces the demo the moment the tour ends.
  useEffect(() => {
    if (active !== prevActive.current) {
      prevActive.current = active
      if (!active) navigate({ to: '/' })
      queryClient.resetQueries()
    }
  }, [active, navigate, queryClient])

  const current = STEPS[step]

  // Navigate to the step's route, then track its target element's position
  // (polling covers lazy-loaded chunks and window resizes alike).
  useEffect(() => {
    if (!active || !current) return
    navigate({ to: current.route })
    setRect(null)
    if (!current.target) return
    let tries = 0
    const find = () => {
      const el = document.querySelector(`[data-tour="${current.target}"]`)
      if (el) {
        // Tall targets scroll to their top — centering a section bigger
        // than the screen would land the user in its middle.
        const tall = el.getBoundingClientRect().height > window.innerHeight * 0.7
        el.scrollIntoView({ block: tall ? 'start' : 'center', behavior: tries === 0 ? 'auto' : 'smooth' })
        setRect(el.getBoundingClientRect())
      } else if (tries === 40) {
        setRect(null) // give up: show the popup centered
      }
      tries++
    }
    find()
    const poll = window.setInterval(find, 150)
    return () => window.clearInterval(poll)
  }, [active, step, current, navigate])

  if (!active || !current) return null

  const last = step === STEPS.length - 1
  const popStyle = popoverPosition(current.target ? rect : null)

  return (
    <div className="fixed inset-0 z-50" role="dialog" aria-modal="true" aria-label="Waypoint tour">
      {/* Spotlight: the shadow dims everything but the target. */}
      {rect && current.target ? (
        <div
          className="pointer-events-none fixed rounded-xl ring-2 ring-sky-400 transition-all duration-300"
          style={{
            left: rect.left - 6,
            top: rect.top - 6,
            width: rect.width + 12,
            height: rect.height + 12,
            boxShadow: '0 0 0 9999px rgb(15 23 42 / 0.6)',
          }}
        />
      ) : (
        <div className="fixed inset-0 bg-slate-900/60" />
      )}
      {/* Click shield: the tour drives the app, not stray clicks. */}
      <div className="fixed inset-0" />

      <div
        className="fixed w-[min(92vw,22rem)] rounded-xl bg-white p-4 shadow-2xl dark:bg-slate-900"
        style={popStyle}
      >
        <p className="text-xs font-medium text-slate-400 dark:text-slate-500">
          Step {step + 1} of {STEPS.length}
        </p>
        <h2 className="mt-1 font-semibold text-slate-900 dark:text-slate-100">{current.title}</h2>
        <p className="mt-1 text-sm text-slate-600 dark:text-slate-400">{current.body}</p>
        <div className="mt-3 flex items-center justify-between">
          <button
            type="button"
            onClick={() => endTutorial('skipped')}
            className="text-sm text-slate-400 dark:text-slate-500 hover:text-slate-900 dark:hover:text-slate-100"
          >
            Exit tour
          </button>
          <div className="flex gap-2">
            {step > 0 && (
              <button
                type="button"
                onClick={() => setTutorialStep(step - 1)}
                className="rounded-lg border border-slate-300 dark:border-slate-600 px-3 py-1.5 text-sm text-slate-600 dark:text-slate-400 hover:bg-slate-50 dark:hover:bg-slate-800"
              >
                Back
              </button>
            )}
            <button
              type="button"
              onClick={() => (last ? endTutorial('done') : setTutorialStep(step + 1))}
              className="rounded-lg bg-slate-900 dark:bg-slate-100 px-4 py-1.5 text-sm font-medium text-white dark:text-slate-900 hover:bg-slate-700 dark:hover:bg-slate-300"
            >
              {last ? 'Finish' : 'Next'}
            </button>
          </div>
        </div>
      </div>
    </div>
  )
}

/** Below the target when there's room, above otherwise; targets taller
 * than the viewport pin the popup to the bottom so it always stays
 * readable on screen. */
function popoverPosition(rect: DOMRect | null): React.CSSProperties {
  if (!rect) {
    return { left: '50%', top: '50%', transform: 'translate(-50%, -50%)' }
  }
  const width = Math.min(window.innerWidth * 0.92, 352)
  const left = Math.max(12, Math.min(rect.left, window.innerWidth - width - 12))
  const below = rect.bottom + 16
  if (below + 220 < window.innerHeight) return { left, top: below }
  if (rect.top - 236 > 0 && rect.top < window.innerHeight) {
    return { left, bottom: window.innerHeight - rect.top + 16 }
  }
  // The target dominates (or overflows) the viewport.
  return { left: '50%', transform: 'translateX(-50%)', bottom: 24 }
}

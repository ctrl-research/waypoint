/**
 * Tutorial state (#96), module-level like theme.ts so the api layer can
 * consult it without React context. While active, all /api/v1 reads are
 * served from demo fixtures and writes are blocked, so nothing persists.
 */
const STORAGE_KEY = 'waypoint-tutorial'

let active = false
let step = 0
const listeners = new Set<() => void>()

function notify() {
  for (const fn of listeners) fn()
}

export function tutorialActive(): boolean {
  return active
}

export function tutorialStep(): number {
  return step
}

export function subscribeTutorial(onChange: () => void): () => void {
  listeners.add(onChange)
  return () => listeners.delete(onChange)
}

export function startTutorial() {
  active = true
  step = 0
  notify()
}

/** Ends the tour, remembering why so it stops auto-launching. */
export function endTutorial(reason: 'done' | 'skipped') {
  active = false
  step = 0
  localStorage.setItem(STORAGE_KEY, reason)
  notify()
}

export function setTutorialStep(next: number) {
  step = next
  notify()
}

/** The tour auto-launches on login until it was completed or skipped. */
export function tutorialSeen(): boolean {
  return localStorage.getItem(STORAGE_KEY) !== null
}

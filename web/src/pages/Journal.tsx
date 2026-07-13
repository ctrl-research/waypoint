import { useMemo, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import Markdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import {
  ApiError,
  createJournalEntry,
  deleteJournalEntry,
  deleteJournalPhoto,
  listJournal,
  updateJournalEntry,
  uploadJournalPhoto,
  type ItineraryItem,
  type JournalEntry,
  type JournalEntryInput,
  type Stop,
  type Trip,
} from '../api'
import { categoryIcons } from './ItineraryBoard'
import { defaultTripDay } from './Home'

const field =
  'rounded-lg border border-slate-300 dark:border-slate-600 px-3 py-2 text-sm focus:border-slate-500 focus:outline-none'

/**
 * Timeline (#17): one section per day, interleaving what was planned
 * (itinerary items, compact) with what actually happened (journal entries
 * with rendered markdown and photos).
 */
export function JournalTimeline({
  trip,
  items,
  stops,
  canEdit,
}: {
  trip: Trip
  items: ItineraryItem[]
  stops: Stop[]
  canEdit: boolean
}) {
  const tripId = trip.id
  const entriesQuery = useQuery({
    queryKey: ['journal', tripId],
    queryFn: () => listJournal(tripId),
  })
  const [composing, setComposing] = useState(false)

  const entries = entriesQuery.data ?? []
  const days = useMemo(() => {
    const set = new Set<string>()
    for (const item of items) set.add(item.day)
    for (const entry of entries) set.add(entry.entryDate)
    return [...set].sort()
  }, [items, entries])

  return (
    <section className="mt-10">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-lg font-semibold text-slate-900 dark:text-slate-100">Journal</h2>
          <p className="text-sm text-slate-500 dark:text-slate-400">The plan and what actually happened, day by day.</p>
        </div>
        {canEdit && (
          <button
            type="button"
            onClick={() => setComposing((v) => !v)}
            className="rounded-lg bg-slate-900 dark:bg-slate-100 px-4 py-2 text-sm font-medium text-white dark:text-slate-900 hover:bg-slate-700 dark:hover:bg-slate-300"
          >
            {composing ? 'Cancel' : 'New entry'}
          </button>
        )}
      </div>

      {composing && (
        <div className="mt-4">
          <EntryForm trip={trip} onDone={() => setComposing(false)} />
        </div>
      )}

      <div className="mt-6 space-y-8">
        {days.length === 0 && !composing && (
          <p className="text-sm text-slate-400 dark:text-slate-500">
            Nothing here yet — plan itinerary items or write your first entry.
          </p>
        )}
        {days.map((day) => (
          <TimelineDay
            key={day}
            trip={trip}
            day={day}
            items={items.filter((i) => i.day === day)}
            entries={entries.filter((e) => e.entryDate === day)}
            stops={stops}
            canEdit={canEdit}
          />
        ))}
      </div>
    </section>
  )
}

function TimelineDay({
  trip,
  day,
  items,
  entries,
  stops,
  canEdit,
}: {
  trip: Trip
  day: string
  items: ItineraryItem[]
  entries: JournalEntry[]
  stops: Stop[]
  canEdit: boolean
}) {
  const stopName = (id: string | null) => stops.find((s) => s.id === id)?.name

  return (
    <div className="relative border-l-2 border-slate-200 dark:border-slate-800 pl-6">
      <div className="absolute -left-[7px] top-1 h-3 w-3 rounded-full bg-slate-400 dark:bg-slate-500" />
      <h3 className="text-sm font-semibold text-slate-900 dark:text-slate-100">
        {new Date(day + 'T00:00:00').toLocaleDateString(undefined, {
          weekday: 'long',
          month: 'long',
          day: 'numeric',
          year: 'numeric',
        })}
      </h3>

      {items.length > 0 && (
        <ul className="mt-2 space-y-0.5">
          {items.map((item) => (
            <li key={item.id} className="flex items-center gap-2 text-sm text-slate-500 dark:text-slate-400">
              <span>{categoryIcons[item.category]}</span>
              {item.startTime && <span className="tabular-nums">{item.startTime}</span>}
              <span>{item.title}</span>
              {stopName(item.stopId) && <span className="text-xs">@ {stopName(item.stopId)}</span>}
            </li>
          ))}
        </ul>
      )}

      <div className="mt-3 space-y-4">
        {entries.map((entry) => (
          <EntryCard key={entry.id} trip={trip} entry={entry} canEdit={canEdit} />
        ))}
      </div>
    </div>
  )
}

function EntryCard({ trip, entry, canEdit }: { trip: Trip; entry: JournalEntry; canEdit: boolean }) {
  const tripId = trip.id
  const queryClient = useQueryClient()
  const [editing, setEditing] = useState(false)

  const remove = useMutation({
    mutationFn: () => deleteJournalEntry(tripId, entry.id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['journal', tripId] }),
  })

  if (editing) {
    return <EntryForm trip={trip} entry={entry} onDone={() => setEditing(false)} />
  }

  return (
    <article className="rounded-xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 p-5 shadow-sm">
      <div className="flex items-start justify-between gap-3">
        {entry.title ? (
          <h4 className="font-medium text-slate-900 dark:text-slate-100">{entry.title}</h4>
        ) : (
          <span className="text-sm text-slate-400 dark:text-slate-500">Untitled entry</span>
        )}
        {canEdit && (
        <div className="flex shrink-0 gap-2 text-sm">
          <button
            type="button"
            onClick={() => setEditing(true)}
            className="text-slate-400 dark:text-slate-500 hover:text-slate-900 dark:hover:text-slate-100"
          >
            Edit
          </button>
          <button
            type="button"
            onClick={() => {
              if (window.confirm('Delete this entry and its photos?')) remove.mutate()
            }}
            className="text-slate-400 dark:text-slate-500 hover:text-red-600 dark:hover:text-red-400"
          >
            Delete
          </button>
        </div>
        )}
      </div>

      {entry.body && (
        <div className="prose-sm mt-2 max-w-none text-slate-700 dark:text-slate-300 [&_a]:underline [&_h1]:text-lg [&_h1]:font-semibold [&_h2]:font-semibold [&_img]:max-h-80 [&_img]:rounded-lg [&_li]:ml-4 [&_li]:list-disc [&_p]:mt-2">
          <Markdown remarkPlugins={[remarkGfm]}>{entry.body}</Markdown>
        </div>
      )}

      {entry.photos.length > 0 && (
        <div className="mt-3 flex flex-wrap gap-2">
          {entry.photos.map((photo) => (
            <a key={photo.id} href={photo.url} target="_blank" rel="noreferrer" title={photo.caption}>
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
  )
}

/** EntryForm creates or edits an entry: markdown with preview (#18), and in
 * edit mode a photo strip with upload, markdown-insert, and delete. */
function EntryForm({
  trip,
  entry,
  onDone,
}: {
  trip: Trip
  entry?: JournalEntry
  onDone: () => void
}) {
  const tripId = trip.id
  const queryClient = useQueryClient()
  const [entryDate, setEntryDate] = useState(
    entry?.entryDate ?? defaultTripDay(trip.startDate, trip.endDate),
  )
  const [title, setTitle] = useState(entry?.title ?? '')
  const [body, setBody] = useState(entry?.body ?? '')
  const [tab, setTab] = useState<'write' | 'preview'>('write')
  const invalidate = () => queryClient.invalidateQueries({ queryKey: ['journal', tripId] })

  const save = useMutation({
    mutationFn: () => {
      const input: JournalEntryInput = { entryDate, title, body }
      return entry
        ? updateJournalEntry(tripId, entry.id, input)
        : createJournalEntry(tripId, input)
    },
    onSuccess: async () => {
      await invalidate()
      onDone()
    },
  })

  return (
    <form
      className="space-y-3 rounded-xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 p-5 shadow-sm"
      onSubmit={(e) => {
        e.preventDefault()
        if (entryDate) save.mutate()
      }}
    >
      <div className="flex flex-wrap gap-2">
        <input
          type="date"
          required
          value={entryDate}
          min={trip.startDate ?? undefined}
          max={trip.endDate ?? undefined}
          onChange={(e) => setEntryDate(e.target.value)}
          className={field}
        />
        <input
          value={title}
          onChange={(e) => setTitle(e.target.value)}
          placeholder="Title (optional)"
          className={`${field} min-w-40 flex-1`}
        />
      </div>

      <div>
        <div className="flex gap-1 text-sm">
          {(['write', 'preview'] as const).map((t) => (
            <button
              key={t}
              type="button"
              onClick={() => setTab(t)}
              className={`rounded-t-lg px-3 py-1.5 ${
                tab === t ? 'bg-slate-100 dark:bg-slate-800 font-medium text-slate-900 dark:text-slate-100' : 'text-slate-400 dark:text-slate-500 hover:text-slate-700 dark:hover:text-slate-300'
              }`}
            >
              {t === 'write' ? 'Write' : 'Preview'}
            </button>
          ))}
        </div>
        {tab === 'write' ? (
          <textarea
            value={body}
            onChange={(e) => setBody(e.target.value)}
            rows={6}
            placeholder="What happened today? Markdown supported."
            className={`${field} w-full rounded-tl-none font-mono text-[13px]`}
          />
        ) : (
          <div className="min-h-32 rounded-lg rounded-tl-none border border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-4 py-3 text-sm text-slate-700 dark:text-slate-300 [&_img]:max-h-60 [&_img]:rounded [&_li]:ml-4 [&_li]:list-disc [&_p]:mt-2 first:[&_p]:mt-0">
            {body ? (
              <Markdown remarkPlugins={[remarkGfm]}>{body}</Markdown>
            ) : (
              <span className="text-slate-400 dark:text-slate-500">Nothing to preview.</span>
            )}
          </div>
        )}
      </div>

      {entry && (
        <PhotoStrip
          tripId={tripId}
          entry={entry}
          onInsert={(md) => {
            setBody((b) => (b ? b + '\n\n' + md : md))
            setTab('write')
          }}
        />
      )}
      {!entry && (
        <p className="text-xs text-slate-400 dark:text-slate-500">Save the entry first to attach photos.</p>
      )}

      {save.error && (
        <p className="text-sm text-red-600 dark:text-red-400">
          {save.error instanceof ApiError ? save.error.message : 'Could not save entry'}
        </p>
      )}
      <div className="flex gap-2">
        <button
          type="submit"
          disabled={save.isPending || !entryDate}
          className="rounded-lg bg-slate-900 dark:bg-slate-100 px-4 py-2 text-sm font-medium text-white dark:text-slate-900 hover:bg-slate-700 dark:hover:bg-slate-300 disabled:opacity-50"
        >
          {save.isPending ? 'Saving…' : 'Save entry'}
        </button>
        <button
          type="button"
          onClick={onDone}
          className="rounded-lg border border-slate-300 dark:border-slate-600 px-4 py-2 text-sm text-slate-600 dark:text-slate-400 hover:bg-slate-50 dark:hover:bg-slate-800"
        >
          Cancel
        </button>
      </div>
    </form>
  )
}

function PhotoStrip({
  tripId,
  entry,
  onInsert,
}: {
  tripId: string
  entry: JournalEntry
  onInsert: (markdown: string) => void
}) {
  const queryClient = useQueryClient()
  const invalidate = () => queryClient.invalidateQueries({ queryKey: ['journal', tripId] })

  const upload = useMutation({
    mutationFn: (file: File) => uploadJournalPhoto(tripId, entry.id, file, ''),
    onSuccess: invalidate,
  })
  const removePhoto = useMutation({
    mutationFn: (photoId: string) => deleteJournalPhoto(tripId, photoId),
    onSuccess: invalidate,
  })

  return (
    <div>
      <div className="flex flex-wrap items-center gap-2">
        {entry.photos.map((photo) => (
          <div key={photo.id} className="group relative">
            <img
              src={photo.url}
              alt={photo.caption}
              className="h-20 w-20 rounded-lg border border-slate-200 dark:border-slate-800 object-cover"
            />
            <div className="absolute inset-0 hidden items-center justify-center gap-1 rounded-lg bg-slate-900/60 group-hover:flex">
              <button
                type="button"
                title="Insert into text"
                onClick={() => onInsert(`![${photo.caption || 'photo'}](${photo.url})`)}
                className="rounded bg-white/90 px-1.5 text-xs"
              >
                ↳md
              </button>
              <button
                type="button"
                title="Delete photo"
                onClick={() => removePhoto.mutate(photo.id)}
                className="rounded bg-white/90 px-1.5 text-xs text-red-600 dark:text-red-400"
              >
                ✕
              </button>
            </div>
          </div>
        ))}
        <label className="flex h-20 w-20 cursor-pointer items-center justify-center rounded-lg border border-dashed border-slate-300 dark:border-slate-600 text-2xl text-slate-300 dark:text-slate-600 hover:border-slate-400 dark:hover:border-slate-500 hover:text-slate-500 dark:hover:text-slate-400">
          {upload.isPending ? '…' : '+'}
          <input
            type="file"
            accept="image/jpeg,image/png,image/webp,image/gif"
            className="hidden"
            onChange={(e) => {
              const file = e.target.files?.[0]
              if (file) upload.mutate(file)
              e.target.value = ''
            }}
          />
        </label>
      </div>
      {upload.error && (
        <p className="mt-1 text-sm text-red-600 dark:text-red-400">
          {upload.error instanceof ApiError ? upload.error.message : 'Upload failed'}
        </p>
      )}
    </div>
  )
}

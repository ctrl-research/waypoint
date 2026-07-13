import { useEffect, useMemo, useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import {
  DndContext,
  PointerSensor,
  closestCorners,
  useDroppable,
  useSensor,
  useSensors,
  type DragEndEvent,
} from '@dnd-kit/core'
import {
  SortableContext,
  arrayMove,
  useSortable,
  verticalListSortingStrategy,
} from '@dnd-kit/sortable'
import { CSS } from '@dnd-kit/utilities'
import {
  deleteItem,
  reorderItems,
  updateItem,
  type ItineraryCategory,
  type ItineraryItem,
  type Stop,
  type Trip,
} from '../api'

export const categoryIcons: Record<ItineraryCategory, string> = {
  activity: '🎟️',
  food: '🍜',
  lodging: '🛏️',
  transport: '🚆',
  flight: '✈️',
  other: '📌',
}

const MAX_RANGE_DAYS = 60

/** Days shown as columns: the trip's date range (capped) ∪ days with items. */
function boardDays(trip: Trip, items: ItineraryItem[]): string[] {
  const days = new Set<string>()
  if (trip.startDate && trip.endDate) {
    const start = new Date(trip.startDate + 'T00:00:00')
    const end = new Date(trip.endDate + 'T00:00:00')
    for (let d = start, i = 0; d <= end && i < MAX_RANGE_DAYS; d.setDate(d.getDate() + 1), i++) {
      days.add(d.toISOString().slice(0, 10))
    }
  }
  for (const item of items) days.add(item.day)
  return [...days].sort()
}

export function ItineraryBoard({
  trip,
  items,
  stops,
  readOnly = false,
}: {
  trip: Trip
  items: ItineraryItem[]
  stops: Stop[]
  readOnly?: boolean
}) {
  const queryClient = useQueryClient()
  // Local copy so drags feel instant; server state re-syncs it on refetch.
  const [local, setLocal] = useState(items)
  useEffect(() => setLocal(items), [items])

  const invalidate = () => queryClient.invalidateQueries({ queryKey: ['trip', trip.id] })
  const days = useMemo(() => boardDays(trip, local), [trip, local])

  const byDay = useMemo(() => {
    const m = new Map<string, ItineraryItem[]>()
    for (const day of days) m.set(day, [])
    for (const item of local) m.get(item.day)?.push(item)
    return m
  }, [days, local])

  const reorder = useMutation({
    mutationFn: ({ day, ids }: { day: string; ids: string[] }) => reorderItems(trip.id, day, ids),
    onSettled: invalidate,
  })
  const move = useMutation({
    mutationFn: async ({ itemId, day, ids }: { itemId: string; day: string; ids: string[] }) => {
      // Server appends the item to the new day, then the permutation PUT
      // places it exactly where it was dropped.
      await updateItem(trip.id, itemId, { day })
      await reorderItems(trip.id, day, ids)
    },
    onSettled: invalidate,
  })
  const remove = useMutation({
    mutationFn: (itemId: string) => deleteItem(trip.id, itemId),
    onSuccess: invalidate,
  })

  const sensors = useSensors(useSensor(PointerSensor, { activationConstraint: { distance: 5 } }))

  function onDragEnd({ active, over }: DragEndEvent) {
    if (!over) return
    const item = local.find((i) => i.id === active.id)
    if (!item) return

    // Target: either an item (take its day + index) or a day column (append).
    const overId = String(over.id)
    const overItem = local.find((i) => i.id === overId)
    const targetDay = overItem ? overItem.day : overId.startsWith('day:') ? overId.slice(4) : null
    if (!targetDay) return

    const sourceIds = (byDay.get(item.day) ?? []).map((i) => i.id)
    const targetIds = (byDay.get(targetDay) ?? []).map((i) => i.id)

    if (item.day === targetDay) {
      const from = sourceIds.indexOf(item.id)
      const to = overItem ? sourceIds.indexOf(overItem.id) : sourceIds.length - 1
      if (from === to || from < 0 || to < 0) return
      const ids = arrayMove(sourceIds, from, to)
      setLocal(applyOrder(local, item.day, ids))
      reorder.mutate({ day: item.day, ids })
    } else {
      const insertAt = overItem ? targetIds.indexOf(overItem.id) : targetIds.length
      const ids = [...targetIds]
      ids.splice(insertAt, 0, item.id)
      setLocal(
        applyOrder(
          local.map((i) => (i.id === item.id ? { ...i, day: targetDay } : i)),
          targetDay,
          ids,
        ),
      )
      move.mutate({ itemId: item.id, day: targetDay, ids })
    }
  }

  return (
    <DndContext sensors={sensors} collisionDetection={closestCorners} onDragEnd={onDragEnd}>
      <div className="mt-4 space-y-4">
        {days.length === 0 && (
          <p className="text-sm text-slate-400">
            Set trip dates or add a first item to start the day-by-day plan.
          </p>
        )}
        {days.map((day) => (
          <DayColumn
            key={day}
            day={day}
            items={byDay.get(day) ?? []}
            stops={stops}
            readOnly={readOnly}
            onDelete={(id) => remove.mutate(id)}
          />
        ))}
      </div>
    </DndContext>
  )
}

function DayColumn({
  day,
  items,
  stops,
  readOnly,
  onDelete,
}: {
  day: string
  items: ItineraryItem[]
  stops: Stop[]
  readOnly: boolean
  onDelete: (id: string) => void
}) {
  const { setNodeRef, isOver } = useDroppable({ id: `day:${day}` })

  return (
    <div>
      <h3 className="text-sm font-semibold text-slate-700">
        {new Date(day + 'T00:00:00').toLocaleDateString(undefined, {
          weekday: 'short',
          month: 'short',
          day: 'numeric',
        })}
      </h3>
      <SortableContext items={items.map((i) => i.id)} strategy={verticalListSortingStrategy}>
        <div
          ref={setNodeRef}
          className={`mt-1 min-h-9 space-y-1 rounded-lg p-0.5 ${isOver ? 'bg-slate-100' : ''}`}
        >
          {items.length === 0 && (
            <p className="px-3 py-1.5 text-xs text-slate-300">drop items here</p>
          )}
          {items.map((item) => (
            <BoardItem key={item.id} item={item} stops={stops} readOnly={readOnly} onDelete={onDelete} />
          ))}
        </div>
      </SortableContext>
    </div>
  )
}

function BoardItem({
  item,
  stops,
  readOnly,
  onDelete,
}: {
  item: ItineraryItem
  stops: Stop[]
  readOnly: boolean
  onDelete: (id: string) => void
}) {
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({
    id: item.id,
  })
  const stopName = stops.find((s) => s.id === item.stopId)?.name

  return (
    <div
      ref={setNodeRef}
      style={{ transform: CSS.Transform.toString(transform), transition }}
      className={`flex items-center justify-between rounded-lg border border-slate-200 bg-white px-2 py-2 ${
        isDragging ? 'z-10 opacity-70 shadow-lg' : ''
      }`}
    >
      <div className="flex min-w-0 items-center gap-2 text-sm">
        {!readOnly && (
        <button
          type="button"
          {...attributes}
          {...listeners}
          className="cursor-grab touch-none px-1 text-slate-300 hover:text-slate-500 active:cursor-grabbing"
          aria-label={`Drag ${item.title}`}
        >
          ⠿
        </button>
        )}
        <span>{categoryIcons[item.category]}</span>
        {item.startTime && (
          <span className="tabular-nums text-slate-500">
            {item.startTime}
            {item.endTime && `–${item.endTime}`}
          </span>
        )}
        <span className="truncate font-medium text-slate-900">{item.title}</span>
        {stopName && (
          <span className="truncate text-xs text-slate-400">
            {item.category === 'flight' ? stopName : `@ ${stopName}`}
            {item.category === 'flight' &&
              destName(stops, item.destinationStopId) &&
              ` → ${destName(stops, item.destinationStopId)}`}
          </span>
        )}
      </div>
      {!readOnly && (
      <button
        type="button"
        onClick={() => onDelete(item.id)}
        className="px-1 text-sm text-slate-400 hover:text-red-600"
        aria-label={`Remove ${item.title}`}
      >
        ✕
      </button>
      )}
    </div>
  )
}

function destName(stops: Stop[], id: string | null): string | undefined {
  return stops.find((s) => s.id === id)?.name
}

/** Returns items with the given day's rows resequenced to match ids. */
function applyOrder(items: ItineraryItem[], day: string, ids: string[]): ItineraryItem[] {
  const rank = new Map(ids.map((id, i) => [id, i]))
  return [...items].sort((a, b) => {
    if (a.day !== b.day) return a.day < b.day ? -1 : 1
    if (a.day !== day) return a.position - b.position
    return (rank.get(a.id) ?? a.position) - (rank.get(b.id) ?? b.position)
  })
}

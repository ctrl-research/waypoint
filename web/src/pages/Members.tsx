import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  ApiError,
  addMember,
  fetchMe,
  listMembers,
  removeMember,
  type TripRole,
} from '../api'

const field =
  'rounded-lg border border-slate-300 px-3 py-2 text-sm focus:border-slate-500 focus:outline-none'

/** MembersSection lists who a trip is shared with. The owner can add by
 * email and remove; members can leave. */
export function MembersSection({ tripId, role }: { tripId: string; role: TripRole }) {
  const queryClient = useQueryClient()
  const { data: me } = useQuery({ queryKey: ['me'], queryFn: fetchMe })
  const members = useQuery({ queryKey: ['members', tripId], queryFn: () => listMembers(tripId) })
  const isOwner = role === 'owner'

  const [email, setEmail] = useState('')
  const [newRole, setNewRole] = useState<'viewer' | 'editor'>('viewer')

  const invalidate = () => queryClient.invalidateQueries({ queryKey: ['members', tripId] })
  const add = useMutation({
    mutationFn: () => addMember(tripId, email, newRole),
    onSuccess: async () => {
      setEmail('')
      await invalidate()
    },
  })
  const remove = useMutation({
    mutationFn: (userId: string) => removeMember(tripId, userId),
    onSuccess: async (_, userId) => {
      if (userId === me?.id) {
        // We just left the trip — bounce home.
        window.location.href = '/'
        return
      }
      await invalidate()
    },
  })

  return (
    <section className="mt-10">
      <h2 className="text-lg font-semibold text-slate-900">Shared with</h2>
      <p className="text-sm text-slate-500">
        {isOwner
          ? 'People you invite can view or co-plan this trip. They need an account here first.'
          : 'This trip is shared with you.'}
      </p>

      <div className="mt-4 space-y-2">
        {members.data?.length === 0 && (
          <p className="text-sm text-slate-400">Not shared with anyone yet.</p>
        )}
        {members.data?.map((m) => (
          <div
            key={m.userId}
            className="flex items-center justify-between rounded-lg border border-slate-200 bg-white px-4 py-2.5"
          >
            <div className="flex items-center gap-3">
              {m.avatarUrl ? (
                <img src={m.avatarUrl} alt="" className="h-7 w-7 rounded-full" referrerPolicy="no-referrer" />
              ) : (
                <div className="flex h-7 w-7 items-center justify-center rounded-full bg-slate-200 text-xs font-medium text-slate-600">
                  {(m.displayName || m.email).charAt(0).toUpperCase()}
                </div>
              )}
              <div>
                <p className="text-sm font-medium text-slate-900">{m.displayName || m.email}</p>
                <p className="text-xs text-slate-500">{m.email}</p>
              </div>
              <span className="rounded-full bg-slate-100 px-2 py-0.5 text-xs text-slate-600">
                {m.role}
              </span>
            </div>
            {(isOwner || m.userId === me?.id) && (
              <button
                type="button"
                onClick={() => remove.mutate(m.userId)}
                className="text-sm text-slate-400 hover:text-red-600"
              >
                {m.userId === me?.id ? 'Leave' : 'Remove'}
              </button>
            )}
          </div>
        ))}

        {isOwner && (
          <form
            className="flex flex-wrap gap-2"
            onSubmit={(e) => {
              e.preventDefault()
              if (email.trim()) add.mutate()
            }}
          >
            <input
              type="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              placeholder="friend@example.com"
              className={`${field} min-w-52 flex-1`}
            />
            <select
              value={newRole}
              onChange={(e) => setNewRole(e.target.value as 'viewer' | 'editor')}
              className={field}
            >
              <option value="viewer">viewer</option>
              <option value="editor">editor</option>
            </select>
            <button
              type="submit"
              disabled={add.isPending || !email.trim()}
              className="rounded-lg bg-slate-900 px-4 py-2 text-sm font-medium text-white hover:bg-slate-700 disabled:opacity-50"
            >
              Share
            </button>
          </form>
        )}
        {add.error && (
          <p className="text-sm text-red-600">
            {add.error instanceof ApiError ? add.error.message : 'Could not add member'}
          </p>
        )}
      </div>
    </section>
  )
}

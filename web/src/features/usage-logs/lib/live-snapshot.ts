/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { replaceEqualDeep } from '@tanstack/react-query'
import type { GetLogsResponse, UsageLog } from '../types'

export type LogPage = NonNullable<GetLogsResponse['data']> & {
  updates?: UsageLog[]
}

// Match by ID rather than array position: inserting a row must not recreate
// every existing row in the query cache.
export function shareLogPage(
  previous: LogPage | undefined,
  next: LogPage
): LogPage {
  if (!previous) return next
  const byID = new Map(previous.items.map((item) => [item.id, item]))
  const items = next.items.map((item) =>
    replaceEqualDeep(byID.get(item.id), item)
  )
  const sameItems =
    items.length === previous.items.length &&
    items.every((item, index) => item === previous.items[index])
  const updates = replaceEqualDeep(previous.updates, next.updates)
  if (
    sameItems &&
    updates === previous.updates &&
    next.total === previous.total &&
    next.page === previous.page &&
    next.page_size === previous.page_size
  )
    return previous
  return {
    ...next,
    items: sameItems ? previous.items : items,
    updates,
  } as LogPage
}

export interface LogSnapshot {
  scope: string
  source?: LogPage
  display?: LogPage
  updatedAt: number
  manualVersion: number
  acceptNext: boolean
  pendingCount: number
  hasUpdates: boolean
}

export interface SnapshotInput {
  scope: string
  data?: LogPage
  updatedAt: number
  manualVersion: number
  hold: boolean
  placeholder: boolean
}

export function createLogSnapshot(input: SnapshotInput): LogSnapshot {
  return {
    scope: input.scope,
    source: input.placeholder ? undefined : input.data,
    display: input.placeholder ? undefined : input.data,
    updatedAt: input.updatedAt,
    manualVersion: input.manualVersion,
    acceptNext: false,
    pendingCount: 0,
    hasUpdates: false,
  }
}

export function advanceLogSnapshot(
  previous: LogSnapshot,
  input: SnapshotInput
): LogSnapshot {
  if (previous.scope !== input.scope) return createLogSnapshot(input)
  let state = previous
  if (state.manualVersion !== input.manualVersion) {
    state = { ...state, manualVersion: input.manualVersion, acceptNext: true }
  }
  if (!input.data || input.placeholder) return state
  if (state.source === input.data && state.updatedAt === input.updatedAt)
    return state
  if (
    !state.display ||
    state.acceptNext ||
    (!input.hold && !state.hasUpdates)
  ) {
    return createLogSnapshot(input)
  }

  const latest = new Map(
    [...input.data.items, ...(input.data.updates ?? [])].map((item) => [
      item.id,
      item,
    ])
  )
  const items = state.display.items.map((item) =>
    replaceEqualDeep(item, latest.get(item.id) ?? item)
  )
  const changedMembership =
    input.data.items.length !== items.length ||
    input.data.items.some((item, index) => item.id !== items[index]?.id)
  return {
    ...state,
    source: input.data,
    updatedAt: input.updatedAt,
    display: replaceEqualDeep(state.display, {
      ...state.display,
      items,
    }) as LogPage,
    pendingCount: Math.max(0, input.data.total - state.display.total),
    hasUpdates: changedMembership || input.data.total !== state.display.total,
  }
}

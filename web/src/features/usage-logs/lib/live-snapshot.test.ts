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
import assert from 'node:assert/strict'
import { test } from 'node:test'
import type { UsageLog } from '../types'
import {
  advanceLogSnapshot,
  createLogSnapshot,
  shareLogPage,
  type LogPage,
  type SnapshotInput,
} from './live-snapshot'

function row(id: number, fields: Partial<UsageLog> = {}): UsageLog {
  return {
    id,
    user_id: 42,
    created_at: id,
    type: 2,
    model_name: 'test',
    quota: 0,
    prompt_tokens: 0,
    completion_tokens: 0,
    use_time: 0,
    channel: 0,
    other: '{}',
    ...fields,
  }
}
function page(items: UsageLog[], total = items.length): LogPage {
  return { items, total, page: 1, page_size: 2 }
}
function input(
  data: LogPage,
  fields: Partial<SnapshotInput> = {}
): SnapshotInput {
  return {
    scope: 'common-page-1',
    data,
    updatedAt: 1,
    manualVersion: 0,
    hold: false,
    placeholder: false,
    ...fields,
  }
}

test('unchanged rows retain identity when new logs shift their array positions', () => {
  const before = page([row(2), row(1)])
  const after = shareLogPage(before, page([row(3), row(2)], 3))
  assert.equal(after.items[1], before.items[0])
  assert.equal(shareLogPage(before, page([row(2), row(1)])), before)
})

test('following the top of the first page immediately displays new logs', () => {
  const before = createLogSnapshot(input(page([row(2), row(1)])))
  const data = page([row(3), row(2)], 3)
  const after = advanceLogSnapshot(before, input(data, { updatedAt: 2 }))
  assert.equal(after.display, data)
  assert.equal(after.hasUpdates, false)
})

test('reading preserves order and updates pinned requests that left the page', () => {
  const before = createLogSnapshot(input(page([row(2, { type: 7 }), row(1)])))
  const data = {
    ...page([row(4), row(3)], 4),
    updates: [row(2, { type: 2, quota: 50, completion_tokens: 30 })],
  }
  const after = advanceLogSnapshot(
    before,
    input(data, { hold: true, updatedAt: 2 })
  )
  assert.deepEqual(
    after.display?.items.map((item) => item.id),
    [2, 1]
  )
  assert.equal((after.display?.items[0] as UsageLog).quota, 50)
  assert.equal((after.display?.items[0] as UsageLog).completion_tokens, 30)
  assert.equal(after.display?.items[1], before.display?.items[1])
  assert.equal(after.pendingCount, 2)
  assert.equal(after.hasUpdates, true)
})

test('pending count is cumulative without counting the same refresh twice', () => {
  const before = createLogSnapshot(input(page([row(2), row(1)])))
  const first = advanceLogSnapshot(
    before,
    input(page([row(3), row(2)], 3), { hold: true, updatedAt: 2 })
  )
  const second = advanceLogSnapshot(
    first,
    input(page([row(4), row(3)], 4), { hold: false, updatedAt: 3 })
  )
  assert.equal(second.pendingCount, 2)
  assert.deepEqual(
    second.display?.items.map((item) => item.id),
    [2, 1]
  )
  assert.equal(
    advanceLogSnapshot(second, input(second.source!, { updatedAt: 3 })),
    second
  )
})

test('status-only updates do not display a new-log notification', () => {
  const before = createLogSnapshot(input(page([row(2, { type: 7 }), row(1)])))
  const after = advanceLogSnapshot(
    before,
    input(page([row(2, { quota: 10 }), row(1)]), { hold: true, updatedAt: 2 })
  )
  assert.equal(after.hasUpdates, false)
  assert.equal((after.display?.items[0] as UsageLog).quota, 10)
})

test('pagination and filter changes discard buffered membership, including placeholders', () => {
  const before = createLogSnapshot(input(page([row(2), row(1)])))
  const placeholder = advanceLogSnapshot(
    before,
    input(before.source!, { scope: 'filtered-page-2', placeholder: true })
  )
  assert.equal(placeholder.display, undefined)
  const data = { ...page([row(10)]), page: 2 }
  const after = advanceLogSnapshot(
    placeholder,
    input(data, { scope: 'filtered-page-2', hold: true, updatedAt: 2 })
  )
  assert.equal(after.display, data)
  assert.equal(after.hasUpdates, false)
})

test('manual refresh accepts the next response while reading, even if data is structurally unchanged', () => {
  const before = createLogSnapshot(input(page([row(2), row(1)])))
  const buffered = advanceLogSnapshot(
    before,
    input(page([row(3), row(2)], 3), { hold: true, updatedAt: 2 })
  )
  const waiting = advanceLogSnapshot(
    buffered,
    input(buffered.source!, { hold: true, updatedAt: 2, manualVersion: 1 })
  )
  assert.equal(waiting.hasUpdates, true)
  const after = advanceLogSnapshot(
    waiting,
    input(buffered.source!, { hold: true, updatedAt: 3, manualVersion: 1 })
  )
  assert.equal(after.display, buffered.source)
  assert.equal(after.hasUpdates, false)
})

test('deletion or filter membership changes show an update notice without a negative new-log count', () => {
  const before = createLogSnapshot(input(page([row(2), row(1)], 10)))
  const after = advanceLogSnapshot(
    before,
    input(page([row(2), row(0)], 9), { hold: true, updatedAt: 2 })
  )
  assert.equal(after.pendingCount, 0)
  assert.equal(after.hasUpdates, true)
})

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
import { buildGroupStatsRows } from './group-stats'

test('places members directly below their combination in configured order', () => {
  const groups = [
    { group: 'standalone', quota: 100 },
    { group: 'C', quota: 60 },
    {
      group: 'A',
      isCombinationGroup: true,
      memberGroups: ['B', 'C'],
      quota: 90,
    },
    { group: 'B', quota: 30 },
  ]
  const rows = buildGroupStatsRows(groups)

  assert.deepEqual(
    rows.map((row) => [row.stats.group, row.depth]),
    [
      ['standalone', 0],
      ['A', 0],
      ['B', 1],
      ['C', 1],
    ]
  )
  assert.equal(rows[2].stats, groups[3])
  assert.equal(rows[3].stats, groups[1])
})

test('shows a shared member under each parent with independent row identities', () => {
  const rows = buildGroupStatsRows([
    { group: 'A', isCombinationGroup: true, memberGroups: ['B', 'C'] },
    { group: 'D', isCombinationGroup: true, memberGroups: ['C'] },
    { group: 'B' },
    { group: 'C' },
  ])

  assert.deepEqual(
    rows.map((row) => row.stats.group),
    ['A', 'B', 'C', 'D', 'C']
  )
  assert.equal(new Set(rows.map((row) => row.key)).size, rows.length)
})

test('keeps legacy flat data visible and skips members without statistics', () => {
  const groups = [
    { group: 'A', isCombinationGroup: true },
    { group: 'B' },
    {
      group: 'D',
      isCombinationGroup: true,
      memberGroups: ['missing', 'C', 'C'],
    },
    { group: 'C' },
  ]

  assert.deepEqual(
    buildGroupStatsRows(groups).map((row) => [row.stats.group, row.depth]),
    [
      ['A', 0],
      ['B', 0],
      ['D', 0],
      ['C', 1],
    ]
  )
})

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
import {
  groupTimedRatiosSchema,
  timedRatioRuleSchema,
} from './group-timed-ratios'

const daily = {
  enabled: true,
  ratio: 0,
  kind: 'daily',
  start: '23:00',
  end: '02:00',
  remark: 'night',
}
test('accepts zero ratios, recurring wrap, midnight, hourly and weekly schedules', () => {
  for (const rule of [
    daily,
    { ...daily, start: '00:00', end: '24:00' },
    { ...daily, kind: 'hourly', start: '00:45', end: '00:15' },
    { ...daily, kind: 'hourly', start: '00:00', end: '01:00' },
    { ...daily, kind: 'weekly', weekdays: [0, 1, 6] },
    {
      ...daily,
      kind: 'once',
      start: '2028-02-29T09:00',
      end: '2028-02-29T10:00',
    },
  ])
    assert.equal(
      groupTimedRatiosSchema.safeParse(JSON.stringify({ a: [rule] })).success,
      true
    )
})
test('rejects malformed or incomplete rules before saving', () => {
  for (const rule of [
    { ...daily, ratio: -1 },
    { ...daily, ratio: null },
    { ...daily, start: '23:00', end: '23:00' },
    { ...daily, kind: 'weekly', weekdays: [] },
    { ...daily, kind: 'weekly', weekdays: [7] },
    { ...daily, kind: 'hourly', start: '01:15', end: '01:45' },
    {
      ...daily,
      kind: 'once',
      start: '2026-02-29T09:00',
      end: '2026-03-01T10:00',
    },
    {
      ...daily,
      kind: 'once',
      start: '2026-10-13T10:00',
      end: '2026-10-13T09:00',
    },
  ])
    assert.equal(timedRatioRuleSchema.safeParse(rule).success, false)
  for (const value of ['null', '[]', '{', '{" ":[]}'])
    assert.equal(groupTimedRatiosSchema.safeParse(value).success, false)
})

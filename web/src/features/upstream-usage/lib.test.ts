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
import { describe, test } from 'node:test'
import {
  estimateUsageCapacity,
  formatResetCountdown,
  nextUsageReset,
  remainingUsage,
  usageColor,
} from './lib'
import { usageProviderSchema } from './schema'
import type { UsageAccount } from './types'

function account(id: number, five: number, seven: number): UsageAccount {
  const window = (utilization: number) => ({
    utilization,
    resets_at: '',
    remaining_seconds: 0,
    window_stats: null,
  })
  return {
    id,
    account_id: id,
    available: true,
    groups: ['a'],
    fetched_at: 1,
    last_error: '',
    usage: {
      updated_at: '',
      five_hour: window(five),
      seven_day: window(seven),
    },
  }
}

describe('upstream usage battery', () => {
  test('reset countdown uses days/hours or hours/minutes and clamps elapsed resets', () => {
    const now = Date.parse('2026-09-28T00:00:00Z')
    assert.equal(formatResetCountdown(now + 49 * 3600000, now), '2 d 1 h')
    assert.equal(formatResetCountdown(now + 24 * 3600000, now), '1 d 0 h')
    assert.equal(
      formatResetCountdown(now + 23 * 3600000 + 45 * 60000, now),
      '23 h 45 m'
    )
    assert.equal(formatResetCountdown(now + 1000, now), '0 h 1 m')
    assert.equal(formatResetCountdown(now - 1000, now), '0 h 0 m')
  })
  test('groups count down to the earliest account reset without inventing missing dates', () => {
    const first = account(1, 100, 100)
    const second = account(2, 100, 100)
    first.usage!.seven_day!.resets_at = '2026-09-30T00:00:00+08:00'
    second.usage!.seven_day!.resets_at = '2026-09-29T00:00:00+08:00'
    assert.equal(
      nextUsageReset([first, second]),
      Date.parse(second.usage!.seven_day!.resets_at)
    )
    assert.equal(nextUsageReset([]), null)
    second.usage!.seven_day!.resets_at = ''
    assert.equal(nextUsageReset([first, second]), null)
  })
  test('estimates total capacity from utilization, not time elapsed', () => {
    const window = {
      utilization: 20,
      resets_at: '',
      remaining_seconds: 0,
      window_stats: {
        cost: 78,
        tokens: 46000000,
        requests: 10,
        standard_cost: 90,
      },
    }
    assert.deepEqual(estimateUsageCapacity(window), {
      cost: 390,
      tokens: 230000000,
    })
    assert.deepEqual(estimateUsageCapacity({ ...window, utilization: 100 }), {
      cost: 78,
      tokens: 46000000,
    })
    for (const utilization of [0, -1, 101, NaN, Infinity]) {
      assert.equal(estimateUsageCapacity({ ...window, utilization }), null)
    }
    assert.equal(estimateUsageCapacity({ ...window, window_stats: null }), null)
    assert.equal(
      estimateUsageCapacity({
        ...window,
        window_stats: { ...window.window_stats, cost: Infinity },
      }),
      null
    )
  })
  test('invalid URLs produce validation errors without throwing', () => {
    for (const base_url of [
      'invalid',
      'ftp://example.com',
      'https://user:password@example.com',
    ]) {
      assert.equal(
        usageProviderSchema.safeParse({
          base_url,
          access_token: '',
          refresh_token: '',
          interval_minutes: 5,
          accounts: [],
        }).success,
        false
      )
    }
  })
  test('averages remaining quota across accounts, including explicit zero', () => {
    const accounts = [account(1, 0, 30), account(2, 80, 90)]
    assert.equal(remainingUsage(accounts, 'seven_day'), 40)
    assert.equal(remainingUsage(accounts, 'five_hour'), 60)
    assert.equal(remainingUsage([account(1, 0, 0)], 'five_hour'), 100)
    assert.equal(remainingUsage([account(1, 100, 100)], 'five_hour'), 0)
  })
  test('missing, failed and stale accounts must not imply available capacity', () => {
    assert.equal(remainingUsage([], 'seven_day'), null)
    assert.equal(
      remainingUsage(
        [account(1, 0, 0), { ...account(2, 0, 0), available: false }],
        'seven_day'
      ),
      null
    )
    const missing = account(1, 0, 0)
    missing.usage!.five_hour = null
    assert.equal(remainingUsage([missing], 'five_hour'), null)
    assert.equal(remainingUsage([missing], 'seven_day'), 100)
  })
  test('color thresholds include 10 and 30 in yellow', () => {
    assert.match(usageColor(9.9), /red/)
    assert.match(usageColor(10), /yellow/)
    assert.match(usageColor(30), /yellow/)
    assert.match(usageColor(30.1), /green/)
    assert.match(usageColor(null), /muted/)
  })
})

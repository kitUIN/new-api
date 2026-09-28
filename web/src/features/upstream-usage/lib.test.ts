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
import { remainingUsage, usageColor } from './lib'
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

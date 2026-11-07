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
import { strict as assert } from 'node:assert'
import { test } from 'node:test'
import {
  formatBytes,
  formatRate,
  historyBuckets,
  maskIpAddress,
  remainingQuotaPercent,
} from './lib'

test('quota and rate formatting distinguish absent values, zero quotas, and bits from bytes', () => {
  assert.equal(formatBytes(null), '—')
  assert.equal(formatBytes(0), '0 B')
  assert.equal(formatBytes(1024), '1.00 KiB')
  assert.equal(formatRate(null), '—')
  assert.equal(formatRate(undefined), '—')
  assert.equal(formatRate(Number.NaN), '—')
  assert.equal(formatRate(0), '0 B/s')
  assert.equal(formatRate(1000), '125 B/s')
  assert.equal(formatRate(8000), '1.00 KB/s')
  assert.equal(formatRate(8_000_000), '1.00 MB/s')
  assert.equal(formatRate(8_000_000_000), '1.00 GB/s')
  assert.equal(remainingQuotaPercent(null, 1024), null)
  assert.equal(remainingQuotaPercent(1024, null), null)
  assert.equal(remainingQuotaPercent(0, 0), 0)
  assert.equal(remainingQuotaPercent(1, 0), 0)
  assert.equal(remainingQuotaPercent(0, 1024), 100)
  assert.equal(remainingQuotaPercent(256, 1024), 75)
  assert.equal(remainingQuotaPercent(1024, 1024), 0)
  assert.equal(remainingQuotaPercent(2048, 1024), 0)
})

test('IP masking never exposes the last three IPv4 segments or non-IPv4 addresses', () => {
  assert.equal(maskIpAddress('154.44.14.120'), '154.*.*.*')
  assert.equal(maskIpAddress('0.0.0.0'), '0.*.*.*')
  assert.equal(maskIpAddress('2001:db8::1'), '****')
  assert.equal(maskIpAddress('::ffff:192.0.2.1'), '****')
  assert.equal(maskIpAddress('unexpected-address'), '****')
  assert.equal(maskIpAddress(null), '—')
  assert.equal(maskIpAddress(''), '—')
})

test('timeline clips intervals at bucket boundaries and retains short outages and unknown time', () => {
  const buckets = historyBuckets(
    {
      from: '2026-11-07T00:00:00Z',
      to: '2026-11-07T00:02:00Z',
      asOf: '2026-11-07T00:02:00Z',
      onlineSeconds: 59,
      offlineSeconds: 1,
      unknownSeconds: 60,
      intervals: [
        {
          from: '2026-11-06T23:59:00Z',
          to: '2026-11-07T00:00:59Z',
          status: 'online',
        },
        {
          from: '2026-11-07T00:00:59Z',
          to: '2026-11-07T00:01:00Z',
          status: 'offline',
        },
        {
          from: '2026-11-07T00:01:00Z',
          to: '2026-11-07T00:03:00Z',
          status: 'unknown',
        },
      ],
    },
    2
  )
  assert.deepEqual(buckets[0].seconds, { online: 59, offline: 1, unknown: 0 })
  assert.deepEqual(buckets[1].seconds, { online: 0, offline: 0, unknown: 60 })
})

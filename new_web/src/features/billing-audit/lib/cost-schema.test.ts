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
import { isBillingDate, nextBillingDate } from '../lib'
import type { CostInput } from '../types'
import { costSchema } from './cost-schema'

const messages = {
  invalidCost: 'cost',
  invalidMonth: 'month',
  invalidDates: 'dates',
}
const base: CostInput = {
  month: '2026-10',
  name: 'Subscription',
  amount: '1400',
  remark: '',
  recurring: false,
  scope: 'month',
  allocation: 'subscription',
  start_date: '2026-10-07',
  end_date: '2026-11-07',
}

test('subscription dates default to next month, clamp month end and cross year', () => {
  assert.equal(nextBillingDate('2026-10-07'), '2026-11-07')
  assert.equal(nextBillingDate('2027-01-31'), '2027-02-28')
  assert.equal(nextBillingDate('2028-01-31'), '2028-02-29')
  assert.equal(nextBillingDate('2026-12-07'), '2027-01-07')
  assert.equal(nextBillingDate('2026-02-29'), '')
  assert.equal(isBillingDate('2028-02-29'), true)
  assert.equal(isBillingDate('2026-02-31'), false)
})

test('single subscriptions can span months but monthly renewals require the anniversary', () => {
  const schema = costSchema({ kind: 'create' }, messages)
  assert.equal(schema.safeParse(base).success, true)
  assert.equal(schema.safeParse({ ...base, recurring: true }).success, true)
  assert.equal(
    schema.safeParse({ ...base, end_date: '2027-10-07' }).success,
    true
  )
  assert.equal(
    schema.safeParse({ ...base, recurring: true, end_date: '2027-10-07' })
      .success,
    false
  )
  for (const end_date of ['', '2026-10-07', '2026-10-06', '2027-02-29']) {
    assert.equal(schema.safeParse({ ...base, end_date }).success, false)
  }
})

test('legacy monthly costs do not require subscription dates', () => {
  assert.equal(
    costSchema({ kind: 'create' }, messages).safeParse({
      ...base,
      allocation: 'month',
      start_date: '',
      end_date: '',
    }).success,
    true
  )
})

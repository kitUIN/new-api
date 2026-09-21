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
import { z } from 'zod'
import { isBillingDate, nextBillingDate } from '../lib'
import type { CostAction } from '../types'

export function costSchema(
  action: CostAction,
  messages: { invalidCost: string; invalidMonth: string; invalidDates: string }
) {
  return z
    .object({
      month: z.string().regex(/^(?:20\d{2}|[3-9]\d{3})-(0[1-9]|1[0-2])$/),
      name: z.string().trim().max(128),
      amount: z.string(),
      remark: z.string().trim().max(1000),
      recurring: z.boolean(),
      scope: z.enum(['month', 'future']),
      allocation: z.enum(['month', 'subscription']),
      start_date: z.string(),
      end_date: z.string(),
    })
    .superRefine((value, ctx) => {
      if (
        value.month > '9998-12' ||
        (action.cost && value.month < action.cost.start_month)
      ) {
        ctx.addIssue({
          code: 'custom',
          path: ['month'],
          message: messages.invalidMonth,
        })
      }
      if (
        action.kind !== 'delete' &&
        (!value.name ||
          !/^\d+(\.\d{1,2})?$/.test(value.amount) ||
          Number(value.amount) <= 0 ||
          Number(value.amount) > 1_000_000_000)
      ) {
        ctx.addIssue({
          code: 'custom',
          path: ['amount'],
          message: messages.invalidCost,
        })
      }
      if (action.kind === 'create' && value.allocation === 'subscription') {
        if (
          !isBillingDate(value.start_date) ||
          !isBillingDate(value.end_date) ||
          value.end_date <= value.start_date ||
          (value.recurring &&
            value.end_date !== nextBillingDate(value.start_date))
        ) {
          ctx.addIssue({
            code: 'custom',
            path: ['end_date'],
            message: messages.invalidDates,
          })
        }
      }
    })
}

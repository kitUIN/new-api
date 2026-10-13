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
import * as z from 'zod'

const clock = /^([01]\d|2[0-3]):[0-5]\d$/
const endClock = /^(([01]\d|2[0-3]):[0-5]\d|24:00)$/
const dateTime = /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}$/
function validDate(value: string): boolean {
  if (!dateTime.test(value)) return false
  const parsed = new Date(`${value}:00Z`)
  return (
    Number.isFinite(parsed.getTime()) &&
    parsed.toISOString().slice(0, 16) === value
  )
}

export const timedRatioRuleSchema = z
  .object({
    enabled: z.boolean(),
    ratio: z.number().finite().min(0),
    kind: z.enum(['once', 'hourly', 'daily', 'weekly']),
    start: z.string(),
    end: z.string(),
    weekdays: z.array(z.number().int().min(0).max(6)).optional(),
    remark: z.string(),
  })
  .refine((rule) => {
    if (rule.start === rule.end) return false
    if (rule.kind === 'once') {
      return (
        validDate(rule.start) && validDate(rule.end) && rule.end > rule.start
      )
    }
    if (rule.kind === 'hourly') {
      return (
        /^00:[0-5]\d$/.test(rule.start) && /^(00:[0-5]\d|01:00)$/.test(rule.end)
      )
    }
    if (!clock.test(rule.start) || !endClock.test(rule.end)) return false
    return rule.kind !== 'weekly' || Boolean(rule.weekdays?.length)
  })

export type TimedRatioRule = z.infer<typeof timedRatioRuleSchema>
export const timedRatiosMapSchema = z.record(
  z.string().trim().min(1),
  z.array(timedRatioRuleSchema)
)
export const groupTimedRatiosSchema = z.string().refine((value) => {
  try {
    return timedRatiosMapSchema.safeParse(JSON.parse(value)).success
  } catch {
    return false
  }
}, 'timedRatio.invalid')

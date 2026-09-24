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

export const openingHoursMapSchema = z.record(
  z.string().min(1),
  z.array(
    z
      .object({
        start: z.string().regex(/^([01]\d|2[0-3]):[0-5]\d$/),
        end: z.string().regex(/^(([01]\d|2[0-3]):[0-5]\d|24:00)$/),
      })
      .refine((interval) => interval.start !== interval.end)
  )
)

export const groupOpeningHoursSchema = z.string().refine((value) => {
  try {
    return openingHoursMapSchema.safeParse(JSON.parse(value)).success
  } catch {
    return false
  }
}, 'Invalid opening hours. Use HH:mm intervals with different start and end times.')

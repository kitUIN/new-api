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

export const usageProviderSchema = z.object({
  base_url: z
    .string()
    .trim()
    .url()
    .refine((value) => {
      try {
        const url = new URL(value)
        return (
          ['https:', 'http:'].includes(url.protocol) &&
          !url.username &&
          !url.password &&
          !url.search &&
          !url.hash
        )
      } catch {
        return false
      }
    }),
  access_token: z.string().trim(),
  refresh_token: z.string().trim(),
  interval_minutes: z.number().int().min(1).max(1440),
  accounts: z
    .array(
      z.object({
        account_id: z.number().int().positive().max(Number.MAX_SAFE_INTEGER),
        groups: z.array(z.string().trim().min(1).max(128)),
      })
    )
    .max(200)
    .refine(
      (accounts) =>
        new Set(accounts.map((a) => a.account_id)).size === accounts.length
    ),
})

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

export const parameterLimits = {
  temperature: { min: 0, max: 2, step: 0.1 },
  top_p: { min: 0, max: 1, step: 0.1 },
  max_tokens: { min: 1, max: Number.MAX_SAFE_INTEGER, step: 1 },
  frequency_penalty: { min: -2, max: 2, step: 0.1 },
  presence_penalty: { min: -2, max: 2, step: 0.1 },
  seed: { min: Number.MIN_SAFE_INTEGER, max: Number.MAX_SAFE_INTEGER, step: 1 },
} as const

export const parameterSchema = z.object({
  values: z.object({
    temperature: z.number().min(0).max(2),
    top_p: z.number().min(0).max(1),
    max_tokens: z.number().int().min(1),
    frequency_penalty: z.number().min(-2).max(2),
    presence_penalty: z.number().min(-2).max(2),
    seed: z.number().int(),
    stream: z.boolean(),
  }),
  enabled: z.object({
    temperature: z.boolean(),
    top_p: z.boolean(),
    max_tokens: z.boolean(),
    frequency_penalty: z.boolean(),
    presence_penalty: z.boolean(),
    seed: z.boolean(),
  }),
})

export type ParameterFormValues = z.infer<typeof parameterSchema>

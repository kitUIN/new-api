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

export function serviceProbeSettingsSchema(
  t: (key: string) => string,
  hasKey: boolean
) {
  return z
    .object({
      enabled: z.boolean(),
      url: z
        .string()
        .trim()
        .max(2048)
        .refine((value) => {
          if (!value) return true
          try {
            const url = new URL(value)
            return (
              ['http:', 'https:'].includes(url.protocol) &&
              !!url.hostname &&
              !url.username &&
              !url.password &&
              !value.includes('?') &&
              !value.includes('#')
            )
          } catch {
            return false
          }
        }, t('serviceProbe.invalidConfig')),
      read_api_key: z
        .string()
        .trim()
        .refine(
          (value) =>
            !value ||
            (value.length >= 32 &&
              value.length <= 4096 &&
              !Array.from(value).some(
                (char) => char.charCodeAt(0) < 32 || char.charCodeAt(0) === 127
              )),
          t('serviceProbe.settings.invalidKey')
        ),
    })
    .superRefine((values, ctx) => {
      if (values.enabled && !values.url)
        ctx.addIssue({
          code: 'custom',
          path: ['url'],
          message: t('serviceProbe.settings.required'),
        })
      if (values.enabled && !values.read_api_key && !hasKey)
        ctx.addIssue({
          code: 'custom',
          path: ['read_api_key'],
          message: t('serviceProbe.settings.required'),
        })
    })
}

export type ServiceProbeSettingsValues = z.infer<
  ReturnType<typeof serviceProbeSettingsSchema>
>

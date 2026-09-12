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

export const MAX_TICKET_IMAGES = 4
export const MAX_TICKET_IMAGE_BYTES = 2 * 1024 * 1024
export const TICKET_IMAGE_TYPES = [
  'image/png',
  'image/jpeg',
  'image/gif',
  'image/webp',
]

export function ticketSchema(creating: boolean) {
  return z
    .object({
      title: z
        .string()
        .trim()
        .refine(
          (value) =>
            !creating || (value.length > 0 && [...value].length <= 120),
          'tickets.titleInvalid'
        ),
      content: z
        .string()
        .trim()
        .refine(
          (value) => [...value].length <= 10000,
          'tickets.contentTooLong'
        ),
      images: z
        .array(z.instanceof(File))
        .max(MAX_TICKET_IMAGES, 'tickets.imagesLimit')
        .refine(
          (images) =>
            images.every(
              (file) =>
                file.size > 0 &&
                file.size <= MAX_TICKET_IMAGE_BYTES &&
                TICKET_IMAGE_TYPES.includes(file.type)
            ),
          'tickets.imageInvalid'
        ),
    })
    .refine((value) => value.content.length > 0 || value.images.length > 0, {
      path: ['content'],
      message: 'tickets.messageRequired',
    })
}

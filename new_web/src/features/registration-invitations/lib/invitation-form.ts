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

export const invitationFormSchema = z.object({
  code: z
    .string()
    .trim()
    .min(1, 'Invitation code is required')
    .regex(/^\d{5,20}$/, 'Invitation code must be a valid QQ number'),
  remark: z
    .string()
    .trim()
    .min(1, 'Remark is required')
    .max(255, 'Remark must be at most 255 characters'),
})

export type InvitationFormValues = z.infer<typeof invitationFormSchema>

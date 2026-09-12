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
import { api } from '@/lib/api'
import type {
  Ticket,
  TicketInput,
  TicketList,
  TicketMessagePage,
} from './types'

type Response<T> = { success: boolean; message: string; data: T }

function unwrap<T>(response: Response<T>): T {
  if (!response.success) throw new Error(response.message)
  return response.data
}

export async function getTickets(
  page: number,
  status: string
): Promise<TicketList> {
  const response = await api.get<Response<TicketList>>('/api/ticket/', {
    params: { p: page, page_size: 20, status },
  })
  return unwrap(response.data)
}

export async function getTicket(id: number): Promise<Ticket> {
  const response = await api.get<Response<Ticket>>(`/api/ticket/${id}`)
  return unwrap(response.data)
}

export async function getTicketMessages(
  id: number,
  before: number
): Promise<TicketMessagePage> {
  const response = await api.get<Response<TicketMessagePage>>(
    `/api/ticket/${id}/messages`,
    { params: { before } }
  )
  return unwrap(response.data)
}

export async function sendTicketMessage(
  input: TicketInput,
  id?: number
): Promise<Ticket> {
  const body = new FormData()
  body.append('title', input.title)
  body.append('content', input.content)
  input.images.forEach((file) => body.append('images', file))
  const url = id ? `/api/ticket/${id}/messages` : '/api/ticket/'
  const response = await api.post<Response<Ticket>>(url, body)
  return unwrap(response.data)
}

export async function closeTicket(id: number): Promise<Ticket> {
  const response = await api.post<Response<Ticket>>(`/api/ticket/${id}/close`)
  return unwrap(response.data)
}

export async function getTicketImage(
  ticketId: number,
  attachmentId: number,
  signal: AbortSignal
): Promise<Blob> {
  const response = await api.get<Blob>(
    `/api/ticket/${ticketId}/attachments/${attachmentId}`,
    {
      responseType: 'blob',
      signal,
      ...{ disableDuplicate: true },
    }
  )
  return response.data
}

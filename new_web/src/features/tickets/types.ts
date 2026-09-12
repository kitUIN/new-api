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
export type Ticket = {
  id: number
  user_id: number
  username: string
  title: string
  status: 'open' | 'closed'
  last_reply_by_admin: boolean
  message_count: number
  created_time: number
  updated_time: number
  closed_time: number
  closed_by: number
}

export type TicketAttachment = {
  id: number
  ticket_id: number
  message_id: number
  mime_type: string
  size: number
}

export type TicketMessage = {
  id: number
  ticket_id: number
  user_id: number
  username: string
  is_admin: boolean
  content: string
  created_time: number
  attachments: TicketAttachment[]
}

export type TicketMessagePage = {
  items: TicketMessage[]
  has_more: boolean
}

export type TicketList = {
  items: Ticket[]
  total: number
  page: number
  page_size: number
}

export type TicketInput = { title: string; content: string; images: File[] }

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

export interface RelaySession {
  id: string
  user_id: number
  username: string
  token_id: number
  token_name: string
  session_key: string
  source: string
  model_name: string
  requested_group: string
  last_group: string
  override_group: string
  created_at: number
  last_seen_at: number
}

interface ApiResponse<T> {
  success: boolean
  message?: string
  data: T
}

export async function getRelaySessions(
  isAdmin: boolean,
  page: number,
  search: string
) {
  const path = isAdmin ? '/api/log/sessions' : '/api/log/self/sessions'
  const response = await api.get<
    ApiResponse<{ items: RelaySession[]; total: number }>
  >(path, {
    params: { p: page, page_size: 20, search },
  })
  if (!response.data.success) throw new Error(response.data.message)
  return response.data.data
}

export async function getRelaySessionGroups(id: string) {
  const response = await api.get<ApiResponse<string[]>>(
    `/api/log/sessions/${id}/groups`
  )
  if (!response.data.success) throw new Error(response.data.message)
  return response.data.data
}

export async function updateRelaySessionGroup(id: string, group: string) {
  const response = await api.put<ApiResponse<null>>(
    `/api/log/sessions/${id}/group`,
    { group }
  )
  if (!response.data.success) throw new Error(response.data.message)
}

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
import type { UsageAccount, UsageProvider, UsageProviderInput } from './types'

type Response<T> = { success: boolean; message?: string; data: T }
function unwrap<T>(response: Response<T>): T {
  if (!response.success) throw new Error(response.message || 'Request failed')
  return response.data
}

export async function getUpstreamUsage(): Promise<UsageProvider[]> {
  return unwrap(
    (await api.get<Response<UsageProvider[]>>('/api/upstream-usage')).data
  )
}

export async function getGroupUpstreamUsage(): Promise<
  Record<string, UsageAccount[]>
> {
  return unwrap(
    (
      await api.get<Response<Record<string, UsageAccount[]>>>(
        '/api/perf-metrics/upstream-usage'
      )
    ).data
  )
}

export async function saveUpstreamUsage(
  id: number | undefined,
  input: UsageProviderInput
): Promise<void> {
  const path = id ? `/api/upstream-usage/${id}` : '/api/upstream-usage'
  const response = id
    ? await api.put<Response<null>>(path, input)
    : await api.post<Response<null>>(path, input)
  unwrap(response.data)
}

export async function deleteUpstreamUsage(id: number): Promise<void> {
  unwrap((await api.delete<Response<null>>(`/api/upstream-usage/${id}`)).data)
}

export async function refreshUpstreamUsage(
  id: number
): Promise<UsageProvider[]> {
  return unwrap(
    (
      await api.post<Response<UsageProvider[]>>(
        `/api/upstream-usage/${id}/refresh`,
        {},
        { timeout: 0 }
      )
    ).data
  )
}

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
  CurrentTraffic,
  OnlineHistory,
  ProbeOverview,
  ProbeWindow,
} from './types'

async function read<T>(
  path: string,
  signal?: AbortSignal,
  params?: Record<string, string>
): Promise<T> {
  const result = await api.get<{ success: boolean; message?: string; data: T }>(
    `/api/service-probes${path}`,
    // React Query owns cancellation and deduplication. Sharing an Axios promise
    // across observers can reuse a request whose AbortSignal was already aborted.
    {
      signal,
      params,
      ...{
        disableDuplicate: true,
        skipBusinessError: true,
        skipErrorHandler: true,
      },
    }
  )
  if (!result.data.success)
    throw new Error(result.data.message || 'serviceProbe.loadFailed')
  return result.data.data
}

export const getProbes = (signal?: AbortSignal) =>
  read<ProbeOverview>('', signal)
export const getCurrent = (id: string, signal?: AbortSignal) =>
  read<CurrentTraffic | null>(`/${encodeURIComponent(id)}/current`, signal)
export const getHistory = (
  id: string,
  window: ProbeWindow,
  signal?: AbortSignal
) =>
  read<OnlineHistory>(`/${encodeURIComponent(id)}/history`, signal, { window })

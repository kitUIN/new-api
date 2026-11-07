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
export type ProbeWindow = '1h' | '24h' | '7d'

export interface BeeQuota {
  id: string
  name: string | null
  region: string | null
  ip: string | null
  online: boolean
  quota: {
    limitBytes: number | null
    period: string
    resetDay: number
    utcOffsetHours: number
  }
  periodStart: string
  periodEnd: string
  receivedBytes: number | null
  sentBytes: number | null
  totalBytes: number | null
  remainingBytes: number | null
  isExceeded: boolean | null
  hasGaps: boolean
  lastSampledAt: string | null
  lastReceivedAt: string | null
}

export interface ProbeOverview {
  configured: boolean
  quotas: { asOf: string; bees: BeeQuota[] } | null
}

export interface CurrentTraffic {
  sampledAt: string
  available: boolean
  downloadBitsPerSecond: number | null
  uploadBitsPerSecond: number | null
}

export type OnlineStatus = 'online' | 'offline' | 'unknown'
export interface OnlineHistory {
  from: string
  to: string
  asOf: string
  onlineSeconds: number
  offlineSeconds: number
  unknownSeconds: number
  intervals: { from: string; to: string; status: OnlineStatus }[]
}

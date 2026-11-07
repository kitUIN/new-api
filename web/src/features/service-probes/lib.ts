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
import type { OnlineHistory } from './types'

function formatUnit(
  value: number | null | undefined,
  base: number,
  units: string[]
): string {
  if (value == null || !Number.isFinite(value)) return '—'
  const index =
    value > 0
      ? Math.min(Math.floor(Math.log(value) / Math.log(base)), units.length - 1)
      : 0
  const safeIndex = Math.max(0, index)
  return `${(value / base ** safeIndex).toFixed(safeIndex === 0 ? 0 : 2)} ${units[safeIndex]}`
}

export const formatBytes = (value: number | null | undefined): string =>
  formatUnit(value, 1024, ['B', 'KiB', 'MiB', 'GiB', 'TiB', 'PiB'])
export const formatRate = (value: number | null | undefined): string =>
  formatUnit(value == null ? value : value / 8, 1000, [
    'B/s',
    'KB/s',
    'MB/s',
    'GB/s',
    'TB/s',
  ])

export function maskIpAddress(ip: string | null): string {
  if (!ip) return '—'
  const ipv4 = /^(\d{1,3})\.\d{1,3}\.\d{1,3}\.\d{1,3}$/.exec(ip)
  return ipv4 ? `${ipv4[1]}.*.*.*` : '****'
}

export const formatTime = (
  value: string | null | undefined,
  language: string
): string =>
  value
    ? new Date(value).toLocaleString(language, {
        month: '2-digit',
        day: '2-digit',
        hour: '2-digit',
        minute: '2-digit',
        second: '2-digit',
      })
    : '—'

export function remainingQuotaPercent(
  used: number | null,
  limit: number | null
): number | null {
  if (used === null || limit === null) return null
  if (limit <= 0) return 0
  return Math.min(100, Math.max(0, ((limit - used) / limit) * 100))
}

export const formatCompactTime = (value: string, language: string): string =>
  new Date(value).toLocaleString(language, {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  })

// Fixed time buckets keep the timeline readable even with thousands of reconnects.
// Preserve every outage and unknown span in each bucket's duration summary.
export function historyBuckets(history: OnlineHistory, count = 48) {
  const start = Date.parse(history.from)
  const end = Date.parse(history.to)
  const width = (end - start) / count
  const intervals = history.intervals.map((interval) => ({
    from: Date.parse(interval.from),
    to: Date.parse(interval.to),
    status: interval.status,
  }))
  return Array.from({ length: count }, (_, index) => {
    const from = start + width * index
    const to = from + width
    const seconds = { online: 0, offline: 0, unknown: 0 }
    for (const interval of intervals) {
      const overlap =
        Math.max(0, Math.min(to, interval.to) - Math.max(from, interval.from)) /
        1000
      seconds[interval.status] += overlap
    }
    return {
      from: new Date(from).toISOString(),
      to: new Date(to).toISOString(),
      seconds,
    }
  })
}

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
import type { UsageAccount, UsageWindow } from './types'

// Estimate the full window's capacity from the upstream's reported usage ratio.
export function estimateUsageCapacity(window: UsageWindow): {
  cost: number
  tokens: number
} | null {
  const stats = window.window_stats
  if (
    !stats ||
    !Number.isFinite(window.utilization) ||
    window.utilization <= 0 ||
    window.utilization > 100 ||
    !Number.isFinite(stats.cost) ||
    !Number.isFinite(stats.tokens) ||
    stats.cost < 0 ||
    stats.tokens < 0
  )
    return null
  const cost = (stats.cost / window.utilization) * 100
  const tokens = (stats.tokens / window.utilization) * 100
  return Number.isFinite(cost) && Number.isFinite(tokens)
    ? { cost, tokens }
    : null
}

export function remainingUsage(
  accounts: UsageAccount[],
  window: 'five_hour' | 'seven_day'
): number | null {
  if (!accounts.length || accounts.some((account) => !account.available))
    return null
  const windows = accounts.map((account) => account.usage?.[window])
  if (windows.some((value) => !value || !Number.isFinite(value.utilization)))
    return null
  return (
    windows.reduce(
      (sum, value) =>
        sum + 100 - Math.min(100, Math.max(0, value!.utilization)),
      0
    ) / windows.length
  )
}

export function usageColor(remaining: number | null): string {
  if (remaining === null) return 'text-muted-foreground'
  if (remaining < 10) return 'text-red-400/80 dark:text-red-300/70'
  if (remaining <= 30) return 'text-yellow-500/70 dark:text-yellow-200/70'
  return 'text-green-500/65 dark:text-green-300/70'
}

export function nextUsageReset(accounts: UsageAccount[]): number | null {
  if (!accounts.length) return null
  const resets = accounts.map((account) =>
    Date.parse(account.usage?.seven_day?.resets_at ?? '')
  )
  if (resets.some((reset) => !Number.isFinite(reset))) return null
  return Math.min(...resets)
}

export function formatResetCountdown(resetAt: number, now: number): string {
  const minutes = Math.max(0, Math.ceil((resetAt - now) / 60000))
  const hours = Math.floor(minutes / 60)
  if (hours >= 24) return `${Math.floor(hours / 24)}d ${hours % 24}h`
  return `${hours}h ${minutes % 60}m`
}

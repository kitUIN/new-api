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
import type { UsageAccount } from './types'

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

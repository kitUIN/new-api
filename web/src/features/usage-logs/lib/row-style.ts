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
import { cn } from '@/lib/utils'
import { LOG_TYPE_ENUM } from '../constants'
import type { LogCategory } from '../types'
import { getServiceTierBillingMultiplier, parseLogOther } from './format'
import { isDisplayableLogType } from './utils'

const logTypeRowTint: Record<number, string> = {
  [LOG_TYPE_ENUM.PENDING]: 'bg-amber-50/40 dark:bg-amber-950/20',
  [LOG_TYPE_ENUM.ERROR]: 'bg-rose-50/40 dark:bg-rose-950/20',
  [LOG_TYPE_ENUM.REFUND]: 'bg-blue-50/30 dark:bg-blue-950/15',
}

export function getUsageLogRowClassName(
  log: Record<string, unknown>,
  category: LogCategory
): string | undefined {
  if (category !== 'common' || typeof log.type !== 'number') return undefined

  const other =
    isDisplayableLogType(log.type) && typeof log.other === 'string'
      ? parseLogOther(log.other)
      : null
  const isFastUltra =
    other?.reasoning_effort?.trim().toLowerCase() === 'ultra' &&
    getServiceTierBillingMultiplier(other) > 1

  return cn(
    logTypeRowTint[log.type],
    isFastUltra && 'usage-log-fast-ultra',
    isFastUltra &&
      log.type === LOG_TYPE_ENUM.ERROR &&
      'usage-log-fast-ultra-error',
    isFastUltra &&
      log.type === LOG_TYPE_ENUM.REFUND &&
      'usage-log-fast-ultra-refund'
  )
}

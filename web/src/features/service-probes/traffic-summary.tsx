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
import {
  Alert02Icon,
  ArrowDown02Icon,
  ArrowUp02Icon,
  ArrowUpDownIcon,
} from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useTranslation } from 'react-i18next'
import { cn } from '@/lib/utils'
import { ProbeHint } from './hint'
import { formatBytes, formatTime, remainingQuotaPercent } from './lib'
import { QuotaRing } from './quota-ring'
import type { BeeQuota } from './types'

export function TrafficSummary(props: { bee: BeeQuota }) {
  const { t, i18n } = useTranslation()
  const bee = props.bee
  const percent = remainingQuotaPercent(bee.totalBytes, bee.quota.limitBytes)
  const resetHint = t('serviceProbe.resetAt', {
    time: formatTime(bee.periodEnd, i18n.language),
  })
  const usageHint =
    bee.totalBytes === null
      ? t('serviceProbe.noCycleData')
      : t('serviceProbe.cycleTraffic')
  const usageDetails = [
    usageHint,
    resetHint,
    bee.hasGaps ? t('serviceProbe.gaps') : null,
  ]
    .filter(Boolean)
    .join(' · ')
  return (
    <div className='flex min-w-0 items-center gap-3'>
      <div className='flex min-w-0 flex-1 flex-col items-start gap-1.5 text-xs tabular-nums'>
        <ProbeHint
          label={usageDetails}
          className='flex min-w-0 flex-wrap items-center gap-1.5'
        >
          <HugeiconsIcon
            icon={ArrowUpDownIcon}
            className='text-muted-foreground size-3.5 shrink-0'
            aria-hidden='true'
          />
          <span
            className={cn(
              'font-semibold',
              bee.isExceeded && 'text-destructive'
            )}
          >
            {formatBytes(bee.totalBytes)}
          </span>
          <span className='text-muted-foreground'>
            /{' '}
            {bee.quota.limitBytes === null
              ? '∞'
              : formatBytes(bee.quota.limitBytes)}
          </span>
          {bee.quota.limitBytes === null && (
            <span className='sr-only'>{t('serviceProbe.unlimited')}</span>
          )}
        </ProbeHint>
        <div className='text-muted-foreground flex flex-wrap items-center gap-x-2 gap-y-1'>
          <ProbeHint
            label={`${t('serviceProbe.cycleTraffic')} · ${t('serviceProbe.download')} · ${resetHint}`}
            className='flex items-center gap-1'
          >
            <HugeiconsIcon
              icon={ArrowDown02Icon}
              className='size-3.5 shrink-0'
              aria-hidden='true'
            />
            {formatBytes(bee.receivedBytes)}
          </ProbeHint>
          <ProbeHint
            label={`${t('serviceProbe.cycleTraffic')} · ${t('serviceProbe.upload')} · ${resetHint}`}
            className='flex items-center gap-1'
          >
            <HugeiconsIcon
              icon={ArrowUp02Icon}
              className='size-3.5 shrink-0'
              aria-hidden='true'
            />
            {formatBytes(bee.sentBytes)}
          </ProbeHint>
        </div>
      </div>
      {bee.isExceeded && (
        <ProbeHint
          label={t('serviceProbe.exceeded')}
          className='bg-destructive/10 text-destructive ml-auto flex size-6 shrink-0 items-center justify-center rounded-md'
        >
          <HugeiconsIcon
            icon={Alert02Icon}
            className='size-3.5'
            aria-hidden='true'
          />
        </ProbeHint>
      )}
      {(percent !== null || bee.quota.limitBytes === null) && (
        <ProbeHint
          label={`${t('Remaining quota')}: ${bee.quota.limitBytes === null ? t('serviceProbe.unlimited') : formatBytes(bee.remainingBytes)} · ${usageDetails}`}
          className='ml-auto shrink-0'
        >
          <QuotaRing
            percent={percent}
            exceeded={bee.isExceeded === true}
            unlimited={bee.quota.limitBytes === null}
          />
        </ProbeHint>
      )}
    </div>
  )
}

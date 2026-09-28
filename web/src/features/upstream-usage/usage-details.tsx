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
import { ChartNoAxesCombined, TrendingUp } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import dayjs from '@/lib/dayjs'
import { cn } from '@/lib/utils'
import { estimateUsageCapacity, usageColor } from './lib'
import type { UsageAccount, UsageWindow } from './types'

function WindowDetails(props: {
  label: string
  window: UsageWindow | null | undefined
}) {
  const { t, i18n } = useTranslation()
  const window = props.window
  if (!window || !Number.isFinite(window.utilization)) return null

  const remaining = Math.min(100, Math.max(0, 100 - window.utilization))
  const stats = window.window_stats
  const estimate = estimateUsageCapacity(window)
  const money = (value: number | undefined) =>
    value !== undefined && Number.isFinite(value)
      ? new Intl.NumberFormat(i18n.language, {
          style: 'currency',
          currency: 'USD',
          currencyDisplay: 'narrowSymbol',
          minimumFractionDigits: 2,
          maximumFractionDigits: 2,
        }).format(value)
      : '—'
  const tokens = (value: number | undefined) => {
    if (value === undefined || !Number.isFinite(value)) return '—'
    // Keep the compact token units consistent with upstream dashboards.
    for (const [unit, divisor] of [
      ['B', 1e9],
      ['M', 1e6],
      ['K', 1e3],
    ] as const) {
      if (value >= divisor) return `${(value / divisor).toFixed(1)}${unit}`
    }
    return Math.round(value).toLocaleString(i18n.language)
  }
  const reset = dayjs(window.resets_at)
  let resetLabel = '—'
  if (window.resets_at && reset.isValid()) {
    const seconds = Math.max(0, reset.diff(dayjs(), 'second'))
    const relative = new Intl.RelativeTimeFormat(i18n.language, {
      numeric: 'always',
    })
    if (seconds <= 0) resetLabel = t('upstreamUsage.resetDue')
    else if (seconds >= 86400)
      resetLabel = relative.format(Math.ceil(seconds / 86400), 'day')
    else if (seconds >= 3600)
      resetLabel = relative.format(Math.ceil(seconds / 3600), 'hour')
    else resetLabel = relative.format(Math.ceil(seconds / 60), 'minute')
  }
  const usedLabel = `${t('upstreamUsage.usedAmount')}: ${money(stats?.cost)} / ${tokens(stats?.tokens)} Tokens`
  const estimatedLabel = `${t('upstreamUsage.estimatedAmount')}: ${money(estimate?.cost)} / ${tokens(estimate?.tokens)} Tokens. ${t('upstreamUsage.estimateHint')}`

  return (
    <section className='space-y-2.5 border-t py-3 text-xs tabular-nums'>
      <div className='flex items-center justify-between gap-2'>
        <span className='bg-muted/40 text-muted-foreground rounded-md border px-2 py-0.5 font-medium'>
          {props.label}
        </span>
        <span className='text-muted-foreground'>
          {t('upstreamUsage.remainingLabel')}{' '}
          <strong className='text-foreground text-sm'>
            {Number(remaining.toFixed(1))}%
          </strong>
        </span>
      </div>
      <div
        role='progressbar'
        aria-label={`${props.label} · ${t('upstreamUsage.remainingLabel')}`}
        aria-valuemin={0}
        aria-valuemax={100}
        aria-valuenow={remaining}
        className='bg-muted h-2 overflow-hidden rounded-full'
      >
        <div
          className={cn(
            'h-full rounded-full bg-current',
            usageColor(remaining)
          )}
          style={{ width: `${remaining}%` }}
        />
      </div>
      <div className='text-muted-foreground flex flex-wrap items-center gap-x-3 gap-y-2'>
        <span
          className='inline-flex items-center gap-1 whitespace-nowrap'
          title={usedLabel}
          aria-label={usedLabel}
        >
          <ChartNoAxesCombined
            aria-hidden='true'
            className='size-5 rounded bg-blue-50 p-0.5 text-blue-400 dark:bg-blue-400/10 dark:text-blue-300'
            strokeWidth={1.5}
          />
          <span className='sr-only'>{t('upstreamUsage.usedAmount')}</span>
          <span className='text-foreground font-medium'>
            {money(stats?.cost)}
          </span>
          <span className='opacity-60'>/</span> {tokens(stats?.tokens)}
        </span>
        <span
          className='inline-flex items-center gap-1 whitespace-nowrap'
          title={estimatedLabel}
          aria-label={estimatedLabel}
        >
          <TrendingUp
            aria-hidden='true'
            className='size-5 rounded bg-violet-50 p-0.5 text-violet-400 dark:bg-violet-400/10 dark:text-violet-300'
            strokeWidth={1.5}
          />
          <span className='sr-only'>{t('upstreamUsage.estimatedAmount')}</span>
          <span className='font-medium'>{money(estimate?.cost)}</span>
          <span className='opacity-60'>/</span> {tokens(estimate?.tokens)}
        </span>
        <span
          className='ml-auto whitespace-nowrap'
          title={`${t('upstreamUsage.resetsAt')}: ${reset.isValid() ? reset.format('YYYY-MM-DD HH:mm:ss') : '—'}`}
        >
          <span className='sr-only'>{t('upstreamUsage.resetsAt')}: </span>
          {resetLabel}
        </span>
      </div>
    </section>
  )
}

export function UsageDetails(props: {
  account: UsageAccount
  baseURL?: string
}) {
  const { t } = useTranslation()
  const account = props.account
  return (
    <div className='space-y-2 border-t pt-3'>
      <div className='text-sm font-medium'>
        account_id: {account.account_id}
      </div>
      {props.baseURL && (
        <p className='text-muted-foreground truncate text-xs'>
          {props.baseURL}
        </p>
      )}
      {!account.available && (
        <p className='text-destructive text-xs'>
          {t('upstreamUsage.unavailable')}
          {account.last_error && ` · ${account.last_error}`}
        </p>
      )}
      {account.usage && (
        <>
          {!account.available && (
            <p className='text-muted-foreground text-xs'>
              {t('upstreamUsage.lastKnown')}
            </p>
          )}
          <WindowDetails
            label={t('upstreamUsage.fiveHourLimit')}
            window={account.usage.five_hour}
          />
          <WindowDetails
            label={t('upstreamUsage.weeklyLimit')}
            window={account.usage.seven_day}
          />
          <p className='text-muted-foreground text-xs'>
            {t('upstreamUsage.updatedAt')}:{' '}
            {account.usage.updated_at
              ? dayjs(account.usage.updated_at).format('YYYY-MM-DD HH:mm:ss')
              : '—'}
          </p>
        </>
      )}
      {account.fetched_at > 0 && (
        <p className='text-muted-foreground text-xs'>
          {t('upstreamUsage.fetchedAt')}:{' '}
          {dayjs.unix(account.fetched_at).format('YYYY-MM-DD HH:mm:ss')}
        </p>
      )}
    </div>
  )
}

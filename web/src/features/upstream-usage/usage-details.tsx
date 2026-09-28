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
import { useTranslation } from 'react-i18next'
import dayjs from '@/lib/dayjs'
import type { UsageAccount, UsageWindow } from './types'

function WindowDetails(props: {
  label: string
  window: UsageWindow | null | undefined
}) {
  const { t } = useTranslation()
  const window = props.window
  if (!window)
    return (
      <div>
        {props.label}: {t('upstreamUsage.unavailable')}
      </div>
    )
  const stats = window.window_stats
  return (
    <div className='bg-muted/30 space-y-1 rounded-md border p-2 text-xs'>
      <div className='flex justify-between font-medium'>
        <span>{props.label}</span>
        <span>
          {t('upstreamUsage.usedRemaining', {
            used: window.utilization,
            remaining: Number((100 - window.utilization).toFixed(1)),
          })}
        </span>
      </div>
      <div>
        {t('upstreamUsage.resetsAt')}:{' '}
        {window.resets_at
          ? dayjs(window.resets_at).format('YYYY-MM-DD HH:mm:ss')
          : '—'}
      </div>
      <div>
        {t('upstreamUsage.remainingSeconds')}:{' '}
        {window.remaining_seconds.toLocaleString()}
      </div>
      {stats && (
        <dl className='grid grid-cols-2 gap-1'>
          <dt>{t('upstreamUsage.requests')}</dt>
          <dd className='text-right'>{stats.requests.toLocaleString()}</dd>
          <dt>Tokens</dt>
          <dd className='text-right'>{stats.tokens.toLocaleString()}</dd>
          <dt>{t('upstreamUsage.cost')}</dt>
          <dd className='text-right'>
            {stats.cost.toLocaleString(undefined, { maximumFractionDigits: 6 })}
          </dd>
          <dt>{t('upstreamUsage.standardCost')}</dt>
          <dd className='text-right'>
            {stats.standard_cost.toLocaleString(undefined, {
              maximumFractionDigits: 6,
            })}
          </dd>
        </dl>
      )}
    </div>
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
          <WindowDetails label='5h' window={account.usage.five_hour} />
          <WindowDetails label='7d' window={account.usage.seven_day} />
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

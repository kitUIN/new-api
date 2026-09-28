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
import { AlertTriangle } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import {
  Popover,
  PopoverContent,
  PopoverTitle,
  PopoverTrigger,
} from '@/components/ui/popover'
import { remainingUsage, usageColor } from './lib'
import { ResetCountdown } from './reset-countdown'
import type { GroupUsageAccount } from './types'
import { UsageDetails } from './usage-details'

export function UsageBattery(props: {
  accounts: GroupUsageAccount[]
  showResetCountdown?: boolean
}) {
  const { t } = useTranslation()
  const sevenDay = remainingUsage(props.accounts, 'seven_day')
  const fiveHour = remainingUsage(props.accounts, 'five_hour')
  if (sevenDay === null) return null
  const showCountdown = props.showResetCountdown && sevenDay === 0

  return (
    <Popover>
      <PopoverTrigger
        render={
          <Button
            variant='ghost'
            size='sm'
            className='h-7 gap-1.5 px-1.5 text-xs tabular-nums'
            aria-label={
              showCountdown
                ? undefined
                : `${t('upstreamUsage.details')} · 7d ${Number(sevenDay.toFixed(1))}%`
            }
          />
        }
      >
        {showCountdown ? (
          <ResetCountdown accounts={props.accounts} />
        ) : (
          <>
            <span
              className={cn(
                'relative mr-0.5 inline-flex',
                usageColor(sevenDay)
              )}
            >
              <svg
                className='size-auto h-3.5 w-6'
                width='24'
                height='14'
                viewBox='0 0 24 14'
                fill='none'
                aria-hidden='true'
              >
                <rect
                  x='1'
                  y='2'
                  width='19'
                  height='10'
                  rx='2'
                  className='stroke-muted-foreground/45'
                  strokeWidth='1'
                />
                <path
                  d='M22 5v4'
                  className='stroke-muted-foreground/45'
                  strokeWidth='1'
                  strokeLinecap='round'
                />
                <rect
                  x='3'
                  y='4'
                  width={(15 * sevenDay) / 100}
                  height='6'
                  rx='1'
                  fill='currentColor'
                />
              </svg>
              {fiveHour !== null && fiveHour < 100 && (
                <span
                  className={cn(
                    'bg-card absolute -right-1 -bottom-1 rounded-full',
                    usageColor(fiveHour)
                  )}
                >
                  {fiveHour <= 0 ? (
                    <AlertTriangle
                      className='size-3'
                      strokeWidth={1.5}
                      aria-label={t('upstreamUsage.fiveHourExhausted')}
                    />
                  ) : (
                    <svg
                      className='size-3'
                      width='12'
                      height='12'
                      viewBox='0 0 14 14'
                      aria-label={t('upstreamUsage.fiveHourRemaining', {
                        percent: fiveHour.toFixed(1),
                      })}
                    >
                      <circle
                        cx='7'
                        cy='7'
                        r='5'
                        fill='none'
                        stroke='currentColor'
                        strokeOpacity='.15'
                        strokeWidth='1.5'
                      />
                      <circle
                        cx='7'
                        cy='7'
                        r='5'
                        fill='none'
                        stroke='currentColor'
                        strokeWidth='1.5'
                        strokeLinecap='round'
                        pathLength='100'
                        strokeDasharray={`${fiveHour} 100`}
                        transform='rotate(-90 7 7)'
                      />
                    </svg>
                  )}
                </span>
              )}
            </span>
            <span className='text-muted-foreground'>
              {Number(sevenDay.toFixed(1))}%
            </span>
          </>
        )}
      </PopoverTrigger>
      <PopoverContent className='max-h-[70vh] w-[min(28rem,calc(100vw-2rem))] overflow-y-auto p-4'>
        <PopoverTitle>{t('upstreamUsage.details')}</PopoverTitle>
        <p className='text-muted-foreground text-xs'>
          {t('upstreamUsage.averageHint')}
        </p>
        {props.accounts.map((account) => (
          <UsageDetails
            key={account.id}
            account={account}
            baseURL={account.base_url}
          />
        ))}
      </PopoverContent>
    </Popover>
  )
}

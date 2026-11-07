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
import { cn } from '@/lib/utils'

export function QuotaRing(props: {
  percent: number | null
  exceeded: boolean
  unlimited: boolean
}) {
  const { t, i18n } = useTranslation()
  const percent = Math.min(100, Math.max(0, props.percent ?? 0))
  let color = 'text-success'
  if (!props.unlimited && percent <= 20) color = 'text-warning'
  if (!props.unlimited && (props.exceeded || percent === 0))
    color = 'text-destructive'
  let label =
    percent > 0 && percent < 0.1
      ? `<${new Intl.NumberFormat(i18n.language).format(0.1)}%`
      : `${new Intl.NumberFormat(i18n.language, { maximumFractionDigits: 1 }).format(percent)}%`
  if (props.unlimited) label = '∞'

  return (
    <span
      role={props.unlimited ? 'img' : 'progressbar'}
      aria-label={
        props.unlimited ? t('serviceProbe.unlimited') : t('Remaining quota')
      }
      aria-valuemin={props.unlimited ? undefined : 0}
      aria-valuemax={props.unlimited ? undefined : 100}
      aria-valuenow={props.unlimited ? undefined : percent}
      aria-valuetext={props.unlimited ? undefined : label}
      className={cn('relative flex size-11 items-center justify-center', color)}
    >
      <svg
        viewBox='0 0 40 40'
        className='absolute inset-0 size-full -rotate-90'
        aria-hidden='true'
      >
        <circle
          cx='20'
          cy='20'
          r='17'
          fill='none'
          stroke='currentColor'
          strokeWidth='4'
          className='text-muted'
        />
        <circle
          cx='20'
          cy='20'
          r='17'
          fill='none'
          stroke='currentColor'
          strokeWidth='4'
          pathLength='100'
          strokeDasharray={`${props.unlimited ? 100 : percent} 100`}
        />
      </svg>
      <span
        className={cn(
          'text-foreground text-[10px] font-medium tabular-nums',
          props.unlimited && 'text-lg'
        )}
      >
        {label}
      </span>
    </span>
  )
}

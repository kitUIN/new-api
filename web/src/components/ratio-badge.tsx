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
/* eslint-disable react-refresh/only-export-components */
import { Star } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import type { TimedRatioSnapshot } from '@/lib/api'
import { cn } from '@/lib/utils'
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { StatusBadge } from './status-badge'

const TIMED_RATIO_CLASS =
  'bg-amber-400/15 text-amber-600 ring-1 ring-amber-400/40 ring-inset dark:text-amber-300'

/** Formats a ratio as a compact string without trailing zeros, e.g. `1.5`. */
export function formatRatioValue(ratio: number): string {
  if (!Number.isFinite(ratio)) return '-'
  return ratio % 1 === 0 ? String(ratio) : String(Number(ratio.toFixed(4)))
}

/** Formats a ratio with the `x` suffix; strings are returned unchanged. */
export function formatRatioLabel(ratio: number | string): string {
  return typeof ratio === 'number' ? `${formatRatioValue(ratio)}x` : ratio
}

function formatTimedRatioEnd(end: number): string {
  return new Date(end * 1000).toLocaleString('sv-SE', {
    timeZone: 'Asia/Shanghai',
  })
}

/** Remaining minutes (rounded up) until the timed window ends; 0 when ended. */
export function getTimedRatioRemainingMinutes(
  end: number,
  now: number = Date.now()
): number {
  return Math.max(0, Math.ceil((end * 1000 - now) / 60000))
}

/** Hover-card body for a timed ratio: original, timed, end time and remark. */
export function TimedRatioDetails(props: { snapshot: TimedRatioSnapshot }) {
  const { t } = useTranslation()
  const remaining = getTimedRatioRemainingMinutes(props.snapshot.end)
  const time = `${formatTimedRatioEnd(props.snapshot.end)} (UTC+8)`
  return (
    <div className='space-y-1'>
      <p>
        {t('timedRatio.original')}:{' '}
        <s>{formatRatioLabel(props.snapshot.original_ratio)}</s>
      </p>
      <p className='font-medium text-amber-400'>
        {t('timedRatio.title')}: {formatRatioLabel(props.snapshot.ratio)}
      </p>
      <p>
        {remaining > 0
          ? t('timedRatio.untilRemaining', { time, minutes: remaining })
          : t('timedRatio.untilEnded', { time })}
      </p>
      {props.snapshot.remark && (
        <p className='break-words whitespace-pre-wrap'>
          {t('timedRatio.remark')}: {props.snapshot.remark}
        </p>
      )}
    </div>
  )
}

export type TimedRatioMember = {
  group: string
  timed: TimedRatioSnapshot
}

type RatioBadgeProps = {
  ratio?: number | string | null
  timed?: TimedRatioSnapshot | null
  /** Combination groups: members currently on a timed ratio. */
  timedMembers?: TimedRatioMember[]
  className?: string
  /** Skip the hover card, e.g. when the badge already sits inside a tooltip. */
  disableTooltip?: boolean
}

function TimedMembersDetails(props: { members: TimedRatioMember[] }) {
  return (
    <div className='space-y-2'>
      {props.members.map((member) => (
        <div
          key={member.group}
          className='border-background/20 space-y-1 border-t pt-2 first:border-t-0 first:pt-0'
        >
          <p className='font-semibold break-all'>{member.group}</p>
          <TimedRatioDetails snapshot={member.timed} />
        </div>
      ))}
    </div>
  )
}

/**
 * Unified ratio badge: green `1x` normally; gold with a star in the top-right
 * corner when a timed ratio is active, showing the window on hover.
 */
export function RatioBadge(props: RatioBadgeProps) {
  const { t } = useTranslation()
  const timed = props.timed
  const timedMembers = props.timedMembers ?? []
  const ratio = timed ? timed.ratio : props.ratio
  if (ratio == null || ratio === '') return null
  const label = formatRatioLabel(ratio)

  if (!timed && timedMembers.length === 0) {
    return (
      <StatusBadge
        label={label}
        variant='success'
        copyable={false}
        className={cn('font-mono tabular-nums', props.className)}
      />
    )
  }

  const timedBadge = (
    <>
      <StatusBadge
        label={label}
        copyable={false}
        className={cn(
          'font-mono tabular-nums',
          TIMED_RATIO_CLASS,
          props.className
        )}
      />
      <Star
        className='absolute -top-1 -right-1 size-3 fill-amber-400 text-amber-500'
        aria-hidden='true'
      />
    </>
  )
  const wrapperClass = 'relative inline-flex w-fit shrink-0'
  const ariaLabel = `${t('timedRatio.title')} ${label}`

  if (props.disableTooltip) {
    return (
      <span className={wrapperClass} aria-label={ariaLabel}>
        {timedBadge}
      </span>
    )
  }

  return (
    <TooltipProvider delay={200}>
      <Tooltip>
        <TooltipTrigger
          render={
            <span
              className={cn(wrapperClass, 'cursor-help')}
              aria-label={ariaLabel}
            />
          }
        >
          {timedBadge}
        </TooltipTrigger>
        <TooltipContent className='block max-w-sm'>
          {timed ? (
            <TimedRatioDetails snapshot={timed} />
          ) : (
            <TimedMembersDetails members={timedMembers} />
          )}
        </TooltipContent>
      </Tooltip>
    </TooltipProvider>
  )
}

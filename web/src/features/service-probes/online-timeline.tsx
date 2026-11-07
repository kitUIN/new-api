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
import { useMemo } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Alert02Icon, Clock01Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useTranslation } from 'react-i18next'
import { cn } from '@/lib/utils'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { getHistory } from './api'
import { ProbeHint } from './hint'
import { formatCompactTime, formatTime, historyBuckets } from './lib'
import type { ProbeWindow } from './types'

export function OnlineTimeline(props: {
  id: string
  window: ProbeWindow
  userId: number
  enabled: boolean
}) {
  const { t, i18n } = useTranslation()
  const query = useQuery({
    queryKey: [
      'service-probes',
      props.userId,
      props.id,
      'history',
      props.window,
    ],
    queryFn: ({ signal }) => getHistory(props.id, props.window, signal),
    enabled: props.enabled,
    refetchInterval: 60_000,
    retry: false,
    gcTime: 0,
  })
  const buckets = useMemo(
    () => (query.data ? historyBuckets(query.data) : []),
    [query.data]
  )
  if (query.isPending) return <Skeleton className='h-10 w-full' />
  if (query.isError || !query.data)
    return (
      <ProbeHint
        label={t('serviceProbe.historyFailed')}
        className='text-muted-foreground flex h-10 items-center gap-2'
      >
        <HugeiconsIcon icon={Alert02Icon} size={14} aria-hidden='true' />
        <span>—</span>
      </ProbeHint>
    )
  const history = query.data
  const total =
    history.onlineSeconds + history.offlineSeconds + history.unknownSeconds
  const onlinePercent =
    total > 0 ? ((history.onlineSeconds / total) * 100).toFixed(1) : '—'
  const durationSummary = t('serviceProbe.durationSummary', {
    online: (history.onlineSeconds / 3600).toFixed(2),
    offline: (history.offlineSeconds / 3600).toFixed(2),
    unknown: (history.unknownSeconds / 3600).toFixed(2),
  })
  return (
    <div className='flex flex-col gap-1.5'>
      <div className='flex items-center gap-2'>
        <div
          className='flex min-w-0 flex-1 gap-0.5'
          aria-label={t('serviceProbe.onlineHistory')}
        >
          {buckets.map((bucket) => {
            const label = `${formatTime(bucket.from, i18n.language)} – ${formatTime(bucket.to, i18n.language)}`
            const summary = t('serviceProbe.intervalSummary', {
              online: Math.round(bucket.seconds.online),
              offline: Math.round(bucket.seconds.offline),
              unknown: Math.round(bucket.seconds.unknown),
            })
            return (
              <Tooltip key={bucket.from}>
                <TooltipTrigger
                  aria-label={`${label}: ${summary}`}
                  className={cn(
                    'focus-visible:outline-ring h-4 min-w-0 flex-1 rounded-xs focus-visible:outline-2 focus-visible:outline-offset-2',
                    'bg-success',
                    bucket.seconds.unknown > 0 && 'bg-muted-foreground/40',
                    bucket.seconds.offline > 0 && 'bg-destructive'
                  )}
                />
                <TooltipContent>
                  <div className='flex flex-col gap-1'>
                    <span>{label}</span>
                    <span>{summary}</span>
                  </div>
                </TooltipContent>
              </Tooltip>
            )
          })}
        </div>
        <ProbeHint
          label={`${t('serviceProbe.onlinePercent', { percent: onlinePercent })} · ${durationSummary}`}
          className='shrink-0 text-xs font-medium tabular-nums'
        >
          {onlinePercent}%
        </ProbeHint>
      </div>
      <div className='text-muted-foreground flex flex-wrap justify-between gap-x-3 gap-y-1 text-[10px] tabular-nums'>
        <span className='flex items-center gap-1'>
          <HugeiconsIcon
            icon={Clock01Icon}
            className='size-3 shrink-0'
            aria-hidden='true'
          />
          {formatCompactTime(history.from, i18n.language)}
        </span>
        <span className='flex items-center gap-1'>
          <HugeiconsIcon
            icon={Clock01Icon}
            className='size-3 shrink-0'
            aria-hidden='true'
          />
          {formatCompactTime(history.to, i18n.language)}
        </span>
      </div>
    </div>
  )
}

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
import { useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Activity, Gauge, HeartPulse, RefreshCw, Timer } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import dayjs from '@/lib/dayjs'
import { cn } from '@/lib/utils'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from '@/components/ui/empty'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { getPerfGroupHealth } from '@/features/performance-metrics/api'
import {
  formatLatency,
  formatThroughput,
  formatUptimePct,
} from '@/features/performance-metrics/lib/format'
import type {
  PerfGroupHealth,
  PerfGroupHealthBucket,
  PerfGroupHealthStatus,
} from '@/features/performance-metrics/types'

const HEALTH_WINDOW_HOURS = 24
const HEALTH_BAR_WINDOW_HOURS = 6
const HEALTH_INTERVAL_MINUTES = 10
const HEALTH_REFRESH_INTERVAL_MS = 60 * 1000

function statusDotClassName(status: PerfGroupHealthStatus): string {
  switch (status) {
    case 'ok':
      return 'bg-success'
    case 'warning':
      return 'bg-warning'
    case 'error':
      return 'bg-destructive'
    default:
      return 'bg-muted-foreground/30'
  }
}

function statusTextClassName(rate: number, requestCount: number): string {
  if (requestCount <= 0) return 'text-muted-foreground'
  if (rate >= 99) return 'text-success'
  if (rate >= 95) return 'text-warning'
  return 'text-destructive'
}

function formatRatio(ratio: number): string {
  if (!Number.isFinite(ratio)) return 'x1'
  return `x${ratio
    .toFixed(ratio % 1 === 0 ? 0 : 3)
    .replace(/0+$/, '')
    .replace(/\.$/, '')}`
}

function formatWindow(bucket: PerfGroupHealthBucket): string {
  return `${dayjs.unix(bucket.ts).format('MM-DD HH:mm')} ~ ${dayjs
    .unix(bucket.end_ts)
    .format('HH:mm')}`
}

export function GroupHealthGrid() {
  const { t } = useTranslation()
  const [autoRefresh, setAutoRefresh] = useState(true)
  const healthQuery = useQuery({
    queryKey: [
      'perf-group-health',
      HEALTH_WINDOW_HOURS,
      HEALTH_INTERVAL_MINUTES,
    ],
    queryFn: () =>
      getPerfGroupHealth(HEALTH_WINDOW_HOURS, HEALTH_INTERVAL_MINUTES),
    refetchInterval: autoRefresh ? HEALTH_REFRESH_INTERVAL_MS : false,
    refetchOnMount: 'always',
    refetchOnReconnect: 'always',
    refetchOnWindowFocus: 'always',
    staleTime: 0,
    retry: false,
  })

  const groups = useMemo(
    () => healthQuery.data?.data.groups ?? [],
    [healthQuery.data]
  )

  if (healthQuery.isLoading) {
    return (
      <div className='flex flex-col gap-3'>
        <GroupHealthToolbar
          autoRefresh={autoRefresh}
          isFetching={healthQuery.isFetching}
          onRefresh={() => healthQuery.refetch()}
          onToggleAutoRefresh={() => setAutoRefresh((value) => !value)}
        />
        <GroupHealthSkeleton />
      </div>
    )
  }

  if (!groups.length) {
    return (
      <div className='flex flex-col gap-3'>
        <GroupHealthToolbar
          autoRefresh={autoRefresh}
          isFetching={healthQuery.isFetching}
          onRefresh={() => healthQuery.refetch()}
          onToggleAutoRefresh={() => setAutoRefresh((value) => !value)}
        />
        <Empty className='min-h-72 border'>
          <EmptyHeader>
            <EmptyMedia variant='icon'>
              <HeartPulse />
            </EmptyMedia>
            <EmptyTitle>{t('No group health data')}</EmptyTitle>
            <EmptyDescription>
              {t('Enable performance metrics to collect group health data.')}
            </EmptyDescription>
          </EmptyHeader>
        </Empty>
      </div>
    )
  }

  return (
    <div className='flex flex-col gap-3'>
      <GroupHealthToolbar
        autoRefresh={autoRefresh}
        isFetching={healthQuery.isFetching}
        onRefresh={() => healthQuery.refetch()}
        onToggleAutoRefresh={() => setAutoRefresh((value) => !value)}
      />
      <div className='grid grid-cols-1 gap-3 lg:grid-cols-2 2xl:grid-cols-3'>
        {groups.map((group) => (
          <GroupHealthCard key={group.group} group={group} />
        ))}
      </div>
      <HealthLegend />
    </div>
  )
}

function GroupHealthToolbar(props: {
  autoRefresh: boolean
  isFetching: boolean
  onRefresh: () => void
  onToggleAutoRefresh: () => void
}) {
  const { t } = useTranslation()

  return (
    <div className='flex justify-end gap-2'>
      <Tooltip>
        <TooltipTrigger
          render={
            <Button
              type='button'
              variant='outline'
              size='sm'
              onClick={props.onRefresh}
              disabled={props.isFetching}
            >
              <RefreshCw
                data-icon='inline-start'
                className={cn(props.isFetching && 'animate-spin')}
                aria-hidden='true'
              />
              {t('Refresh')}
            </Button>
          }
        />
        <TooltipContent>{t('Refresh group health data')}</TooltipContent>
      </Tooltip>
      <Tooltip>
        <TooltipTrigger
          render={
            <Button
              type='button'
              variant={props.autoRefresh ? 'default' : 'outline'}
              size='sm'
              onClick={props.onToggleAutoRefresh}
              aria-pressed={props.autoRefresh}
            >
              <RefreshCw data-icon='inline-start' aria-hidden='true' />
              {t('Auto refresh')} (1m)
            </Button>
          }
        />
        <TooltipContent>
          {props.autoRefresh
            ? t('Auto refresh is on')
            : t('Auto refresh is off')}
        </TooltipContent>
      </Tooltip>
    </div>
  )
}

function GroupHealthCard(props: { group: PerfGroupHealth }) {
  const { t } = useTranslation()
  const group = props.group
  const hasSamples = group.request_count > 0
  const visibleBucketCount = Math.ceil(
    (HEALTH_BAR_WINDOW_HOURS * 60) / HEALTH_INTERVAL_MINUTES
  )
  const visibleBuckets = group.buckets.slice(-visibleBucketCount)

  return (
    <section className='bg-card overflow-hidden rounded-lg border shadow-xs'>
      <div className='flex items-start justify-between gap-3 border-b px-4 py-3'>
        <div className='min-w-0'>
          <div className='truncate text-sm font-semibold'>{group.group}</div>
          <div className='mt-1 flex flex-wrap items-center gap-1.5'>
            <Badge variant='outline' className='font-mono'>
              {formatRatio(group.ratio)}
            </Badge>
            <Badge variant='secondary'>
              {t('{{count}} providers', { count: group.provider_count })}
            </Badge>
          </div>
        </div>
        <div className='shrink-0 text-right'>
          <div className='text-muted-foreground text-[11px]'>
            {t('24h success rate')}
          </div>
          <div
            className={cn(
              'font-mono text-lg font-semibold tabular-nums',
              statusTextClassName(group.success_rate, group.request_count)
            )}
          >
            {hasSamples ? formatUptimePct(group.success_rate) : '—'}
          </div>
        </div>
      </div>

      <div className='flex flex-col gap-3 px-4 py-3'>
        <div className='grid grid-cols-3 gap-2'>
          <MetricCell
            icon={Timer}
            label={t('Average first-token latency')}
            value={formatLatency(group.avg_ttft_ms)}
          />
          <MetricCell
            icon={Activity}
            label={t('Average latency')}
            value={formatLatency(group.avg_latency_ms)}
          />
          <MetricCell
            icon={Gauge}
            label={t('Average token/s')}
            value={formatThroughput(group.avg_tps)}
          />
        </div>

        <div
          className='grid gap-px'
          style={{
            gridTemplateColumns: visibleBuckets.length
              ? `repeat(${visibleBuckets.length}, minmax(0, 1fr))`
              : undefined,
          }}
        >
          {visibleBuckets.map((bucket) => (
            <BucketDot key={bucket.ts} bucket={bucket} />
          ))}
        </div>
      </div>
    </section>
  )
}

function MetricCell(props: {
  icon: React.ComponentType<{ className?: string }>
  label: string
  value: string
}) {
  const Icon = props.icon
  return (
    <div className='bg-muted/40 min-w-0 rounded-md px-2.5 py-2'>
      <div className='text-muted-foreground flex items-center gap-1 text-[10px] font-medium'>
        <Icon className='size-3 shrink-0' aria-hidden='true' />
        <span className='truncate'>{props.label}</span>
      </div>
      <div className='mt-1 truncate font-mono text-xs font-semibold tabular-nums'>
        {props.value}
      </div>
    </div>
  )
}

function BucketDot(props: { bucket: PerfGroupHealthBucket }) {
  const { t } = useTranslation()
  const bucket = props.bucket
  return (
    <Tooltip>
      <TooltipTrigger
        render={
          <button
            type='button'
            className={cn(
              'focus-visible:ring-ring h-3 min-w-0 rounded-full focus-visible:ring-2 focus-visible:outline-none',
              statusDotClassName(bucket.status)
            )}
            aria-label={formatWindow(bucket)}
          />
        }
      />
      <TooltipContent side='top' className='flex-col items-start font-mono'>
        <div className='font-medium'>{formatWindow(bucket)}</div>
        <div>
          {t('Success rate')}:{' '}
          {bucket.request_count > 0
            ? formatUptimePct(bucket.success_rate)
            : '—'}
        </div>
        <div>
          {t('Average first-token latency')}:{' '}
          {formatLatency(bucket.avg_ttft_ms)}
        </div>
        <div>
          {t('Average latency')}: {formatLatency(bucket.avg_latency_ms)}
        </div>
        <div>
          {t('Average token/s')}: {formatThroughput(bucket.avg_tps)}
        </div>
        <div>
          {t('Requests')}: {bucket.request_count}
        </div>
      </TooltipContent>
    </Tooltip>
  )
}

function HealthLegend() {
  const { t } = useTranslation()
  const items: Array<{ label: string; status: PerfGroupHealthStatus }> = [
    { label: t('Excellent >= 99%'), status: 'ok' },
    { label: t('Warning 95%-99%'), status: 'warning' },
    { label: t('Error < 95%'), status: 'error' },
    { label: t('No data'), status: 'empty' },
  ]

  return (
    <div className='text-muted-foreground flex flex-wrap items-center gap-x-4 gap-y-2 rounded-lg border px-3 py-2 text-xs'>
      {items.map((item) => (
        <span key={item.status} className='inline-flex items-center gap-1.5'>
          <span
            className={cn(
              'size-2 rounded-full',
              statusDotClassName(item.status)
            )}
            aria-hidden='true'
          />
          {item.label}
        </span>
      ))}
    </div>
  )
}

function GroupHealthSkeleton() {
  return (
    <div className='grid grid-cols-1 gap-3 lg:grid-cols-2 2xl:grid-cols-3'>
      {Array.from({ length: 6 }).map((_, index) => (
        <div key={index} className='overflow-hidden rounded-lg border'>
          <div className='flex items-start justify-between border-b px-4 py-3'>
            <div>
              <Skeleton className='h-4 w-32' />
              <div className='mt-2 flex gap-1.5'>
                <Skeleton className='h-5 w-12 rounded-full' />
                <Skeleton className='h-5 w-20 rounded-full' />
              </div>
            </div>
            <Skeleton className='h-8 w-20' />
          </div>
          <div className='flex flex-col gap-3 px-4 py-3'>
            <div className='grid grid-cols-3 gap-2'>
              {Array.from({ length: 3 }).map((__, metricIndex) => (
                <Skeleton key={metricIndex} className='h-12 rounded-md' />
              ))}
            </div>
            <Skeleton className='h-4 w-full rounded-sm' />
          </div>
        </div>
      ))}
    </div>
  )
}

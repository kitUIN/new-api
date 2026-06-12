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
import { useEffect, useMemo, useRef, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Activity, Gauge, HeartPulse, RefreshCw, Timer } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import dayjs from '@/lib/dayjs'
import { getLobeIcon } from '@/lib/lobe-icon'
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
  Popover,
  PopoverContent,
  PopoverTrigger,
} from '@/components/ui/popover'
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
const HEALTH_RECENT_WINDOW_HOURS = 2
const HEALTH_INTERVAL_MINUTES = 10
const HEALTH_REFRESH_INTERVAL_MS = 60 * 1000
const HEALTH_DOT_SIZE_PX = 10
const HEALTH_DOT_GAP_PX = 4
const PROVIDER_ICON_MATCHERS: Array<[RegExp, string]> = [
  [/^codex/i, 'OpenAI.Color'],
  [/^cc/i, 'Claude.Color'],
  [/openai|chatgpt|gpt/i, 'OpenAI.Color'],
  [/anthropic|claude/i, 'Claude.Color'],
  [/gemini|google|vertex/i, 'Gemini.Color'],
  [/azure/i, 'Azure.Color'],
  [/aws|bedrock/i, 'Aws.Color'],
  [/openrouter/i, 'OpenRouter.Color'],
  [/deepseek/i, 'DeepSeek.Color'],
  [/grok|xai/i, 'XAI.Color'],
  [/moonshot|kimi/i, 'Moonshot.Color'],
  [/mistral/i, 'Mistral.Color'],
  [/cohere/i, 'Cohere.Color'],
  [/ollama/i, 'Ollama.Color'],
]

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

function balanceDotClassName(level: PerfGroupHealth['balance_level']): string {
  if (level === 0) return 'bg-destructive'
  if (level === 1) return 'bg-warning'
  return 'bg-success'
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

function formatBucketThroughput(tps: number): string {
  if (!Number.isFinite(tps) || tps <= 0) return '—'
  return tps.toFixed(tps < 10 ? 2 : 1)
}

function getGroupProviderIcon(groupName: string): React.ReactNode {
  const matched = PROVIDER_ICON_MATCHERS.find(([pattern]) =>
    pattern.test(groupName)
  )
  return getLobeIcon(matched?.[1] || groupName, 18)
}

function calculateRecentSuccessRate(
  buckets: PerfGroupHealthBucket[],
  hours: number
) {
  const bucketCount = Math.ceil((hours * 60) / HEALTH_INTERVAL_MINUTES)
  const recentBuckets = buckets.slice(-bucketCount)
  const totals = recentBuckets.reduce(
    (acc, bucket) => {
      acc.requests += bucket.request_count || 0
      acc.successes += bucket.success_count || 0
      return acc
    },
    { requests: 0, successes: 0 }
  )

  return {
    requestCount: totals.requests,
    successRate:
      totals.requests > 0 ? (totals.successes / totals.requests) * 100 : 0,
  }
}

function useElementWidth<T extends HTMLElement>() {
  const ref = useRef<T | null>(null)
  const [width, setWidth] = useState(0)

  useEffect(() => {
    const element = ref.current
    if (!element) return

    const updateWidth = () => setWidth(element.clientWidth)
    updateWidth()

    const resizeObserver = new ResizeObserver(updateWidth)
    resizeObserver.observe(element)

    return () => resizeObserver.disconnect()
  }, [])

  return [ref, width] as const
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
  const [bucketBarRef, bucketBarWidth] = useElementWidth<HTMLDivElement>()
  const defaultVisibleBucketCount = Math.ceil(
    (HEALTH_BAR_WINDOW_HOURS * 60) / HEALTH_INTERVAL_MINUTES
  )
  const visibleBucketCount = bucketBarWidth
    ? Math.max(
        1,
        Math.min(
          group.buckets.length,
          Math.floor(
            (bucketBarWidth + HEALTH_DOT_GAP_PX) /
              (HEALTH_DOT_SIZE_PX + HEALTH_DOT_GAP_PX)
          )
        )
      )
    : Math.min(group.buckets.length, defaultVisibleBucketCount)
  const visibleBuckets = group.buckets.slice(-visibleBucketCount)
  const recentHealth = calculateRecentSuccessRate(
    group.buckets,
    HEALTH_RECENT_WINDOW_HOURS
  )

  return (
    <section className='bg-card overflow-hidden rounded-lg border shadow-xs'>
      <div className='flex items-start justify-between gap-3 border-b px-4 py-3'>
        <div className='min-w-0'>
          <div className='flex min-w-0 items-center gap-2'>
            <span className='flex size-5 shrink-0 items-center justify-center'>
              {getGroupProviderIcon(group.group)}
            </span>
            <div className='truncate text-sm font-semibold'>{group.group}</div>
          </div>
          <div className='mt-1 flex flex-wrap items-center gap-1.5'>
            <Badge variant='outline' className='font-mono'>
              {formatRatio(group.ratio)}
            </Badge>
            <Badge variant='secondary' className='gap-1.5'>
              <span
                className={cn(
                  'size-2 shrink-0 rounded-full',
                  balanceDotClassName(group.balance_level)
                )}
                aria-hidden='true'
              />
              {t('{{count}} providers', { count: group.provider_count })}
            </Badge>
          </div>
        </div>
        <div className='grid shrink-0 grid-cols-[auto_auto] items-start gap-6 text-right'>
          <div>
            <div className='text-muted-foreground text-[10px] leading-none'>
              {t('24h success rate')}
            </div>
            <div
              className={cn(
                'mt-1 font-mono text-xs font-semibold tabular-nums',
                statusTextClassName(group.success_rate, group.request_count)
              )}
            >
              {hasSamples ? formatUptimePct(group.success_rate) : '—'}
            </div>
          </div>
          <div>
            <div className='text-muted-foreground text-[11px] leading-none'>
              {t('2h success rate')}
            </div>
            <div
              className={cn(
                'mt-1 font-mono text-xl font-semibold tabular-nums',
                statusTextClassName(
                  recentHealth.successRate,
                  recentHealth.requestCount
                )
              )}
            >
              {recentHealth.requestCount > 0
                ? formatUptimePct(recentHealth.successRate)
                : '—'}
            </div>
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
          ref={bucketBarRef}
          className='grid items-center justify-items-center gap-1'
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
  const bucket = props.bucket
  return (
    <Popover>
      <PopoverTrigger
        render={
          <button
            type='button'
            className={cn(
              'focus-visible:ring-ring size-2.5 shrink-0 rounded-full transition-transform hover:scale-125 focus-visible:ring-2 focus-visible:outline-none data-[popup-open]:scale-125',
              statusDotClassName(bucket.status)
            )}
            aria-label={formatWindow(bucket)}
          />
        }
      />
      <PopoverContent
        side='top'
        className='w-56 max-w-[calc(100vw-2rem)] rounded-lg p-3.5 shadow-lg'
      >
        <BucketDetails bucket={bucket} />
      </PopoverContent>
    </Popover>
  )
}

function BucketDetails(props: { bucket: PerfGroupHealthBucket }) {
  const { t } = useTranslation()
  const bucket = props.bucket
  const errorCount = Math.max(
    0,
    (bucket.request_count || 0) - (bucket.success_count || 0)
  )
  const rows = [
    {
      label: t('Success rate'),
      value:
        bucket.request_count > 0 ? formatUptimePct(bucket.success_rate) : '—',
      active: bucket.request_count > 0,
      valueClassName: 'text-success',
    },
    {
      label: t('First-token latency'),
      value: formatLatency(bucket.avg_ttft_ms),
    },
    {
      label: t('Latency'),
      value: formatLatency(bucket.avg_latency_ms),
    },
    {
      label: t('token/s'),
      value: formatBucketThroughput(bucket.avg_tps),
    },
    {
      label: t('Requests'),
      value: String(bucket.request_count || 0),
    },
    {
      label: t('Error count'),
      value: String(errorCount),
    },
  ]

  return (
    <div className='text-popover-foreground w-full'>
      <div className='mb-3 font-mono text-sm font-semibold tracking-normal'>
        {formatWindow(bucket)}
      </div>
      <div className='flex flex-col gap-2'>
        {rows.map((row) => (
          <div key={row.label} className='grid grid-cols-[1fr_auto] gap-3'>
            <div className='text-muted-foreground flex min-w-0 items-center gap-2 text-xs font-medium'>
              <span
                className={cn(
                  'size-2 shrink-0 rounded-full',
                  row.active ? 'bg-success' : 'bg-muted-foreground/45'
                )}
                aria-hidden='true'
              />
              <span className='truncate'>{row.label}</span>
            </div>
            <div
              className={cn(
                'font-mono text-xs font-semibold tabular-nums',
                row.valueClassName
              )}
            >
              {row.value}
            </div>
          </div>
        ))}
      </div>
    </div>
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

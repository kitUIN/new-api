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
import {
  Activity,
  CalendarDays,
  Gauge,
  HeartPulse,
  RefreshCw,
  Timer,
} from 'lucide-react'
import {
  CartesianGrid,
  Line,
  LineChart,
  XAxis,
  YAxis,
} from 'recharts'
import { useTranslation } from 'react-i18next'
import dayjs from '@/lib/dayjs'
import { getLobeIcon } from '@/lib/lobe-icon'
import { cn } from '@/lib/utils'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  ChartContainer,
  ChartTooltip,
  ChartTooltipContent,
  type ChartConfig,
} from '@/components/ui/chart'
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
import { Input } from '@/components/ui/input'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import {
  getGroupRatioHistory,
  getPerfGroupHealth,
} from '@/features/performance-metrics/api'
import {
  formatLatency,
  formatThroughput,
  formatUptimePct,
} from '@/features/performance-metrics/lib/format'
import type {
  GroupRatioHistorySeries,
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
const DEFAULT_RATIO_HISTORY_DAYS = 7
type RatioHistoryRangeMode = '7d' | 'week' | 'month' | 'custom'

type RatioHistoryRange = {
  mode: RatioHistoryRangeMode
  startTs: number
  endTs: number
}

type RatioChartPoint = {
  ts: number
  ratio: number
  source?: string
}

const ratioChartConfig = {
  ratio: {
    label: 'Ratio',
    color: 'var(--chart-1)',
  },
} satisfies ChartConfig
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

function getDefaultRatioHistoryRange(): RatioHistoryRange {
  const now = dayjs()
  return {
    mode: '7d',
    startTs: now.subtract(DEFAULT_RATIO_HISTORY_DAYS, 'day').unix(),
    endTs: now.unix(),
  }
}

function getPresetRatioHistoryRange(
  mode: Exclude<RatioHistoryRangeMode, 'custom'>
): RatioHistoryRange {
  const now = dayjs()
  if (mode === 'week') {
    return {
      mode,
      startTs: now.startOf('week').unix(),
      endTs: now.unix(),
    }
  }
  if (mode === 'month') {
    return {
      mode,
      startTs: now.startOf('month').unix(),
      endTs: now.unix(),
    }
  }
  return getDefaultRatioHistoryRange()
}

function toInputValue(ts: number): string {
  return dayjs.unix(ts).format('YYYY-MM-DDTHH:mm')
}

function parseInputTs(value: string): number | null {
  if (!value) return null
  const parsed = dayjs(value)
  return parsed.isValid() ? parsed.unix() : null
}

function formatRatioHistoryRangeLabel(range: RatioHistoryRange): string {
  return `${dayjs.unix(range.startTs).format('YYYY-MM-DD HH:mm')} ~ ${dayjs
    .unix(range.endTs)
    .format('YYYY-MM-DD HH:mm')}`
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
  const [ratioRange, setRatioRange] = useState<RatioHistoryRange>(
    getDefaultRatioHistoryRange
  )
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
  const ratioHistoryQuery = useQuery({
    queryKey: [
      'group-ratio-history',
      ratioRange.startTs,
      ratioRange.endTs,
    ],
    queryFn: () =>
      getGroupRatioHistory({
        start_ts: ratioRange.startTs,
        end_ts: ratioRange.endTs,
      }),
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
  const ratioHistoryMap = useMemo(() => {
    const map = new Map<string, GroupRatioHistorySeries>()
    for (const item of ratioHistoryQuery.data?.data.groups ?? []) {
      map.set(item.group, item)
    }
    return map
  }, [ratioHistoryQuery.data])
  const refetchAll = () => {
    healthQuery.refetch()
    ratioHistoryQuery.refetch()
  }

  if (healthQuery.isLoading) {
    return (
      <div className='flex flex-col gap-3'>
        <GroupHealthToolbar
          ratioRange={ratioRange}
          autoRefresh={autoRefresh}
          isFetching={healthQuery.isFetching || ratioHistoryQuery.isFetching}
          onRatioRangeChange={setRatioRange}
          onRefresh={refetchAll}
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
          ratioRange={ratioRange}
          autoRefresh={autoRefresh}
          isFetching={healthQuery.isFetching || ratioHistoryQuery.isFetching}
          onRatioRangeChange={setRatioRange}
          onRefresh={refetchAll}
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
        ratioRange={ratioRange}
        autoRefresh={autoRefresh}
        isFetching={healthQuery.isFetching || ratioHistoryQuery.isFetching}
        onRatioRangeChange={setRatioRange}
        onRefresh={refetchAll}
        onToggleAutoRefresh={() => setAutoRefresh((value) => !value)}
      />
      <div className='grid grid-cols-1 gap-3 lg:grid-cols-2 2xl:grid-cols-3'>
        {groups.map((group) => (
          <GroupHealthCard
            key={group.group}
            group={group}
            ratioHistory={ratioHistoryMap.get(group.group)}
            ratioRange={ratioRange}
          />
        ))}
      </div>
      <HealthLegend />
    </div>
  )
}

function GroupHealthToolbar(props: {
  ratioRange: RatioHistoryRange
  autoRefresh: boolean
  isFetching: boolean
  onRatioRangeChange: (range: RatioHistoryRange) => void
  onRefresh: () => void
  onToggleAutoRefresh: () => void
}) {
  const { t } = useTranslation()

  return (
    <div className='flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between'>
      <RatioHistoryRangePicker
        range={props.ratioRange}
        onChange={props.onRatioRangeChange}
      />
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
    </div>
  )
}

function RatioHistoryRangePicker(props: {
  range: RatioHistoryRange
  onChange: (range: RatioHistoryRange) => void
}) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  const [draftStart, setDraftStart] = useState(toInputValue(props.range.startTs))
  const [draftEnd, setDraftEnd] = useState(toInputValue(props.range.endTs))

  const handlePreset = (mode: Exclude<RatioHistoryRangeMode, 'custom'>) => {
    props.onChange(getPresetRatioHistoryRange(mode))
  }

  const handleOpenChange = (nextOpen: boolean) => {
    if (nextOpen) {
      setDraftStart(toInputValue(props.range.startTs))
      setDraftEnd(toInputValue(props.range.endTs))
    }
    setOpen(nextOpen)
  }

  const applyCustomRange = () => {
    const startTs = parseInputTs(draftStart)
    const endTs = parseInputTs(draftEnd)
    if (!startTs || !endTs || startTs >= endTs) return
    props.onChange({
      mode: 'custom',
      startTs,
      endTs,
    })
    setOpen(false)
  }

  return (
    <div className='flex flex-wrap items-center gap-1.5'>
      <Button
        type='button'
        variant={props.range.mode === '7d' ? 'default' : 'outline'}
        size='sm'
        onClick={() => handlePreset('7d')}
      >
        {t('Ratio range: last 7 days')}
      </Button>
      <Button
        type='button'
        variant={props.range.mode === 'week' ? 'default' : 'outline'}
        size='sm'
        onClick={() => handlePreset('week')}
      >
        {t('Ratio range: this week')}
      </Button>
      <Button
        type='button'
        variant={props.range.mode === 'month' ? 'default' : 'outline'}
        size='sm'
        onClick={() => handlePreset('month')}
      >
        {t('Ratio range: this month')}
      </Button>
      <Popover open={open} onOpenChange={handleOpenChange}>
        <PopoverTrigger
          render={
            <Button
              type='button'
              variant={props.range.mode === 'custom' ? 'default' : 'outline'}
              size='sm'
              className='max-w-full'
            />
          }
        >
          <CalendarDays data-icon='inline-start' aria-hidden='true' />
          <span className='truncate'>
            {props.range.mode === 'custom'
              ? formatRatioHistoryRangeLabel(props.range)
              : t('Custom')}
          </span>
        </PopoverTrigger>
        <PopoverContent
          align='start'
          className='w-[min(520px,calc(100vw-2rem))] p-3'
        >
          <div className='space-y-3'>
            <div className='grid gap-2 sm:grid-cols-[1fr_auto_1fr] sm:items-end'>
              <div className='space-y-1.5'>
                <div className='text-muted-foreground text-xs'>
                  {t('Start Time')}
                </div>
                <Input
                  type='datetime-local'
                  value={draftStart}
                  onChange={(event) => setDraftStart(event.target.value)}
                  className='h-8 text-sm leading-5 tabular-nums'
                />
              </div>
              <span className='text-muted-foreground hidden pb-2 text-xs sm:block'>
                ~
              </span>
              <div className='space-y-1.5'>
                <div className='text-muted-foreground text-xs'>
                  {t('End Time')}
                </div>
                <Input
                  type='datetime-local'
                  value={draftEnd}
                  onChange={(event) => setDraftEnd(event.target.value)}
                  className='h-8 text-sm leading-5 tabular-nums'
                />
              </div>
            </div>
            <div className='flex justify-end'>
              <Button size='sm' className='h-8' onClick={applyCustomRange}>
                {t('Confirm')}
              </Button>
            </div>
          </div>
        </PopoverContent>
      </Popover>
    </div>
  )
}

function GroupHealthCard(props: {
  group: PerfGroupHealth
  ratioHistory: GroupRatioHistorySeries | undefined
  ratioRange: RatioHistoryRange
}) {
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

        <RatioHistoryChart
          currentRatio={group.ratio}
          history={props.ratioHistory}
          range={props.ratioRange}
        />
      </div>
    </section>
  )
}

function buildRatioChartData(
  currentRatio: number,
  history: GroupRatioHistorySeries | undefined,
  range: RatioHistoryRange
): RatioChartPoint[] {
  const points: RatioChartPoint[] = [...(history?.points ?? [])]
    .filter((point) => point.ts >= range.startTs && point.ts <= range.endTs)
    .sort((a, b) => a.ts - b.ts)
    .map((point) => ({
      ts: point.ts,
      ratio: point.ratio,
      source: point.source,
    }))

  if (!points.length) {
    points.push({
      ts: range.startTs,
      ratio: currentRatio,
    })
  }

  const last = points[points.length - 1]
  if (last && last.ts < range.endTs) {
    points.push({
      ts: range.endTs,
      ratio: last.ratio,
    })
  }

  return points
}

function RatioHistoryChart(props: {
  currentRatio: number
  history: GroupRatioHistorySeries | undefined
  range: RatioHistoryRange
}) {
  const { t } = useTranslation()
  const data = useMemo(
    () => buildRatioChartData(props.currentRatio, props.history, props.range),
    [props.currentRatio, props.history, props.range]
  )
  const changedCount = Math.max(0, data.length - 2)

  return (
    <div className='rounded-md border bg-muted/20 px-2.5 py-2'>
      <div className='mb-1 flex items-center justify-between gap-2'>
        <div className='text-muted-foreground truncate text-[10px] font-medium'>
          {t('Ratio history')}
        </div>
        <div className='text-muted-foreground shrink-0 font-mono text-[10px] tabular-nums'>
          {changedCount > 0
            ? t('{{count}} changes', { count: changedCount })
            : t('No ratio changes')}
        </div>
      </div>
      <ChartContainer
        config={ratioChartConfig}
        className='h-24 w-full aspect-auto'
        initialDimension={{ width: 320, height: 96 }}
      >
        <LineChart data={data} margin={{ top: 8, right: 8, bottom: 0, left: 0 }}>
          <CartesianGrid vertical={false} strokeDasharray='3 3' />
          <XAxis
            dataKey='ts'
            type='number'
            domain={[props.range.startTs, props.range.endTs]}
            tickFormatter={(value) => dayjs.unix(Number(value)).format('MM-DD')}
            tickLine={false}
            axisLine={false}
            tickMargin={6}
            minTickGap={28}
          />
          <YAxis
            dataKey='ratio'
            width={32}
            tickFormatter={(value) => formatRatio(Number(value))}
            tickLine={false}
            axisLine={false}
            tickMargin={4}
            domain={['dataMin', 'dataMax']}
          />
          <ChartTooltip
            cursor={false}
            content={
              <ChartTooltipContent
                hideLabel
                formatter={(value, _name, item) => (
                  <div className='grid gap-1'>
                    <div className='font-mono text-xs font-semibold tabular-nums'>
                      {formatRatio(Number(value))}
                    </div>
                    <div className='text-muted-foreground text-[11px]'>
                      {dayjs
                        .unix(Number(item.payload?.ts ?? 0))
                        .format('YYYY-MM-DD HH:mm')}
                    </div>
                  </div>
                )}
              />
            }
          />
          <Line
            type='stepAfter'
            dataKey='ratio'
            stroke='var(--color-ratio)'
            strokeWidth={2}
            dot={{ r: 2 }}
            activeDot={{ r: 4 }}
            isAnimationActive={false}
          />
        </LineChart>
      </ChartContainer>
    </div>
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

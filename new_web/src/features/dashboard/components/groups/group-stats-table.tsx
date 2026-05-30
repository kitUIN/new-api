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
import { Fragment, useCallback, useEffect, useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import {
  ChevronRight,
  CircleDollarSign,
  Hash,
  Layers3,
  Loader2,
  Sigma,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { formatQuotaWithCurrency } from '@/lib/currency'
import { getRollingDateRange } from '@/lib/time'
import { cn } from '@/lib/utils'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { getGroupQuotaData } from '@/features/dashboard/api'
import { TIME_RANGE_PRESETS } from '@/features/dashboard/constants'
import type { GroupQuotaDataItem } from '@/features/dashboard/types'

interface GroupModelStats {
  model: string
  quota: number
  count: number
  tokens: number
  promptTokens: number
  completionTokens: number
  cacheReadTokens: number
  cacheWriteTokens: number
}

interface GroupStats {
  group: string
  quota: number
  count: number
  tokens: number
  promptTokens: number
  completionTokens: number
  cacheReadTokens: number
  cacheWriteTokens: number
  models: GroupModelStats[]
}

interface GroupStatsSummary {
  totalQuota: number
  totalCount: number
  totalTokens: number
  groupCount: number
}

function formatInt(value: number): string {
  return Intl.NumberFormat(undefined, { maximumFractionDigits: 0 }).format(
    value
  )
}

function formatCost(value: number): string {
  return formatQuotaWithCurrency(value, {
    digitsLarge: 2,
    digitsSmall: 4,
    abbreviate: false,
  })
}

function normalizeGroup(group?: string, fallback = 'unknown') {
  const trimmed = group?.trim()
  return trimmed || fallback
}

function normalizeModel(model?: string) {
  const trimmed = model?.trim()
  return trimmed || 'Unknown'
}

function getTokenTotal(item: GroupQuotaDataItem) {
  const promptTokens = Number(item.prompt_tokens) || 0
  const completionTokens = Number(item.completion_tokens) || 0
  const cacheReadTokens = Number(item.cache_read_tokens) || 0
  const cacheWriteTokens = Number(item.cache_write_tokens) || 0
  const breakdownTotal =
    promptTokens + completionTokens + cacheReadTokens + cacheWriteTokens
  return Number(item.token_used) || breakdownTotal
}

function emptyModelStats(model: string): GroupModelStats {
  return {
    model,
    quota: 0,
    count: 0,
    tokens: 0,
    promptTokens: 0,
    completionTokens: 0,
    cacheReadTokens: 0,
    cacheWriteTokens: 0,
  }
}

function emptyGroupStats(group: string): GroupStats {
  return {
    group,
    quota: 0,
    count: 0,
    tokens: 0,
    promptTokens: 0,
    completionTokens: 0,
    cacheReadTokens: 0,
    cacheWriteTokens: 0,
    models: [],
  }
}

function processGroupStats(data: GroupQuotaDataItem[]) {
  const groups = new Map<string, GroupStats>()
  const groupModels = new Map<string, Map<string, GroupModelStats>>()

  data.forEach((item) => {
    const group = normalizeGroup(item.group)
    const model = normalizeModel(item.model_name)
    const quota = Number(item.quota) || 0
    const count = Number(item.count) || 0
    const tokens = getTokenTotal(item)
    const promptTokens = Number(item.prompt_tokens) || 0
    const completionTokens = Number(item.completion_tokens) || 0
    const cacheReadTokens = Number(item.cache_read_tokens) || 0
    const cacheWriteTokens = Number(item.cache_write_tokens) || 0

    const groupStats = groups.get(group) ?? emptyGroupStats(group)
    groupStats.quota += quota
    groupStats.count += count
    groupStats.tokens += tokens
    groupStats.promptTokens += promptTokens
    groupStats.completionTokens += completionTokens
    groupStats.cacheReadTokens += cacheReadTokens
    groupStats.cacheWriteTokens += cacheWriteTokens
    groups.set(group, groupStats)

    if (!groupModels.has(group)) groupModels.set(group, new Map())
    const modelMap = groupModels.get(group)!
    const modelStats = modelMap.get(model) ?? emptyModelStats(model)
    modelStats.quota += quota
    modelStats.count += count
    modelStats.tokens += tokens
    modelStats.promptTokens += promptTokens
    modelStats.completionTokens += completionTokens
    modelStats.cacheReadTokens += cacheReadTokens
    modelStats.cacheWriteTokens += cacheWriteTokens
    modelMap.set(model, modelStats)
  })

  const rows = Array.from(groups.values())
    .map((group) => ({
      ...group,
      models: Array.from(groupModels.get(group.group)?.values() ?? []).sort(
        (a, b) => b.quota - a.quota || b.tokens - a.tokens
      ),
    }))
    .sort((a, b) => b.quota - a.quota || b.tokens - a.tokens)

  const summary: GroupStatsSummary = rows.reduce(
    (acc, group) => ({
      totalQuota: acc.totalQuota + group.quota,
      totalCount: acc.totalCount + group.count,
      totalTokens: acc.totalTokens + group.tokens,
      groupCount: acc.groupCount,
    }),
    {
      totalQuota: 0,
      totalCount: 0,
      totalTokens: 0,
      groupCount: rows.length,
    }
  )

  return { rows, summary }
}

function SummaryItem(props: {
  label: string
  value: string
  icon: typeof CircleDollarSign
}) {
  const Icon = props.icon

  return (
    <div className='rounded-lg border px-4 py-3 shadow-xs'>
      <div className='text-muted-foreground flex items-center gap-2 text-xs font-medium'>
        <Icon className='size-3.5' aria-hidden='true' />
        {props.label}
      </div>
      <div className='mt-1.5 truncate text-xl font-semibold tabular-nums'>
        {props.value}
      </div>
    </div>
  )
}

function MetricCells(props: { stats: GroupStats | GroupModelStats }) {
  const cacheTokens = props.stats.cacheReadTokens + props.stats.cacheWriteTokens

  return (
    <>
      <TableCell className='text-right font-medium'>
        {formatCost(props.stats.quota)}
      </TableCell>
      <TableCell className='text-right'>
        {formatInt(props.stats.tokens)}
      </TableCell>
      <TableCell className='text-right'>
        {formatInt(props.stats.count)}
      </TableCell>
      <TableCell className='text-right'>
        {formatInt(props.stats.promptTokens)}
      </TableCell>
      <TableCell className='text-right'>
        {formatInt(props.stats.completionTokens)}
      </TableCell>
      <TableCell className='text-right'>{formatInt(cacheTokens)}</TableCell>
    </>
  )
}

function TableSkeletonRows() {
  return (
    <>
      {Array.from({ length: 6 }).map((_, index) => (
        <TableRow key={index}>
          {Array.from({ length: 7 }).map((__, cellIndex) => (
            <TableCell key={cellIndex}>
              <Skeleton className='h-4 w-full' />
            </TableCell>
          ))}
        </TableRow>
      ))}
    </>
  )
}

export function GroupStatsTable() {
  const { t } = useTranslation()
  const [selectedRange, setSelectedRange] = useState(7)
  const [expandedGroups, setExpandedGroups] = useState<Set<string>>(new Set())
  const [timeRange, setTimeRange] = useState(() => {
    const { start, end } = getRollingDateRange(7)
    return {
      start_timestamp: Math.floor(start.getTime() / 1000),
      end_timestamp: Math.floor(end.getTime() / 1000),
    }
  })

  const handleRangeChange = useCallback((days: number) => {
    setSelectedRange(days)
    const { start, end } = getRollingDateRange(days)
    setTimeRange({
      start_timestamp: Math.floor(start.getTime() / 1000),
      end_timestamp: Math.floor(end.getTime() / 1000),
    })
  }, [])

  const { data, isLoading, isFetching } = useQuery({
    queryKey: ['dashboard', 'group-quota', timeRange],
    queryFn: () => getGroupQuotaData(timeRange),
    select: (res) => (res.success ? res.data : []),
    staleTime: 60_000,
  })

  const { rows, summary } = useMemo(
    () => processGroupStats(isLoading ? [] : (data ?? [])),
    [data, isLoading]
  )

  useEffect(() => {
    setExpandedGroups(new Set(rows.map((row) => row.group)))
  }, [rows])

  const toggleGroup = useCallback((group: string) => {
    setExpandedGroups((current) => {
      const next = new Set(current)
      if (next.has(group)) {
        next.delete(group)
      } else {
        next.add(group)
      }
      return next
    })
  }, [])

  return (
    <div className='space-y-3'>
      <div className='flex items-center gap-1.5 overflow-x-auto pb-1 sm:gap-2'>
        <Tabs
          value={String(selectedRange)}
          onValueChange={(value) => handleRangeChange(Number(value))}
          className='shrink-0'
        >
          <TabsList>
            {TIME_RANGE_PRESETS.map((preset) => (
              <TabsTrigger
                key={preset.days}
                value={String(preset.days)}
                className='px-2.5 text-xs'
              >
                {t(preset.label)}
              </TabsTrigger>
            ))}
          </TabsList>
        </Tabs>

        {isFetching && (
          <Loader2 className='text-muted-foreground size-4 animate-spin' />
        )}
      </div>

      <div className='grid gap-3 sm:grid-cols-2 xl:grid-cols-4'>
        <SummaryItem
          label={t('Total cost')}
          value={formatCost(summary.totalQuota)}
          icon={CircleDollarSign}
        />
        <SummaryItem
          label={t('Total tokens')}
          value={formatInt(summary.totalTokens)}
          icon={Sigma}
        />
        <SummaryItem
          label={t('Total calls')}
          value={formatInt(summary.totalCount)}
          icon={Hash}
        />
        <SummaryItem
          label={t('Groups')}
          value={formatInt(summary.groupCount)}
          icon={Layers3}
        />
      </div>

      <div className='overflow-hidden rounded-lg border'>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className='min-w-56'>{t('Group / Model')}</TableHead>
              <TableHead className='text-right'>{t('Cost')}</TableHead>
              <TableHead className='text-right'>{t('Tokens')}</TableHead>
              <TableHead className='text-right'>{t('Calls')}</TableHead>
              <TableHead className='text-right'>{t('Input tokens')}</TableHead>
              <TableHead className='text-right'>{t('Output tokens')}</TableHead>
              <TableHead className='text-right'>{t('Cache tokens')}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {isLoading ? (
              <TableSkeletonRows />
            ) : rows.length === 0 ? (
              <TableRow>
                <TableCell
                  colSpan={7}
                  className='text-muted-foreground h-32 text-center'
                >
                  {t('No data available')}
                </TableCell>
              </TableRow>
            ) : (
              rows.map((group) => {
                const expanded = expandedGroups.has(group.group)

                return (
                  <Fragment key={group.group}>
                    <TableRow
                      aria-expanded={expanded}
                      className='bg-muted/35 font-medium'
                    >
                      <TableCell>
                        <button
                          type='button'
                          onClick={() => toggleGroup(group.group)}
                          className='focus-visible:ring-ring flex min-w-0 items-center gap-2 rounded-md outline-none focus-visible:ring-2'
                        >
                          <ChevronRight
                            className={cn(
                              'text-muted-foreground size-4 shrink-0 transition-transform',
                              expanded && 'rotate-90'
                            )}
                            aria-hidden='true'
                          />
                          <span className='truncate'>{group.group}</span>
                          <span className='text-muted-foreground text-xs'>
                            {t('{{count}} models', {
                              count: group.models.length,
                            })}
                          </span>
                        </button>
                      </TableCell>
                      <MetricCells stats={group} />
                    </TableRow>
                    {expanded &&
                      group.models.map((model) => (
                        <TableRow key={`${group.group}-${model.model}`}>
                          <TableCell className='pl-10'>
                            <span className='block max-w-80 truncate'>
                              {model.model}
                            </span>
                          </TableCell>
                          <MetricCells stats={model} />
                        </TableRow>
                      ))}
                  </Fragment>
                )
              })
            )}
          </TableBody>
        </Table>
      </div>
    </div>
  )
}

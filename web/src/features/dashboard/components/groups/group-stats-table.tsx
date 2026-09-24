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
  Users,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import {
  getQQAvatarUrl,
  getUserAvatarFallback,
  getUserAvatarStyle,
} from '@/lib/avatar'
import { formatQuotaWithCurrency } from '@/lib/currency'
import { formatPercent } from '@/lib/format'
import { cn } from '@/lib/utils'
import { Avatar, AvatarFallback, AvatarImage } from '@/components/ui/avatar'
import { Combobox } from '@/components/ui/combobox'
import { Skeleton } from '@/components/ui/skeleton'
import { Switch } from '@/components/ui/switch'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { CompactDateTimeRangePicker } from '@/components/compact-date-time-range-picker'
import { getGroupQuotaData, getGroupQuotaUsers } from '@/features/dashboard/api'
import {
  DASHBOARD_STATS_ALL_RANGE_VALUE,
  DASHBOARD_STATS_CUSTOM_RANGE_VALUE,
  TIME_RANGE_PRESETS,
} from '@/features/dashboard/constants'
import {
  getAllUnixTimeRange,
  getPresetDateRange,
  getPresetUnixTimeRange,
  toUnixTimeRange,
} from '@/features/dashboard/lib'
import type { GroupQuotaDataItem } from '@/features/dashboard/types'
import {
  formatLatency,
  formatThroughput,
} from '@/features/performance-metrics/lib/format'

type GroupBreakdownDimension = 'model' | 'user'

interface GroupDetailStats {
  key: string
  label: string
  userId?: number
  username?: string
  displayName?: string
  qqId?: string
  quota: number
  count: number
  tokens: number
  promptTokens: number
  completionTokens: number
  cacheReadTokens: number
  cacheWriteTokens: number
  perfRequestCount: number
  latencyCount: number
  totalLatencyMs: number
  totalTTFTMs: number
  ttftCount: number
  perfCompletionTokens: number
  totalTPSLatencyMs: number
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
  perfRequestCount: number
  latencyCount: number
  totalLatencyMs: number
  totalTTFTMs: number
  ttftCount: number
  perfCompletionTokens: number
  totalTPSLatencyMs: number
  details: GroupDetailStats[]
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

function getUserDisplayName(
  displayName?: string,
  username?: string,
  userId?: number
) {
  const nickname = displayName?.trim()
  const trimmed = username?.trim()
  return nickname || trimmed || (userId ? `#${userId}` : 'Unknown')
}

function formatUserSecondary(
  displayName?: string,
  username?: string,
  userId?: number
) {
  const nickname = displayName?.trim()
  const trimmed = username?.trim()
  const usernameLabel = trimmed && trimmed !== nickname ? trimmed : ''
  const idLabel = userId ? `#${userId}` : ''
  return [usernameLabel, idLabel].filter(Boolean).join(' / ')
}

function formatUserLabel(
  displayName?: string,
  username?: string,
  userId?: number
) {
  const primary = getUserDisplayName(displayName, username, userId)
  const secondary = formatUserSecondary(displayName, username, userId)
  return secondary ? `${primary} (${secondary})` : primary
}

function getTokenTotal(item: GroupQuotaDataItem) {
  const promptTokens = Number(item.prompt_tokens) || 0
  const completionTokens = Number(item.completion_tokens) || 0
  const cacheWriteTokens = Number(item.cache_write_tokens) || 0
  const breakdownTotal = promptTokens + completionTokens + cacheWriteTokens
  const tokenUsed = Number(item.token_used) || 0
  return tokenUsed > 0 ? tokenUsed : breakdownTotal
}

const ALL_USERS_VALUE = 'all'

function emptyDetailStats(key: string, label: string): GroupDetailStats {
  return {
    key,
    label,
    quota: 0,
    count: 0,
    tokens: 0,
    promptTokens: 0,
    completionTokens: 0,
    cacheReadTokens: 0,
    cacheWriteTokens: 0,
    perfRequestCount: 0,
    latencyCount: 0,
    totalLatencyMs: 0,
    totalTTFTMs: 0,
    ttftCount: 0,
    perfCompletionTokens: 0,
    totalTPSLatencyMs: 0,
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
    perfRequestCount: 0,
    latencyCount: 0,
    totalLatencyMs: 0,
    totalTTFTMs: 0,
    ttftCount: 0,
    perfCompletionTokens: 0,
    totalTPSLatencyMs: 0,
    details: [],
  }
}

function getPerformanceSummary(stats: GroupStats | GroupDetailStats) {
  const avgTTFTMs =
    stats.ttftCount > 0 ? stats.totalTTFTMs / stats.ttftCount : 0
  const avgLatencyMs =
    stats.latencyCount > 0 ? stats.totalLatencyMs / stats.latencyCount : 0
  const avgTps =
    stats.totalTPSLatencyMs > 0
      ? stats.perfCompletionTokens / (stats.totalTPSLatencyMs / 1000)
      : 0

  return { avgTTFTMs, avgLatencyMs, avgTps }
}

function processGroupStats(
  data: GroupQuotaDataItem[],
  dimension: GroupBreakdownDimension
) {
  const groups = new Map<string, GroupStats>()
  const groupDetails = new Map<string, Map<string, GroupDetailStats>>()

  data.forEach((item) => {
    const group = normalizeGroup(item.group)
    const detailKey =
      dimension === 'user'
        ? String(item.user_id || item.username || 'unknown')
        : normalizeModel(item.model_name)
    const detailLabel =
      dimension === 'user'
        ? formatUserLabel(item.display_name, item.username, item.user_id)
        : normalizeModel(item.model_name)
    const quota = Number(item.quota) || 0
    const count = Number(item.count) || 0
    const tokens = getTokenTotal(item)
    const promptTokens = Number(item.prompt_tokens) || 0
    const completionTokens = Number(item.completion_tokens) || 0
    const cacheReadTokens = Number(item.cache_read_tokens) || 0
    const cacheWriteTokens = Number(item.cache_write_tokens) || 0
    const perfRequestCount = Number(item.perf_request_count) || 0
    const latencyCount = Number(item.latency_count) || 0
    const totalLatencyMs = Number(item.total_latency_ms) || 0
    const totalTTFTMs = Number(item.total_ttft_ms) || 0
    const ttftCount = Number(item.ttft_count) || 0
    const perfCompletionTokens = Number(item.perf_completion_tokens) || 0
    const totalTPSLatencyMs = Number(item.total_tps_latency_ms) || 0

    const groupStats = groups.get(group) ?? emptyGroupStats(group)
    groupStats.quota += quota
    groupStats.count += count
    groupStats.tokens += tokens
    groupStats.promptTokens += promptTokens
    groupStats.completionTokens += completionTokens
    groupStats.cacheReadTokens += cacheReadTokens
    groupStats.cacheWriteTokens += cacheWriteTokens
    groupStats.perfRequestCount += perfRequestCount
    groupStats.latencyCount += latencyCount
    groupStats.totalLatencyMs += totalLatencyMs
    groupStats.totalTTFTMs += totalTTFTMs
    groupStats.ttftCount += ttftCount
    groupStats.perfCompletionTokens += perfCompletionTokens
    groupStats.totalTPSLatencyMs += totalTPSLatencyMs
    groups.set(group, groupStats)

    if (!groupDetails.has(group)) groupDetails.set(group, new Map())
    const detailMap = groupDetails.get(group)!
    const detailStats =
      detailMap.get(detailKey) ?? emptyDetailStats(detailKey, detailLabel)
    if (dimension === 'user') {
      detailStats.userId = item.user_id
      detailStats.username = item.username
      detailStats.displayName = item.display_name
      detailStats.qqId = item.qq_id
    }
    detailStats.quota += quota
    detailStats.count += count
    detailStats.tokens += tokens
    detailStats.promptTokens += promptTokens
    detailStats.completionTokens += completionTokens
    detailStats.cacheReadTokens += cacheReadTokens
    detailStats.cacheWriteTokens += cacheWriteTokens
    detailStats.perfRequestCount += perfRequestCount
    detailStats.latencyCount += latencyCount
    detailStats.totalLatencyMs += totalLatencyMs
    detailStats.totalTTFTMs += totalTTFTMs
    detailStats.ttftCount += ttftCount
    detailStats.perfCompletionTokens += perfCompletionTokens
    detailStats.totalTPSLatencyMs += totalTPSLatencyMs
    detailMap.set(detailKey, detailStats)
  })

  const rows = Array.from(groups.values())
    .map((group) => ({
      ...group,
      details: Array.from(groupDetails.get(group.group)?.values() ?? []).sort(
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

function MetricCells(props: {
  stats: GroupStats | GroupDetailStats
  showPerformance: boolean
}) {
  const cacheTokens = props.stats.cacheReadTokens + props.stats.cacheWriteTokens
  const cacheRatio =
    props.stats.promptTokens > 0
      ? (props.stats.cacheReadTokens / props.stats.promptTokens) * 100
      : 0
  const perf = getPerformanceSummary(props.stats)

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
      {props.showPerformance && (
        <>
          <TableCell className='text-right'>
            {formatLatency(perf.avgTTFTMs)}
          </TableCell>
          <TableCell className='text-right'>
            {formatLatency(perf.avgLatencyMs)}
          </TableCell>
          <TableCell className='text-right'>
            {formatThroughput(perf.avgTps)}
          </TableCell>
        </>
      )}
      <TableCell className='text-right'>
        <span>{formatInt(props.stats.promptTokens)}</span>
        <span className='text-muted-foreground ml-1'>
          ({formatPercent(cacheRatio)})
        </span>
      </TableCell>
      <TableCell className='text-right'>
        {formatInt(props.stats.completionTokens)}
      </TableCell>
      <TableCell className='text-right'>{formatInt(cacheTokens)}</TableCell>
    </>
  )
}

function TableSkeletonRows({ columnCount }: { columnCount: number }) {
  return (
    <>
      {Array.from({ length: 6 }).map((_, index) => (
        <TableRow key={index}>
          {Array.from({ length: columnCount }).map((__, cellIndex) => (
            <TableCell key={cellIndex}>
              <Skeleton className='h-4 w-full' />
            </TableCell>
          ))}
        </TableRow>
      ))}
    </>
  )
}

function UserAvatar(props: {
  displayName?: string
  username?: string
  userId?: number
  qqId?: string
  className?: string
}) {
  const avatarName = getUserDisplayName(
    props.displayName,
    props.username,
    props.userId
  )
  const avatarUrl = getQQAvatarUrl(props.qqId)

  return (
    <Avatar className={cn('ring-border/60 size-6 ring-1', props.className)}>
      {avatarUrl && <AvatarImage src={avatarUrl} alt={avatarName} />}
      <AvatarFallback
        className='text-[10px] font-semibold text-white'
        style={getUserAvatarStyle(avatarName)}
      >
        {getUserAvatarFallback(avatarName)}
      </AvatarFallback>
    </Avatar>
  )
}

function UserIdentity(props: {
  displayName?: string
  username?: string
  userId?: number
  qqId?: string
}) {
  const primary = getUserDisplayName(
    props.displayName,
    props.username,
    props.userId
  )
  const secondary = formatUserSecondary(
    props.displayName,
    props.username,
    props.userId
  )

  return (
    <div className='flex max-w-80 min-w-0 items-center gap-2'>
      <UserAvatar
        displayName={props.displayName}
        username={props.username}
        userId={props.userId}
        qqId={props.qqId}
      />
      <div className='min-w-0'>
        <div className='truncate font-medium'>{primary}</div>
        {secondary && (
          <div className='text-muted-foreground truncate text-xs'>
            {secondary}
          </div>
        )}
      </div>
    </div>
  )
}

interface GroupStatsTableProps {
  isAdmin?: boolean
}

export function GroupStatsTable({ isAdmin = false }: GroupStatsTableProps) {
  const { t } = useTranslation()
  const [selectedRange, setSelectedRange] = useState<string>('7')
  const [selectedUserId, setSelectedUserId] = useState(ALL_USERS_VALUE)
  const [breakdownDimension, setBreakdownDimension] =
    useState<GroupBreakdownDimension>('model')
  const [expandedGroups, setExpandedGroups] = useState<Set<string>>(new Set())
  const [customRange, setCustomRange] = useState<{
    start?: Date
    end?: Date
  }>(() => getPresetDateRange(7))
  const [timeRange, setTimeRange] = useState(() => {
    return getPresetUnixTimeRange(7)
  })

  const handleRangeChange = useCallback(
    (value: string) => {
      setSelectedRange(value)

      if (value === DASHBOARD_STATS_ALL_RANGE_VALUE) {
        setTimeRange(getAllUnixTimeRange())
        return
      }

      if (value === DASHBOARD_STATS_CUSTOM_RANGE_VALUE) {
        if (customRange.start && customRange.end) {
          setTimeRange(
            toUnixTimeRange({ start: customRange.start, end: customRange.end })
          )
        }
        return
      }

      setTimeRange(getPresetUnixTimeRange(Number(value)))
    },
    [customRange.end, customRange.start]
  )

  const handleCustomRangeChange = useCallback(
    (range: { start?: Date; end?: Date }) => {
      setSelectedRange(DASHBOARD_STATS_CUSTOM_RANGE_VALUE)
      setCustomRange(range)
      if (range.start && range.end) {
        setTimeRange(toUnixTimeRange({ start: range.start, end: range.end }))
      }
    },
    []
  )

  const {
    data: quotaUsersData,
    isFetching: isUsersFetching,
    isLoading: isUsersLoading,
  } = useQuery({
    queryKey: ['dashboard', 'group-quota-users', timeRange],
    queryFn: () => getGroupQuotaUsers(timeRange),
    select: (res) => (res.success ? res.data : []),
    staleTime: 60_000,
    enabled: isAdmin,
  })

  const userOptions = useMemo(
    () => [
      {
        value: ALL_USERS_VALUE,
        label: t('All users'),
        icon: <Users className='text-muted-foreground size-4' />,
      },
      ...(quotaUsersData ?? [])
        .slice()
        .sort(
          (a, b) =>
            (Number(b.quota) || 0) - (Number(a.quota) || 0) ||
            a.username.localeCompare(b.username)
        )
        .map((user) => ({
          value: String(user.user_id),
          label: formatUserLabel(
            user.display_name,
            user.username,
            user.user_id
          ),
          icon: (
            <UserAvatar
              displayName={user.display_name}
              username={user.username}
              userId={user.user_id}
              qqId={user.qq_id}
              className='size-5'
            />
          ),
        })),
    ],
    [quotaUsersData, t]
  )

  useEffect(() => {
    if (
      !isUsersLoading &&
      quotaUsersData !== undefined &&
      selectedUserId !== ALL_USERS_VALUE &&
      !userOptions.some((option) => option.value === selectedUserId)
    ) {
      setSelectedUserId(ALL_USERS_VALUE)
    }
  }, [isUsersLoading, quotaUsersData, selectedUserId, userOptions])

  const { data, isLoading, isFetching } = useQuery({
    queryKey: [
      'dashboard',
      'group-quota',
      isAdmin,
      timeRange,
      selectedUserId,
      breakdownDimension,
    ],
    queryFn: () =>
      getGroupQuotaData(
        {
          ...timeRange,
          ...(isAdmin && selectedUserId !== ALL_USERS_VALUE
            ? { user_id: Number(selectedUserId) }
            : {}),
          ...(isAdmin ? { dimension: breakdownDimension } : {}),
        },
        isAdmin
      ),
    select: (res) => (res.success ? res.data : []),
    staleTime: 60_000,
  })

  const { rows, summary } = useMemo(
    () =>
      processGroupStats(
        isLoading ? [] : (data ?? []),
        isAdmin ? breakdownDimension : 'model'
      ),
    [breakdownDimension, data, isAdmin, isLoading]
  )

  const showUserBreakdown = isAdmin && breakdownDimension === 'user'
  const showPerformance =
    !showUserBreakdown && (!isAdmin || selectedUserId === ALL_USERS_VALUE)
  const columnCount =
    7 + (showUserBreakdown ? 1 : 0) + (showPerformance ? 3 : 0)

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
      <div className='flex flex-wrap items-center gap-1.5 pb-1 sm:gap-2'>
        <Tabs
          value={selectedRange}
          onValueChange={handleRangeChange}
          className='shrink-0'
        >
          <TabsList>
            <TabsTrigger
              value={DASHBOARD_STATS_ALL_RANGE_VALUE}
              className='px-2.5 text-xs'
            >
              {t('All')}
            </TabsTrigger>
            {TIME_RANGE_PRESETS.map((preset) => (
              <TabsTrigger
                key={preset.days}
                value={String(preset.days)}
                className='px-2.5 text-xs'
              >
                {t(preset.label)}
              </TabsTrigger>
            ))}
            <TabsTrigger
              value={DASHBOARD_STATS_CUSTOM_RANGE_VALUE}
              className='px-2.5 text-xs'
            >
              {t('Custom')}
            </TabsTrigger>
          </TabsList>
        </Tabs>

        {selectedRange === DASHBOARD_STATS_CUSTOM_RANGE_VALUE && (
          <CompactDateTimeRangePicker
            start={customRange.start}
            end={customRange.end}
            onChange={handleCustomRangeChange}
            className='h-8 w-[min(26rem,calc(100vw-2rem))] text-xs'
          />
        )}

        {isAdmin && (
          <>
            <div className='flex items-center gap-2'>
              <label
                htmlFor='group-user-filter'
                className='text-xs font-medium whitespace-nowrap'
              >
                {t('User')}
              </label>
              <Combobox
                id='group-user-filter'
                options={userOptions}
                value={selectedUserId}
                onValueChange={(value) =>
                  setSelectedUserId(value ?? ALL_USERS_VALUE)
                }
                searchPlaceholder={t('Select user')}
                emptyText={t('No users found')}
                className='h-8 w-56 text-xs'
              />
            </div>
            <label
              htmlFor='group-user-dimension'
              className='flex h-8 cursor-pointer items-center gap-2 px-1 text-xs font-medium whitespace-nowrap'
            >
              <Users
                className='text-muted-foreground size-4'
                aria-hidden='true'
              />
              {t('User dimension')}
              <Switch
                id='group-user-dimension'
                checked={breakdownDimension === 'user'}
                onCheckedChange={(checked) =>
                  setBreakdownDimension(checked ? 'user' : 'model')
                }
                aria-label={t('User dimension')}
              />
            </label>
          </>
        )}

        {(isFetching || isUsersFetching) && (
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
              <TableHead className='min-w-56'>
                {showUserBreakdown ? t('Group / User') : t('Group / Model')}
              </TableHead>
              {showUserBreakdown && (
                <TableHead className='text-right'>{t('Share')}</TableHead>
              )}
              <TableHead className='text-right'>{t('Cost')}</TableHead>
              <TableHead className='text-right'>{t('Tokens')}</TableHead>
              <TableHead className='text-right'>{t('Calls')}</TableHead>
              {showPerformance && (
                <>
                  <TableHead className='text-right'>
                    {t('First-token latency')}
                  </TableHead>
                  <TableHead className='text-right'>
                    {t('Average latency')}
                  </TableHead>
                  <TableHead className='text-right'>
                    {t('Average token/s')}
                  </TableHead>
                </>
              )}
              <TableHead className='text-right'>{t('Input tokens')}</TableHead>
              <TableHead className='text-right'>{t('Output tokens')}</TableHead>
              <TableHead className='text-right'>{t('Cache tokens')}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {isLoading ? (
              <TableSkeletonRows columnCount={columnCount} />
            ) : rows.length === 0 ? (
              <TableRow>
                <TableCell
                  colSpan={columnCount}
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
                            {showUserBreakdown
                              ? t('{{count}} users', {
                                  count: group.details.length,
                                })
                              : t('{{count}} models', {
                                  count: group.details.length,
                                })}
                          </span>
                        </button>
                      </TableCell>
                      {showUserBreakdown && (
                        <TableCell className='text-right'>100%</TableCell>
                      )}
                      <MetricCells
                        stats={group}
                        showPerformance={showPerformance}
                      />
                    </TableRow>
                    {expanded &&
                      group.details.map((detail) => (
                        <TableRow key={`${group.group}-${detail.key}`}>
                          <TableCell className='pl-10'>
                            {showUserBreakdown ? (
                              <UserIdentity
                                displayName={detail.displayName}
                                username={detail.username}
                                userId={detail.userId}
                                qqId={detail.qqId}
                              />
                            ) : (
                              <span className='block max-w-80 truncate'>
                                {detail.label}
                              </span>
                            )}
                          </TableCell>
                          {showUserBreakdown && (
                            <TableCell className='text-right font-medium'>
                              {formatPercent(
                                group.quota !== 0
                                  ? (detail.quota / group.quota) * 100
                                  : 0
                              )}
                            </TableCell>
                          )}
                          <MetricCells
                            stats={detail}
                            showPerformance={showPerformance}
                          />
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

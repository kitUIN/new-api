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
import { useQuery } from '@tanstack/react-query'
import { getRouteApi } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { formatLogQuota } from '@/lib/format'
import { cn } from '@/lib/utils'
import { useIsAdmin } from '@/hooks/use-admin'
import { Skeleton } from '@/components/ui/skeleton'
import { getLogStats, getUserLogStats } from '../api'
import { DEFAULT_LOG_STATS } from '../constants'
import { buildApiParams } from '../lib/utils'
import { useUsageLogsContext } from './usage-logs-provider'

const route = getRouteApi('/_authenticated/usage-logs/$section')

function formatTokenStat(value: number): string {
  if (!Number.isFinite(value)) return '0'

  const absValue = Math.abs(value)
  const units = [
    { threshold: 1_000_000_000, suffix: 'B' },
    { threshold: 1_000_000, suffix: 'M' },
    { threshold: 1_000, suffix: 'K' },
  ]
  const unit = units.find((item) => absValue >= item.threshold)

  if (!unit) {
    return Intl.NumberFormat(undefined, { maximumFractionDigits: 0 }).format(
      value
    )
  }

  const formatted = (value / unit.threshold).toFixed(1).replace(/\.0$/, '')
  return `${formatted}${unit.suffix}`
}

function StatBadge(props: {
  label: string
  value: string | number
  accent: string
}) {
  return (
    <span className='border-border/60 bg-muted/25 inline-flex h-7 items-center gap-2 rounded-md border px-2.5 text-xs shadow-xs'>
      <span className={cn('h-3.5 w-0.5 rounded-full', props.accent)} />
      <span className='text-muted-foreground'>{props.label}</span>
      <span className='text-foreground/85 font-mono font-semibold tabular-nums'>
        {props.value}
      </span>
    </span>
  )
}

export function CommonLogsStats() {
  const { t } = useTranslation()
  const isAdmin = useIsAdmin()
  const searchParams = route.useSearch()
  const { sensitiveVisible } = useUsageLogsContext()

  const { data: stats, isLoading } = useQuery({
    queryKey: ['usage-logs-stats', isAdmin, searchParams],
    queryFn: async () => {
      const params = buildApiParams({
        page: 1,
        pageSize: 1,
        searchParams,
        columnFilters: [],
        isAdmin,
      })

      const result = isAdmin
        ? await getLogStats(params)
        : await getUserLogStats(params)

      return result.success
        ? result.data || DEFAULT_LOG_STATS
        : DEFAULT_LOG_STATS
    },
    placeholderData: (previousData) => previousData,
  })

  if (isLoading) {
    return (
      <div className='flex flex-wrap items-center gap-2'>
        <Skeleton className='h-7 w-[150px] rounded-md' />
        <Skeleton className='h-7 w-[100px] rounded-md' />
        <Skeleton className='h-7 w-[120px] rounded-md' />
        <Skeleton className='h-7 w-[130px] rounded-md' />
        <Skeleton className='h-7 w-[130px] rounded-md' />
        <Skeleton className='h-7 w-[130px] rounded-md' />
        <Skeleton className='h-7 w-[120px] rounded-md' />
        <Skeleton className='h-7 w-[120px] rounded-md' />
        <Skeleton className='h-7 w-[110px] rounded-md' />
      </div>
    )
  }

  const promptTokens = stats?.prompt_tokens || 0
  const completionTokens = stats?.completion_tokens || 0
  const cacheTokens = stats?.cache_tokens || 0
  const cacheCreationTokens = stats?.cache_creation_tokens || 0
  const totalTokens = promptTokens + completionTokens
  const cacheRatio = promptTokens > 0 ? (cacheTokens / promptTokens) * 100 : 0

  return (
    <div className='flex flex-wrap items-center gap-2'>
      <StatBadge
        label={t('Usage')}
        value={sensitiveVisible ? formatLogQuota(stats?.quota || 0) : '••••'}
        accent='bg-sky-500/70'
      />
      <StatBadge
        label={t('RPM')}
        value={stats?.rpm || 0}
        accent='bg-rose-500/65'
      />
      <StatBadge
        label={t('TPM')}
        value={stats?.tpm || 0}
        accent='bg-slate-400/70'
      />
      <StatBadge
        label={t('Total Tokens')}
        value={formatTokenStat(totalTokens)}
        accent='bg-indigo-500/70'
      />
      <StatBadge
        label={t('Input Tokens')}
        value={formatTokenStat(promptTokens)}
        accent='bg-emerald-500/70'
      />
      <StatBadge
        label={t('Output Tokens')}
        value={formatTokenStat(completionTokens)}
        accent='bg-amber-500/70'
      />
      <StatBadge
        label={t('Cache Hit')}
        value={formatTokenStat(cacheTokens)}
        accent='bg-teal-500/70'
      />
      <StatBadge
        label={t('Cache Write')}
        value={formatTokenStat(cacheCreationTokens)}
        accent='bg-violet-500/70'
      />
      <StatBadge
        label={t('Cache Ratio')}
        value={`${cacheRatio.toFixed(2)}%`}
        accent='bg-fuchsia-500/70'
      />
    </div>
  )
}

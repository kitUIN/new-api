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
import { useState } from 'react'
import { type ColumnDef } from '@tanstack/react-table'
import {
  CircleAlert,
  Sparkles,
  KeyRound,
  Upload,
  Download,
  Package,
  SquarePen,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import {
  getQQAvatarUrl,
  getUserAvatarFallback,
  getUserAvatarStyle,
} from '@/lib/avatar'
import {
  formatBillingCurrencyFromUSD,
  getCurrencyDisplay,
} from '@/lib/currency'
import {
  formatUseTime,
  formatLogQuota,
  formatTimestampToDate,
} from '@/lib/format'
import { cn } from '@/lib/utils'
import { Avatar, AvatarFallback, AvatarImage } from '@/components/ui/avatar'
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { DataTableColumnHeader } from '@/components/data-table'
import { StatusBadge, type StatusBadgeProps } from '@/components/status-badge'
import { LOG_TYPE_ALL_VALUE } from '../../constants'
import type { UsageLog } from '../../data/schema'
import {
  formatModelName,
  getFirstResponseTimeColor,
  getResponseTimeColor,
  getTieredBillingSummary,
  hasAnyCacheTokens,
  parseLogOther,
  isViolationFeeLog,
} from '../../lib/format'
import {
  isDisplayableLogType,
  isTimingLogType,
  getLogTypeConfig,
  isPerCallBilling,
} from '../../lib/utils'
import type { LogOtherData } from '../../types'
import { DetailsDialog } from '../dialogs/details-dialog'
import { ModelBadge } from '../model-badge'
import { useUsageLogsContext } from '../usage-logs-provider'

interface DetailSegment {
  text: string
  muted?: boolean
  danger?: boolean
}

const DETAIL_TOOLTIP_CONTENT_CLASS = 'w-fit max-w-[calc(100vw-2rem)]'
const DETAIL_TOOLTIP_ROW_CLASS =
  'grid min-w-48 grid-cols-[1fr_max-content] items-center gap-x-4'
const DETAIL_TOOLTIP_BORDER_ROW_CLASS = cn(
  DETAIL_TOOLTIP_ROW_CLASS,
  'border-border/60 border-t pt-1'
)
const DETAIL_TOOLTIP_VALUE_CLASS = 'text-right font-mono tabular-nums'

function formatRatioCompact(ratio: number | undefined): string {
  if (ratio == null || !Number.isFinite(ratio)) return '-'
  return ratio % 1 === 0
    ? String(ratio)
    : ratio.toFixed(4).replace(/\.?0+$/, '')
}

function getGroupRatioText(other: LogOtherData | null): string | null {
  const userGroupRatio = other?.user_group_ratio
  if (
    userGroupRatio != null &&
    userGroupRatio !== -1 &&
    Number.isFinite(userGroupRatio)
  ) {
    return `${formatRatioCompact(userGroupRatio)}x`
  }

  const groupRatio = other?.group_ratio
  if (groupRatio != null && groupRatio !== 1 && Number.isFinite(groupRatio)) {
    return `${formatRatioCompact(groupRatio)}x`
  }

  return null
}

function toPositiveNumber(value: unknown): number {
  const parsed = Number(value)
  if (!Number.isFinite(parsed) || parsed <= 0) return 0
  return parsed
}

function formatTokenCount(value: number): string {
  return toPositiveNumber(value).toLocaleString()
}

function getCacheReadTokens(log: UsageLog, other: LogOtherData | null): number {
  return toPositiveNumber(log.cache_read_tokens || other?.cache_tokens)
}

function getCacheWriteTokens(
  log: UsageLog,
  other: LogOtherData | null
): number {
  const cacheWriteTokens = toPositiveNumber(log.cache_write_tokens)
  if (cacheWriteTokens > 0) return cacheWriteTokens

  const cacheWrite5m = toPositiveNumber(other?.cache_creation_tokens_5m)
  const cacheWrite1h = toPositiveNumber(other?.cache_creation_tokens_1h)
  const splitCacheWriteTokens = cacheWrite5m + cacheWrite1h
  if (splitCacheWriteTokens > 0) return splitCacheWriteTokens

  return toPositiveNumber(other?.cache_creation_tokens)
}

function getPrimaryInputTokens(
  log: UsageLog,
  other: LogOtherData | null
): number {
  const explicitTotalInputTokens = toPositiveNumber(other?.input_tokens_total)
  const inputTokens =
    explicitTotalInputTokens > 0
      ? explicitTotalInputTokens
      : toPositiveNumber(log.prompt_tokens)
  const cacheTokens =
    getCacheReadTokens(log, other) + getCacheWriteTokens(log, other)

  if (inputTokens <= 0 || cacheTokens <= 0) return inputTokens
  return inputTokens >= cacheTokens ? inputTokens - cacheTokens : inputTokens
}

function getEffectiveGroupRatio(
  groupRatio: number | undefined,
  userGroupRatio: number | undefined
): number {
  if (
    userGroupRatio != null &&
    Number.isFinite(userGroupRatio) &&
    userGroupRatio !== -1
  ) {
    return userGroupRatio
  }

  return groupRatio != null && Number.isFinite(groupRatio) ? groupRatio : 1
}

function getCacheWriteBreakdown(log: UsageLog, other: LogOtherData | null) {
  const cacheCreationTokens = toPositiveNumber(other?.cache_creation_tokens)
  const cacheCreationTokens5m = toPositiveNumber(
    other?.cache_creation_tokens_5m
  )
  const cacheCreationTokens1h = toPositiveNumber(
    other?.cache_creation_tokens_1h
  )
  const splitCacheCreationTokens = cacheCreationTokens5m + cacheCreationTokens1h
  const fallbackCacheWriteTokens = toPositiveNumber(log.cache_write_tokens)

  if (splitCacheCreationTokens > 0) {
    return {
      legacyTokens: Math.max(cacheCreationTokens - splitCacheCreationTokens, 0),
      tokens5m: cacheCreationTokens5m,
      tokens1h: cacheCreationTokens1h,
    }
  }

  return {
    legacyTokens: cacheCreationTokens || fallbackCacheWriteTokens,
    tokens5m: 0,
    tokens1h: 0,
  }
}

function quotaToUSD(quota: number | undefined): number | undefined {
  if (quota == null || !Number.isFinite(quota)) return undefined
  const { config } = getCurrencyDisplay()
  return quota / config.quotaPerUnit
}

type TieredPriceEntries = NonNullable<
  ReturnType<typeof getTieredBillingSummary>
>['priceEntries']

function hasTieredPriceField(
  priceEntries: TieredPriceEntries,
  field: string
): boolean {
  return priceEntries.some((entry) => entry.field === field)
}

function getTieredUnitPrice(
  priceEntries: TieredPriceEntries,
  field: string
): number | undefined {
  return priceEntries.find((entry) => entry.field === field)?.price
}

interface CostDetail {
  billedQuota: number
  groupRatio: number
  serviceTier: string
  billingMode?: string
  matchedTier?: string
  crossedTier?: boolean
  beforeGroupQuota?: number
  afterGroupQuota?: number
  originalAmount?: number
  inputAmount?: number
  outputAmount?: number
  inputUnitPrice?: number
  outputUnitPrice?: number
  cacheReadAmount?: number
  cacheWriteAmount?: number
  cacheReadUnitPrice?: number
  cacheWriteUnitPrice?: number | null
  cacheWriteUnitPrice5m?: number | null
  cacheWriteUnitPrice1h?: number | null
  extraAmounts?: Array<{
    label: string
    amount: number
    unitPrice: number
  }>
}

function buildCostDetail(
  log: UsageLog,
  other: LogOtherData | null
): CostDetail {
  const groupRatio = getEffectiveGroupRatio(
    other?.group_ratio,
    other?.user_group_ratio
  )
  const billedQuota = toPositiveNumber(log.quota)
  const serviceTier =
    ((other as (LogOtherData & { service_tier?: string; tier?: string }) | null)
      ?.service_tier ||
      (
        other as
          | (LogOtherData & { service_tier?: string; tier?: string })
          | null
      )?.tier ||
      '') ??
    ''

  if (other?.billing_mode === 'tiered_expr') {
    const tieredSummary = getTieredBillingSummary(other)
    const priceEntries = tieredSummary?.priceEntries ?? []
    const inputUnitPrice = getTieredUnitPrice(priceEntries, 'inputPrice')
    const outputUnitPrice = getTieredUnitPrice(priceEntries, 'outputPrice')
    const cacheReadUnitPrice = getTieredUnitPrice(
      priceEntries,
      'cacheReadPrice'
    )
    const cacheWriteUnitPrice = getTieredUnitPrice(
      priceEntries,
      'cacheCreatePrice'
    )
    const cacheWriteUnitPrice1h = getTieredUnitPrice(
      priceEntries,
      'cacheCreate1hPrice'
    )

    const totalInputTokens = toPositiveNumber(
      other.input_tokens_total || log.prompt_tokens
    )
    const cacheReadTokens = getCacheReadTokens(log, other)
    const cacheWriteBreakdown = getCacheWriteBreakdown(log, other)
    const cacheWriteTokens =
      cacheWriteBreakdown.legacyTokens +
      cacheWriteBreakdown.tokens5m +
      cacheWriteBreakdown.tokens1h
    const imageTokens = toPositiveNumber(other.image_output)
    const audioInputTokens = toPositiveNumber(other.audio_input_token_count)
    let inputTokens = totalInputTokens

    if (hasTieredPriceField(priceEntries, 'cacheReadPrice')) {
      inputTokens -= cacheReadTokens
    }
    if (
      hasTieredPriceField(priceEntries, 'cacheCreatePrice') ||
      hasTieredPriceField(priceEntries, 'cacheCreate1hPrice')
    ) {
      inputTokens -= cacheWriteTokens
    }
    if (hasTieredPriceField(priceEntries, 'imagePrice')) {
      inputTokens -= imageTokens
    }
    if (hasTieredPriceField(priceEntries, 'audioInputPrice')) {
      inputTokens -= audioInputTokens
    }
    inputTokens = Math.max(inputTokens, 0)

    const outputTokens = toPositiveNumber(log.completion_tokens)
    const inputAmount =
      inputUnitPrice == null
        ? undefined
        : (inputTokens / 1_000_000) * inputUnitPrice
    const outputAmount =
      outputUnitPrice == null
        ? undefined
        : (outputTokens / 1_000_000) * outputUnitPrice
    const cacheReadAmount =
      cacheReadUnitPrice == null
        ? undefined
        : (cacheReadTokens / 1_000_000) * cacheReadUnitPrice
    const cacheWriteAmount =
      (cacheWriteBreakdown.legacyTokens / 1_000_000) *
        (cacheWriteUnitPrice ?? 0) +
      (cacheWriteBreakdown.tokens5m / 1_000_000) * (cacheWriteUnitPrice ?? 0) +
      (cacheWriteBreakdown.tokens1h / 1_000_000) *
        (cacheWriteUnitPrice1h ?? cacheWriteUnitPrice ?? 0)

    const extraAmounts = priceEntries
      .filter(
        (entry) =>
          ![
            'inputPrice',
            'outputPrice',
            'cacheReadPrice',
            'cacheCreatePrice',
            'cacheCreate1hPrice',
          ].includes(entry.field)
      )
      .map((entry) => {
        const tokens =
          entry.field === 'imagePrice'
            ? imageTokens
            : entry.field === 'audioInputPrice'
              ? audioInputTokens
              : 0
        return {
          label: entry.shortLabel,
          amount: (tokens / 1_000_000) * entry.price,
          unitPrice: entry.price,
        }
      })
      .filter((entry) => entry.amount > 0)

    const calculatedOriginalAmount =
      [inputAmount, outputAmount, cacheReadAmount, cacheWriteAmount]
        .filter((amount): amount is number => amount != null)
        .reduce((sum, amount) => sum + amount, 0) +
      extraAmounts.reduce((sum, entry) => sum + entry.amount, 0)

    return {
      billingMode: 'tiered_expr',
      matchedTier:
        other.matched_tier || other.estimated_tier || tieredSummary?.tier.label,
      crossedTier: other.tiered_crossed_tier,
      beforeGroupQuota:
        other.tiered_actual_quota_before_group ??
        other.estimated_quota_before_group,
      afterGroupQuota:
        other.tiered_actual_quota_after_group ??
        other.estimated_quota_after_group,
      inputAmount,
      outputAmount,
      cacheReadAmount,
      cacheWriteAmount: cacheWriteAmount > 0 ? cacheWriteAmount : undefined,
      inputUnitPrice,
      outputUnitPrice,
      cacheReadUnitPrice,
      cacheWriteUnitPrice:
        cacheWriteBreakdown.tokens1h > 0 ? null : cacheWriteUnitPrice,
      cacheWriteUnitPrice5m:
        cacheWriteBreakdown.tokens5m > 0 ? cacheWriteUnitPrice : null,
      cacheWriteUnitPrice1h:
        cacheWriteBreakdown.tokens1h > 0 ? cacheWriteUnitPrice1h : null,
      extraAmounts,
      groupRatio,
      originalAmount:
        quotaToUSD(other.tiered_actual_quota_before_group) ??
        calculatedOriginalAmount,
      billedQuota,
      serviceTier,
    }
  }

  const modelPrice = Number(other?.model_price)
  if (Number.isFinite(modelPrice) && modelPrice !== -1) {
    return {
      groupRatio,
      originalAmount: modelPrice,
      billedQuota,
      serviceTier,
    }
  }

  const modelRatio = Number(other?.model_ratio)
  if (!Number.isFinite(modelRatio)) {
    return {
      groupRatio,
      billedQuota,
      serviceTier,
    }
  }

  const completionRatio = Number(other?.completion_ratio || 0)
  const cacheRatio = Number(other?.cache_ratio || 1)
  const cacheCreationRatio = Number(other?.cache_creation_ratio || 1)
  const cacheCreationRatio5m = Number(
    other?.cache_creation_ratio_5m || cacheCreationRatio
  )
  const cacheCreationRatio1h = Number(
    other?.cache_creation_ratio_1h || cacheCreationRatio
  )
  const inputUnitPrice = modelRatio * 2.0
  const outputUnitPrice = inputUnitPrice * completionRatio
  const cacheReadUnitPrice = inputUnitPrice * cacheRatio
  const cacheWriteUnitPrice = inputUnitPrice * cacheCreationRatio
  const cacheWriteUnitPrice5m = inputUnitPrice * cacheCreationRatio5m
  const cacheWriteUnitPrice1h = inputUnitPrice * cacheCreationRatio1h
  const inputTokens = getPrimaryInputTokens(log, other)
  const outputTokens = toPositiveNumber(log.completion_tokens)
  const cacheReadTokens = getCacheReadTokens(log, other)
  const cacheWriteBreakdown = getCacheWriteBreakdown(log, other)
  const inputAmount = (inputTokens / 1_000_000) * inputUnitPrice
  const outputAmount = (outputTokens / 1_000_000) * outputUnitPrice
  const cacheReadAmount = (cacheReadTokens / 1_000_000) * cacheReadUnitPrice
  const cacheWriteAmount =
    (cacheWriteBreakdown.legacyTokens / 1_000_000) * cacheWriteUnitPrice +
    (cacheWriteBreakdown.tokens5m / 1_000_000) * cacheWriteUnitPrice5m +
    (cacheWriteBreakdown.tokens1h / 1_000_000) * cacheWriteUnitPrice1h
  const originalAmount =
    inputAmount + outputAmount + cacheReadAmount + cacheWriteAmount

  return {
    inputAmount,
    outputAmount,
    inputUnitPrice,
    outputUnitPrice,
    cacheReadAmount,
    cacheWriteAmount,
    cacheReadUnitPrice,
    cacheWriteUnitPrice:
      cacheWriteBreakdown.tokens5m > 0 || cacheWriteBreakdown.tokens1h > 0
        ? null
        : cacheWriteUnitPrice,
    cacheWriteUnitPrice5m:
      cacheWriteBreakdown.tokens5m > 0 ? cacheWriteUnitPrice5m : null,
    cacheWriteUnitPrice1h:
      cacheWriteBreakdown.tokens1h > 0 ? cacheWriteUnitPrice1h : null,
    groupRatio,
    originalAmount,
    billedQuota,
    serviceTier,
  }
}

function formatCostAmount(amount: number | undefined): string {
  if (amount == null || !Number.isFinite(amount)) return '-'
  return formatBillingCurrencyFromUSD(amount, {
    digitsLarge: 4,
    digitsSmall: 6,
    abbreviate: false,
  })
}

function formatUnitPrice(amount: number | undefined | null): string {
  if (amount == null || !Number.isFinite(amount)) return '-'
  return `${formatBillingCurrencyFromUSD(amount, {
    digitsLarge: 4,
    digitsSmall: 6,
    abbreviate: false,
  })}/M`
}

function getReasoningEffortVariant(
  effort: string | undefined
): StatusBadgeProps['variant'] {
  switch (effort?.toLowerCase()) {
    case 'none':
      return 'grey'
    case 'minimal':
      return 'teal'
    case 'low':
      return 'green'
    case 'medium':
      return 'blue'
    case 'high':
      return 'orange'
    case 'xhigh':
      return 'red'
    default:
      return 'violet'
  }
}

function buildDetailSegments(
  log: UsageLog,
  other: LogOtherData | null,
  t: (key: string, opts?: Record<string, unknown>) => string
): DetailSegment[] {
  if (log.type === 6) {
    return [{ text: t('Async task refund') }]
  }

  if (log.type !== 2) return []

  const isViolation = isViolationFeeLog(other)
  if (isViolation) {
    const segments: DetailSegment[] = []
    segments.push({ text: t('Violation Fee'), danger: true })
    if (other?.violation_fee_code) {
      segments.push({
        text: other.violation_fee_code,
        muted: true,
      })
    }
    segments.push({
      text: `${t('Fee')}: ${formatLogQuota(other?.fee_quota ?? log.quota)}`,
      muted: true,
    })
    return segments
  }

  if (!other) return []

  const segments: DetailSegment[] = []

  const priceOpts = { digitsLarge: 4, digitsSmall: 6, abbreviate: false }
  const formatPrice = (price: number) =>
    `${formatBillingCurrencyFromUSD(price, priceOpts)}/M`
  const formatPriceCompact = (price: number) =>
    formatBillingCurrencyFromUSD(price, priceOpts)
  const formatPriceList = (prices: string[], showUnit: boolean) => {
    const text = prices.join(' / ')
    return showUnit ? `${text}/M` : text
  }
  const isTieredExpr = other.billing_mode === 'tiered_expr'
  const tieredSummary = getTieredBillingSummary(other)
  if (isTieredExpr) {
    if (tieredSummary) {
      const baseEntries = tieredSummary.priceEntries
        .filter((entry) => ['inputPrice', 'outputPrice'].includes(entry.field))
        .map((entry) => formatPriceCompact(entry.price))
      if (baseEntries.length > 0) {
        const tierLabel = tieredSummary.tier.label || t('Default')
        segments.push({
          text: `${tierLabel} · ${formatPriceList(baseEntries, true)}`,
        })
      }

      const cacheEntries = tieredSummary.priceEntries
        .filter((entry) =>
          ['cacheReadPrice', 'cacheCreatePrice', 'cacheCreate1hPrice'].includes(
            entry.field
          )
        )
        .map((entry) => {
          return formatPriceCompact(entry.price)
        })
      if (cacheEntries.length > 0) {
        segments.push({
          text: `${t('Cache')} ${formatPriceList(cacheEntries, false)}`,
          muted: true,
        })
      }

      const otherEntries = tieredSummary.priceEntries
        .filter(
          (entry) =>
            ![
              'inputPrice',
              'outputPrice',
              'cacheReadPrice',
              'cacheCreatePrice',
              'cacheCreate1hPrice',
            ].includes(entry.field)
        )
        .map((entry) => `${t(entry.shortLabel)} ${formatPrice(entry.price)}`)
      if (otherEntries.length > 0) {
        segments.push({
          text: otherEntries.join(' · '),
          muted: true,
        })
      }
    } else {
      segments.push({
        text: `${t('Dynamic Pricing')} · ${t('No matching results')}`,
        muted: true,
      })
    }
  } else {
    const isPerCall = isPerCallBilling(other.model_price)
    if (isPerCall) {
      segments.push({
        text: `${t('Per-call')} · ${formatBillingCurrencyFromUSD(other.model_price!, priceOpts)}`,
      })
    } else if (other.model_ratio != null) {
      const inputPriceUSD = other.model_ratio * 2.0
      const baseEntries = [formatPriceCompact(inputPriceUSD)]
      if (other.completion_ratio != null) {
        baseEntries.push(
          formatPriceCompact(inputPriceUSD * other.completion_ratio)
        )
      }
      segments.push({
        text: `${t('Standard')} · ${formatPriceList(baseEntries, true)}`,
      })

      if (hasAnyCacheTokens(other)) {
        const cacheEntries = [
          other.cache_ratio != null && other.cache_ratio !== 1
            ? formatPriceCompact(inputPriceUSD * other.cache_ratio)
            : null,
          other.cache_creation_ratio != null && other.cache_creation_ratio !== 1
            ? formatPriceCompact(inputPriceUSD * other.cache_creation_ratio)
            : null,
          other.cache_creation_ratio_1h != null &&
          other.cache_creation_ratio_1h !== 0
            ? formatPriceCompact(inputPriceUSD * other.cache_creation_ratio_1h)
            : null,
        ].filter(Boolean) as string[]

        if (cacheEntries.length > 0) {
          segments.push({
            text: `${t('Cache')} ${formatPriceList(cacheEntries, false)}`,
            muted: true,
          })
        }
      }
    } else {
      const userGroupRatio = other.user_group_ratio
      const groupRatio = other.group_ratio
      const isUserGroup =
        userGroupRatio != null &&
        Number.isFinite(userGroupRatio) &&
        userGroupRatio !== -1
      const effectiveRatio = isUserGroup ? userGroupRatio : groupRatio
      const ratioLabel = isUserGroup
        ? t('User Exclusive Ratio')
        : t('Group Ratio')

      if (effectiveRatio != null && Number.isFinite(effectiveRatio)) {
        segments.push({
          text: `${ratioLabel} ${formatRatioCompact(effectiveRatio)}x`,
        })
      }
    }
  }

  if (other.is_system_prompt_overwritten) {
    segments.push({
      text: t('System Prompt Override'),
      danger: true,
    })
  }

  return segments
}

export function useCommonLogsColumns(isAdmin: boolean): ColumnDef<UsageLog>[] {
  const { t } = useTranslation()
  const columns: ColumnDef<UsageLog>[] = [
    {
      accessorKey: 'created_at',
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title={t('Time')} />
      ),
      cell: ({ row }) => {
        const log = row.original
        const timestamp = row.getValue('created_at') as number
        const config = getLogTypeConfig(log.type)

        return (
          <div className='flex flex-col gap-0.5'>
            <span className='font-mono text-xs tabular-nums'>
              {formatTimestampToDate(timestamp)}
            </span>
            <StatusBadge
              label={t(config.label)}
              variant={config.color as StatusBadgeProps['variant']}
              size='sm'
              copyable={false}
            />
          </div>
        )
      },
      filterFn: (row, _id, value) => {
        if (!Array.isArray(value) || value.length === 0) return true
        if (value.includes(LOG_TYPE_ALL_VALUE)) return true
        return value.includes(String(row.original.type))
      },
      enableHiding: false,
      meta: { label: t('Time') },
    },
  ]

  if (isAdmin) {
    columns.push(
      {
        id: 'channel',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Channel')} />
        ),
        cell: function ChannelCell({ row }) {
          const { sensitiveVisible, setAffinityTarget, setAffinityDialogOpen } =
            useUsageLogsContext()
          const log = row.original

          if (!isDisplayableLogType(log.type)) return null

          const other = parseLogOther(log.other)
          const affinity = other?.admin_info?.channel_affinity
          const useChannel = other?.admin_info?.use_channel
          const channelChain =
            useChannel && useChannel.length > 0
              ? useChannel.join(' → ')
              : undefined
          const channelDisplay = log.channel_name
            ? `${log.channel_name} #${log.channel}`
            : `#${log.channel}`
          const channelIdDisplay = `#${log.channel}`
          const channelName = sensitiveVisible ? log.channel_name : '••••'

          return (
            <TooltipProvider>
              <Tooltip>
                <TooltipTrigger
                  render={
                    <div className='flex max-w-[160px] flex-col gap-0.5' />
                  }
                >
                  <div className='relative inline-flex w-fit'>
                    <StatusBadge
                      label={channelIdDisplay}
                      autoColor={String(log.channel)}
                      copyText={String(log.channel)}
                      size='sm'
                      className='font-mono'
                    />
                    {affinity && (
                      <button
                        type='button'
                        className='absolute -top-1 -right-1 leading-none text-amber-500'
                        onClick={(e) => {
                          e.stopPropagation()
                          setAffinityTarget({
                            rule_name: affinity.rule_name || '',
                            using_group:
                              affinity.using_group ||
                              affinity.selected_group ||
                              '',
                            key_hint: affinity.key_hint || '',
                            key_fp: affinity.key_fp || '',
                          })
                          setAffinityDialogOpen(true)
                        }}
                      >
                        <Sparkles className='size-3 fill-current' />
                      </button>
                    )}
                  </div>
                  {log.channel_name && (
                    <span className='text-muted-foreground/70 truncate text-[11px]'>
                      {channelName}
                    </span>
                  )}
                </TooltipTrigger>
                <TooltipContent>
                  <div className='space-y-1'>
                    <p>
                      {sensitiveVisible ? channelDisplay : channelIdDisplay}
                    </p>
                    {channelChain && (
                      <p className='text-muted-foreground text-xs'>
                        {t('Chain')}: {channelChain}
                      </p>
                    )}
                    {affinity && (
                      <div className='border-t pt-1 text-xs'>
                        <p className='font-medium'>{t('Channel Affinity')}</p>
                        <p>
                          {t('Rule')}: {affinity.rule_name || '-'}
                        </p>
                        <p>
                          {t('Group')}:{' '}
                          {sensitiveVisible
                            ? affinity.using_group ||
                              affinity.selected_group ||
                              '-'
                            : '••••'}
                        </p>
                      </div>
                    )}
                  </div>
                </TooltipContent>
              </Tooltip>
            </TooltipProvider>
          )
        },
        meta: { label: t('Channel'), mobileHidden: true },
      },
      {
        id: 'user',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('User')} />
        ),
        cell: function UserCell({ row }) {
          const { sensitiveVisible, setSelectedUserId, setUserInfoDialogOpen } =
            useUsageLogsContext()
          const log = row.original

          if (!log.username) return null
          const qqAvatarUrl = sensitiveVisible
            ? getQQAvatarUrl((log as { qq_id?: string }).qq_id)
            : undefined

          return (
            <button
              type='button'
              className='flex items-center gap-1.5 text-left'
              onClick={(e) => {
                e.stopPropagation()
                setSelectedUserId(log.user_id)
                setUserInfoDialogOpen(true)
              }}
            >
              <Avatar className='ring-border/60 size-6 ring-1'>
                {qqAvatarUrl && (
                  <AvatarImage src={qqAvatarUrl} alt={log.username} />
                )}
                <AvatarFallback
                  className={cn(
                    'text-[11px] font-semibold',
                    !sensitiveVisible && 'bg-muted text-muted-foreground'
                  )}
                  style={
                    sensitiveVisible
                      ? getUserAvatarStyle(log.username)
                      : undefined
                  }
                >
                  {sensitiveVisible ? getUserAvatarFallback(log.username) : '•'}
                </AvatarFallback>
              </Avatar>
              <TooltipProvider delay={300}>
                <Tooltip>
                  <TooltipTrigger
                    render={
                      <span className='text-muted-foreground max-w-[100px] truncate text-sm hover:underline' />
                    }
                  >
                    {sensitiveVisible ? log.username : '••••'}
                  </TooltipTrigger>
                  {sensitiveVisible && log.username.length > 12 && (
                    <TooltipContent side='top'>{log.username}</TooltipContent>
                  )}
                </Tooltip>
              </TooltipProvider>
            </button>
          )
        },
        meta: { label: t('User'), mobileHidden: true },
      }
    )
  }

  columns.push({
    accessorKey: 'token_name',
    header: ({ column }) => (
      <DataTableColumnHeader column={column} title={t('Token')} />
    ),
    cell: function TokenNameCell({ row }) {
      const { sensitiveVisible } = useUsageLogsContext()
      const log = row.original
      if (!isDisplayableLogType(log.type)) return null

      const tokenName = log.token_name
      if (!tokenName) return null

      const other = parseLogOther(log.other)
      const displayName = sensitiveVisible ? tokenName : '••••'
      let group = log.group
      if (!group) group = other?.group || ''

      const metaParts: string[] = []
      const groupRatioText = getGroupRatioText(other)
      if (group) {
        metaParts.push(sensitiveVisible ? group : '••••')
      }
      if (groupRatioText) metaParts.push(groupRatioText)

      return (
        <div className='flex max-w-[200px] flex-col gap-0.5'>
          <TooltipProvider delay={300}>
            <Tooltip>
              <TooltipTrigger render={<div className='max-w-full' />}>
                <StatusBadge
                  label={displayName}
                  icon={KeyRound}
                  copyText={sensitiveVisible ? tokenName : undefined}
                  size='sm'
                  className='border-border/60 bg-muted/30 text-foreground max-w-full overflow-hidden rounded-md border px-1.5 py-0.5 font-mono'
                />
              </TooltipTrigger>
              {sensitiveVisible && tokenName.length > 16 && (
                <TooltipContent side='top' className='max-w-xs break-all'>
                  {tokenName}
                </TooltipContent>
              )}
            </Tooltip>
          </TooltipProvider>
          {metaParts.length > 0 && (
            <span className='text-muted-foreground/60 truncate text-[11px]'>
              {metaParts.join(' · ')}
            </span>
          )}
        </div>
      )
    },
    meta: { label: t('Token') },
    size: 160,
  })

  columns.push(
    {
      accessorKey: 'model_name',
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title={t('Model')} />
      ),
      cell: function ModelCell({ row }) {
        const log = row.original
        if (!isDisplayableLogType(log.type)) return null

        const modelInfo = formatModelName(log)

        return (
          <div className='flex max-w-[220px] flex-col gap-0.5'>
            <ModelBadge
              modelName={modelInfo.name}
              actualModel={modelInfo.actualModel}
            />
          </div>
        )
      },
      meta: { label: t('Model'), mobileTitle: true },
    },

    {
      id: 'reasoning_effort',
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title={t('Reasoning Effort')} />
      ),
      cell: ({ row }) => {
        const log = row.original
        if (!isDisplayableLogType(log.type)) return null

        const other = parseLogOther(log.other)
        const effort = other?.reasoning_effort
        if (!effort) return <span className='text-muted-foreground/40'>-</span>

        return (
          <StatusBadge
            label={t(effort)}
            variant={getReasoningEffortVariant(effort)}
            size='sm'
            copyable={false}
          />
        )
      },
      meta: { label: t('Reasoning Effort'), mobileHidden: true },
    },

    {
      accessorKey: 'use_time',
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title={t('Timing')} />
      ),
      cell: ({ row }) => {
        const log = row.original
        if (!isTimingLogType(log.type)) return null

        const useTime = row.getValue('use_time') as number
        const other = parseLogOther(log.other)
        const frt = other?.frt
        const tokensPerSecond =
          useTime > 0 && log.completion_tokens > 0
            ? log.completion_tokens / useTime
            : null
        const timeVariant = getResponseTimeColor(useTime, log.completion_tokens)
        const frtVariant = frt ? getFirstResponseTimeColor(frt / 1000) : null
        const streamStatus = other?.stream_status
        const hasStreamError =
          log.is_stream && streamStatus && streamStatus.status !== 'ok'

        return (
          <div className='flex flex-col gap-1'>
            <div className='flex items-center gap-1.5 whitespace-nowrap'>
              <TooltipProvider>
                <Tooltip>
                  <TooltipTrigger
                    render={
                      <StatusBadge
                        label={formatUseTime(useTime)}
                        variant={timeVariant as StatusBadgeProps['variant']}
                        size='sm'
                        copyable={false}
                        className='font-mono'
                      />
                    }
                  />
                  <TooltipContent>{t('Duration')}</TooltipContent>
                </Tooltip>
              </TooltipProvider>
              {log.is_stream &&
                (frt != null && frt > 0 ? (
                  <TooltipProvider>
                    <Tooltip>
                      <TooltipTrigger
                        render={
                          <StatusBadge
                            label={formatUseTime(frt / 1000)}
                            variant={frtVariant as StatusBadgeProps['variant']}
                            size='sm'
                            copyable={false}
                            className='font-mono'
                          />
                        }
                      />
                      <TooltipContent>TTFT</TooltipContent>
                    </Tooltip>
                  </TooltipProvider>
                ) : (
                  <StatusBadge
                    label='N/A'
                    variant='neutral'
                    size='sm'
                    copyable={false}
                  />
                ))}
            </div>
            <div className='flex items-center gap-1 text-[11px]'>
              <span className='text-muted-foreground/60'>
                {log.is_stream ? t('Stream') : t('Non-stream')}
                {tokensPerSecond != null && (
                  <>
                    {' · '}
                    <span
                      className={cn(
                        'font-mono tabular-nums',
                        tokensPerSecond >= 30
                          ? 'text-success'
                          : tokensPerSecond >= 15
                            ? 'text-warning'
                            : 'text-destructive'
                      )}
                    >
                      {tokensPerSecond.toFixed(1)}
                    </span>
                    {' t/s'}
                  </>
                )}
              </span>
              {hasStreamError && (
                <TooltipProvider>
                  <Tooltip>
                    <TooltipTrigger
                      render={<CircleAlert className='size-3 text-red-500' />}
                    />
                    <TooltipContent>
                      <div className='space-y-0.5 text-xs'>
                        <p>
                          {t('Stream Status')}: {t('Error')}
                        </p>
                        <p>{streamStatus.end_reason || 'unknown'}</p>
                        {(streamStatus.error_count ?? 0) > 0 && (
                          <p>
                            {t('Soft Errors')}: {streamStatus.error_count}
                          </p>
                        )}
                        {streamStatus.end_error && (
                          <p>{streamStatus.end_error}</p>
                        )}
                      </div>
                    </TooltipContent>
                  </Tooltip>
                </TooltipProvider>
              )}
            </div>
          </div>
        )
      },
      meta: { label: t('Timing'), mobileHidden: true },
    },

    {
      accessorKey: 'prompt_tokens',
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title='Tokens' />
      ),
      cell: ({ row }) => {
        const log = row.original
        if (!isDisplayableLogType(log.type)) return null

        const other = parseLogOther(log.other)

        const inputTokens = getPrimaryInputTokens(log, other)
        const outputTokens = toPositiveNumber(log.completion_tokens)
        const cacheReadTokens = getCacheReadTokens(log, other)
        const cacheWriteTokens = getCacheWriteTokens(log, other)
        const totalTokens =
          inputTokens + outputTokens + cacheReadTokens + cacheWriteTokens

        if (totalTokens === 0) {
          return <span className='text-muted-foreground text-xs'>-</span>
        }

        return (
          <TooltipProvider>
            <Tooltip>
              <TooltipTrigger
                render={
                  <div className='flex flex-col gap-0.5 font-mono text-xs tabular-nums' />
                }
              >
                <div className='flex items-center gap-2 font-medium'>
                  <span className='inline-flex items-center gap-1'>
                    <Upload className='text-muted-foreground size-3' />
                    {formatTokenCount(inputTokens)}
                  </span>
                  <span className='inline-flex items-center gap-1'>
                    <Download className='text-muted-foreground size-3' />
                    {formatTokenCount(outputTokens)}
                  </span>
                </div>
                {(cacheReadTokens > 0 || cacheWriteTokens > 0) && (
                  <div className='text-muted-foreground/60 flex items-center gap-2 text-[11px]'>
                    {cacheReadTokens > 0 && (
                      <span className='inline-flex items-center gap-1'>
                        <Package className='size-3' />
                        {formatTokenCount(cacheReadTokens)}
                      </span>
                    )}
                    {cacheWriteTokens > 0 && (
                      <span className='inline-flex items-center gap-1'>
                        <SquarePen className='size-3' />
                        {formatTokenCount(cacheWriteTokens)}
                      </span>
                    )}
                  </div>
                )}
              </TooltipTrigger>
              <TooltipContent className={DETAIL_TOOLTIP_CONTENT_CLASS}>
                <div className='space-y-1 text-xs'>
                  <p className='text-muted-foreground font-medium'>
                    {t('Token Breakdown')}
                  </p>
                  <div className={DETAIL_TOOLTIP_ROW_CLASS}>
                    <span>{t('Input Tokens')}</span>
                    <span className={DETAIL_TOOLTIP_VALUE_CLASS}>
                      {formatTokenCount(inputTokens)}
                    </span>
                  </div>
                  <div className={DETAIL_TOOLTIP_ROW_CLASS}>
                    <span>{t('Output Tokens')}</span>
                    <span className={DETAIL_TOOLTIP_VALUE_CLASS}>
                      {formatTokenCount(outputTokens)}
                    </span>
                  </div>
                  {cacheReadTokens > 0 && (
                    <div className={DETAIL_TOOLTIP_ROW_CLASS}>
                      <span>{t('Cache Read')}</span>
                      <span className={DETAIL_TOOLTIP_VALUE_CLASS}>
                        {formatTokenCount(cacheReadTokens)}
                      </span>
                    </div>
                  )}
                  {cacheWriteTokens > 0 && (
                    <div className={DETAIL_TOOLTIP_ROW_CLASS}>
                      <span>{t('Cache Write')}</span>
                      <span className={DETAIL_TOOLTIP_VALUE_CLASS}>
                        {formatTokenCount(cacheWriteTokens)}
                      </span>
                    </div>
                  )}
                  <div
                    className={cn(
                      DETAIL_TOOLTIP_BORDER_ROW_CLASS,
                      'font-medium'
                    )}
                  >
                    <span>{t('Total Tokens')}</span>
                    <span className={DETAIL_TOOLTIP_VALUE_CLASS}>
                      {formatTokenCount(totalTokens)}
                    </span>
                  </div>
                </div>
              </TooltipContent>
            </Tooltip>
          </TooltipProvider>
        )
      },
      meta: { label: 'Tokens', mobileHidden: true },
    },

    {
      accessorKey: 'quota',
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title={t('Cost')} />
      ),
      cell: ({ row }) => {
        const log = row.original
        if (!isDisplayableLogType(log.type)) return null

        const quota = row.getValue('quota') as number
        const other = parseLogOther(log.other)
        const isSubscription = other?.billing_source === 'subscription'
        const costDetail = buildCostDetail(log, other)
        const cacheReadQuota = toPositiveNumber(log.cache_read_quota)
        const cacheWriteQuota = toPositiveNumber(log.cache_write_quota)
        const hasCacheQuota = cacheReadQuota > 0 || cacheWriteQuota > 0

        const costTooltip = (
          <TooltipContent className={DETAIL_TOOLTIP_CONTENT_CLASS}>
            <div className='space-y-1 text-xs'>
              <p className='text-muted-foreground font-medium'>
                {t('Billing Details')}
              </p>
              {costDetail.billingMode === 'tiered_expr' && (
                <div className={DETAIL_TOOLTIP_ROW_CLASS}>
                  <span>{t('Billing Mode')}</span>
                  <span className={DETAIL_TOOLTIP_VALUE_CLASS}>
                    {t('Expression')}
                  </span>
                </div>
              )}
              {costDetail.matchedTier && (
                <div className={DETAIL_TOOLTIP_ROW_CLASS}>
                  <span>{t('Matched Tier')}</span>
                  <span className={DETAIL_TOOLTIP_VALUE_CLASS}>
                    {costDetail.matchedTier}
                  </span>
                </div>
              )}
              {costDetail.inputAmount != null && (
                <div className={DETAIL_TOOLTIP_ROW_CLASS}>
                  <span>{t('Input')}</span>
                  <span className={DETAIL_TOOLTIP_VALUE_CLASS}>
                    {formatCostAmount(costDetail.inputAmount)}
                  </span>
                </div>
              )}
              {costDetail.outputAmount != null && (
                <div className={DETAIL_TOOLTIP_ROW_CLASS}>
                  <span>{t('Output')}</span>
                  <span className={DETAIL_TOOLTIP_VALUE_CLASS}>
                    {formatCostAmount(costDetail.outputAmount)}
                  </span>
                </div>
              )}
              {toPositiveNumber(costDetail.cacheReadAmount) > 0 && (
                <div className={DETAIL_TOOLTIP_ROW_CLASS}>
                  <span>{t('Cache Read')}</span>
                  <span className={DETAIL_TOOLTIP_VALUE_CLASS}>
                    {formatCostAmount(costDetail.cacheReadAmount)}
                  </span>
                </div>
              )}
              {toPositiveNumber(costDetail.cacheWriteAmount) > 0 && (
                <div className={DETAIL_TOOLTIP_ROW_CLASS}>
                  <span>{t('Cache Write')}</span>
                  <span className={DETAIL_TOOLTIP_VALUE_CLASS}>
                    {formatCostAmount(costDetail.cacheWriteAmount)}
                  </span>
                </div>
              )}
              {costDetail.extraAmounts?.map((entry) => (
                <div key={entry.label} className={DETAIL_TOOLTIP_ROW_CLASS}>
                  <span>{t(entry.label)}</span>
                  <span className={DETAIL_TOOLTIP_VALUE_CLASS}>
                    {formatCostAmount(entry.amount)}
                  </span>
                </div>
              ))}
              {costDetail.inputUnitPrice != null && (
                <div className={DETAIL_TOOLTIP_BORDER_ROW_CLASS}>
                  <span>{t('Input price')}</span>
                  <span className={DETAIL_TOOLTIP_VALUE_CLASS}>
                    {formatUnitPrice(costDetail.inputUnitPrice)}
                  </span>
                </div>
              )}
              {costDetail.outputUnitPrice != null && (
                <div className={DETAIL_TOOLTIP_ROW_CLASS}>
                  <span>{t('Output price')}</span>
                  <span className={DETAIL_TOOLTIP_VALUE_CLASS}>
                    {formatUnitPrice(costDetail.outputUnitPrice)}
                  </span>
                </div>
              )}
              {costDetail.cacheReadUnitPrice != null && (
                <div className={DETAIL_TOOLTIP_ROW_CLASS}>
                  <span>{t('Cache read price')}</span>
                  <span className={DETAIL_TOOLTIP_VALUE_CLASS}>
                    {formatUnitPrice(costDetail.cacheReadUnitPrice)}
                  </span>
                </div>
              )}
              {costDetail.cacheWriteUnitPrice != null && (
                <div className={DETAIL_TOOLTIP_ROW_CLASS}>
                  <span>{t('Cache write price')}</span>
                  <span className={DETAIL_TOOLTIP_VALUE_CLASS}>
                    {formatUnitPrice(costDetail.cacheWriteUnitPrice)}
                  </span>
                </div>
              )}
              {costDetail.cacheWriteUnitPrice5m != null && (
                <div className={DETAIL_TOOLTIP_ROW_CLASS}>
                  <span>{t('Cache create price')}</span>
                  <span className={DETAIL_TOOLTIP_VALUE_CLASS}>
                    {formatUnitPrice(costDetail.cacheWriteUnitPrice5m)}
                  </span>
                </div>
              )}
              {costDetail.cacheWriteUnitPrice1h != null && (
                <div className={DETAIL_TOOLTIP_ROW_CLASS}>
                  <span>{t('Cache create (1h) price')}</span>
                  <span className={DETAIL_TOOLTIP_VALUE_CLASS}>
                    {formatUnitPrice(costDetail.cacheWriteUnitPrice1h)}
                  </span>
                </div>
              )}
              {costDetail.extraAmounts?.map((entry) => (
                <div
                  key={`${entry.label}-price`}
                  className={DETAIL_TOOLTIP_ROW_CLASS}
                >
                  <span>
                    {t(entry.label)} {t('price')}
                  </span>
                  <span className={DETAIL_TOOLTIP_VALUE_CLASS}>
                    {formatUnitPrice(entry.unitPrice)}
                  </span>
                </div>
              ))}
              {costDetail.serviceTier && (
                <div className={DETAIL_TOOLTIP_BORDER_ROW_CLASS}>
                  <span>{t('Service Tier')}</span>
                  <span className={DETAIL_TOOLTIP_VALUE_CLASS}>
                    {costDetail.serviceTier}
                  </span>
                </div>
              )}
              {costDetail.crossedTier === true && (
                <div className={DETAIL_TOOLTIP_ROW_CLASS}>
                  <span>{t('Tier changed after completion')}</span>
                  <span className={DETAIL_TOOLTIP_VALUE_CLASS}>{t('Yes')}</span>
                </div>
              )}
              {costDetail.beforeGroupQuota != null && (
                <div className={DETAIL_TOOLTIP_ROW_CLASS}>
                  <span>{t('Before group ratio')}</span>
                  <span className={DETAIL_TOOLTIP_VALUE_CLASS}>
                    {formatLogQuota(costDetail.beforeGroupQuota)}
                  </span>
                </div>
              )}
              <div
                className={
                  costDetail.serviceTier
                    ? DETAIL_TOOLTIP_ROW_CLASS
                    : DETAIL_TOOLTIP_BORDER_ROW_CLASS
                }
              >
                <span>{t('Group Ratio')}</span>
                <span className={DETAIL_TOOLTIP_VALUE_CLASS}>
                  {formatRatioCompact(costDetail.groupRatio)}x
                </span>
              </div>
              {costDetail.originalAmount != null && (
                <div className={DETAIL_TOOLTIP_ROW_CLASS}>
                  <span>{t('Original')}</span>
                  <span className={DETAIL_TOOLTIP_VALUE_CLASS}>
                    {formatCostAmount(costDetail.originalAmount)}
                  </span>
                </div>
              )}
              <div
                className={cn(DETAIL_TOOLTIP_BORDER_ROW_CLASS, 'font-medium')}
              >
                <span>{t('Total Cost')}</span>
                <span className={DETAIL_TOOLTIP_VALUE_CLASS}>
                  {formatLogQuota(quota)}
                </span>
              </div>
            </div>
          </TooltipContent>
        )

        if (isSubscription) {
          return (
            <TooltipProvider>
              <Tooltip>
                <TooltipTrigger
                  render={
                    <StatusBadge
                      label={t('Subscription')}
                      variant='success'
                      size='sm'
                      copyable={false}
                      className='cursor-help'
                    />
                  }
                />
                {costTooltip}
              </Tooltip>
            </TooltipProvider>
          )
        }

        return (
          <TooltipProvider>
            <Tooltip>
              <TooltipTrigger
                render={
                  <div className='flex flex-col gap-0.5 font-mono tabular-nums' />
                }
              >
                <span className='border-border/80 bg-muted/60 inline-flex w-fit items-center rounded-md border px-1.5 py-0.5 text-xs font-semibold'>
                  {formatLogQuota(quota)}
                </span>
                {hasCacheQuota && (
                  <div className='text-muted-foreground/60 flex flex-col gap-0.5 text-[11px]'>
                    {cacheReadQuota > 0 && (
                      <span className='inline-flex items-center gap-1'>
                        <Package className='size-3' />
                        {t('Cache Read')} {formatLogQuota(cacheReadQuota)}
                      </span>
                    )}
                    {cacheWriteQuota > 0 && (
                      <span className='inline-flex items-center gap-1'>
                        <SquarePen className='size-3' />
                        {t('Cache Write')} {formatLogQuota(cacheWriteQuota)}
                      </span>
                    )}
                  </div>
                )}
              </TooltipTrigger>
              {costTooltip}
            </Tooltip>
          </TooltipProvider>
        )
      },
      meta: { label: t('Cost') },
    },

    {
      accessorKey: 'content',
      header: t('Details'),
      cell: function DetailsCell({ row }) {
        const [dialogOpen, setDialogOpen] = useState(false)
        const log = row.original
        const other = parseLogOther(log.other)

        const segments = buildDetailSegments(log, other, t)
        const primary = segments[0]
        const hasMore = segments.length > 1

        return (
          <>
            <button
              type='button'
              className='group flex max-w-[200px] items-center gap-1 text-left text-xs'
              onClick={() => setDialogOpen(true)}
              title={t('Click to view full details')}
            >
              {primary ? (
                <span
                  className={cn(
                    'truncate leading-snug group-hover:underline',
                    primary.muted
                      ? 'text-muted-foreground/60'
                      : primary.danger
                        ? 'text-red-600 dark:text-red-400'
                        : 'text-foreground'
                  )}
                >
                  {primary.text}
                  {hasMore && (
                    <span className='text-muted-foreground/40 ml-0.5'>
                      +{segments.length - 1}
                    </span>
                  )}
                </span>
              ) : log.content ? (
                <span className='text-muted-foreground truncate group-hover:underline'>
                  {log.content}
                </span>
              ) : (
                <span className='text-muted-foreground/40'>—</span>
              )}
            </button>
            <DetailsDialog
              log={log}
              isAdmin={isAdmin}
              open={dialogOpen}
              onOpenChange={setDialogOpen}
            />
          </>
        )
      },
      meta: { label: t('Details') },
      size: 180,
      maxSize: 200,
    }
  )

  return columns
}

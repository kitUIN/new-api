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
import type { TFunction } from 'i18next'
import type { Row, SheetData } from 'write-excel-file'
import {
  formatLogQuota,
  formatTimestampToDate,
  formatUseTime,
} from '@/lib/format'
import {
  buildDetailSegments,
  getCacheReadTokens,
  getCacheWriteTokens,
  getGroupRatioText,
  getPrimaryInputTokens,
} from '../components/columns/common-logs-columns'
import { LOG_CATEGORY_LABELS } from '../constants'
import type {
  FetchLogsConfig,
  LogCategory,
  MidjourneyLog,
  TaskLog,
  UsageLog,
} from '../types'
import {
  formatDuration,
  formatModelName,
  getServiceTierBillingMultiplier,
  parseLogOther,
} from './format'
import {
  mjStatusMapper,
  mjSubmitResultMapper,
  mjTaskTypeMapper,
  taskActionMapper,
  taskStatusMapper,
} from './mappers'
import {
  fetchLogsByCategory,
  formatChannelAffinitySessionKey,
  getChannelAffinitySessionKey,
  getLogTypeConfig,
  getUsageLogSessionKeyInfo,
  isDisplayableLogType,
  isTimingLogType,
} from './utils'

const EXPORT_PAGE_SIZE = 100

type ExportLog = UsageLog | MidjourneyLog | TaskLog

export interface ExportColumn {
  id: string
  label: string
}

export interface ExportUsageLogsOptions {
  logCategory: LogCategory
  isAdmin: boolean
  searchParams: Record<string, unknown>
  columnFilters: Array<{ id: string; value: unknown }>
  columns: ExportColumn[]
  sensitiveVisible: boolean
  t: TFunction
  onProgress?: (exported: number, total: number) => void
}

export interface ExportUsageLogsResult {
  count: number
  fileName?: string
}

interface ColumnValueContext {
  sensitiveVisible: boolean
  t: TFunction
}

function joinLines(values: Array<string | null | undefined>): string {
  return values.filter((value): value is string => Boolean(value)).join('\n')
}

function formatDurationCell(
  submitTime: number | undefined,
  finishTime: number | undefined,
  unit: 'seconds' | 'milliseconds'
): string {
  const duration = formatDuration(submitTime, finishTime, unit)
  return duration ? `${duration.durationSec.toFixed(1)}s` : '-'
}

function getCommonLogCell(
  log: UsageLog,
  columnId: string,
  context: ColumnValueContext
): string {
  const other = parseLogOther(log.other)
  const displayable = isDisplayableLogType(log.type)

  switch (columnId) {
    case 'created_at':
      return joinLines([
        formatTimestampToDate(log.created_at),
        context.t(getLogTypeConfig(log.type).label),
      ])
    case 'channel': {
      if (!displayable) return ''
      const name = log.channel_name
        ? context.sensitiveVisible
          ? log.channel_name
          : '••••'
        : undefined
      const channel = `#${log.channel}`
      const chain = other?.admin_info?.use_channel?.length
        ? `${context.t('Chain')}: ${other.admin_info.use_channel.join(' -> ')}`
        : undefined
      return joinLines([channel, name, chain])
    }
    case 'user':
      if (!log.username) return ''
      return context.sensitiveVisible ? log.username : '••••'
    case 'session_key': {
      if (!displayable) return ''
      const affinity = other?.admin_info?.channel_affinity
      const sessionKeyInfo = getUsageLogSessionKeyInfo(
        other?.admin_info?.session_key,
        affinity
      )
      return formatChannelAffinitySessionKey(
        getChannelAffinitySessionKey(sessionKeyInfo)
      )
    }
    case 'request_id':
      return log.request_id || '-'
    case 'token_name': {
      if (!displayable || !log.token_name) return ''
      const tokenName = context.sensitiveVisible ? log.token_name : '••••'
      const group = log.group || other?.group
      const groupText = group
        ? context.sensitiveVisible
          ? group
          : '••••'
        : undefined
      const ratio = getGroupRatioText(other)
      return joinLines([tokenName, groupText, ratio || undefined])
    }
    case 'model_name': {
      if (!displayable) return ''
      const model = formatModelName(log)
      const modelName = model.actualModel
        ? `${model.name} -> ${model.actualModel}`
        : model.name
      return getServiceTierBillingMultiplier(other) > 1
        ? `${modelName}\n${context.t('Fast Mode')}`
        : modelName
    }
    case 'reasoning_effort':
      return displayable && other?.reasoning_effort
        ? context.t(other.reasoning_effort)
        : '-'
    case 'use_time': {
      if (!isTimingLogType(log.type)) return ''
      const tokensPerSecond =
        log.use_time > 0 && log.completion_tokens > 0
          ? `${(log.completion_tokens / log.use_time).toFixed(1)} t/s`
          : undefined
      const mode = other?.ws
        ? 'WS'
        : log.is_stream
          ? context.t('Stream')
          : context.t('Non-stream')
      const ttft =
        log.is_stream && other?.frt
          ? `TTFT ${formatUseTime(other.frt / 1000)}`
          : undefined
      const streamError =
        log.is_stream && other?.stream_status?.status !== 'ok'
          ? joinLines([
              `${context.t('Stream Status')}: ${context.t('Error')}`,
              other?.stream_status?.end_reason,
              other?.stream_status?.end_error,
            ])
          : undefined
      return joinLines([
        formatUseTime(log.use_time),
        ttft,
        joinLines([mode, tokensPerSecond]),
        streamError,
      ])
    }
    case 'prompt_tokens': {
      if (!displayable) return ''
      const input = getPrimaryInputTokens(log, other)
      const output = Math.max(Number(log.completion_tokens) || 0, 0)
      const cacheRead = getCacheReadTokens(log, other)
      const cacheWrite = getCacheWriteTokens(log, other)
      if (input + output + cacheRead + cacheWrite === 0) return '-'
      return joinLines([
        `${context.t('Input')}: ${input.toLocaleString()}`,
        `${context.t('Output')}: ${output.toLocaleString()}`,
        cacheRead > 0
          ? `${context.t('Cache Read')}: ${cacheRead.toLocaleString()}`
          : undefined,
        cacheWrite > 0
          ? `${context.t('Cache Write')}: ${cacheWrite.toLocaleString()}`
          : undefined,
      ])
    }
    case 'quota': {
      if (!displayable) return ''
      const cacheReadQuota = Math.max(Number(log.cache_read_quota) || 0, 0)
      const cacheWriteQuota = Math.max(Number(log.cache_write_quota) || 0, 0)
      return joinLines([
        other?.billing_source === 'subscription'
          ? context.t('Subscription')
          : formatLogQuota(log.quota),
        cacheReadQuota > 0
          ? `${context.t('Cache Read')}: ${formatLogQuota(cacheReadQuota)}`
          : undefined,
        cacheWriteQuota > 0
          ? `${context.t('Cache Write')}: ${formatLogQuota(cacheWriteQuota)}`
          : undefined,
      ])
    }
    case 'content': {
      const details = buildDetailSegments(log, other, context.t)
        .map((segment) => segment.text)
        .join('\n')
      return details || log.content || '-'
    }
    default:
      return String(log[columnId] ?? '')
  }
}

function getDrawingLogCell(
  log: MidjourneyLog,
  columnId: string,
  context: ColumnValueContext
): string {
  switch (columnId) {
    case 'submit_time':
      return joinLines([
        formatTimestampToDate(log.submit_time),
        context.t(mjStatusMapper.getLabel(log.status)),
      ])
    case 'channel_id':
      return log.channel_id ? `#${log.channel_id}` : '-'
    case 'action':
      return context.t(mjTaskTypeMapper.getLabel(log.action))
    case 'mj_id':
      return log.mj_id || '-'
    case 'duration':
      return formatDurationCell(
        log.submit_time,
        log.finish_time,
        'milliseconds'
      )
    case 'code':
      return context.t(mjSubmitResultMapper.getLabel(String(log.code)))
    case 'progress':
      return log.progress || '-'
    case 'image_url':
      return log.image_url || '-'
    case 'prompt':
      return joinLines([log.prompt || '-', log.prompt_en])
    case 'fail_reason':
      return log.fail_reason || '-'
    default:
      return String(log[columnId as keyof MidjourneyLog] ?? '')
  }
}

function getTaskLogCell(
  log: TaskLog,
  columnId: string,
  context: ColumnValueContext
): string {
  switch (columnId) {
    case 'submit_time':
      return joinLines([
        formatTimestampToDate(log.submit_time, 'seconds'),
        log.finish_time
          ? formatTimestampToDate(log.finish_time, 'seconds')
          : undefined,
      ])
    case 'channel_id':
      return log.channel_id ? `#${log.channel_id}` : '-'
    case 'user': {
      const name = log.username || String(log.user_id || '?')
      return context.sensitiveVisible ? name : '••••'
    }
    case 'task_id':
      return joinLines([
        log.task_id || '-',
        `${context.t(log.platform)} · ${context.t(
          taskActionMapper.getLabel(log.action)
        )}`,
      ])
    case 'duration':
      return formatDurationCell(log.submit_time, log.finish_time, 'seconds')
    case 'status':
      return context.t(
        taskStatusMapper.getLabel(log.status, log.status || 'Submitting')
      )
    case 'progress':
      return log.progress || '-'
    case 'fail_reason':
      return log.fail_reason || '-'
    default:
      return String(log[columnId as keyof TaskLog] ?? '')
  }
}

function getCellValue(
  log: ExportLog,
  logCategory: LogCategory,
  columnId: string,
  context: ColumnValueContext
): string {
  if (logCategory === 'common') {
    return getCommonLogCell(log as UsageLog, columnId, context)
  }
  if (logCategory === 'drawing') {
    return getDrawingLogCell(log as MidjourneyLog, columnId, context)
  }
  return getTaskLogCell(log as TaskLog, columnId, context)
}

async function fetchAllLogs(
  options: ExportUsageLogsOptions
): Promise<ExportLog[]> {
  const baseConfig: Omit<FetchLogsConfig, 'page' | 'pageSize'> = {
    logCategory: options.logCategory,
    isAdmin: options.isAdmin,
    searchParams: options.searchParams,
    columnFilters: options.columnFilters,
  }
  const first = await fetchLogsByCategory({
    ...baseConfig,
    page: 1,
    pageSize: EXPORT_PAGE_SIZE,
  })

  if (!first.success) {
    throw new Error(first.message || 'Failed to load logs')
  }

  const total = first.data?.total || 0
  const logs: ExportLog[] = [...(first.data?.items || [])]
  options.onProgress?.(logs.length, total)

  const pageCount = Math.ceil(total / EXPORT_PAGE_SIZE)
  for (let page = 2; page <= pageCount; page++) {
    const response = await fetchLogsByCategory({
      ...baseConfig,
      page,
      pageSize: EXPORT_PAGE_SIZE,
    })
    if (!response.success) {
      throw new Error(response.message || 'Failed to load logs')
    }
    const items = response.data?.items || []
    logs.push(...items)
    options.onProgress?.(Math.min(logs.length, total), total)
    if (items.length === 0) break
  }

  return logs
}

export async function exportUsageLogsToExcel(
  options: ExportUsageLogsOptions
): Promise<ExportUsageLogsResult> {
  const logs = await fetchAllLogs(options)
  if (logs.length === 0) return { count: 0 }

  const categoryLabel = options.t(LOG_CATEGORY_LABELS[options.logCategory])
  const context: ColumnValueContext = {
    sensitiveVisible: options.sensitiveVisible,
    t: options.t,
  }
  const header: Row = options.columns.map((column) => ({
    value: column.label,
    type: String,
    fontWeight: 'bold',
    color: '#ffffff',
    backgroundColor: '#334155',
    align: 'center',
    alignVertical: 'center',
  }))
  const rows: SheetData = [header]
  for (const log of logs) {
    rows.push(
      options.columns.map((column) => ({
        value: getCellValue(log, options.logCategory, column.id, context),
        type: String,
        wrap: true,
        alignVertical: 'top',
      }))
    )
  }

  const timestamp = new Date().toISOString().replace(/[:.]/g, '-').slice(0, 19)
  const fileName = `usage-logs-${options.logCategory}-${timestamp}.xlsx`
  const { default: writeXlsxFile } = await import('write-excel-file')
  await writeXlsxFile(rows, {
    fileName,
    sheet: categoryLabel.slice(0, 31),
    columns: options.columns.map(() => ({ width: 22 })),
    stickyRowsCount: 1,
  })

  return { count: logs.length, fileName }
}

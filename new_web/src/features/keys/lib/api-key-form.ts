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
import { z } from 'zod'
import type { TFunction } from 'i18next'
import { parseQuotaFromDollars, quotaUnitsToDollars } from '@/lib/format'
import { DEFAULT_GROUP } from '../constants'
import { type ApiKeyFormData, type ApiKey } from '../types'

// ============================================================================
// Form Schema
// ============================================================================

export function getApiKeyFormSchema(t: TFunction) {
  return z
    .object({
      name: z.string().min(1, t('Please enter a name')),
      remain_quota_dollars: z.number().optional(),
      expired_time: z.date().optional(),
      unlimited_quota: z.boolean(),
      model_limits: z.array(z.string()),
      allow_ips: z.string().optional(),
      group: z.string().optional(),
      cross_group_retry: z.boolean().optional(),
      auto_group_mode: z.enum(['low_ratio', 'balanced']).optional(),
      session_group_failover_enabled: z.boolean().optional(),
      session_failover_groups: z.array(z.string()),
      session_failover_threshold: z.number().min(1),
      tokenCount: z.number().min(1).optional(),
    })
    .superRefine((data, ctx) => {
      if (data.unlimited_quota) {
        return
      }

      if (
        data.remain_quota_dollars === undefined ||
        data.remain_quota_dollars < 0
      ) {
        ctx.addIssue({
          code: 'custom',
          path: ['remain_quota_dollars'],
          message: t('Quota must be zero or greater'),
        })
      }

      if (!data.session_group_failover_enabled) {
        return
      }

      const groups = data.session_failover_groups.map((group) => group.trim())
      const uniqueGroups = new Set(groups)
      if (groups.length < 2) {
        ctx.addIssue({
          code: 'custom',
          path: ['session_failover_groups'],
          message: t('Select at least two failover groups'),
        })
      }
      if (groups.some((group) => group === 'auto')) {
        ctx.addIssue({
          code: 'custom',
          path: ['session_failover_groups'],
          message: t('Auto group cannot be used in API key failover'),
        })
      }
      if (uniqueGroups.size !== groups.length) {
        ctx.addIssue({
          code: 'custom',
          path: ['session_failover_groups'],
          message: t('Failover groups cannot be duplicated'),
        })
      }
    })
}

export type ApiKeyFormValues = z.infer<ReturnType<typeof getApiKeyFormSchema>>

// ============================================================================
// Form Defaults
// ============================================================================

export const API_KEY_FORM_DEFAULT_VALUES: ApiKeyFormValues = {
  name: '',
  remain_quota_dollars: 10,
  expired_time: undefined,
  unlimited_quota: true,
  model_limits: [],
  allow_ips: '',
  group: DEFAULT_GROUP,
  cross_group_retry: true,
  auto_group_mode: 'low_ratio',
  session_group_failover_enabled: false,
  session_failover_groups: [],
  session_failover_threshold: 3,
  tokenCount: 1,
}

export function getApiKeyFormDefaultValues(
  defaultUseAutoGroup: boolean
): ApiKeyFormValues {
  return {
    ...API_KEY_FORM_DEFAULT_VALUES,
    group: defaultUseAutoGroup ? 'auto' : DEFAULT_GROUP,
    cross_group_retry: defaultUseAutoGroup,
  }
}

// ============================================================================
// Form Data Transformation
// ============================================================================

/**
 * Transform form data to API payload
 */
export function transformFormDataToPayload(
  data: ApiKeyFormValues
): ApiKeyFormData {
  const failoverGroups = data.session_failover_groups.map((group) =>
    group.trim()
  )
  const failoverEnabled =
    !!data.session_group_failover_enabled && failoverGroups.length >= 2
  const group = failoverEnabled ? failoverGroups[0] : data.group || ''

  return {
    name: data.name,
    remain_quota: data.unlimited_quota
      ? 0
      : parseQuotaFromDollars(data.remain_quota_dollars || 0),
    expired_time: data.expired_time
      ? Math.floor(data.expired_time.getTime() / 1000)
      : -1,
    unlimited_quota: data.unlimited_quota,
    model_limits_enabled: data.model_limits.length > 0,
    model_limits: data.model_limits.join(','),
    allow_ips: data.allow_ips || '',
    group,
    cross_group_retry:
      !failoverEnabled && group === 'auto' ? !!data.cross_group_retry : false,
    auto_group_mode:
      !failoverEnabled && group.startsWith('auto:')
        ? data.auto_group_mode || 'low_ratio'
        : '',
    session_group_failover_enabled: failoverEnabled,
    session_failover_groups: failoverEnabled
      ? JSON.stringify(failoverGroups)
      : '',
    session_failover_threshold: Math.max(
      1,
      data.session_failover_threshold || 3
    ),
  }
}

export function parseSessionFailoverGroups(raw?: string | null): string[] {
  if (!raw) return []
  try {
    const parsed = JSON.parse(raw)
    if (!Array.isArray(parsed)) return []
    return parsed.map((group) =>
      typeof group === 'string' ? group.trim() : ''
    )
  } catch {
    return []
  }
}

/**
 * Transform API key data to form defaults
 */
export function transformApiKeyToFormDefaults(
  apiKey: ApiKey
): ApiKeyFormValues {
  return {
    name: apiKey.name,
    remain_quota_dollars: apiKey.unlimited_quota
      ? 0
      : quotaUnitsToDollars(apiKey.remain_quota),
    expired_time:
      apiKey.expired_time > 0
        ? new Date(apiKey.expired_time * 1000)
        : undefined,
    unlimited_quota: apiKey.unlimited_quota,
    model_limits: apiKey.model_limits
      ? apiKey.model_limits.split(',').filter(Boolean)
      : [],
    allow_ips: apiKey.allow_ips || '',
    group: apiKey.group || DEFAULT_GROUP,
    cross_group_retry: !!apiKey.cross_group_retry,
    auto_group_mode:
      apiKey.auto_group_mode === 'balanced' ? 'balanced' : 'low_ratio',
    session_group_failover_enabled: !!apiKey.session_group_failover_enabled,
    session_failover_groups: parseSessionFailoverGroups(
      apiKey.session_failover_groups
    ),
    session_failover_threshold: Math.max(
      1,
      apiKey.session_failover_threshold || 3
    ),
    tokenCount: 1,
  }
}

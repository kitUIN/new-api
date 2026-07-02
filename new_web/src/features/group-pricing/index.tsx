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
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import * as z from 'zod'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { useQueryClient } from '@tanstack/react-query'
import { PageTransition } from '@/components/page-transition'
import { useSystemOptions } from '@/features/system-settings/hooks/use-system-options'
import { useUpdateOption } from '@/features/system-settings/hooks/use-update-option'
import { GroupRatioForm } from '@/features/system-settings/models/group-ratio-form'
import {
  normalizeJsonString,
  validateJsonString,
} from '@/features/system-settings/models/utils'

const groupSchema = z.object({
  GroupRatio: z.string().superRefine((value, ctx) => {
    const result = validateJsonString(value)
    if (!result.valid) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: result.message || 'Invalid JSON',
      })
    }
  }),
  TopupGroupRatio: z.string().superRefine((value, ctx) => {
    const result = validateJsonString(value)
    if (!result.valid) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: result.message || 'Invalid JSON',
      })
    }
  }),
  UserUsableGroups: z.string().superRefine((value, ctx) => {
    const result = validateJsonString(value)
    if (!result.valid) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: result.message || 'Invalid JSON',
      })
    }
  }),
  GroupGroupRatio: z.string().superRefine((value, ctx) => {
    const result = validateJsonString(value)
    if (!result.valid) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: result.message || 'Invalid JSON',
      })
    }
  }),
  GroupTypes: z.string().superRefine((value, ctx) => {
    const result = validateJsonString(value)
    if (!result.valid) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: result.message || 'Invalid JSON',
      })
    }
  }),
  AutoGroups: z.string().superRefine((value, ctx) => {
    const result = validateJsonString(value)
    if (!result.valid) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: result.message || 'Invalid JSON',
      })
    }
  }),
  AutoGroupOrderType: z.enum(['priority', 'ratio_asc']),
  DefaultUseAutoGroup: z.boolean(),
  GroupSpecialUsableGroup: z.string().superRefine((value, ctx) => {
    const result = validateJsonString(value)
    if (!result.valid) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: result.message || 'Invalid JSON',
      })
    }
  }),
  UpstreamGroupRatioBindings: z.string().superRefine((value, ctx) => {
    const result = validateJsonString(value)
    if (!result.valid) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: result.message || 'Invalid JSON',
      })
    }
  }),
})

type GroupFormValues = z.infer<typeof groupSchema>

function preserveBoundGroupRatios(
  nextGroupRatio: string,
  currentGroupRatio: string,
  bindings: string
): string {
  try {
    const ratioMap = JSON.parse(nextGroupRatio || '{}') as Record<string, number>
    const bindingMap = JSON.parse(bindings || '{}') as Record<string, unknown>
    const currentMap = JSON.parse(currentGroupRatio || '{}') as Record<
      string,
      number
    >
    Object.keys(bindingMap).forEach((group) => {
      if (Object.prototype.hasOwnProperty.call(currentMap, group)) {
        ratioMap[group] = currentMap[group]
      }
    })
    return JSON.stringify(ratioMap)
  } catch {
    return nextGroupRatio
  }
}

export function GroupPricingPage() {
  const { t } = useTranslation()
  const { data: settings, isLoading } = useSystemOptions()
  const updateOption = useUpdateOption()
  const queryClient = useQueryClient()
  const [isSaving, setIsSaving] = useState(false)

  const getValue = useCallback(
    (key: string, defaultValue: string | boolean = '') => {
      if (!settings?.data) return defaultValue
      const option = settings.data.find((opt) => opt.key === key)
      if (typeof defaultValue === 'boolean') {
        return option?.value === 'true' || option?.value === '1'
      }
      return option?.value ?? defaultValue
    },
    [settings]
  )

  const groupNormalizedDefaults = useRef({
    GroupRatio: normalizeJsonString(getValue('GroupRatio', '')),
    TopupGroupRatio: normalizeJsonString(getValue('TopupGroupRatio', '')),
    UserUsableGroups: normalizeJsonString(getValue('UserUsableGroups', '')),
    GroupGroupRatio: normalizeJsonString(getValue('GroupGroupRatio', '')),
    GroupTypes: normalizeJsonString(getValue('GroupTypes', '{}')),
    AutoGroups: normalizeJsonString(getValue('AutoGroups', '')),
    AutoGroupOrderType: getValue('AutoGroupOrderType', 'priority') as
      | 'priority'
      | 'ratio_asc',
    DefaultUseAutoGroup: getValue('DefaultUseAutoGroup', false) as boolean,
    GroupSpecialUsableGroup: normalizeJsonString(
      getValue('group_ratio_setting.group_special_usable_group', '{}')
    ),
    UpstreamGroupRatioBindings: normalizeJsonString(
      getValue('group_ratio_setting.upstream_group_ratio_bindings', '{}')
    ),
  })

  const groupForm = useForm<GroupFormValues>({
    resolver: zodResolver(groupSchema),
    defaultValues: groupNormalizedDefaults.current,
  })

  useEffect(() => {
    if (settings?.data) {
      const newDefaults = {
        GroupRatio: normalizeJsonString(getValue('GroupRatio', '')),
        TopupGroupRatio: normalizeJsonString(getValue('TopupGroupRatio', '')),
        UserUsableGroups: normalizeJsonString(getValue('UserUsableGroups', '')),
        GroupGroupRatio: normalizeJsonString(getValue('GroupGroupRatio', '')),
        GroupTypes: normalizeJsonString(getValue('GroupTypes', '{}')),
        AutoGroups: normalizeJsonString(getValue('AutoGroups', '')),
        AutoGroupOrderType: getValue('AutoGroupOrderType', 'priority') as
          | 'priority'
          | 'ratio_asc',
        DefaultUseAutoGroup: getValue('DefaultUseAutoGroup', false) as boolean,
        GroupSpecialUsableGroup: normalizeJsonString(
          getValue('group_ratio_setting.group_special_usable_group', '{}')
        ),
        UpstreamGroupRatioBindings: normalizeJsonString(
          getValue('group_ratio_setting.upstream_group_ratio_bindings', '{}')
        ),
      }
      groupNormalizedDefaults.current = newDefaults
      groupForm.reset(newDefaults)
    }
  }, [settings, getValue, groupForm])

  const saveGroupRatios = useCallback(
    async (values: GroupFormValues) => {
      setIsSaving(true)
      try {
        const normalizedBindings = normalizeJsonString(
          values.UpstreamGroupRatioBindings
        )
        const normalized = {
          GroupRatio: preserveBoundGroupRatios(
            normalizeJsonString(values.GroupRatio),
            groupNormalizedDefaults.current.GroupRatio,
            normalizedBindings
          ),
          TopupGroupRatio: normalizeJsonString(values.TopupGroupRatio),
          UserUsableGroups: normalizeJsonString(values.UserUsableGroups),
          GroupGroupRatio: normalizeJsonString(values.GroupGroupRatio),
          GroupTypes: normalizeJsonString(values.GroupTypes),
          AutoGroups: normalizeJsonString(values.AutoGroups),
          AutoGroupOrderType: values.AutoGroupOrderType,
          DefaultUseAutoGroup: values.DefaultUseAutoGroup,
          GroupSpecialUsableGroup: normalizeJsonString(
            values.GroupSpecialUsableGroup
          ),
          UpstreamGroupRatioBindings: normalizedBindings,
        }

        const apiKeyMap: Record<string, string> = {
          GroupSpecialUsableGroup:
            'group_ratio_setting.group_special_usable_group',
          UpstreamGroupRatioBindings:
            'group_ratio_setting.upstream_group_ratio_bindings',
        }

        const updates = (
          Object.keys(normalized) as Array<keyof typeof normalized>
        ).filter(
          (key) => normalized[key] !== groupNormalizedDefaults.current[key]
        )

        updates.sort((left, right) => {
          if (left === 'UserUsableGroups') return -1
          if (right === 'UserUsableGroups') return 1
          return 0
        })

        for (const key of updates) {
          const apiKey = apiKeyMap[key] || key
          await updateOption.mutateAsync({
            key: apiKey,
            value: normalized[key],
          })
        }

        groupNormalizedDefaults.current = normalized
        queryClient.invalidateQueries({ queryKey: ['system-options'] })
        toast.success(t('Settings saved successfully'))
      } catch (error) {
        toast.error(t('Failed to save settings'))
      } finally {
        setIsSaving(false)
      }
    },
    [updateOption, queryClient, t]
  )

  if (isLoading) {
    return (
      <PageTransition className='flex h-full items-center justify-center'>
        <div className='text-muted-foreground'>{t('Loading...')}</div>
      </PageTransition>
    )
  }

  return (
    <PageTransition className='flex h-full flex-col'>
      <div className='shrink-0 px-3 pt-3 pb-2.5 sm:px-4 sm:pt-5 sm:pb-3'>
        <div className='flex flex-wrap items-center justify-between gap-x-3 gap-y-2 sm:gap-x-4'>
          <div className='min-w-0 flex-1'>
            <h2 className='truncate text-base font-bold tracking-tight sm:text-lg'>
              {t('Group Pricing')}
            </h2>
            <p className='text-muted-foreground mt-1 text-sm'>
              {t('Configure group-based pricing multipliers and settings')}
            </p>
          </div>
        </div>
      </div>

      <div className='min-h-0 flex-1 overflow-auto px-3 pt-1 pb-3 sm:px-4 sm:pt-1.5 sm:pb-4'>
        <GroupRatioForm
          form={groupForm}
          onSave={saveGroupRatios}
          isSaving={isSaving}
        />
      </div>
    </PageTransition>
  )
}

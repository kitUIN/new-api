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
import { useEffect, useMemo } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { AuthenticatedLayout } from '@/components/layout'
import { PageTransition } from '@/components/page-transition'
import { RatioSettingsCard } from '@/features/system-settings/models/ratio-settings-card'
import { useSystemOptions } from '@/features/system-settings/hooks/use-system-options'

type GroupPricingFormValues = {
  ModelPrice: string
  ModelRatio: string
  CacheRatio: string
  CreateCacheRatio: string
  CompletionRatio: string
  ImageRatio: string
  AudioRatio: string
  AudioCompletionRatio: string
  ExposeRatioEnabled: boolean
  BillingMode: string
  BillingExpr: string
  TopupGroupRatio: string
  GroupRatio: string
  UserUsableGroups: string
  GroupGroupRatio: string
  GroupTypes: string
  AutoGroups: string
  AutoGroupOrderType: 'priority' | 'ratio_asc'
  DefaultUseAutoGroup: boolean
  GroupSpecialUsableGroup: string
  UpstreamGroupRatioBindings: string
}

export function GroupPricingPage() {
  const { t } = useTranslation()
  const { data: settings, isLoading } = useSystemOptions()

  const form = useForm<GroupPricingFormValues>({
    defaultValues: {
      ModelPrice: '',
      ModelRatio: '',
      CacheRatio: '',
      CreateCacheRatio: '',
      CompletionRatio: '',
      ImageRatio: '',
      AudioRatio: '',
      AudioCompletionRatio: '',
      ExposeRatioEnabled: false,
      BillingMode: '{}',
      BillingExpr: '{}',
      TopupGroupRatio: '',
      GroupRatio: '',
      UserUsableGroups: '',
      GroupGroupRatio: '',
      GroupTypes: '{}',
      AutoGroups: '',
      AutoGroupOrderType: 'priority',
      DefaultUseAutoGroup: false,
      GroupSpecialUsableGroup: '{}',
      UpstreamGroupRatioBindings: '{}',
    },
  })

  useEffect(() => {
    if (settings) {
      const options = settings.data || []
      const getValue = (key: string, defaultValue: string | boolean = '') => {
        const option = options.find((opt) => opt.key === key)
        if (typeof defaultValue === 'boolean') {
          return option?.value === 'true' || option?.value === '1'
        }
        return option?.value ?? defaultValue
      }

      form.reset({
        ModelPrice: getValue('ModelPrice', ''),
        ModelRatio: getValue('ModelRatio', ''),
        CacheRatio: getValue('CacheRatio', ''),
        CreateCacheRatio: getValue('CreateCacheRatio', ''),
        CompletionRatio: getValue('CompletionRatio', ''),
        ImageRatio: getValue('ImageRatio', ''),
        AudioRatio: getValue('AudioRatio', ''),
        AudioCompletionRatio: getValue('AudioCompletionRatio', ''),
        ExposeRatioEnabled: getValue('ExposeRatioEnabled', false) as boolean,
        BillingMode: getValue('billing_setting.billing_mode', '{}'),
        BillingExpr: getValue('billing_setting.billing_expr', '{}'),
        TopupGroupRatio: getValue('TopupGroupRatio', ''),
        GroupRatio: getValue('GroupRatio', ''),
        UserUsableGroups: getValue('UserUsableGroups', ''),
        GroupGroupRatio: getValue('GroupGroupRatio', ''),
        GroupTypes: getValue('GroupTypes', '{}'),
        AutoGroups: getValue('AutoGroups', ''),
        AutoGroupOrderType: getValue('AutoGroupOrderType', 'priority') as
          | 'priority'
          | 'ratio_asc',
        DefaultUseAutoGroup: getValue('DefaultUseAutoGroup', false) as boolean,
        GroupSpecialUsableGroup: getValue(
          'group_ratio_setting.group_special_usable_group',
          '{}'
        ),
        UpstreamGroupRatioBindings: getValue(
          'group_ratio_setting.upstream_group_ratio_bindings',
          '{}'
        ),
      })
    }
  }, [settings, form])

  const modelDefaults = useMemo(
    () => ({
      ModelPrice: form.watch('ModelPrice'),
      ModelRatio: form.watch('ModelRatio'),
      CacheRatio: form.watch('CacheRatio'),
      CreateCacheRatio: form.watch('CreateCacheRatio'),
      CompletionRatio: form.watch('CompletionRatio'),
      ImageRatio: form.watch('ImageRatio'),
      AudioRatio: form.watch('AudioRatio'),
      AudioCompletionRatio: form.watch('AudioCompletionRatio'),
      ExposeRatioEnabled: form.watch('ExposeRatioEnabled'),
      BillingMode: form.watch('BillingMode'),
      BillingExpr: form.watch('BillingExpr'),
    }),
    [form]
  )

  const groupDefaults = useMemo(
    () => ({
      TopupGroupRatio: form.watch('TopupGroupRatio'),
      GroupRatio: form.watch('GroupRatio'),
      UserUsableGroups: form.watch('UserUsableGroups'),
      GroupGroupRatio: form.watch('GroupGroupRatio'),
      GroupTypes: form.watch('GroupTypes'),
      AutoGroups: form.watch('AutoGroups'),
      AutoGroupOrderType: form.watch('AutoGroupOrderType'),
      DefaultUseAutoGroup: form.watch('DefaultUseAutoGroup'),
      GroupSpecialUsableGroup: form.watch('GroupSpecialUsableGroup'),
      UpstreamGroupRatioBindings: form.watch('UpstreamGroupRatioBindings'),
    }),
    [form]
  )

  const toolPricesDefault = useMemo(() => {
    if (!settings) return '{}'
    const options = settings.data || []
    const toolPriceOption = options.find(
      (opt) => opt.key === 'tool_price_setting.prices'
    )
    return toolPriceOption?.value ?? '{}'
  }, [settings])

  if (isLoading) {
    return (
      <AuthenticatedLayout>
        <PageTransition>
          <div className='flex items-center justify-center h-screen'>
            <div className='text-muted-foreground'>{t('Loading...')}</div>
          </div>
        </PageTransition>
      </AuthenticatedLayout>
    )
  }

  return (
    <AuthenticatedLayout>
      <PageTransition>
        <div className='container mx-auto py-6 px-4 max-w-7xl'>
          <div className='mb-6'>
            <h1 className='text-3xl font-bold tracking-tight'>
              {t('Group Pricing')}
            </h1>
            <p className='text-muted-foreground mt-2'>
              {t('Configure group-based pricing multipliers and settings')}
            </p>
          </div>

          <RatioSettingsCard
            titleKey='Group Pricing'
            modelDefaults={modelDefaults}
            groupDefaults={groupDefaults}
            toolPricesDefault={toolPricesDefault}
            visibleTabs={['groups']}
          />
        </div>
      </PageTransition>
    </AuthenticatedLayout>
  )
}

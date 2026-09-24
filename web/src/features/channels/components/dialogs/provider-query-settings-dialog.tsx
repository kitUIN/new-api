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
import { useEffect, useMemo, useState } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { RefreshCw, Route } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Textarea } from '@/components/ui/textarea'
import {
  updateChannelProvider,
  updateProviderBalance,
  updateProviderGroups,
} from '../../api'
import { channelsQueryKeys } from '../../lib'
import {
  BALANCE_QUERY_NEWAPI_TEMPLATE,
  BALANCE_QUERY_SUB2API_TEMPLATE,
  GROUP_QUERY_TEMPLATES,
  getBalanceQueryTemplateKey,
  normalizeBalanceQueryTemplate,
} from '../../lib/channel-form'
import type { ChannelProviderSettings, ProviderRow } from '../../types'

type Props = {
  open: boolean
  onOpenChange: (open: boolean) => void
  provider: ProviderRow | null
}

type FormState = {
  access_token: string
  refresh_token: string
  user_id: string
  query_proxy: string
  sub2api_auto_login_enabled: boolean
  sub2api_email: string
  sub2api_password: string
  balance_enabled: boolean
  balance_template: string
  balance_interval_seconds: number
  balance_source_channel_id: number
  balance_request_url: string
  balance_request_method: string
  balance_request_headers: string
  balance_request_body: string
  balance_plan_name_path: string
  balance_remaining_path: string
  balance_used_path: string
  balance_total_path: string
  balance_unit_path: string
  balance_unit: string
  balance_divisor: number
  balance_success_path: string
  balance_success_value: string
  balance_success_optional: boolean
  balance_message_path: string
  group_enabled: boolean
  group_template: string
  group_interval_seconds: number
  group_source_channel_id: number
  group_request_url: string
  group_request_method: string
  group_request_headers: string
  group_request_body: string
  group_data_path: string
  group_desc_path: string
  group_ratio_path: string
  group_success_path: string
  group_success_value: string
  group_success_optional: boolean
  group_message_path: string
}

function parseSettings(settings?: string): ChannelProviderSettings {
  if (!settings) return {}
  try {
    const parsed = JSON.parse(settings)
    return parsed && typeof parsed === 'object' ? parsed : {}
  } catch {
    return {}
  }
}

function headers(value: unknown, fallback: Record<string, string>) {
  const source = value && typeof value === 'object' ? value : fallback
  return JSON.stringify(source, null, 2)
}

function parseHeaders(value: string) {
  if (!value.trim()) return {}
  const parsed = JSON.parse(value)
  return parsed && typeof parsed === 'object' && !Array.isArray(parsed)
    ? parsed
    : {}
}

function normalizeIntervalSeconds(value: number) {
  if (!Number.isFinite(value)) return 300
  return Math.max(0, Math.trunc(value))
}

function defaultState(provider: ProviderRow | null): FormState {
  const settings = parseSettings(provider?.settings)
  const b = settings.balance_query || {}
  const bTemplateName = normalizeBalanceQueryTemplate(b.template)
  const bTemplateKey = getBalanceQueryTemplateKey(bTemplateName)
  const bTemplate =
    bTemplateKey === 'sub2api'
      ? BALANCE_QUERY_SUB2API_TEMPLATE
      : BALANCE_QUERY_NEWAPI_TEMPLATE
  const g = settings.group_query || {}
  const gTemplateKey =
    g.template && g.template in GROUP_QUERY_TEMPLATES
      ? (g.template as keyof typeof GROUP_QUERY_TEMPLATES)
      : 'newapi'
  const gTemplate = GROUP_QUERY_TEMPLATES[gTemplateKey]

  return {
    access_token: b.access_token || g.access_token || '',
    refresh_token: b.refresh_token || g.refresh_token || '',
    user_id: b.user_id || g.user_id || '',
    query_proxy: settings.query_proxy || '',
    sub2api_auto_login_enabled: settings.sub2api_auto_login_enabled === true,
    sub2api_email: settings.sub2api_email || '',
    sub2api_password: settings.sub2api_password || '',
    balance_enabled: b.enabled === true,
    balance_template: bTemplateName,
    balance_interval_seconds: b.interval_seconds || 300,
    balance_source_channel_id:
      b.source_channel_id || provider?.children?.[0]?.id || 0,
    balance_request_url: b.request?.url || bTemplate.request.url,
    balance_request_method: b.request?.method || bTemplate.request.method,
    balance_request_headers: headers(
      b.request?.headers,
      bTemplate.request.headers
    ),
    balance_request_body: b.request?.body || '',
    balance_plan_name_path:
      b.extractor?.plan_name_path || bTemplate.extractor.plan_name_path,
    balance_remaining_path:
      b.extractor?.remaining_path || bTemplate.extractor.remaining_path,
    balance_used_path: b.extractor?.used_path || bTemplate.extractor.used_path,
    balance_total_path:
      b.extractor?.total_path || bTemplate.extractor.total_path,
    balance_unit_path: b.extractor?.unit_path || bTemplate.extractor.unit_path,
    balance_unit: b.extractor?.unit || bTemplate.extractor.unit,
    balance_divisor: b.extractor?.divisor || bTemplate.extractor.divisor,
    balance_success_path:
      b.extractor?.success_path || bTemplate.extractor.success_path,
    balance_success_value:
      b.extractor?.success_value || bTemplate.extractor.success_value,
    balance_success_optional:
      typeof b.extractor?.success_optional === 'boolean'
        ? b.extractor.success_optional
        : bTemplate.extractor.success_optional,
    balance_message_path:
      b.extractor?.message_path || bTemplate.extractor.message_path,
    group_enabled: g.enabled === true,
    group_template: gTemplateKey,
    group_interval_seconds: g.interval_seconds || 300,
    group_source_channel_id:
      g.source_channel_id || provider?.children?.[0]?.id || 0,
    group_request_url: g.request?.url || gTemplate.request.url,
    group_request_method: g.request?.method || gTemplate.request.method,
    group_request_headers: headers(
      g.request?.headers,
      gTemplate.request.headers
    ),
    group_request_body: g.request?.body || '',
    group_data_path: g.extractor?.data_path || gTemplate.extractor.data_path,
    group_desc_path: g.extractor?.desc_path || gTemplate.extractor.desc_path,
    group_ratio_path: g.extractor?.ratio_path || gTemplate.extractor.ratio_path,
    group_success_path:
      g.extractor?.success_path || gTemplate.extractor.success_path,
    group_success_value:
      g.extractor?.success_value || gTemplate.extractor.success_value,
    group_success_optional:
      typeof g.extractor?.success_optional === 'boolean'
        ? g.extractor.success_optional
        : gTemplate.extractor.success_optional,
    group_message_path:
      g.extractor?.message_path || gTemplate.extractor.message_path,
  }
}

export function ProviderQuerySettingsDialog(props: Props) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [form, setForm] = useState<FormState>(() =>
    defaultState(props.provider)
  )
  const [saving, setSaving] = useState(false)
  const isSub2API =
    normalizeBalanceQueryTemplate(form.balance_template) === 'sub2api' ||
    form.group_template === 'sub2api'

  useEffect(() => {
    if (props.open) setForm(defaultState(props.provider))
  }, [props.open, props.provider])

  const channelOptions = useMemo(
    () =>
      props.provider?.children?.filter(
        (child) => !child.channel_info?.is_multi_key
      ) || [],
    [props.provider]
  )

  const set = <K extends keyof FormState>(key: K, value: FormState[K]) =>
    setForm((prev) => ({ ...prev, [key]: value }))

  const applyBalanceTemplate = (templateKey: string | null) => {
    if (!templateKey) return
    const normalizedTemplateKey = normalizeBalanceQueryTemplate(templateKey)
    const tpl =
      getBalanceQueryTemplateKey(normalizedTemplateKey) === 'sub2api'
        ? BALANCE_QUERY_SUB2API_TEMPLATE
        : BALANCE_QUERY_NEWAPI_TEMPLATE
    setForm((prev) => ({
      ...prev,
      balance_template: normalizedTemplateKey,
      balance_request_url: tpl.request.url,
      balance_request_method: tpl.request.method,
      balance_request_headers: JSON.stringify(tpl.request.headers, null, 2),
      balance_plan_name_path: tpl.extractor.plan_name_path,
      balance_remaining_path: tpl.extractor.remaining_path,
      balance_used_path: tpl.extractor.used_path,
      balance_total_path: tpl.extractor.total_path,
      balance_unit_path: tpl.extractor.unit_path,
      balance_unit: tpl.extractor.unit,
      balance_divisor: tpl.extractor.divisor,
      balance_success_path: tpl.extractor.success_path,
      balance_success_value: tpl.extractor.success_value,
      balance_success_optional: tpl.extractor.success_optional,
      balance_message_path: tpl.extractor.message_path,
    }))
  }

  const applyGroupTemplate = (templateKey: string | null) => {
    if (!templateKey) return
    const normalizedTemplateKey =
      templateKey in GROUP_QUERY_TEMPLATES
        ? (templateKey as keyof typeof GROUP_QUERY_TEMPLATES)
        : 'newapi'
    const tpl = GROUP_QUERY_TEMPLATES[normalizedTemplateKey]
    setForm((prev) => ({
      ...prev,
      group_template: normalizedTemplateKey,
      group_request_url: tpl.request.url,
      group_request_method: tpl.request.method,
      group_request_headers: JSON.stringify(tpl.request.headers, null, 2),
      group_data_path: tpl.extractor.data_path,
      group_desc_path: tpl.extractor.desc_path,
      group_ratio_path: tpl.extractor.ratio_path,
      group_success_path: tpl.extractor.success_path,
      group_success_value: tpl.extractor.success_value,
      group_success_optional: tpl.extractor.success_optional,
      group_message_path: tpl.extractor.message_path,
    }))
  }

  const buildSettings = () => {
    const previous = parseSettings(props.provider?.settings)
    return JSON.stringify({
      ...previous,
      sub2api_auto_login_enabled: form.sub2api_auto_login_enabled,
      sub2api_email: form.sub2api_email,
      sub2api_password: form.sub2api_password,
      query_proxy: form.query_proxy.trim(),
      balance_query: {
        ...previous.balance_query,
        enabled: form.balance_enabled,
        template: normalizeBalanceQueryTemplate(form.balance_template),
        interval_seconds: normalizeIntervalSeconds(
          Number(form.balance_interval_seconds)
        ),
        source_channel_id: Number(form.balance_source_channel_id) || 0,
        access_token: form.access_token,
        refresh_token: form.refresh_token,
        user_id: form.user_id,
        request: {
          url: form.balance_request_url,
          method: form.balance_request_method || 'GET',
          headers: parseHeaders(form.balance_request_headers),
          body: form.balance_request_body,
        },
        extractor: {
          plan_name_path: form.balance_plan_name_path,
          remaining_path: form.balance_remaining_path,
          used_path: form.balance_used_path,
          total_path: form.balance_total_path,
          unit_path: form.balance_unit_path,
          unit: form.balance_unit || 'USD',
          divisor: Number(form.balance_divisor) || 1,
          success_path: form.balance_success_path,
          success_value: form.balance_success_value,
          success_optional: form.balance_success_optional,
          message_path: form.balance_message_path,
        },
      },
      group_query: {
        ...previous.group_query,
        enabled: form.group_enabled,
        template: form.group_template,
        interval_seconds: normalizeIntervalSeconds(
          Number(form.group_interval_seconds)
        ),
        source_channel_id: Number(form.group_source_channel_id) || 0,
        access_token: form.access_token,
        refresh_token: form.refresh_token,
        user_id: form.user_id,
        request: {
          url: form.group_request_url,
          method: form.group_request_method || 'GET',
          headers: parseHeaders(form.group_request_headers),
          body: form.group_request_body,
        },
        extractor: {
          data_path: form.group_data_path,
          desc_path: form.group_desc_path,
          ratio_path: form.group_ratio_path,
          success_path: form.group_success_path,
          success_value: form.group_success_value,
          success_optional: form.group_success_optional,
          message_path: form.group_message_path,
        },
      },
    })
  }

  const save = async () => {
    if (!props.provider) return false
    setSaving(true)
    try {
      const res = await updateChannelProvider({
        ...props.provider,
        settings: buildSettings(),
      })
      if (!res.success) throw new Error(res.message || t('Failed to save'))
      toast.success(t('Saved successfully'))
      await queryClient.invalidateQueries({
        queryKey: channelsQueryKeys.lists(),
      })
      props.onOpenChange(false)
      return true
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('Failed to save'))
      return false
    } finally {
      setSaving(false)
    }
  }

  const runBalance = async () => {
    if (!props.provider) return
    if (!(await save())) return
    const res = await updateProviderBalance(props.provider.provider_id)
    if (res.success) toast.success(t('Balance updated successfully'))
    else toast.error(res.message || t('Failed to update balance'))
    await queryClient.invalidateQueries({ queryKey: channelsQueryKeys.lists() })
  }

  const runGroups = async () => {
    if (!props.provider) return
    if (!(await save())) return
    const res = await updateProviderGroups(props.provider.provider_id)
    if (res.success) toast.success(t('Updated successfully'))
    else toast.error(res.message || t('Update failed'))
    await queryClient.invalidateQueries({ queryKey: channelsQueryKeys.lists() })
  }

  return (
    <Dialog open={props.open} onOpenChange={props.onOpenChange}>
      <DialogContent className='max-h-[90vh] w-[calc(100vw-2rem)] max-w-6xl overflow-y-auto sm:max-w-6xl'>
        <DialogHeader>
          <DialogTitle>{t('Provider query settings')}</DialogTitle>
        </DialogHeader>
        <div className='space-y-1.5'>
          <Label htmlFor='provider-query-proxy'>{t('Proxy Address')}</Label>
          <Input
            id='provider-query-proxy'
            value={form.query_proxy}
            placeholder={t('socks5://user:pass@host:port')}
            onChange={(event) => set('query_proxy', event.target.value)}
          />
          <p className='text-muted-foreground text-sm'>
            {t(
              'Used for balance and upstream group queries. Leave blank to use the selected source channel proxy.'
            )}
          </p>
        </div>
        <div className='grid gap-3 sm:grid-cols-3'>
          <TextField
            label='Access Token'
            value={form.access_token}
            onChange={(v) => set('access_token', v)}
          />
          <TextField
            label='Refresh Token'
            value={form.refresh_token}
            onChange={(v) => set('refresh_token', v)}
          />
          <TextField
            label={t('User ID')}
            value={form.user_id}
            onChange={(v) => set('user_id', v)}
          />
        </div>
        {isSub2API && (
          <div className='space-y-3'>
            <SwitchRow
              label={t('Enable sub2api auto login')}
              checked={form.sub2api_auto_login_enabled}
              onCheckedChange={(v) => set('sub2api_auto_login_enabled', v)}
            />
            <div className='grid gap-3 sm:grid-cols-2'>
              <TextField
                label={t('Email')}
                value={form.sub2api_email}
                onChange={(v) => set('sub2api_email', v)}
                disabled={!form.sub2api_auto_login_enabled}
              />
              <TextField
                label={t('Password')}
                type='password'
                value={form.sub2api_password}
                onChange={(v) => set('sub2api_password', v)}
                disabled={!form.sub2api_auto_login_enabled}
              />
            </div>
          </div>
        )}
        <Tabs defaultValue='balance'>
          <TabsList>
            <TabsTrigger value='balance'>{t('Balance')}</TabsTrigger>
            <TabsTrigger value='groups'>{t('Upstream Groups')}</TabsTrigger>
          </TabsList>
          <TabsContent value='balance' className='space-y-4 pt-4'>
            <SwitchRow
              label={t('Enable balance query')}
              checked={form.balance_enabled}
              onCheckedChange={(v) => set('balance_enabled', v)}
            />
            <QuerySourceSelect
              value={form.balance_source_channel_id}
              channels={channelOptions}
              onChange={(v) => set('balance_source_channel_id', v)}
            />
            <div className='grid gap-3 sm:grid-cols-3'>
              <Field label={t('Template')}>
                <Select
                  value={form.balance_template}
                  onValueChange={applyBalanceTemplate}
                >
                  <SelectTrigger className='w-full'>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value='newapi'>New API</SelectItem>
                    <SelectItem value='sub2api'>sub2api</SelectItem>
                  </SelectContent>
                </Select>
              </Field>
              <NumberField
                label={t('Check interval (seconds)')}
                value={form.balance_interval_seconds}
                onChange={(v) => set('balance_interval_seconds', v)}
              />
              <NumberField
                label={t('Value divisor')}
                value={form.balance_divisor}
                onChange={(v) => set('balance_divisor', v)}
              />
            </div>
            <RequestFields prefix='balance' form={form} set={set} />
            <div className='grid gap-3 sm:grid-cols-3'>
              <TextField
                label={t('Remaining path')}
                value={form.balance_remaining_path}
                onChange={(v) => set('balance_remaining_path', v)}
              />
              <TextField
                label={t('Used path')}
                value={form.balance_used_path}
                onChange={(v) => set('balance_used_path', v)}
              />
              <TextField
                label={t('Total path')}
                value={form.balance_total_path}
                onChange={(v) => set('balance_total_path', v)}
              />
              <TextField
                label={t('Plan name path')}
                value={form.balance_plan_name_path}
                onChange={(v) => set('balance_plan_name_path', v)}
              />
              <TextField
                label={t('Unit path')}
                value={form.balance_unit_path}
                onChange={(v) => set('balance_unit_path', v)}
              />
              <TextField
                label={t('Unit')}
                value={form.balance_unit}
                onChange={(v) => set('balance_unit', v)}
              />
              <TextField
                label={t('Success path')}
                value={form.balance_success_path}
                onChange={(v) => set('balance_success_path', v)}
              />
              <TextField
                label={t('Success value')}
                value={form.balance_success_value}
                onChange={(v) => set('balance_success_value', v)}
              />
              <TextField
                label={t('Error message path')}
                value={form.balance_message_path}
                onChange={(v) => set('balance_message_path', v)}
              />
            </div>
            <SwitchRow
              label={t('Success may be absent')}
              checked={form.balance_success_optional}
              onCheckedChange={(v) => set('balance_success_optional', v)}
            />
          </TabsContent>
          <TabsContent value='groups' className='space-y-4 pt-4'>
            <SwitchRow
              label={t('Enable group query')}
              checked={form.group_enabled}
              onCheckedChange={(v) => set('group_enabled', v)}
            />
            <QuerySourceSelect
              value={form.group_source_channel_id}
              channels={channelOptions}
              onChange={(v) => set('group_source_channel_id', v)}
            />
            <div className='grid gap-3 sm:grid-cols-2'>
              <NumberField
                label={t('Check interval (seconds)')}
                value={form.group_interval_seconds}
                onChange={(v) => set('group_interval_seconds', v)}
              />
              <Field label={t('Template')}>
                <Select
                  value={form.group_template}
                  onValueChange={applyGroupTemplate}
                >
                  <SelectTrigger className='w-full'>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value='newapi'>New API</SelectItem>
                    <SelectItem value='sub2api'>sub2api</SelectItem>
                  </SelectContent>
                </Select>
              </Field>
            </div>
            <RequestFields prefix='group' form={form} set={set} />
            <div className='grid gap-3 sm:grid-cols-3'>
              <TextField
                label={t('Group data path')}
                value={form.group_data_path}
                onChange={(v) => set('group_data_path', v)}
              />
              <TextField
                label={t('Description path')}
                value={form.group_desc_path}
                onChange={(v) => set('group_desc_path', v)}
              />
              <TextField
                label={t('Ratio path')}
                value={form.group_ratio_path}
                onChange={(v) => set('group_ratio_path', v)}
              />
              <TextField
                label={t('Success path')}
                value={form.group_success_path}
                onChange={(v) => set('group_success_path', v)}
              />
              <TextField
                label={t('Success value')}
                value={form.group_success_value}
                onChange={(v) => set('group_success_value', v)}
              />
              <TextField
                label={t('Error message path')}
                value={form.group_message_path}
                onChange={(v) => set('group_message_path', v)}
              />
            </div>
            <SwitchRow
              label={t('Success may be absent')}
              checked={form.group_success_optional}
              onCheckedChange={(v) => set('group_success_optional', v)}
            />
          </TabsContent>
        </Tabs>
        <DialogFooter className='gap-2'>
          <Button
            type='button'
            variant='outline'
            onClick={runBalance}
            disabled={saving}
          >
            <RefreshCw className='size-4' />
            {t('Update Balance')}
          </Button>
          <Button
            type='button'
            variant='outline'
            onClick={runGroups}
            disabled={saving}
          >
            <Route className='size-4' />
            {t('Update Groups')}
          </Button>
          <Button type='button' onClick={save} disabled={saving}>
            {saving ? t('Saving...') : t('Save')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

function Field(props: { label: string; children: React.ReactNode }) {
  return (
    <div className='space-y-1.5'>
      <Label>{props.label}</Label>
      {props.children}
    </div>
  )
}

function TextField(props: {
  label: string
  type?: React.HTMLInputTypeAttribute
  value: string
  onChange: (v: string) => void
  disabled?: boolean
}) {
  return (
    <Field label={props.label}>
      <Input
        type={props.type}
        value={props.value}
        onChange={(e) => props.onChange(e.target.value)}
        disabled={props.disabled}
      />
    </Field>
  )
}

function NumberField(props: {
  label: string
  value: number
  onChange: (v: number) => void
}) {
  return (
    <Field label={props.label}>
      <Input
        type='number'
        value={props.value}
        onChange={(e) => props.onChange(Number(e.target.value))}
      />
    </Field>
  )
}

function SwitchRow(props: {
  label: string
  checked: boolean
  onCheckedChange: (v: boolean) => void
}) {
  return (
    <div className='flex items-center justify-between rounded-md border px-3 py-2'>
      <Label>{props.label}</Label>
      <Switch checked={props.checked} onCheckedChange={props.onCheckedChange} />
    </div>
  )
}

function QuerySourceSelect(props: {
  value: number
  channels: ProviderRow['children']
  onChange: (v: number) => void
}) {
  const { t } = useTranslation()
  return (
    <Field label={t('Query source channel')}>
      <Select
        value={String(props.value || 0)}
        onValueChange={(v) => props.onChange(Number(v) || 0)}
      >
        <SelectTrigger className='w-full'>
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value='0'>{t('Auto select')}</SelectItem>
          {props.channels.map((channel) => (
            <SelectItem key={channel.id} value={String(channel.id)}>
              {channel.name}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </Field>
  )
}

function RequestFields(props: {
  prefix: 'balance' | 'group'
  form: FormState
  set: <K extends keyof FormState>(key: K, value: FormState[K]) => void
}) {
  const { t } = useTranslation()
  const p = props.prefix
  return (
    <>
      <div className='grid gap-3 sm:grid-cols-[1fr_160px]'>
        <TextField
          label={t('Request URL')}
          value={props.form[`${p}_request_url`]}
          onChange={(v) => props.set(`${p}_request_url`, v)}
        />
        <TextField
          label={t('Request method')}
          value={props.form[`${p}_request_method`]}
          onChange={(v) => props.set(`${p}_request_method`, v)}
        />
      </div>
      <Field label={t('Request headers JSON')}>
        <Textarea
          rows={4}
          value={props.form[`${p}_request_headers`]}
          onChange={(e) => props.set(`${p}_request_headers`, e.target.value)}
        />
      </Field>
      <Field label={t('Request body')}>
        <Textarea
          rows={3}
          value={props.form[`${p}_request_body`]}
          onChange={(e) => props.set(`${p}_request_body`, e.target.value)}
        />
      </Field>
    </>
  )
}

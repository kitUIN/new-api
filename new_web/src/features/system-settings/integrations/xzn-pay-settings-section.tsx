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
import { useEffect, useState } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Textarea } from '@/components/ui/textarea'
import { SettingsSwitchField } from '../components/settings-form-layout'
import { SettingsPageActionsPortal } from '../components/settings-page-context'
import { useUpdateOption } from '../hooks/use-update-option'

export interface XznPaySettingsValues {
  XznPayEnabled: boolean
  XznPayGatewayURL: string
  XznPayCallbackAddress: string
  XznPayPID: string
  XznPaySignType: 'MD5' | 'RSA'
  XznPayMD5Key: string
  XznPayPrivateKey: string
  XznPayPublicKey: string
  XznPayMinTopUp: number
  XznPayMethods: string
}

interface XznPayMethodTemplate {
  name: string
  paytype_code: string
  channel_id?: string
  icon?: string
  min_topup?: number
}

const METHOD_TEMPLATES: XznPayMethodTemplate[] = [
  { name: 'Alipay', paytype_code: 'alipay' },
  { name: 'WeChat Pay', paytype_code: 'wxpay' },
  { name: 'QQ Wallet', paytype_code: 'qqpay' },
  { name: 'Online Banking', paytype_code: 'bank' },
  { name: 'JD Pay', paytype_code: 'jdpay' },
  { name: 'UnionPay', paytype_code: 'unionpay' },
  { name: 'USDT', paytype_code: 'usdt' },
  { name: 'PayPal', paytype_code: 'paypal' },
  { name: 'Douyin Pay', paytype_code: 'douyinpay' },
]

interface Props {
  defaultValues: XznPaySettingsValues
}

function parseMethods(value: string): XznPayMethodTemplate[] {
  const parsed: unknown = JSON.parse(value || '[]')
  if (!Array.isArray(parsed)) {
    throw new Error('XznPay methods must be a JSON array')
  }
  return parsed as XznPayMethodTemplate[]
}

export function XznPaySettingsSection(props: Props) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const [loading, setLoading] = useState(false)
  const [methods, setMethods] = useState(props.defaultValues.XznPayMethods)
  const form = useForm<Omit<XznPaySettingsValues, 'XznPayMethods'>>({
    defaultValues: props.defaultValues,
  })
  const signType = form.watch('XznPaySignType')

  useEffect(() => {
    form.reset(props.defaultValues)
    setMethods(props.defaultValues.XznPayMethods || '[]')
  }, [form, props.defaultValues])

  const addTemplate = (template: XznPayMethodTemplate) => {
    try {
      const current = parseMethods(methods)
      if (
        current.some((method) => method.paytype_code === template.paytype_code)
      ) {
        toast.error(t('Payment method already exists'))
        return
      }
      setMethods(JSON.stringify([...current, template], null, 2))
    } catch {
      toast.error(t('Payment methods must be a JSON array'))
    }
  }

  const handleSave = async () => {
    const values = form.getValues()
    const callbackAddress = values.XznPayCallbackAddress.trim()
    let parsedMethods: XznPayMethodTemplate[]
    try {
      parsedMethods = parseMethods(methods)
      for (const method of parsedMethods) {
        if (!method.name?.trim() || !method.paytype_code?.trim()) {
          throw new Error('invalid method')
        }
      }
    } catch {
      toast.error(t('Payment methods must be a JSON array'))
      return
    }
    if (callbackAddress && !/^https?:\/\//.test(callbackAddress)) {
      toast.error(t('Provide a valid URL starting with http:// or https://'))
      return
    }
    if (values.XznPayEnabled) {
      if (!/^https?:\/\//.test(values.XznPayGatewayURL.trim())) {
        toast.error(t('Provide a valid URL starting with http:// or https://'))
        return
      }
      if (!values.XznPayPID.trim() || parsedMethods.length === 0) {
        toast.error(t('Merchant ID and payment methods are required'))
        return
      }
    }

    setLoading(true)
    try {
      const options: { key: string; value: string }[] = [
        { key: 'XznPayEnabled', value: String(values.XznPayEnabled) },
        {
          key: 'XznPayGatewayURL',
          value: values.XznPayGatewayURL.trim().replace(/\/$/, ''),
        },
        {
          key: 'XznPayCallbackAddress',
          value: callbackAddress.replace(/\/+$/, ''),
        },
        { key: 'XznPayPID', value: values.XznPayPID.trim() },
        { key: 'XznPaySignType', value: values.XznPaySignType },
        { key: 'XznPayPublicKey', value: values.XznPayPublicKey.trim() },
        { key: 'XznPayMinTopUp', value: String(values.XznPayMinTopUp || 1) },
        { key: 'XznPayMethods', value: JSON.stringify(parsedMethods) },
      ]
      if (values.XznPayMD5Key.trim()) {
        options.push({ key: 'XznPayMD5Key', value: values.XznPayMD5Key.trim() })
      }
      if (values.XznPayPrivateKey.trim()) {
        options.push({
          key: 'XznPayPrivateKey',
          value: values.XznPayPrivateKey.trim(),
        })
      }
      for (const option of options) {
        await updateOption.mutateAsync(option)
      }
      toast.success(t('Updated successfully'))
    } catch {
      toast.error(t('Update failed'))
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className='space-y-4 pt-4'>
      <SettingsPageActionsPortal>
        <Button type='button' size='sm' onClick={handleSave} disabled={loading}>
          {loading ? t('Saving...') : t('Save XznPay settings')}
        </Button>
      </SettingsPageActionsPortal>

      <div>
        <h3 className='text-lg font-medium'>{t('XznPay Gateway')}</h3>
        <p className='text-muted-foreground text-sm'>
          {t('Configure the independent XznPay wallet top-up integration.')}
        </p>
      </div>
      <Alert>
        <AlertDescription className='text-xs'>
          {t(
            'Webhook URL: <CallbackAddress>/api/xzn-pay/webhook. Only configured payment methods are exposed to users.'
          )}
        </AlertDescription>
      </Alert>

      <SettingsSwitchField
        checked={form.watch('XznPayEnabled')}
        onCheckedChange={(value) => form.setValue('XznPayEnabled', value)}
        label={t('Enable XznPay')}
        className='border-b-0 py-0'
      />

      <div className='grid gap-4 sm:grid-cols-2'>
        <div className='grid gap-1.5'>
          <Label>{t('Gateway URL')}</Label>
          <Input {...form.register('XznPayGatewayURL')} />
        </div>
        <div className='grid gap-1.5'>
          <Label>{t('Merchant ID')}</Label>
          <Input {...form.register('XznPayPID')} />
        </div>
        <div className='grid gap-1.5 sm:col-span-2'>
          <Label>{t('Callback address')}</Label>
          <Input
            type='url'
            placeholder='https://gateway.example.com'
            {...form.register('XznPayCallbackAddress')}
          />
          <p className='text-muted-foreground text-xs'>
            {t('Optional callback override. Leave blank to use server address')}
          </p>
        </div>
        <div className='grid gap-1.5'>
          <Label>{t('Signature type')}</Label>
          <Select
            value={signType}
            onValueChange={(value) =>
              form.setValue('XznPaySignType', value as 'MD5' | 'RSA')
            }
          >
            <SelectTrigger className='w-full'>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value='MD5'>MD5</SelectItem>
              <SelectItem value='RSA'>RSA</SelectItem>
            </SelectContent>
          </Select>
        </div>
        <div className='grid gap-1.5'>
          <Label>{t('Minimum top-up quantity')}</Label>
          <Input
            type='number'
            min={1}
            {...form.register('XznPayMinTopUp', { valueAsNumber: true })}
          />
        </div>
      </div>

      {signType === 'MD5' ? (
        <div className='grid gap-1.5'>
          <Label>{t('MD5 Secret')}</Label>
          <Input type='password' {...form.register('XznPayMD5Key')} />
        </div>
      ) : (
        <div className='grid gap-4 sm:grid-cols-2'>
          <div className='grid gap-1.5'>
            <Label>{t('RSA Private Key')}</Label>
            <Textarea
              rows={5}
              className='font-mono text-xs'
              {...form.register('XznPayPrivateKey')}
            />
          </div>
          <div className='grid gap-1.5'>
            <Label>{t('Platform Public Key')}</Label>
            <Textarea
              rows={5}
              className='font-mono text-xs'
              {...form.register('XznPayPublicKey')}
            />
          </div>
        </div>
      )}

      <div className='space-y-2'>
        <div className='flex flex-wrap gap-2'>
          {METHOD_TEMPLATES.map((template) => (
            <Button
              key={template.paytype_code}
              type='button'
              size='sm'
              variant='outline'
              onClick={() => addTemplate(template)}
            >
              {t(template.name)}
            </Button>
          ))}
        </div>
        <Label>{t('XznPay payment methods (JSON array)')}</Label>
        <Textarea
          rows={10}
          className='font-mono text-xs'
          value={methods}
          onChange={(event) => setMethods(event.target.value)}
          placeholder='[{"name":"Alipay","paytype_code":"alipay","channel_id":"","min_topup":1}]'
        />
      </div>
    </div>
  )
}

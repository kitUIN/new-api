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
import { useFieldArray, useForm, useWatch } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { RequestClientIcon } from '@/features/usage-logs/components/request-client-icon'
import type { RequestClientConfig } from '@/features/usage-logs/lib/request-client'
import {
  RequestClientRuleFields,
  type RequestClientsForm,
} from './request-client-rule-fields'

export function RequestClientEditor(props: {
  clients: RequestClientConfig[]
  onSave: (clients: RequestClientConfig[]) => Promise<void>
}) {
  const { t } = useTranslation()
  const [error, setError] = useState('')
  const form = useForm<RequestClientsForm>({
    defaultValues: { clients: props.clients },
  })
  const clients = useFieldArray({
    control: form.control,
    name: 'clients',
    keyName: 'fieldKey',
  })
  const values = useWatch({ control: form.control, name: 'clients' })
  const submit = form.handleSubmit(
    async (data) => {
      setError('')
      try {
        await props.onSave(data.clients)
        form.reset(data)
      } catch (err) {
        setError(
          err instanceof Error ? err.message : t('Failed to update setting')
        )
      }
    },
    () => setError(t('请填写所有必填字段'))
  )

  return (
    <form onSubmit={submit} className='space-y-4'>
      <p className='text-muted-foreground text-sm'>
        {t('按顺序匹配客户端，任意一条规则命中即识别。未识别时不显示图标。')}
      </p>
      <p className='text-muted-foreground text-xs'>
        {t(
          '正则使用 Go RE2 语法，(?i) 表示忽略大小写。图标可填写内置标识、站内路径或 HTTP(S) 图片地址。'
        )}
      </p>
      <div className='flex flex-wrap items-center gap-2'>
        <Button
          type='button'
          variant='outline'
          disabled={form.formState.isSubmitting || clients.fields.length >= 100}
          onClick={() =>
            clients.append({
              id: `client-${Date.now()}`,
              name: '',
              icon: 'codex',
              url: '',
              enabled: true,
              rules: [{ header: 'User-Agent', pattern: '' }],
            })
          }
        >
          {t('添加客户端')}
        </Button>
        <Button
          type='submit'
          disabled={form.formState.isSubmitting || !form.formState.isDirty}
        >
          {t(form.formState.isSubmitting ? 'Saving...' : 'Save Changes')}
        </Button>
        <Button
          type='button'
          variant='ghost'
          disabled={form.formState.isSubmitting || !form.formState.isDirty}
          onClick={() => {
            form.reset()
            setError('')
          }}
        >
          {t('Reset')}
        </Button>
        {form.formState.isDirty && (
          <span className='text-muted-foreground text-xs'>
            {t('Unsaved changes')}
          </span>
        )}
      </div>
      {error && (
        <p role='alert' className='text-destructive text-sm'>
          {error}
        </p>
      )}
      <fieldset disabled={form.formState.isSubmitting} className='space-y-3'>
        {clients.fields.map((field, index) => (
          <details
            key={field.fieldKey}
            className='bg-card rounded-lg border'
            open={index === 0 || !values[index]?.name || undefined}
          >
            <summary className='flex cursor-pointer items-center gap-3 p-4'>
              <span className='text-muted-foreground w-5 text-xs tabular-nums'>
                {index + 1}
              </span>
              <RequestClientIcon client={values[index]} />
              <span className='flex-1 text-sm font-medium'>
                {values[index]?.name || t('新客户端')}
              </span>
              <span className='text-muted-foreground text-xs'>
                {t(values[index]?.enabled ? 'Enabled' : 'Disabled')}
              </span>
            </summary>
            <div className='space-y-4 border-t p-4'>
              <div className='grid gap-4 sm:grid-cols-2'>
                <label className='space-y-1 text-sm'>
                  <span>{t('客户端标识')}</span>
                  <Input
                    {...form.register(`clients.${index}.id`, {
                      required: true,
                      pattern: /^[a-z0-9][a-z0-9_-]{0,63}$/,
                    })}
                    maxLength={64}
                  />
                </label>
                <label className='space-y-1 text-sm'>
                  <span>{t('显示名称')}</span>
                  <Input
                    {...form.register(`clients.${index}.name`, {
                      required: true,
                    })}
                    maxLength={128}
                  />
                </label>
                <label className='space-y-1 text-sm'>
                  <span>{t('显示图标')}</span>
                  <Input
                    {...form.register(`clients.${index}.icon`, {
                      required: true,
                    })}
                    list='request-client-icons'
                  />
                </label>
                <label className='space-y-1 text-sm'>
                  <span>{t('点击图标跳转链接')}</span>
                  <Input
                    {...form.register(`clients.${index}.url`)}
                    type='url'
                    placeholder='https://'
                  />
                </label>
              </div>
              <RequestClientRuleFields form={form} index={index} />
              <div className='flex flex-wrap items-center gap-2'>
                <label className='mr-auto flex items-center gap-2 text-sm'>
                  <input
                    type='checkbox'
                    {...form.register(`clients.${index}.enabled`)}
                  />
                  {t('Enabled')}
                </label>
                <Button
                  type='button'
                  size='sm'
                  variant='outline'
                  disabled={index === 0}
                  onClick={() => clients.move(index, index - 1)}
                >
                  {t('上移')}
                </Button>
                <Button
                  type='button'
                  size='sm'
                  variant='outline'
                  disabled={index === clients.fields.length - 1}
                  onClick={() => clients.move(index, index + 1)}
                >
                  {t('下移')}
                </Button>
                <Button
                  type='button'
                  size='sm'
                  variant='ghost'
                  onClick={() => clients.remove(index)}
                >
                  {t('删除客户端')}
                </Button>
              </div>
            </div>
          </details>
        ))}
      </fieldset>
      <datalist id='request-client-icons'>
        {[
          'codex',
          'codex-desktop',
          'codex-tui',
          'codex-vscode',
          'codex-tui-vscode',
          'claude-code',
          'pi',
          'opencode',
          'codebuddy',
          'deepseek-harness',
          '/images/clients/magpie.svg',
          '/images/clients/omp.svg',
          '/images/clients/cc-pocket.svg',
          '/images/clients/rikkahub.png',
        ].map((icon) => (
          <option key={icon} value={icon} />
        ))}
      </datalist>
    </form>
  )
}

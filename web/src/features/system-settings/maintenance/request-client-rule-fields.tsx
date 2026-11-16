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
import { useFieldArray, type UseFormReturn } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import type { RequestClientConfig } from '@/features/usage-logs/lib/request-client'

export type RequestClientsForm = { clients: RequestClientConfig[] }

export function RequestClientRuleFields(props: {
  form: UseFormReturn<RequestClientsForm>
  index: number
}) {
  const { t } = useTranslation()
  const rules = useFieldArray({
    control: props.form.control,
    name: `clients.${props.index}.rules`,
  })
  return (
    <div className='space-y-2'>
      <div className='flex items-center justify-between gap-2'>
        <span className='text-sm font-medium'>{t('识别规则（正则）')}</span>
        <Button
          type='button'
          size='sm'
          variant='outline'
          disabled={rules.fields.length >= 16}
          onClick={() => rules.append({ header: 'User-Agent', pattern: '' })}
        >
          {t('添加规则')}
        </Button>
      </div>
      {rules.fields.map((field, index) => (
        <div key={field.id} className='flex flex-wrap items-end gap-2'>
          <label className='min-w-32 flex-1 space-y-1 text-xs'>
            <span>{t('请求头')}</span>
            <Input
              {...props.form.register(
                `clients.${props.index}.rules.${index}.header`,
                { required: true }
              )}
              placeholder='User-Agent'
              maxLength={64}
            />
          </label>
          <label className='min-w-48 flex-[3] space-y-1 text-xs'>
            <span>{t('正则表达式')}</span>
            <Input
              {...props.form.register(
                `clients.${props.index}.rules.${index}.pattern`,
                { required: true }
              )}
              className='font-mono'
              placeholder='(?i)^my-client/'
              maxLength={1024}
            />
          </label>
          <Button
            type='button'
            size='sm'
            variant='ghost'
            disabled={rules.fields.length === 1}
            onClick={() => rules.remove(index)}
          >
            {t('删除规则')}
          </Button>
        </div>
      ))}
    </div>
  )
}

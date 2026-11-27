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
import { useTranslation } from 'react-i18next'
import { MultiSelect } from '@/components/multi-select'
import { Input } from '@/components/ui/input'

type Props = {
  group: string
  models: string[]
  value: string
  onChange: (value: string) => void
}

export function SubscriptionModelRatiosEditor(props: Props) {
  const { t } = useTranslation()
  let config: Record<string, Record<string, number | null>>
  try {
    config = JSON.parse(props.value || '{}')
    if (!config || typeof config !== 'object' || Array.isArray(config))
      throw new Error()
  } catch {
    return <p role='alert'>{t('subscriptionModelRatio.invalid')}</p>
  }
  const ratios = Object.hasOwn(config, props.group) ? config[props.group] : {}
  if (!ratios || typeof ratios !== 'object' || Array.isArray(ratios)) {
    return <p role='alert'>{t('subscriptionModelRatio.invalid')}</p>
  }
  const update = (next: Record<string, number | null>) => {
    const result = { ...config, [props.group]: next }
    if (Object.keys(next).length === 0) delete result[props.group]
    props.onChange(JSON.stringify(result))
  }
  return (
    <section className='space-y-3 rounded-lg border p-4'>
      <h4 className='text-sm font-medium'>
        {t('subscriptionModelRatio.title')}
      </h4>
      <p className='text-muted-foreground text-sm'>
        {t('subscriptionModelRatio.help')}
      </p>
      <MultiSelect
        options={props.models.map((model) => ({ label: model, value: model }))}
        selected={Object.keys(ratios)}
        placeholder={t('Select models')}
        disabled={!props.group}
        maxVisibleChips={2}
        allowCreate
        onChange={(models) =>
          update(
            Object.fromEntries(
              models.map((model) => [
                model,
                Object.hasOwn(ratios, model) ? ratios[model] : 1,
              ])
            )
          )
        }
      />
      {Object.entries(ratios).map(([model, ratio]) => (
        <label
          key={model}
          className='flex items-center justify-between gap-3 text-sm'
        >
          <span className='min-w-0 break-all font-mono'>{model}</span>
          <Input
            type='number'
            min={0}
            step='any'
            className='w-24 shrink-0'
            value={ratio ?? ''}
            onChange={(event) =>
              update({ ...ratios, [model]: event.target.valueAsNumber })
            }
          />
        </label>
      ))}
    </section>
  )
}

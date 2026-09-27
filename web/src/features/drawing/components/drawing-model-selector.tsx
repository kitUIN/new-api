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
import { Button } from '@/components/ui/button'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'

type DrawingModelSelectorProps = {
  models: string[]
  value: string
  loading: boolean
  error: boolean
  disabled: boolean
  onChange: (value: string) => void
  onReload: () => void
}

export function DrawingModelSelector(props: DrawingModelSelectorProps) {
  const { t } = useTranslation()
  if (props.error) {
    return (
      <Button
        onClick={props.onReload}
        size='sm'
        type='button'
        variant='outline'
      >
        {t('Failed to load drawing models. Retry')}
      </Button>
    )
  }

  return (
    <Select
      disabled={props.disabled || props.loading || props.models.length === 0}
      value={props.value || null}
      onValueChange={(value) => {
        if (value && props.models.includes(value)) props.onChange(value)
      }}
    >
      <SelectTrigger aria-label={t('Model')} className='max-w-56' size='sm'>
        <SelectValue>
          {props.loading
            ? t('Loading...')
            : props.value || t('No models available')}
        </SelectValue>
      </SelectTrigger>
      <SelectContent>
        <SelectGroup>
          {props.models.map((model) => (
            <SelectItem key={model} value={model}>
              {model}
            </SelectItem>
          ))}
        </SelectGroup>
      </SelectContent>
    </Select>
  )
}

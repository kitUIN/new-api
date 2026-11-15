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
import { Controller, useWatch, type UseFormReturn } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { Field, FieldError, FieldLabel } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import { DEFAULT_CONFIG } from '../constants'
import {
  parameterLimits,
  parameterSchema,
  type ParameterFormValues,
} from '../lib/parameters'
import type { ParameterEnabled } from '../types'

interface ParameterFieldProps {
  name: keyof ParameterEnabled
  label: string
  form: UseFormReturn<ParameterFormValues>
}

export function ParameterField(props: ParameterFieldProps) {
  const { t } = useTranslation()
  const enabled = useWatch({
    control: props.form.control,
    name: `enabled.${props.name}`,
  })
  const limits = parameterLimits[props.name]
  const inputId = `playground-${props.name}`

  return (
    <Controller
      control={props.form.control}
      name={`values.${props.name}`}
      render={({ field, fieldState }) => (
        <Field data-invalid={fieldState.invalid}>
          <div className='flex items-center justify-between gap-2'>
            <FieldLabel htmlFor={inputId}>{props.label}</FieldLabel>
            <Switch
              aria-label={t('playground.parameters.send', {
                parameter: props.label,
              })}
              checked={enabled}
              onCheckedChange={(checked) => {
                props.form.setValue(`enabled.${props.name}`, checked)
                if (
                  !checked &&
                  !parameterSchema.shape.values.shape[props.name].safeParse(
                    field.value
                  ).success
                ) {
                  props.form.setValue(
                    `values.${props.name}`,
                    DEFAULT_CONFIG[props.name] ?? 0,
                    { shouldValidate: true }
                  )
                }
              }}
            />
          </div>
          <Input
            {...field}
            {...limits}
            id={inputId}
            type='number'
            disabled={!enabled}
            aria-invalid={fieldState.invalid}
            aria-describedby={
              fieldState.invalid ? `${inputId}-error` : undefined
            }
            value={Number.isFinite(field.value) ? field.value : ''}
            onChange={(event) => field.onChange(event.target.valueAsNumber)}
          />
          {fieldState.invalid && (
            <FieldError id={`${inputId}-error`}>
              {t('playground.parameters.invalid', {
                min: limits.min,
                max: limits.max,
              })}
            </FieldError>
          )}
        </Field>
      )}
    />
  )
}

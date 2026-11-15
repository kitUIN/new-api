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
import { Controller, useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { SlidersHorizontal } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { Field, FieldGroup, FieldLabel } from '@/components/ui/field'
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
  SheetTrigger,
} from '@/components/ui/sheet'
import { Switch } from '@/components/ui/switch'
import { DEFAULT_CONFIG, DEFAULT_PARAMETER_ENABLED } from '../constants'
import { parameterSchema, type ParameterFormValues } from '../lib/parameters'
import type { ParameterEnabled, PlaygroundParameters } from '../types'
import { ParameterField } from './parameter-field'

interface ParameterSettingsProps {
  config: PlaygroundParameters
  enabled: ParameterEnabled
  disabled: boolean
  onApply: (values: PlaygroundParameters, enabled: ParameterEnabled) => void
}

export function ParameterSettings(props: ParameterSettingsProps) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  const form = useForm<ParameterFormValues>({
    resolver: zodResolver(parameterSchema),
    defaultValues: {
      values: { ...props.config, seed: props.config.seed ?? 0 },
      enabled: props.enabled,
    },
  })
  const fields = [
    { name: 'temperature', label: t('playground.parameters.temperature') },
    { name: 'top_p', label: t('playground.parameters.topP') },
    { name: 'max_tokens', label: t('playground.parameters.maxTokens') },
    {
      name: 'frequency_penalty',
      label: t('playground.parameters.frequencyPenalty'),
    },
    {
      name: 'presence_penalty',
      label: t('playground.parameters.presencePenalty'),
    },
    { name: 'seed', label: t('playground.parameters.seed') },
  ] as const

  return (
    <Sheet
      open={open}
      onOpenChange={(nextOpen) => {
        if (nextOpen) {
          form.reset({
            values: { ...props.config, seed: props.config.seed ?? 0 },
            enabled: props.enabled,
          })
        }
        setOpen(nextOpen)
      }}
    >
      <SheetTrigger
        render={
          <Button variant='outline' size='sm' disabled={props.disabled} />
        }
      >
        <SlidersHorizontal data-icon='inline-start' />
        {t('Parameters')}
      </SheetTrigger>
      <SheetContent className='w-full sm:max-w-md'>
        <SheetHeader>
          <SheetTitle>{t('playground.parameters.title')}</SheetTitle>
          <SheetDescription>
            {t('playground.parameters.description')}
          </SheetDescription>
        </SheetHeader>
        <form
          className='flex min-h-0 flex-1 flex-col'
          onSubmit={form.handleSubmit((data) => {
            props.onApply(data.values, data.enabled)
            setOpen(false)
          })}
          noValidate
        >
          <FieldGroup className='min-h-0 flex-1 overflow-y-auto px-4 pb-4'>
            {fields.map((field) => (
              <ParameterField
                key={field.name}
                name={field.name}
                label={field.label}
                form={form}
              />
            ))}
            <Controller
              control={form.control}
              name='values.stream'
              render={({ field }) => (
                <Field orientation='horizontal'>
                  <FieldLabel htmlFor='playground-stream'>
                    {t('Streaming')}
                  </FieldLabel>
                  <Switch
                    id='playground-stream'
                    checked={field.value}
                    onCheckedChange={field.onChange}
                  />
                </Field>
              )}
            />
          </FieldGroup>
          <SheetFooter>
            <Button type='submit' disabled={props.disabled}>
              {t('Apply')}
            </Button>
            <Button
              type='button'
              variant='outline'
              onClick={() =>
                form.reset({
                  values: { ...DEFAULT_CONFIG, seed: 0 },
                  enabled: DEFAULT_PARAMETER_ENABLED,
                })
              }
            >
              {t('Reset to default')}
            </Button>
          </SheetFooter>
        </form>
      </SheetContent>
    </Sheet>
  )
}

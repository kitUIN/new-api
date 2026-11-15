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
import { useId } from 'react'
import { Controller, type Control } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import {
  Field,
  FieldDescription,
  FieldError,
  FieldGroup,
  FieldLegend,
  FieldSet,
  FieldTitle,
} from '@/components/ui/field'
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'
import type { ChannelFormValues } from '../../../lib/channel-form'

const clients = [
  { name: 'codex_access', label: 'Codex' },
  { name: 'claude_code_access', label: 'Claude Code' },
] as const

export function ChannelClientAccessSection(props: {
  control: Control<ChannelFormValues>
}) {
  const { t } = useTranslation()
  const id = useId()

  return (
    <FieldSet>
      <FieldLegend>{t('channels.clientAccess.title')}</FieldLegend>
      <FieldDescription id={`${id}-description`}>
        {t('channels.clientAccess.description')}
      </FieldDescription>
      <FieldGroup>
        {clients.map((client) => (
          <Controller
            key={client.name}
            control={props.control}
            name={client.name}
            render={({ field, fieldState }) => (
              <Field orientation='responsive' data-invalid={fieldState.invalid}>
                <FieldTitle id={`${id}-${client.name}`}>
                  {client.label}
                </FieldTitle>
                <ToggleGroup
                  ref={field.ref}
                  variant='outline'
                  value={[field.value ?? 'unrestricted']}
                  onValueChange={(values) => {
                    if (values.length > 0) field.onChange(values[0])
                  }}
                  onBlur={field.onBlur}
                  aria-labelledby={`${id}-${client.name}`}
                  aria-describedby={`${id}-description`}
                  aria-invalid={fieldState.invalid}
                >
                  <ToggleGroupItem value='unrestricted'>
                    {t('channels.clientAccess.unrestricted')}
                  </ToggleGroupItem>
                  <ToggleGroupItem value='deny'>
                    {t('channels.clientAccess.deny')}
                  </ToggleGroupItem>
                  <ToggleGroupItem value='allow_only'>
                    {t('channels.clientAccess.allowOnly')}
                  </ToggleGroupItem>
                </ToggleGroup>
                {fieldState.error && <FieldError errors={[fieldState.error]} />}
              </Field>
            )}
          />
        ))}
      </FieldGroup>
      <FieldDescription>
        {t('channels.clientAccess.detection')}
      </FieldDescription>
    </FieldSet>
  )
}

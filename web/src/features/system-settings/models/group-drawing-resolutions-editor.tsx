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
import { Checkbox } from '@/components/ui/checkbox'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import {
  drawingResolutions as resolutions,
  drawingResolutionsConfigSchema as configSchema,
} from './group-drawing-resolutions'

type Props = {
  value: string
  group?: string
  onChange: (value: string) => void
}

export function GroupDrawingResolutionsEditor(props: Props) {
  const { t } = useTranslation()
  let config: Record<string, string[]> = {}
  try {
    config = configSchema.parse(JSON.parse(props.value || '{}'))
  } catch {
    if (props.group !== undefined)
      return <p role='alert'>{t('Invalid drawing resolutions')}</p>
  }
  const group = props.group
  const selected =
    group === undefined ? [] : (config[group] ?? [...resolutions])
  return (
    <section className='space-y-3 rounded-lg border p-4'>
      <Label>{t('Drawing resolutions')}</Label>
      <p className='text-muted-foreground text-sm'>
        {t(
          'Configure gpt-image as a combination group to route drawing requests to members supporting the selected resolution. Unconfigured groups support all resolutions; clearing all disables drawing for the group.'
        )}
      </p>
      {group === undefined ? (
        <Textarea
          aria-label={t('Drawing resolutions')}
          value={props.value}
          onChange={(event) => props.onChange(event.target.value)}
        />
      ) : (
        <div className='flex flex-wrap gap-4'>
          {resolutions.map((resolution) => (
            <label key={resolution} className='flex items-center gap-2 text-sm'>
              <Checkbox
                disabled={!group}
                checked={selected.includes(resolution)}
                onCheckedChange={(checked) => {
                  const next = resolutions.filter((item) =>
                    item === resolution ? checked : selected.includes(item)
                  )
                  props.onChange(JSON.stringify({ ...config, [group]: next }))
                }}
              />
              {resolution}
            </label>
          ))}
        </div>
      )}
    </section>
  )
}

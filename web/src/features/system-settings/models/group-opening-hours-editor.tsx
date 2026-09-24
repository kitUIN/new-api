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
import { useTranslation } from 'react-i18next'
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

type Interval = { start: string; end: string }
type Props = {
  value: string
  groups: string
  onChange: (value: string) => void
}

export function GroupOpeningHoursEditor(props: Props) {
  const { t } = useTranslation()
  const [selected, setSelected] = useState('')
  let schedules: Record<string, Interval[]> = {}
  let groups: string[] = []
  try {
    schedules = JSON.parse(props.value || '{}')
    groups = [
      ...new Set([
        ...Object.keys(JSON.parse(props.groups || '{}')),
        ...Object.keys(schedules),
      ]),
    ].sort()
  } catch {
    return <p role='alert'>{t('Invalid JSON')}</p>
  }
  const group = groups.includes(selected) ? selected : groups[0] || ''
  const intervals = schedules[group] || []
  const update = (next: Interval[]) => {
    const result = { ...schedules }
    if (next.length) result[group] = next
    else delete result[group]
    props.onChange(JSON.stringify(result))
  }

  return (
    <section className='space-y-3 rounded-lg border p-4'>
      <Label htmlFor='opening-hours-group'>{t('Group opening hours')}</Label>
      <p className='text-muted-foreground text-sm'>
        {t(
          'Daily in Beijing time (UTC+8). No intervals means always open. An end earlier than the start means the next day. Start is included; end is excluded.'
        )}
      </p>
      <Select value={group} onValueChange={(value) => setSelected(value || '')}>
        <SelectTrigger id='opening-hours-group' className='w-full sm:w-64'>
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {groups.map((name) => (
            <SelectItem key={name} value={name}>
              {name}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
      {intervals.length === 0 && (
        <p className='text-muted-foreground text-sm'>{t('Always open')}</p>
      )}
      {intervals.map((interval, index) => (
        <div key={index} className='flex flex-wrap items-center gap-2'>
          <Input
            aria-label={t('Opening start time')}
            type='time'
            className='w-36'
            value={interval.start}
            onChange={(event) =>
              update(
                intervals.map((item, i) =>
                  i === index ? { ...item, start: event.target.value } : item
                )
              )
            }
          />
          <span>–</span>
          <Input
            aria-label={t('Opening end time')}
            type='text'
            placeholder='HH:mm'
            className='w-36'
            value={interval.end}
            onChange={(event) =>
              update(
                intervals.map((item, i) =>
                  i === index ? { ...item, end: event.target.value } : item
                )
              )
            }
          />
          <Button
            type='button'
            variant='outline'
            onClick={() => update(intervals.filter((_, i) => i !== index))}
          >
            {t('Remove')}
          </Button>
        </div>
      ))}
      <p className='text-muted-foreground text-sm'>
        {t(
          'End time accepts 24:00 for midnight. Requests outside these intervals return a group_not_open error with the opening hours.'
        )}
      </p>
      <Button
        type='button'
        variant='outline'
        disabled={!group}
        onClick={() => update([...intervals, { start: '09:00', end: '18:00' }])}
      >
        {t('Add opening interval')}
      </Button>
    </section>
  )
}

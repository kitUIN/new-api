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
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import type { TimedRatioRule } from './group-timed-ratios'

type Props = { value: string; group: string; onChange: (value: string) => void }

export function GroupTimedRatiosEditor(props: Props) {
  const { t } = useTranslation()
  let schedules: Record<string, TimedRatioRule[]>
  try {
    schedules = JSON.parse(props.value || '{}')
    if (!schedules || Array.isArray(schedules) || typeof schedules !== 'object')
      throw new Error()
    if (schedules[props.group] && !Array.isArray(schedules[props.group]))
      throw new Error()
  } catch {
    return <p role='alert'>{t('Invalid JSON')}</p>
  }
  const rules = schedules[props.group] || []
  const update = (next: TimedRatioRule[]) => {
    const result = { ...schedules }
    if (next.length) result[props.group] = next
    else delete result[props.group]
    props.onChange(JSON.stringify(result))
  }
  const patch = (index: number, value: Partial<TimedRatioRule>) =>
    update(rules.map((rule, i) => (i === index ? { ...rule, ...value } : rule)))
  const kinds = [
    { value: 'once', label: t('timedRatio.once') },
    { value: 'hourly', label: t('timedRatio.hourly') },
    { value: 'daily', label: t('timedRatio.daily') },
    { value: 'weekly', label: t('timedRatio.weekly') },
  ]
  const days = [
    t('Sunday'),
    t('Monday'),
    t('Tuesday'),
    t('Wednesday'),
    t('Thursday'),
    t('Friday'),
    t('Saturday'),
  ]
  return (
    <section className='space-y-3 rounded-lg border p-4'>
      <h3 className='text-sm font-medium'>{t('timedRatio.title')}</h3>
      <p className='text-muted-foreground text-sm'>{t('timedRatio.help')}</p>
      {rules.map((rule, index) => (
        <div key={index} className='space-y-3 rounded-md border p-3'>
          <div className='flex flex-wrap items-center gap-3'>
            <span className='text-muted-foreground text-sm'>#{index + 1}</span>
            <Switch
              checked={rule.enabled}
              aria-label={t('Enabled')}
              onCheckedChange={(enabled) => patch(index, { enabled })}
            />
            <Select
              items={kinds}
              value={rule.kind}
              onValueChange={(value) => {
                const kind = value as TimedRatioRule['kind']
                if (!kind) return
                let start = '09:00',
                  end = '18:00'
                if (kind === 'hourly') {
                  start = '00:00'
                  end = '00:30'
                }
                if (kind === 'once') {
                  const date = new Date(Date.now() + 8 * 3600000)
                    .toISOString()
                    .slice(0, 10)
                  start = `${date}T09:00`
                  end = `${date}T18:00`
                }
                patch(index, { kind, start, end, weekdays: [1, 2, 3, 4, 5] })
              }}
            >
              <SelectTrigger
                aria-label={t('timedRatio.schedule')}
                className='w-36'
              >
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {kinds.map((kind) => (
                  <SelectItem key={kind.value} value={kind.value}>
                    {kind.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            <label className='flex items-center gap-2 text-sm'>
              {t('timedRatio.title')}
              <Input
                className='w-28'
                type='number'
                min={0}
                step='any'
                value={rule.ratio ?? ''}
                onChange={(event) =>
                  patch(index, {
                    ratio:
                      event.target.value === ''
                        ? NaN
                        : Number(event.target.value),
                  })
                }
              />
            </label>
          </div>
          <div className='grid gap-3 sm:grid-cols-2'>
            {(['start', 'end'] as const).map((field) => (
              <label key={field} className='space-y-1 text-sm'>
                <span>
                  {field === 'start'
                    ? t('timedRatio.start')
                    : t('timedRatio.end')}
                </span>
                <Input
                  type={rule.kind === 'once' ? 'datetime-local' : 'text'}
                  placeholder='HH:mm'
                  value={rule[field]}
                  onChange={(event) =>
                    patch(index, { [field]: event.target.value })
                  }
                />
              </label>
            ))}
          </div>
          {rule.kind === 'hourly' && (
            <p className='text-muted-foreground text-xs'>
              {t('timedRatio.hourlyHelp')}
            </p>
          )}
          {rule.kind === 'weekly' && (
            <div className='flex flex-wrap gap-3'>
              {days.map((day, value) => (
                <label key={value} className='flex items-center gap-1 text-sm'>
                  <Checkbox
                    checked={rule.weekdays?.includes(value) ?? false}
                    onCheckedChange={(checked) =>
                      patch(index, {
                        weekdays: checked
                          ? [...(rule.weekdays || []), value]
                          : (rule.weekdays || []).filter(
                              (item) => item !== value
                            ),
                      })
                    }
                  />
                  {day}
                </label>
              ))}
            </div>
          )}
          <label className='block space-y-1 text-sm'>
            <span>{t('timedRatio.remark')}</span>
            <Input
              value={rule.remark}
              onChange={(event) => patch(index, { remark: event.target.value })}
            />
          </label>
          <div className='flex gap-2'>
            <Button
              type='button'
              variant='outline'
              size='sm'
              disabled={index === 0}
              onClick={() => {
                const next = [...rules]
                ;[next[index - 1], next[index]] = [next[index], next[index - 1]]
                update(next)
              }}
            >
              {t('timedRatio.moveUp')}
            </Button>
            <Button
              type='button'
              variant='outline'
              size='sm'
              onClick={() => update(rules.filter((_, i) => i !== index))}
            >
              {t('Remove')}
            </Button>
          </div>
        </div>
      ))}
      <Button
        type='button'
        variant='outline'
        disabled={!props.group}
        onClick={() =>
          update([
            ...rules,
            {
              enabled: true,
              ratio: 1,
              kind: 'daily',
              start: '09:00',
              end: '18:00',
              remark: '',
            },
          ])
        }
      >
        {t('timedRatio.add')}
      </Button>
    </section>
  )
}

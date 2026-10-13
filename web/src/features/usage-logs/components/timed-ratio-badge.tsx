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
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import type { LogOtherData } from '../types'

export function TimedRatioBadge(props: {
  snapshot: NonNullable<LogOtherData['timed_group_ratio']>
}) {
  const { t } = useTranslation()
  const snapshot = props.snapshot
  const formatTime = (value: number) =>
    new Date(value * 1000).toLocaleString('sv-SE', {
      timeZone: 'Asia/Shanghai',
    })
  return (
    <TooltipProvider delay={200}>
      <Tooltip>
        <TooltipTrigger
          className='shrink-0 cursor-help font-medium text-orange-600 dark:text-orange-400'
          aria-label={t('timedRatio.title')}
        >
          {snapshot.ratio}x
        </TooltipTrigger>
        <TooltipContent className='block max-w-sm space-y-1'>
          <p>
            {t('timedRatio.original')}: <s>{snapshot.original_ratio}x</s>
          </p>
          <p className='text-orange-400'>
            {t('timedRatio.title')}: {snapshot.ratio}x
          </p>
          <p>
            {t('timedRatio.window')}: {formatTime(snapshot.start)} –{' '}
            {formatTime(snapshot.end)} (UTC+8)
          </p>
          <p>
            {t('timedRatio.duration', {
              minutes: (snapshot.end - snapshot.start) / 60,
            })}
          </p>
          <p className='break-words whitespace-pre-wrap'>
            {t('timedRatio.remark')}: {snapshot.remark || '—'}
          </p>
        </TooltipContent>
      </Tooltip>
    </TooltipProvider>
  )
}

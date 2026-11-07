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
import { ArrowDown02Icon, ArrowUp02Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useTranslation } from 'react-i18next'
import { cn } from '@/lib/utils'
import { ProbeHint } from './hint'
import { formatRate } from './lib'

export function TrafficRate(props: {
  direction: 'download' | 'upload'
  bitsPerSecond: number | null | undefined
  sampleLabel: string
}) {
  const { t } = useTranslation()
  const download = props.direction === 'download'
  const label = download ? t('serviceProbe.download') : t('serviceProbe.upload')
  const [value, unit] = formatRate(props.bitsPerSecond).split(' ')
  return (
    <ProbeHint
      label={`${label} · ${props.sampleLabel}`}
      className='flex min-w-0 items-center gap-2'
    >
      <span
        className={cn(
          'flex size-7 shrink-0 items-center justify-center rounded-full',
          download ? 'bg-success/10 text-success' : 'bg-info/10 text-info'
        )}
      >
        <HugeiconsIcon
          icon={download ? ArrowDown02Icon : ArrowUp02Icon}
          strokeWidth={2}
          className='size-4'
          aria-hidden='true'
        />
      </span>
      <span className='flex min-w-0 flex-wrap items-baseline gap-x-1 gap-y-0.5 tabular-nums'>
        <span className='text-xl leading-tight font-semibold tracking-tight @min-[440px]:text-2xl'>
          {value}
        </span>
        {unit && (
          <span className='text-muted-foreground text-[11px]'>{unit}</span>
        )}
      </span>
    </ProbeHint>
  )
}

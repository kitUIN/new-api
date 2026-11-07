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
import { useEffect, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { cn } from '@/lib/utils'
import {
  Card,
  CardAction,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Separator } from '@/components/ui/separator'
import { getCurrent } from './api'
import { ProbeHint } from './hint'
import { IpAddress } from './ip-address'
import { formatTime } from './lib'
import { OnlineTimeline } from './online-timeline'
import { RegionIcon } from './region-icon'
import { TrafficRate } from './traffic-rate'
import { TrafficSummary } from './traffic-summary'
import type { BeeQuota, ProbeWindow } from './types'

export function BeeCard(props: {
  bee: BeeQuota
  window: ProbeWindow
  userId: number
  stale: boolean
}) {
  const { t, i18n } = useTranslation()
  const bee = props.bee
  const [now, setNow] = useState(() => Date.now())
  useEffect(() => {
    const timer = globalThis.setInterval(() => setNow(Date.now()), 5_000)
    return () => globalThis.clearInterval(timer)
  }, [])
  const current = useQuery({
    queryKey: ['service-probes', props.userId, bee.id, 'current'],
    queryFn: ({ signal }) => getCurrent(bee.id, signal),
    enabled: bee.online && !props.stale,
    refetchInterval: 10_000,
    retry: false,
    staleTime: 0,
    gcTime: 0,
  })
  const sample =
    bee.online &&
    !props.stale &&
    !current.isError &&
    current.data?.available &&
    now - current.dataUpdatedAt < 20_000
      ? current.data
      : null
  let status = t('serviceProbe.offline')
  if (bee.online) status = t('serviceProbe.online')
  if (props.stale) status = t('serviceProbe.unknown')
  const sampleLabel = sample
    ? t('serviceProbe.sampledAt', {
        time: formatTime(sample.sampledAt, i18n.language),
      })
    : t('serviceProbe.liveUnavailable')
  return (
    <Card className='@container min-w-0 gap-3'>
      <CardHeader className='flex flex-row items-center gap-3'>
        <div className='bg-muted/40 flex size-9 shrink-0 items-center justify-center rounded-lg p-1'>
          <RegionIcon region={bee.region} />
        </div>
        <Separator orientation='vertical' className='my-1' />
        <div className='flex min-w-0 flex-1 flex-col gap-0.5'>
          <CardTitle className='min-w-0'>
            <ProbeHint
              label={`${bee.name || bee.id} · ${bee.id}`}
              className='block truncate'
            >
              {bee.name || bee.id}
            </ProbeHint>
          </CardTitle>
          <CardDescription className='flex min-w-0 items-center gap-1.5 text-xs'>
            <ProbeHint
              label={bee.region || t('serviceProbe.regionUnknown')}
              className='max-w-1/3 truncate'
            >
              {bee.region || '—'}
            </ProbeHint>
            <span aria-hidden='true'>·</span>
            <IpAddress
              key={`${props.userId}:${bee.id}:${bee.ip}`}
              ip={bee.ip}
            />
          </CardDescription>
        </div>
        <CardAction className='shrink-0 self-center'>
          <ProbeHint
            label={`${status} · ${t('serviceProbe.lastSeen', { time: formatTime(bee.lastReceivedAt, i18n.language) })}`}
            className={cn(
              'relative flex size-6 items-center justify-center',
              'text-destructive',
              bee.online && 'text-success',
              props.stale && 'text-muted-foreground'
            )}
          >
            {!props.stale && (
              <>
                <span
                  aria-hidden='true'
                  className='absolute size-2.5 animate-ping rounded-full border border-current opacity-50 [animation-duration:2s] motion-reduce:hidden'
                />
                <span
                  aria-hidden='true'
                  className='absolute size-2.5 animate-ping rounded-full border border-current opacity-50 [animation-delay:-1s] [animation-duration:2s] motion-reduce:hidden'
                />
              </>
            )}
            <span
              aria-hidden='true'
              className='relative size-2 rounded-full bg-current'
            />
          </ProbeHint>
        </CardAction>
      </CardHeader>
      <CardContent className='flex flex-col gap-3'>
        <Separator className='opacity-60' />
        <div className='grid grid-cols-[minmax(0,1fr)_auto_minmax(0,1fr)] items-center gap-3'>
          <TrafficRate
            direction='download'
            bitsPerSecond={sample?.downloadBitsPerSecond}
            sampleLabel={sampleLabel}
          />
          <Separator orientation='vertical' className='my-1 opacity-60' />
          <TrafficRate
            direction='upload'
            bitsPerSecond={sample?.uploadBitsPerSecond}
            sampleLabel={sampleLabel}
          />
        </div>
        <TrafficSummary bee={bee} />
        <OnlineTimeline
          id={bee.id}
          window={props.window}
          userId={props.userId}
          enabled={!props.stale}
        />
      </CardContent>
    </Card>
  )
}

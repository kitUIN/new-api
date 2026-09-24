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
import { Timer } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import dayjs from '@/lib/dayjs'
import type { LogOtherData } from '../../types'

function duration(value: unknown): number | null {
  if (typeof value !== 'number' || !Number.isFinite(value) || value < 0)
    return null
  return value
}

function timestamp(value: string): string {
  const normalized = value.replace(/\.(\d{3})\d*(Z|[+-]\d{2}:\d{2})$/, '.$1$2')
  const parsed = dayjs(normalized)
  return parsed.isValid() ? parsed.format('YYYY-MM-DD HH:mm:ss.SSS') : value
}

export function RelayTimingSection(props: { other: LogOtherData | null }) {
  const { t } = useTranslation()
  const other = props.other
  if (!other) return null

  const received = duration(other.request_body_receive_ms)
  const prepared = duration(other.upstream_prepare_ms)
  const prep = duration(other.pre_upstream_ms)
  const upstreamOffset =
    prep ??
    (received !== null && prepared !== null ? received + prepared : null)
  const split = received !== null && prepared !== null
  const preparationRows = []
  if (split) {
    preparationRows.push(
      {
        label: t('Receive Full Request Body'),
        value: received,
        start: 0,
        color: 'bg-sky-500',
      },
      {
        label: t('Prepare Upstream Request'),
        value: prepared,
        start: received,
        color: 'bg-violet-500',
      }
    )
  } else {
    preparationRows.push({
      label: t('New API Prep'),
      value: prep,
      start: 0,
      color: 'bg-violet-500',
    })
    if (received !== null) {
      preparationRows.push({
        label: t('Receive Full Request Body'),
        value: received,
        start: 0,
        color: 'bg-sky-500',
      })
    }
  }
  const rows = [
    ...preparationRows,
    {
      label: t('Upstream Headers'),
      value: duration(other.upstream_header_ms),
      start: upstreamOffset,
      color: 'bg-amber-500',
    },
    {
      label: t('Upstream Total'),
      value: duration(other.upstream_total_ms),
      start: upstreamOffset,
      color: 'bg-emerald-500',
    },
    {
      label: t('First Response'),
      value: duration(other.first_response_ms),
      start: 0,
      color: 'bg-cyan-500',
    },
    {
      label: t('Upstream to First Response'),
      value: duration(other.upstream_to_first_response_ms),
      start: upstreamOffset,
      color: 'bg-teal-500',
    },
    {
      label: t('Total'),
      value: duration(other.total_ms),
      start: 0,
      color: 'bg-primary',
    },
  ].filter((row) => row.value !== null)
  const timestamps = [
    { label: t('Request Received'), value: other.request_start_at },
    {
      label: t('Request Body Received'),
      value: other.request_body_received_at,
    },
    { label: t('Upstream Started'), value: other.upstream_request_start_at },
    { label: t('Upstream Headers'), value: other.upstream_response_header_at },
    { label: t('Upstream Ended'), value: other.upstream_request_end_at },
  ].filter((row) => !!row.value)
  if (!rows.length && !timestamps.length) return null
  const scale = Math.max(
    1,
    ...rows.map((row) => (row.start ?? 0) + (row.value ?? 0))
  )

  return (
    <section className='min-w-0 space-y-1.5' aria-label={t('Relay Timing')}>
      <h3 className='flex items-center gap-1.5 text-xs font-semibold'>
        <Timer className='size-3.5' aria-hidden='true' />
        {t('Relay Timing')}
      </h3>
      <div className='bg-muted/30 space-y-3 rounded-md border p-3'>
        {rows.length > 0 && (
          <>
            <p className='text-muted-foreground text-xs'>
              {t('Relay Timing Chart Hint')}
            </p>
            <div
              className='text-muted-foreground flex justify-between font-mono text-xs'
              aria-hidden='true'
            >
              <span>0ms</span>
              <span>{scale}ms</span>
            </div>
            <dl className='space-y-3'>
              {rows.map((row) => (
                <div key={row.label} className='space-y-1'>
                  <div className='flex items-baseline justify-between gap-3 text-xs'>
                    <dt>{row.label}</dt>
                    <dd className='shrink-0 font-mono tabular-nums'>
                      {row.value}ms
                    </dd>
                  </div>
                  {row.start !== null && (
                    <div
                      className='bg-muted relative h-2 overflow-hidden rounded-full'
                      aria-hidden='true'
                    >
                      <div
                        className={`absolute h-full rounded-full ${row.color}`}
                        style={{
                          left: `${(row.start / scale) * 100}%`,
                          width: `${((row.value ?? 0) / scale) * 100}%`,
                          minWidth: row.value === 0 ? 0 : 2,
                        }}
                      />
                    </div>
                  )}
                </div>
              ))}
            </dl>
          </>
        )}
        {timestamps.length > 0 && (
          <details className='border-t pt-2'>
            <summary className='text-muted-foreground cursor-pointer rounded-sm text-xs focus-visible:outline-2'>
              {t('Timing Timestamps')}
            </summary>
            <dl className='mt-2 space-y-2 text-xs'>
              {timestamps.map((row) => (
                <div
                  key={row.label}
                  className='flex flex-wrap justify-between gap-x-3 gap-y-1'
                >
                  <dt className='text-muted-foreground'>{row.label}</dt>
                  <dd className='font-mono break-all'>
                    {timestamp(row.value!)}
                  </dd>
                </div>
              ))}
            </dl>
          </details>
        )}
      </div>
    </section>
  )
}

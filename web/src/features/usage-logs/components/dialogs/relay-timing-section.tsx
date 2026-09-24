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
import { Timer } from 'lucide-react'
import { motion, useReducedMotion } from 'motion/react'
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

function addDuration(start: number | null, elapsed: unknown): number | null {
  const value = duration(elapsed)
  return start !== null && value !== null ? start + value : null
}

export function RelayTimingSection(props: { other: LogOtherData | null }) {
  const { t } = useTranslation()
  const reduceMotion = useReducedMotion()
  const [hoveredSegment, setHoveredSegment] = useState<number | null>(null)
  const other = props.other
  if (!other) return null

  // Prefer measured durations; timestamps also support older logs.
  function offset(value: number | null, at?: string): number | null {
    if (value !== null) return value
    if (!at || !other?.request_start_at) return null
    return duration(dayjs(at).diff(dayjs(other.request_start_at)))
  }

  const upstreamOffset = offset(
    duration(other.pre_upstream_ms) ??
      addDuration(
        duration(other.request_body_receive_ms),
        other.upstream_prepare_ms
      ),
    other.upstream_request_start_at
  )
  const legacyTiming = (other.relay_timing_version ?? 1) < 2
  const events = [
    { label: t('收到请求'), value: 0, at: other.request_start_at },
    {
      label: t('开始请求上游'),
      value: upstreamOffset,
      at: other.upstream_request_start_at,
    },
    {
      label: t('上游返回首字'),
      value: offset(
        addDuration(upstreamOffset, other.upstream_first_byte_ms),
        other.upstream_first_byte_at
      ),
      at: other.upstream_first_byte_at,
    },
    {
      label: t('上游结束返回'),
      value: legacyTiming
        ? null
        : offset(
            addDuration(upstreamOffset, other.upstream_total_ms),
            other.upstream_request_end_at
          ),
      at: legacyTiming ? undefined : other.upstream_request_end_at,
    },
    {
      label: t('返回给用户首字'),
      value:
        duration(other.first_response_ms) ??
        addDuration(upstreamOffset, other.upstream_to_first_response_ms),
    },
    { label: t('请求结束'), value: duration(other.total_ms) },
  ]
  // SSE forwards data while the upstream is still streaming. The downstream
  // first response therefore usually precedes the upstream's final byte.
  if (
    events[3].value !== null &&
    events[4].value !== null &&
    events[4].value < events[3].value
  ) {
    ;[events[3], events[4]] = [events[4], events[3]]
  }
  if (
    !other.request_start_at &&
    !events.slice(1).some((event) => event.value !== null || event.at)
  ) {
    return null
  }

  const requestStart = dayjs(other.request_start_at ?? '')
  const timestamps = events.map((event) => {
    let value = event.at
    if (!value && event.value !== null && requestStart.isValid()) {
      value = requestStart.add(event.value, 'millisecond').toISOString()
    }
    return { label: event.label, value }
  })
  const timelineEnd = Math.max(...events.map((event) => event.value ?? 0))
  const segments = events.slice(1).map((event, index) => {
    const start = events[index]
    // Missing or reversed intervals are unknown, not zero-duration stages.
    const elapsed =
      start.value !== null && event.value !== null
        ? duration(event.value - start.value)
        : null
    return {
      label:
        index === 0 ? t('请求上游前准备') : `${start.label} → ${event.label}`,
      start: start.value,
      end: event.value,
      elapsed,
      display: elapsed === null ? '—' : `${elapsed}ms`,
      color: `var(--chart-${index + 1})`,
    }
  })

  return (
    <section
      className='flex min-w-0 flex-col gap-1.5'
      aria-label={t('Relay Timing')}
    >
      <h3 className='flex items-center gap-1.5 text-xs font-semibold'>
        <Timer className='size-3.5' aria-hidden='true' />
        {t('Relay Timing')}
      </h3>
      <div className='bg-muted/30 flex flex-col gap-3 rounded-md border p-3'>
        {segments.length > 0 && (
          <>
            <p className='text-muted-foreground text-xs'>
              {t('Relay Timing Timeline Hint')}
            </p>
            {legacyTiming && (
              <p className='text-muted-foreground text-xs'>
                {t(
                  '旧日志未准确记录上游首字和结束时间，相关耗时无法恢复。请查看更新后产生的新日志。'
                )}
              </p>
            )}
            <div aria-hidden='true'>
              <div
                key={other.request_start_at}
                className='bg-muted relative h-8 overflow-hidden rounded-md'
              >
                {segments.map((segment, index) => (
                  <div
                    key={segment.label}
                    className='absolute top-0 h-full overflow-hidden transition-opacity duration-150 motion-reduce:transition-none'
                    title={`${segment.label}: ${segment.display}`}
                    onMouseEnter={() => setHoveredSegment(index)}
                    onMouseLeave={() => setHoveredSegment(null)}
                    style={{
                      left: `${timelineEnd > 0 ? ((segment.start ?? 0) / timelineEnd) * 100 : 0}%`,
                      width: `${timelineEnd > 0 ? ((segment.elapsed ?? 0) / timelineEnd) * 100 : 0}%`,
                      opacity:
                        hoveredSegment === null || hoveredSegment === index
                          ? 1
                          : 0.2,
                    }}
                  >
                    <motion.div
                      className='h-full origin-left'
                      initial={reduceMotion ? false : { scaleX: 0 }}
                      animate={{
                        scaleX: 1,
                      }}
                      transition={{
                        scaleX: {
                          duration: reduceMotion ? 0 : 0.32,
                          delay: reduceMotion ? 0 : index * 0.045,
                          ease: [0.22, 1, 0.36, 1],
                        },
                      }}
                      style={{ backgroundColor: segment.color }}
                    />
                  </div>
                ))}
              </div>
              <div className='text-muted-foreground mt-1 flex justify-between font-mono text-xs tabular-nums'>
                <span>0ms</span>
                <span>{timelineEnd}ms</span>
              </div>
            </div>
            <ol className='grid list-none grid-cols-1 gap-x-6 gap-y-2 text-xs sm:grid-cols-2'>
              {segments.map((segment, index) => (
                <motion.li
                  key={segment.label}
                  tabIndex={0}
                  aria-label={`${segment.label}: ${segment.display}`}
                  className='focus-visible:ring-ring flex min-w-0 items-baseline gap-2 rounded-sm outline-none focus-visible:ring-2'
                  onMouseEnter={() => setHoveredSegment(index)}
                  onMouseLeave={() => setHoveredSegment(null)}
                  initial={reduceMotion ? false : { y: 4 }}
                  animate={{
                    opacity:
                      hoveredSegment === null || hoveredSegment === index
                        ? 1
                        : 0.4,
                    y: 0,
                  }}
                  transition={{
                    y: {
                      duration: reduceMotion ? 0 : 0.24,
                      delay: reduceMotion ? 0 : index * 0.045,
                    },
                    opacity: { duration: reduceMotion ? 0 : 0.16 },
                  }}
                >
                  <span
                    className='size-2 shrink-0 rounded-sm'
                    style={{ backgroundColor: segment.color }}
                    aria-hidden='true'
                  />
                  <span className='min-w-0 flex-1 break-words'>
                    {index + 1}. {segment.label}
                  </span>
                  <span className='shrink-0 font-mono tabular-nums'>
                    {segment.display}
                  </span>
                </motion.li>
              ))}
            </ol>
          </>
        )}
        {timestamps.length > 0 && (
          <details className='border-t pt-2'>
            <summary className='text-muted-foreground cursor-pointer rounded-sm text-xs focus-visible:outline-2'>
              {t('Timing Timestamps')}
            </summary>
            <dl className='mt-2 flex flex-col gap-2 text-xs'>
              {timestamps.map((row) => (
                <div
                  key={row.label}
                  className='flex flex-wrap justify-between gap-x-3 gap-y-1'
                >
                  <dt className='text-muted-foreground'>{row.label}</dt>
                  <dd className='font-mono break-all'>
                    {row.value ? timestamp(row.value) : '—'}
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

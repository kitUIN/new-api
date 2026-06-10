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
import { Gauge, Layers3 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { formatTokens } from '../lib/format'
import type { GroupRanking } from '../types'

type GroupsRankingSectionProps = {
  rows: GroupRanking[]
}

export function GroupsRankingSection(props: GroupsRankingSectionProps) {
  const { t } = useTranslation()

  return (
    <section className='bg-card overflow-hidden rounded-lg border'>
      <header className='border-b px-5 py-4'>
        <h2 className='text-foreground inline-flex items-center gap-2 text-base font-semibold'>
          <Layers3 className='text-primary size-4' />
          {t('Group Ranking')}
        </h2>
        <p className='text-muted-foreground mt-1 text-sm'>
          {t('Groups ranked by token usage')}
        </p>
      </header>

      {props.rows.length === 0 ? (
        <div className='text-muted-foreground/80 px-5 py-8 text-center text-sm'>
          {t('No group ranking data available')}
        </div>
      ) : (
        <div className='divide-border divide-y'>
          {props.rows.map((row) => (
            <GroupRankingRow key={row.group} row={row} />
          ))}
        </div>
      )}
    </section>
  )
}

function GroupRankingRow(props: { row: GroupRanking }) {
  const { t } = useTranslation()
  const groupInitial = getGroupInitial(props.row.group)

  return (
    <div className='grid grid-cols-[auto_auto_minmax(0,1fr)] items-center gap-3 px-5 py-4 lg:grid-cols-[auto_auto_minmax(0,1fr)_repeat(5,minmax(88px,auto))]'>
      <span className='text-muted-foreground/80 w-8 shrink-0 text-right font-mono text-xs tabular-nums'>
        {props.row.rank}.
      </span>
      <span className='bg-primary/10 text-primary inline-flex size-9 shrink-0 items-center justify-center rounded-full font-mono text-sm font-semibold'>
        {groupInitial}
      </span>
      <div className='min-w-0'>
        <div className='text-foreground truncate font-mono text-sm font-semibold'>
          {props.row.group}
        </div>
        <div className='text-muted-foreground/80 mt-0.5 flex items-center gap-1 text-xs'>
          <Gauge className='size-3' />
          {t('Group multiplier')} {formatRatio(props.row.ratio)}
        </div>
      </div>
      <Metric
        label={t('Tokens')}
        value={formatTokens(props.row.total_tokens)}
      />
      <Metric
        label={t('Success rate')}
        value={formatPercent(props.row.success_rate)}
      />
      <Metric label={t('TTFT')} value={formatLatency(props.row.avg_ttft_ms)} />
      <Metric
        label={t('Avg Latency')}
        value={formatLatency(props.row.avg_latency_ms)}
      />
      <Metric label={t('Avg Speed')} value={formatTps(props.row.avg_tps)} />
    </div>
  )
}

function Metric(props: { label: string; value: string }) {
  return (
    <div className='col-span-3 flex items-center justify-between gap-3 lg:col-span-1 lg:block lg:text-right'>
      <div className='text-muted-foreground/80 text-[11px] font-medium tracking-widest uppercase'>
        {props.label}
      </div>
      <div className='text-foreground mt-0.5 font-mono text-sm font-semibold tabular-nums'>
        {props.value}
      </div>
    </div>
  )
}

function formatPercent(value: number): string {
  if (!Number.isFinite(value) || value <= 0) return '--'
  return `${value.toFixed(value >= 99.95 ? 0 : 1)}%`
}

function formatLatency(value: number): string {
  if (!Number.isFinite(value) || value <= 0) return '--'
  return `${Math.round(value).toLocaleString()} ms`
}

function formatTps(value: number): string {
  if (!Number.isFinite(value) || value <= 0) return '--'
  return `${value.toFixed(value >= 100 ? 0 : 1)} token/s`
}

function formatRatio(value: number): string {
  if (!Number.isFinite(value) || value <= 0) return '1x'
  return `${Number(value.toFixed(3)).toString()}x`
}

function getGroupInitial(group: string): string {
  const trimmed = group.trim()
  if (!trimmed) return 'D'
  const first = Array.from(trimmed)[0] ?? 'D'
  return /^[a-z]$/i.test(first) ? first.toUpperCase() : first
}

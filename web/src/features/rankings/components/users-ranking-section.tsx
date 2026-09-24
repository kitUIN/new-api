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
import { UserRound } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { getUserAvatarFallback, getUserAvatarStyle } from '@/lib/avatar'
import { formatQuota } from '@/lib/format'
import { cn } from '@/lib/utils'
import { Avatar, AvatarFallback, AvatarImage } from '@/components/ui/avatar'
import { Badge } from '@/components/ui/badge'
import { Switch } from '@/components/ui/switch'
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'
import { formatTokens } from '../lib/format'
import type { UserRanking, UserRankingMetric } from '../types'

type UsersRankingSectionProps = {
  rows: UserRanking[]
  self?: UserRanking
  metric: UserRankingMetric
  onMetricChange: (metric: UserRankingMetric) => void
  onPrivacyChange?: (isPublic: boolean) => void
  isPrivacyUpdating?: boolean
}

export function UsersRankingSection(props: UsersRankingSectionProps) {
  const { t } = useTranslation()

  return (
    <section className='bg-card overflow-hidden rounded-lg border'>
      <header className='border-b px-5 py-4'>
        <div className='flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between'>
          <div>
            <h2 className='text-foreground inline-flex items-center gap-2 text-base font-semibold'>
              <UserRound className='text-primary size-4' />
              {t('User Ranking')}
            </h2>
            <p className='text-muted-foreground mt-1 text-sm'>
              {props.metric === 'quota'
                ? t('Users ranked by quota usage')
                : t('Users ranked by token usage')}
            </p>
          </div>
          <ToggleGroup
            value={[props.metric]}
            variant='outline'
            size='sm'
            aria-label={t('User Ranking')}
            onValueChange={(next) => {
              const metric = next[0]
              if (metric === 'tokens' || metric === 'quota') {
                props.onMetricChange(metric)
              }
            }}
          >
            <ToggleGroupItem value='tokens'>Token</ToggleGroupItem>
            <ToggleGroupItem value='quota'>{t('Usage amount')}</ToggleGroupItem>
          </ToggleGroup>
        </div>
      </header>

      {props.rows.length === 0 ? (
        <div className='text-muted-foreground/80 px-5 py-8 text-center text-sm'>
          {t('No user ranking data available')}
        </div>
      ) : (
        <ul className='divide-border divide-y'>
          {props.rows.map((row) => (
            <UserRankingRow key={row.user_id} row={row} metric={props.metric} />
          ))}
        </ul>
      )}

      {props.self && (
        <div className='bg-muted/30 border-t px-5 py-3'>
          <div className='mb-2 flex items-center justify-between gap-3'>
            <div className='text-muted-foreground text-xs font-medium tracking-widest uppercase'>
              {t('My ranking')}
            </div>
            <label className='flex shrink-0 items-center gap-2'>
              <span className='text-muted-foreground text-xs font-medium'>
                {t('Current public status:')}{' '}
                {props.self.ranking_public ? t('Public') : t('Anonymous')}
              </span>
              <Switch
                size='sm'
                checked={Boolean(props.self.ranking_public)}
                disabled={!props.onPrivacyChange || props.isPrivacyUpdating}
                onCheckedChange={props.onPrivacyChange}
              />
            </label>
          </div>
          <UserRankingRow row={props.self} metric={props.metric} pinned />
        </div>
      )}
    </section>
  )
}

function UserRankingRow(props: {
  row: UserRanking
  metric: UserRankingMetric
  pinned?: boolean
}) {
  const { t } = useTranslation()
  const isRanked = props.row.rank > 0
  const RowElement = props.pinned ? 'div' : 'li'
  const rankStyle = getUserRankStyle(props.row.rank)
  const avatarName = props.row.display_name || String(props.row.user_id)
  const avatarFallback = getUserAvatarFallback(avatarName)
  const avatarFallbackStyle = getUserAvatarStyle(avatarName)
  const usageText =
    props.metric === 'quota'
      ? formatQuota(props.row.total_quota)
      : formatTokens(props.row.total_tokens)
  const usageLabel = props.metric === 'quota' ? t('Usage amount') : 'Token'

  return (
    <RowElement
      className={cn(
        'flex items-center gap-3 py-3',
        props.pinned ? 'px-0' : 'px-5'
      )}
    >
      <span
        className={cn(
          'w-8 shrink-0 text-right font-mono tabular-nums',
          rankStyle.rankClass
        )}
      >
        {isRanked ? `${props.row.rank}.` : '--'}
      </span>
      <Avatar className={cn(rankStyle.avatarClass, 'overflow-visible')}>
        {props.row.avatar_url && (
          <AvatarImage
            src={props.row.avatar_url}
            alt={props.row.display_name}
          />
        )}
        <AvatarFallback
          className={cn('font-semibold text-white', rankStyle.avatarTextClass)}
          style={avatarFallbackStyle}
        >
          {avatarFallback}
        </AvatarFallback>
      </Avatar>
      <div className='min-w-0 flex-1'>
        <div
          className={cn(
            'text-foreground truncate text-sm font-medium',
            rankStyle.nameClass
          )}
        >
          {props.row.display_name}
        </div>
        {!isRanked && (
          <div className='text-muted-foreground/80 mt-0.5 text-xs'>
            {t('Not ranked yet')}
          </div>
        )}
      </div>
      <div className='shrink-0 text-right'>
        <div
          className={cn(
            'text-foreground font-mono font-semibold tabular-nums',
            rankStyle.usageClass
          )}
        >
          {usageText}
        </div>
        <Badge variant='secondary' className={rankStyle.metricBadgeClass}>
          {usageLabel}
        </Badge>
      </div>
    </RowElement>
  )
}

type UserRankStyle = {
  avatarClass: string
  avatarTextClass: string
  rankClass: string
  nameClass: string
  usageClass: string
  metricBadgeClass?: string
}

function getUserRankStyle(rank: number): UserRankStyle {
  if (rank === 1) {
    return {
      avatarClass: 'size-11',
      avatarTextClass: 'text-base',
      rankClass: 'text-lg font-extrabold text-amber-700 dark:text-amber-300',
      nameClass: 'text-base font-semibold text-amber-700 dark:text-amber-300',
      usageClass: 'text-base text-amber-700 dark:text-amber-300',
      metricBadgeClass: 'h-6 px-2.5 text-xs',
    }
  }

  if (rank === 2) {
    return {
      avatarClass: 'size-10',
      avatarTextClass: 'text-sm',
      rankClass: 'text-base font-bold text-slate-700 dark:text-slate-300',
      nameClass: 'text-[15px] font-semibold text-slate-700 dark:text-slate-300',
      usageClass: 'text-[15px] text-slate-700 dark:text-slate-300',
      metricBadgeClass: 'h-[22px] px-2 text-[11px]',
    }
  }

  if (rank === 3) {
    return {
      avatarClass: 'size-9',
      avatarTextClass: 'text-sm',
      rankClass: 'text-sm font-bold text-orange-700 dark:text-orange-300',
      nameClass: 'text-sm font-semibold text-orange-700 dark:text-orange-300',
      usageClass: 'text-[15px] text-orange-700 dark:text-orange-300',
      metricBadgeClass: 'h-[22px] px-2 text-xs',
    }
  }

  return {
    avatarClass: 'size-8',
    avatarTextClass: 'text-xs',
    rankClass: 'text-muted-foreground/80 text-xs',
    nameClass: 'text-sm',
    usageClass: 'text-sm',
  }
}

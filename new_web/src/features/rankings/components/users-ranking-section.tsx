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
import { Award, Crown, Medal, Trophy, UserRound } from 'lucide-react'
import type { LucideIcon } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { cn } from '@/lib/utils'
import { Badge } from '@/components/ui/badge'
import { formatTokens } from '../lib/format'
import type { UserRanking } from '../types'

type UsersRankingSectionProps = {
  rows: UserRanking[]
  self?: UserRanking
}

export function UsersRankingSection(props: UsersRankingSectionProps) {
  const { t } = useTranslation()

  return (
    <section className='bg-card overflow-hidden rounded-lg border'>
      <header className='border-b px-5 py-4'>
        <h2 className='text-foreground inline-flex items-center gap-2 text-base font-semibold'>
          <UserRound className='text-primary size-4' />
          {t('User Ranking')}
        </h2>
        <p className='text-muted-foreground mt-1 text-sm'>
          {t('Users ranked by token usage')}
        </p>
      </header>

      {props.rows.length === 0 ? (
        <div className='text-muted-foreground/80 px-5 py-8 text-center text-sm'>
          {t('No user ranking data available')}
        </div>
      ) : (
        <ul className='divide-border divide-y'>
          {props.rows.map((row) => (
            <UserRankingRow key={row.user_id} row={row} />
          ))}
        </ul>
      )}

      {props.self && (
        <div className='bg-muted/30 border-t px-5 py-3'>
          <div className='text-muted-foreground mb-2 text-xs font-medium tracking-widest uppercase'>
            {t('My ranking')}
          </div>
          <UserRankingRow row={props.self} pinned />
        </div>
      )}
    </section>
  )
}

function UserRankingRow(props: { row: UserRanking; pinned?: boolean }) {
  const { t } = useTranslation()
  const isRanked = props.row.rank > 0
  const RowElement = props.pinned ? 'div' : 'li'
  const rankStyle = getUserRankStyle(props.row.rank)
  const RankIcon = rankStyle.Icon

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
      <span
        className={cn(
          'inline-flex shrink-0 items-center justify-center rounded-full transition-shadow',
          rankStyle.avatarClass,
          rankStyle.badgeClass
        )}
      >
        <RankIcon className={rankStyle.iconClass} />
      </span>
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
          {formatTokens(props.row.total_tokens)}
        </div>
        <Badge variant='secondary' className={rankStyle.tokenBadgeClass}>
          {t('tokens')}
        </Badge>
      </div>
    </RowElement>
  )
}

type UserRankStyle = {
  Icon: LucideIcon
  avatarClass: string
  badgeClass: string
  rankClass: string
  iconClass: string
  nameClass: string
  usageClass: string
  tokenBadgeClass?: string
}

function getUserRankStyle(rank: number): UserRankStyle {
  if (rank === 1) {
    return {
      Icon: Crown,
      avatarClass: 'size-11',
      rankClass: 'text-lg font-extrabold text-amber-700 dark:text-amber-300',
      badgeClass:
        'bg-amber-100 text-amber-700 shadow-[0_0_0_1px_rgb(251_191_36_/_0.25),0_8px_22px_rgb(245_158_11_/_0.18)] dark:bg-amber-500/20 dark:text-amber-300',
      iconClass: 'size-5 drop-shadow-sm',
      nameClass: 'text-base font-semibold text-amber-700 dark:text-amber-300',
      usageClass: 'text-base text-amber-700 dark:text-amber-300',
      tokenBadgeClass: 'h-6 px-2.5 text-xs',
    }
  }

  if (rank === 2) {
    return {
      Icon: Medal,
      avatarClass: 'size-10',
      rankClass: 'text-base font-bold text-slate-700 dark:text-slate-300',
      badgeClass:
        'bg-slate-100 text-slate-700 shadow-[0_0_0_1px_rgb(148_163_184_/_0.25),0_8px_22px_rgb(100_116_139_/_0.14)] dark:bg-slate-500/20 dark:text-slate-300',
      iconClass: 'size-[18px]',
      nameClass: 'text-[15px] font-semibold text-slate-700 dark:text-slate-300',
      usageClass: 'text-[15px] text-slate-700 dark:text-slate-300',
      tokenBadgeClass: 'h-[22px] px-2 text-[11px]',
    }
  }

  if (rank === 3) {
    return {
      Icon: Award,
      avatarClass: 'size-9',
      rankClass: 'text-sm font-bold text-orange-700 dark:text-orange-300',
      badgeClass:
        'bg-orange-100 text-orange-700 shadow-[0_0_0_1px_rgb(251_146_60_/_0.25),0_8px_22px_rgb(234_88_12_/_0.14)] dark:bg-orange-500/20 dark:text-orange-300',
      iconClass: 'size-4',
      nameClass: 'text-sm font-semibold text-orange-700 dark:text-orange-300',
      usageClass: 'text-sm text-orange-700 dark:text-orange-300',
      tokenBadgeClass: 'h-5 px-2 text-[10px]',
    }
  }

  return {
    Icon: Trophy,
    avatarClass: 'size-8',
    rankClass: 'text-muted-foreground/80 text-xs',
    badgeClass: 'bg-primary/10 text-primary',
    iconClass: 'size-3.5',
    nameClass: 'text-sm',
    usageClass: 'text-sm',
  }
}

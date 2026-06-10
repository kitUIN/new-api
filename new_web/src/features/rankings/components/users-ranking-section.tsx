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
import { Trophy, UserRound } from 'lucide-react'
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

  return (
    <RowElement
      className={cn(
        'flex items-center gap-3 py-3',
        props.pinned ? 'px-0' : 'px-5'
      )}
    >
      <span className='text-muted-foreground/80 w-8 shrink-0 text-right font-mono text-xs tabular-nums'>
        {isRanked ? `${props.row.rank}.` : '--'}
      </span>
      <span className='bg-primary/10 text-primary inline-flex size-9 shrink-0 items-center justify-center rounded-full'>
        <Trophy className='size-4' />
      </span>
      <div className='min-w-0 flex-1'>
        <div className='text-foreground truncate text-sm font-medium'>
          {props.row.display_name}
        </div>
        {!isRanked && (
          <div className='text-muted-foreground/80 mt-0.5 text-xs'>
            {t('Not ranked yet')}
          </div>
        )}
      </div>
      <div className='shrink-0 text-right'>
        <div className='text-foreground font-mono text-sm font-semibold tabular-nums'>
          {formatTokens(props.row.total_tokens)}
        </div>
        <Badge variant='secondary'>{t('tokens')}</Badge>
      </div>
    </RowElement>
  )
}

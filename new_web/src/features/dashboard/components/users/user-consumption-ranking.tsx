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
import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'
import {
  getQQAvatarUrl,
  getUserAvatarFallback,
  getUserAvatarStyle,
} from '@/lib/avatar'
import { formatQuota } from '@/lib/format'
import { Avatar, AvatarFallback, AvatarImage } from '@/components/ui/avatar'
import type { QuotaDataItem } from '@/features/dashboard/types'

export function UserConsumptionRanking(props: {
  data: QuotaDataItem[]
  limit: number
}) {
  const { t } = useTranslation()
  const rows = useMemo(() => {
    const users = new Map<string, QuotaDataItem & { quota: number }>()
    for (const item of props.data) {
      const key = item.user_id
        ? String(item.user_id)
        : item.username || 'unknown'
      const previous = users.get(key)
      users.set(key, {
        ...item,
        quota: (previous?.quota ?? 0) + (Number(item.quota) || 0),
      })
    }
    return Array.from(users.values())
      .sort((a, b) => b.quota - a.quota)
      .slice(0, props.limit)
  }, [props.data, props.limit])
  const maxQuota = Math.max(0, ...rows.map((row) => row.quota))
  const totalQuota = rows.reduce((total, row) => total + row.quota, 0)

  if (rows.length === 0) {
    return (
      <div className='text-muted-foreground flex h-full items-center justify-center text-sm'>
        {t('No data')}
      </div>
    )
  }

  return (
    <div className='h-full overflow-y-auto px-2 sm:px-3'>
      <div className='text-muted-foreground py-2 text-xs'>
        {t('Total:')} {formatQuota(totalQuota)}
      </div>
      <ol className='divide-y'>
        {rows.map((row, index) => {
          const name =
            row.display_name?.trim() || row.username || `#${row.user_id}`
          return (
            <li
              key={row.user_id || row.username}
              className='flex items-center gap-3 py-3'
            >
              <span className='text-muted-foreground w-5 shrink-0 text-right text-xs tabular-nums'>
                {index + 1}
              </span>
              <Avatar className='size-9 shrink-0'>
                <AvatarImage src={getQQAvatarUrl(row.qq_id)} alt={name} />
                <AvatarFallback style={getUserAvatarStyle(name)}>
                  {getUserAvatarFallback(name)}
                </AvatarFallback>
              </Avatar>
              <div className='min-w-0 flex-1'>
                <div className='flex items-center justify-between gap-3'>
                  <div className='min-w-0'>
                    <div className='truncate text-sm font-medium' title={name}>
                      {name}
                    </div>
                    <div
                      className='text-muted-foreground truncate text-xs'
                      title={row.username}
                    >
                      {row.username}
                      {row.user_id ? ` #${row.user_id}` : ''}
                    </div>
                  </div>
                  <span className='shrink-0 text-sm font-medium tabular-nums'>
                    {formatQuota(row.quota)}
                  </span>
                </div>
                <div
                  className='bg-muted mt-2 h-1.5 overflow-hidden rounded-full'
                  aria-hidden='true'
                >
                  <div
                    className='bg-primary h-full rounded-full'
                    style={{
                      width: `${maxQuota > 0 ? Math.max(0, (row.quota / maxQuota) * 100) : 0}%`,
                    }}
                  />
                </div>
              </div>
            </li>
          )
        })}
      </ol>
    </div>
  )
}

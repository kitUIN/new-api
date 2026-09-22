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
  getQQAvatarUrl,
  getUserAvatarFallback,
  getUserAvatarStyle,
} from '@/lib/avatar'
import { Avatar, AvatarFallback, AvatarImage } from '@/components/ui/avatar'
import type { BillingAuditUser } from '../types'

export function BillingUser(props: { userId: number; user: BillingAuditUser }) {
  const { t } = useTranslation()
  const name =
    props.user.display_name ||
    props.user.username ||
    t('billingAudit.unknownUser')
  const avatar = getQQAvatarUrl(props.user.qq_id)
  return (
    <div className='flex min-w-40 items-center gap-2'>
      <Avatar className='size-9 shrink-0'>
        {avatar && <AvatarImage src={avatar} alt={name} />}
        <AvatarFallback style={getUserAvatarStyle(name)}>
          {getUserAvatarFallback(name)}
        </AvatarFallback>
      </Avatar>
      <div className='min-w-0'>
        <div className='font-medium'>{name}</div>
        <div className='text-muted-foreground text-xs'>
          {props.user.username && `@${props.user.username} · `}#{props.userId}
        </div>
      </div>
    </div>
  )
}

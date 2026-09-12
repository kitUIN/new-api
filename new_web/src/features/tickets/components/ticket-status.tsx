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
import { Badge } from '@/components/ui/badge'
import type { Ticket } from '../types'

export function TicketStatus(props: { ticket: Ticket }) {
  const { t } = useTranslation()
  if (props.ticket.status === 'closed')
    return <Badge variant='secondary'>{t('tickets.closed')}</Badge>
  if (props.ticket.last_reply_by_admin)
    return <Badge variant='outline'>{t('tickets.replied')}</Badge>
  return <Badge>{t('tickets.awaitingReply')}</Badge>
}

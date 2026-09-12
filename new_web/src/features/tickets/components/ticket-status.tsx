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
import {
  Clock01Icon,
  LockKeyIcon,
  Message02Icon,
} from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useTranslation } from 'react-i18next'
import { Badge } from '@/components/ui/badge'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import type { Ticket } from '../types'

function getTicketStatus(ticket: Ticket) {
  if (ticket.status === 'closed') {
    return {
      labelKey: 'tickets.closed',
      variant: 'secondary' as const,
      icon: LockKeyIcon,
      iconClassName: 'text-muted-foreground',
    }
  }
  if (ticket.last_reply_by_admin) {
    return {
      labelKey: 'tickets.replied',
      variant: 'outline' as const,
      icon: Message02Icon,
      iconClassName: 'text-success',
    }
  }
  return {
    labelKey: 'tickets.awaitingReply',
    variant: 'default' as const,
    icon: Clock01Icon,
    iconClassName: 'text-warning',
  }
}

export function TicketStatus(props: { ticket: Ticket; iconOnly?: boolean }) {
  const { t } = useTranslation()
  const status = getTicketStatus(props.ticket)
  const label = t(status.labelKey)
  if (props.iconOnly) {
    return (
      <Tooltip>
        <TooltipTrigger
          render={<span />}
          className='inline-flex size-5 shrink-0 items-center justify-center'
          aria-label={label}
          role='img'
        >
          <HugeiconsIcon
            icon={status.icon}
            size={18}
            className={status.iconClassName}
            aria-hidden='true'
          />
        </TooltipTrigger>
        <TooltipContent>{label}</TooltipContent>
      </Tooltip>
    )
  }
  return <Badge variant={status.variant}>{label}</Badge>
}

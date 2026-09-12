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
import { useEffect, useRef } from 'react'
import { useInfiniteQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { useAuthStore } from '@/stores/auth-store'
import { cn } from '@/lib/utils'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { getTicketMessages } from '../api'
import type { Ticket } from '../types'
import { TicketImage } from './ticket-image'

export function TicketConversation(props: { ticket: Ticket }) {
  const { t } = useTranslation()
  const userId = useAuthStore((state) => state.auth.user?.id)
  const end = useRef<HTMLDivElement>(null)
  const query = useInfiniteQuery({
    queryKey: ['tickets', userId, 'messages', props.ticket.id],
    queryFn: ({ pageParam }) => getTicketMessages(props.ticket.id, pageParam),
    initialPageParam: 0,
    getNextPageParam: (last) => (last.has_more ? last.items[0]?.id : undefined),
    refetchInterval: props.ticket.status === 'open' ? 10_000 : false,
  })
  const messages = [...(query.data?.pages ?? [])]
    .reverse()
    .flatMap((page) => page.items)
  const latestId = messages.at(-1)?.id
  useEffect(() => {
    end.current?.scrollIntoView({ block: 'nearest' })
  }, [latestId])
  if (query.isPending) return <Skeleton className='h-40 w-full' />
  if (query.isError)
    return (
      <Button variant='outline' onClick={() => void query.refetch()}>
        {t('tickets.retry')}
      </Button>
    )

  return (
    <div className='flex min-w-0 flex-col gap-5 py-4'>
      {query.hasNextPage && (
        <Button
          variant='ghost'
          disabled={query.isFetchingNextPage}
          onClick={() => void query.fetchNextPage()}
        >
          {t('tickets.earlierMessages')}
        </Button>
      )}
      {messages.map((message) => (
        <article
          key={message.id}
          className={cn(
            'flex min-w-0 flex-col gap-2 border-l-2 pl-3',
            message.is_admin ? 'border-primary' : 'border-border'
          )}
        >
          <div className='text-muted-foreground flex flex-wrap items-center gap-2 text-xs'>
            <span className='text-foreground max-w-full font-medium wrap-anywhere'>
              {message.username}
            </span>
            {message.is_admin && (
              <Badge variant='secondary'>{t('tickets.admin')}</Badge>
            )}
            <time
              dateTime={new Date(message.created_time * 1000).toISOString()}
            >
              {new Date(message.created_time * 1000).toLocaleString()}
            </time>
          </div>
          {message.content && (
            <p className='text-sm leading-relaxed wrap-anywhere whitespace-pre-wrap'>
              {message.content}
            </p>
          )}
          {message.attachments.length > 0 && (
            <div className='flex flex-wrap gap-2'>
              {message.attachments.map((attachment) => (
                <TicketImage key={attachment.id} attachment={attachment} />
              ))}
            </div>
          )}
        </article>
      ))}
      <div ref={end} />
    </div>
  )
}

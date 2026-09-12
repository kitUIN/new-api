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
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link, useNavigate } from '@tanstack/react-router'
import {
  Add01Icon,
  ArrowLeft01Icon,
  ArrowRight01Icon,
  CustomerSupportIcon,
  RefreshIcon,
} from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useTranslation } from 'react-i18next'
import { useAuthStore } from '@/stores/auth-store'
import { ROLE } from '@/lib/roles'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  Empty,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from '@/components/ui/empty'
import { Skeleton } from '@/components/ui/skeleton'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { SectionPageLayout } from '@/components/layout'
import { getTickets, sendTicketMessage } from './api'
import { MessageComposer } from './components/message-composer'
import { TicketDetail } from './components/ticket-detail'
import { TicketStatus } from './components/ticket-status'
import type { TicketInput } from './types'

export function Tickets(props: { ticketId?: number }) {
  if (props.ticketId)
    return <TicketDetail key={props.ticketId} id={props.ticketId} />
  return <TicketList />
}

function TicketList() {
  const { t } = useTranslation()
  const user = useAuthStore((state) => state.auth.user)
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const [page, setPage] = useState(1)
  const [status, setStatus] = useState('')
  const [creating, setCreating] = useState(false)
  const query = useQuery({
    queryKey: ['tickets', user?.id, 'list', page, status],
    queryFn: () => getTickets(page, status),
    refetchInterval: 15_000,
  })
  const create = useMutation({
    mutationFn: (input: TicketInput) => sendTicketMessage(input),
    onSuccess: (ticket) => {
      void queryClient.invalidateQueries({ queryKey: ['tickets', user?.id] })
      setCreating(false)
      void navigate({ to: '/tickets', search: { ticket: ticket.id } })
    },
  })
  const total = query.data?.total ?? 0
  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>{t('tickets.title')}</SectionPageLayout.Title>
      <SectionPageLayout.Actions>
        <Button
          variant='ghost'
          size='icon'
          disabled={query.isFetching}
          aria-label={t('tickets.refresh')}
          title={t('tickets.refresh')}
          onClick={() => void query.refetch()}
        >
          <HugeiconsIcon icon={RefreshIcon} />
        </Button>
        <Button onClick={() => setCreating(true)}>
          <HugeiconsIcon icon={Add01Icon} data-icon='inline-start' />
          {t('tickets.create')}
        </Button>
      </SectionPageLayout.Actions>
      <SectionPageLayout.Content>
        <div className='flex flex-col gap-4'>
          <Tabs
            value={status}
            onValueChange={(value) => {
              setStatus(String(value))
              setPage(1)
            }}
          >
            <TabsList variant='line' aria-label={t('tickets.status')}>
              <TabsTrigger value=''>{t('tickets.all')}</TabsTrigger>
              <TabsTrigger value='open'>{t('tickets.open')}</TabsTrigger>
              <TabsTrigger value='closed'>{t('tickets.closed')}</TabsTrigger>
            </TabsList>
          </Tabs>
          {query.isPending && <Skeleton className='h-52 w-full' />}
          {query.isError && (
            <Empty>
              <EmptyHeader>
                <EmptyTitle>{t('tickets.loadFailed')}</EmptyTitle>
              </EmptyHeader>
              <Button variant='outline' onClick={() => void query.refetch()}>
                {t('tickets.retry')}
              </Button>
            </Empty>
          )}
          {query.data && query.data.items.length === 0 && (
            <Empty className='min-h-60'>
              <EmptyHeader>
                <EmptyMedia variant='icon'>
                  <HugeiconsIcon icon={CustomerSupportIcon} />
                </EmptyMedia>
                <EmptyTitle>{t('tickets.empty')}</EmptyTitle>
              </EmptyHeader>
            </Empty>
          )}
          <ul className='flex min-w-0 flex-col'>
            {query.data?.items.map((ticket) => (
              <li key={ticket.id} className='border-b'>
                <Link
                  to='/tickets'
                  search={{ ticket: ticket.id }}
                  className='hover:bg-muted/50 focus-visible:ring-ring flex min-w-0 flex-wrap items-center justify-between gap-3 px-2 py-4 focus-visible:ring-2'
                >
                  <div className='flex min-w-0 flex-1 basis-48 flex-col gap-1.5'>
                    <span className='text-sm font-medium wrap-anywhere'>
                      {ticket.title}
                    </span>
                    <span className='text-muted-foreground text-xs wrap-anywhere'>
                      #{ticket.id}
                      {(user?.role ?? 0) >= ROLE.ADMIN &&
                        ` · ${ticket.username}`}{' '}
                      ·{' '}
                      {t('tickets.messageCount', {
                        count: ticket.message_count,
                      })}
                    </span>
                  </div>
                  <div className='flex shrink-0 flex-col items-end gap-1.5'>
                    <TicketStatus ticket={ticket} />
                    <time
                      className='text-muted-foreground text-xs'
                      dateTime={new Date(
                        ticket.updated_time * 1000
                      ).toISOString()}
                    >
                      {new Date(ticket.updated_time * 1000).toLocaleString()}
                    </time>
                  </div>
                </Link>
              </li>
            ))}
          </ul>
          <div className='text-muted-foreground flex flex-wrap items-center justify-between gap-3 text-sm'>
            <span>{t('tickets.total', { count: total })}</span>
            <div className='flex items-center gap-3'>
              <Button
                size='icon'
                variant='outline'
                disabled={page <= 1 || query.isFetching}
                onClick={() => setPage(page - 1)}
                aria-label={t('tickets.previous')}
                title={t('tickets.previous')}
              >
                <HugeiconsIcon icon={ArrowLeft01Icon} />
              </Button>
              <span>
                {page} / {Math.max(1, Math.ceil(total / 20))}
              </span>
              <Button
                size='icon'
                variant='outline'
                disabled={page * 20 >= total || query.isFetching}
                onClick={() => setPage(page + 1)}
                aria-label={t('tickets.next')}
                title={t('tickets.next')}
              >
                <HugeiconsIcon icon={ArrowRight01Icon} />
              </Button>
            </div>
          </div>
        </div>
        <Dialog
          open={creating}
          onOpenChange={(open) => {
            if (!create.isPending) setCreating(open)
          }}
        >
          <DialogContent
            className='max-h-[90dvh] overflow-y-auto sm:max-w-xl'
            showCloseButton={!create.isPending}
          >
            <DialogHeader>
              <DialogTitle>{t('tickets.create')}</DialogTitle>
            </DialogHeader>
            <MessageComposer
              creating
              onSubmit={async (input) => {
                await create.mutateAsync(input)
              }}
            />
          </DialogContent>
        </Dialog>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}

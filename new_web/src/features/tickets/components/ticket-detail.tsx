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
import { Link } from '@tanstack/react-router'
import { ArrowLeft01Icon, LockKeyIcon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useTranslation } from 'react-i18next'
import { useAuthStore } from '@/stores/auth-store'
import { Alert, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Separator } from '@/components/ui/separator'
import { Skeleton } from '@/components/ui/skeleton'
import { ConfirmDialog } from '@/components/confirm-dialog'
import { SectionPageLayout } from '@/components/layout'
import { closeTicket, getTicket, sendTicketMessage } from '../api'
import type { TicketInput } from '../types'
import { MessageComposer } from './message-composer'
import { TicketConversation } from './ticket-conversation'
import { TicketStatus } from './ticket-status'

export function TicketDetail(props: { id: number }) {
  const { t } = useTranslation()
  const userId = useAuthStore((state) => state.auth.user?.id)
  const queryClient = useQueryClient()
  const [closing, setClosing] = useState(false)
  const queryKey = ['tickets', userId, 'detail', props.id]
  const query = useQuery({
    queryKey,
    queryFn: () => getTicket(props.id),
    refetchInterval: (state) =>
      state.state.data?.status === 'closed' ? false : 10_000,
  })
  const refresh = () =>
    queryClient.invalidateQueries({ queryKey: ['tickets', userId] })
  const reply = useMutation({
    mutationFn: (input: TicketInput) => sendTicketMessage(input, props.id),
    onSuccess: (ticket) => {
      queryClient.setQueryData(queryKey, ticket)
      void refresh()
    },
    onError: () => {
      void refresh()
    },
  })
  const close = useMutation({
    mutationFn: () => closeTicket(props.id),
    onSuccess: (ticket) => {
      queryClient.setQueryData(queryKey, ticket)
      setClosing(false)
      void refresh()
    },
  })
  const ticket = query.data
  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>
        {t('tickets.title')} #{props.id}
      </SectionPageLayout.Title>
      <SectionPageLayout.Actions>
        <Button variant='ghost' render={<Link to='/tickets' search={{}} />}>
          <HugeiconsIcon icon={ArrowLeft01Icon} data-icon='inline-start' />
          {t('tickets.back')}
        </Button>
        {ticket?.status === 'open' && (
          <Button
            variant='outline'
            disabled={reply.isPending || close.isPending}
            onClick={() => setClosing(true)}
          >
            <HugeiconsIcon icon={LockKeyIcon} data-icon='inline-start' />
            {t('tickets.close')}
          </Button>
        )}
      </SectionPageLayout.Actions>
      <SectionPageLayout.Content>
        <div className='mx-auto flex w-full max-w-4xl flex-col gap-3'>
          {query.isPending && <Skeleton className='h-48 w-full' />}
          {query.isError && (
            <Alert variant='destructive'>
              <AlertTitle>{t('tickets.loadFailed')}</AlertTitle>
              <Button variant='outline' onClick={() => void query.refetch()}>
                {t('tickets.retry')}
              </Button>
            </Alert>
          )}
          {ticket && (
            <>
              <div className='flex flex-col gap-2 py-2'>
                <h1 className='text-lg font-semibold wrap-anywhere'>
                  {ticket.title}
                </h1>
                <div className='text-muted-foreground flex flex-wrap items-center gap-2 text-sm'>
                  <TicketStatus ticket={ticket} />
                  <span className='wrap-anywhere'>
                    {ticket.username} · #{ticket.user_id}
                  </span>
                </div>
              </div>
              <Separator />
              <TicketConversation ticket={ticket} />
              <Separator />
              {ticket.status === 'closed' ? (
                <Alert>
                  <HugeiconsIcon icon={LockKeyIcon} />
                  <AlertTitle>
                    {t('tickets.closedNotice')} ·{' '}
                    {new Date(ticket.closed_time * 1000).toLocaleString()}
                  </AlertTitle>
                </Alert>
              ) : (
                <MessageComposer
                  disabled={close.isPending}
                  onSubmit={async (input) => {
                    await reply.mutateAsync(input)
                  }}
                />
              )}
            </>
          )}
        </div>
        <ConfirmDialog
          open={closing}
          onOpenChange={setClosing}
          title={t('tickets.close')}
          desc={t('tickets.closeConfirm')}
          confirmText={t('tickets.close')}
          isLoading={close.isPending}
          handleConfirm={() => close.mutate()}
        />
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}

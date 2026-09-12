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
import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { useAuthStore } from '@/stores/auth-store'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogTitle } from '@/components/ui/dialog'
import { Skeleton } from '@/components/ui/skeleton'
import { getTicketImage } from '../api'
import type { TicketAttachment } from '../types'
import { BlobImage } from './blob-image'

export function TicketImage(props: { attachment: TicketAttachment }) {
  const { t } = useTranslation()
  const userId = useAuthStore((state) => state.auth.user?.id)
  const [open, setOpen] = useState(false)
  const query = useQuery({
    queryKey: [
      'tickets',
      userId,
      'image',
      props.attachment.ticket_id,
      props.attachment.id,
    ],
    queryFn: ({ signal }) =>
      getTicketImage(props.attachment.ticket_id, props.attachment.id, signal),
    staleTime: Infinity,
    gcTime: 60_000,
    retry: false,
  })
  if (query.isError)
    return (
      <Button variant='outline' onClick={() => void query.refetch()}>
        {t('tickets.imageRetry')}
      </Button>
    )
  if (!query.data) return <Skeleton className='aspect-square w-28 sm:w-36' />
  return (
    <>
      <button
        type='button'
        onClick={() => setOpen(true)}
        aria-label={t('tickets.viewImage')}
        className='focus-visible:ring-ring w-28 overflow-hidden rounded-md border focus-visible:ring-2 sm:w-36'
      >
        <BlobImage
          blob={query.data}
          alt={t('tickets.attachment')}
          className='bg-muted aspect-square w-full object-contain'
          loading='lazy'
        />
      </button>
      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent className='sm:max-w-4xl'>
          <DialogTitle>{t('tickets.attachment')}</DialogTitle>
          <BlobImage
            blob={query.data}
            alt={t('tickets.attachment')}
            className='max-h-[75dvh] w-full object-contain'
          />
        </DialogContent>
      </Dialog>
    </>
  )
}

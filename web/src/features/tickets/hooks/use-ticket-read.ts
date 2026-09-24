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
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useAuthStore } from '@/stores/auth-store'
import { markTicketRead } from '../api'
import { refreshTicketUnreadCount } from '../lib/queries'

type ReadRequest = { userId: number; ticketId: number; messageId: number }

export function useTicketRead(
  ticketId: number,
  messageId: number | undefined,
  dataUpdatedAt: number
): void {
  const userId = useAuthStore((state) => state.auth.user?.id)
  const queryClient = useQueryClient()
  const requested = useRef<ReadRequest | null>(null)
  const { mutate } = useMutation({
    mutationFn: (request: ReadRequest) =>
      markTicketRead(request.ticketId, request.messageId),
    retry: 1,
    onSuccess: (_data, request) =>
      refreshTicketUnreadCount(queryClient, request.userId),
    onError: (_error, request) => {
      if (requested.current === request) requested.current = null
    },
  })

  useEffect(() => {
    const markVisibleMessages = () => {
      if (!userId || !messageId || document.visibilityState !== 'visible')
        return
      const previous = requested.current
      if (
        previous?.userId === userId &&
        previous.ticketId === ticketId &&
        previous.messageId >= messageId
      ) {
        return
      }
      const request = { userId, ticketId, messageId }
      requested.current = request
      mutate(request)
    }
    markVisibleMessages()
    document.addEventListener('visibilitychange', markVisibleMessages)
    return () =>
      document.removeEventListener('visibilitychange', markVisibleMessages)
  }, [userId, ticketId, messageId, dataUpdatedAt, mutate])
}

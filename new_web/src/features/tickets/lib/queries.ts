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
import type { QueryClient } from '@tanstack/react-query'

export async function refreshTicketUnreadCount(
  queryClient: QueryClient,
  userId: number
): Promise<void> {
  const filters = { queryKey: ['tickets', userId, 'unread'], exact: true }
  // Discard a count fetched before the read acknowledgement was saved.
  await queryClient.cancelQueries(filters)
  await queryClient.invalidateQueries(filters)
}

export function invalidateTicketQueries(
  queryClient: QueryClient,
  userId: number | undefined
): Promise<void> {
  return queryClient.invalidateQueries({
    queryKey: ['tickets', userId],
    // Attachments are immutable; sending a message must not download them again.
    predicate: (query) => query.queryKey[2] !== 'image',
  })
}

export async function refreshClosedTicketMessages(
  queryClient: QueryClient,
  userId: number | undefined,
  ticketId: number
): Promise<void> {
  const queryKey = ['tickets', userId, 'messages', ticketId]
  const filters = { queryKey, exact: true }
  if (queryClient.getQueryState(queryKey)?.fetchStatus === 'fetching') {
    // An existing poll may contain a snapshot taken before the final reply.
    // Wait for it so the shared API request deduplication cannot reuse it.
    await queryClient.refetchQueries(filters, { cancelRefetch: false })
  }
  await queryClient.invalidateQueries(filters)
}

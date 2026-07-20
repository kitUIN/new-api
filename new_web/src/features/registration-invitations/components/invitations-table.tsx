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
import { useEffect, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Loader2, Search } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { formatTimestampToDate } from '@/lib/format'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { deleteInvitation, getInvitations } from '../api'
import { INVITATION_STATUS } from '../types'
import { InvitationDeleteButton } from './invitation-delete-button'

const PAGE_SIZE = 20

export function InvitationsTable() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [page, setPage] = useState(1)
  const [searchInput, setSearchInput] = useState('')
  const [keyword, setKeyword] = useState('')

  const invitationsQuery = useQuery({
    queryKey: ['invitations', page, PAGE_SIZE, keyword],
    queryFn: () => getInvitations({ page, pageSize: PAGE_SIZE, keyword }),
    placeholderData: (previousData) => previousData,
  })

  const deleteMutation = useMutation({
    mutationFn: deleteInvitation,
    onSuccess: async (result) => {
      if (!result.success) return
      toast.success(t('Invitation code deleted successfully'))
      await queryClient.invalidateQueries({ queryKey: ['invitations'] })
    },
  })

  const invitations = invitationsQuery.data?.data?.items || []
  const total = invitationsQuery.data?.data?.total || 0
  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE))

  useEffect(() => {
    if (page > totalPages) setPage(totalPages)
  }, [page, totalPages])

  const submitSearch = (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    setPage(1)
    setKeyword(searchInput.trim())
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t('Invitation Codes')}</CardTitle>
        <CardDescription>
          {t('Each invitation code can register exactly one account.')}
        </CardDescription>
      </CardHeader>
      <CardContent className='space-y-4'>
        <form onSubmit={submitSearch} className='flex max-w-md gap-2'>
          <Input
            value={searchInput}
            onChange={(event) => setSearchInput(event.target.value)}
            placeholder={t('Search by code or remark')}
          />
          <Button type='submit' variant='outline' className='gap-2'>
            <Search className='h-4 w-4' />
            {t('Search')}
          </Button>
        </form>

        {invitationsQuery.isLoading && (
          <div className='text-muted-foreground flex min-h-40 items-center justify-center gap-2'>
            <Loader2 className='h-4 w-4 animate-spin' />
            {t('Loading...')}
          </div>
        )}
        {!invitationsQuery.isLoading && invitations.length === 0 && (
          <div className='text-muted-foreground flex min-h-40 items-center justify-center text-sm'>
            {t('No invitation codes found')}
          </div>
        )}
        {!invitationsQuery.isLoading && invitations.length > 0 && (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t('Invitation code')}</TableHead>
                <TableHead>{t('Remark')}</TableHead>
                <TableHead>{t('Status')}</TableHead>
                <TableHead>{t('Created')}</TableHead>
                <TableHead>{t('Used by')}</TableHead>
                <TableHead className='text-right'>{t('Actions')}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {invitations.map((invitation) => {
                const isUsed = invitation.status === INVITATION_STATUS.USED
                return (
                  <TableRow key={invitation.id}>
                    <TableCell className='font-mono font-medium'>
                      {invitation.code}
                    </TableCell>
                    <TableCell className='max-w-md whitespace-normal'>
                      {invitation.remark}
                    </TableCell>
                    <TableCell>
                      <Badge variant={isUsed ? 'secondary' : 'default'}>
                        {isUsed ? t('Used') : t('Available')}
                      </Badge>
                    </TableCell>
                    <TableCell className='font-mono text-sm'>
                      {formatTimestampToDate(invitation.created_time)}
                    </TableCell>
                    <TableCell>
                      {isUsed ? (
                        <div className='space-y-0.5 text-sm'>
                          <div>
                            {t('User {{id}}', { id: invitation.used_user_id })}
                          </div>
                          <div className='text-muted-foreground font-mono text-xs'>
                            {formatTimestampToDate(invitation.used_time)}
                          </div>
                        </div>
                      ) : (
                        <span className='text-muted-foreground'>-</span>
                      )}
                    </TableCell>
                    <TableCell className='text-right'>
                      {!isUsed ? (
                        <InvitationDeleteButton
                          code={invitation.code}
                          isDeleting={
                            deleteMutation.isPending &&
                            deleteMutation.variables === invitation.id
                          }
                          onDelete={() => deleteMutation.mutate(invitation.id)}
                        />
                      ) : null}
                    </TableCell>
                  </TableRow>
                )
              })}
            </TableBody>
          </Table>
        )}

        <div className='flex items-center justify-between gap-3'>
          <p className='text-muted-foreground text-sm'>
            {t('{{count}} invitation codes', { count: total })}
          </p>
          <div className='flex items-center gap-2'>
            <Button
              type='button'
              variant='outline'
              size='sm'
              disabled={page <= 1 || invitationsQuery.isFetching}
              onClick={() => setPage((current) => current - 1)}
            >
              {t('Previous')}
            </Button>
            <span className='text-muted-foreground text-sm'>
              {t('Page {{page}} of {{pages}}', { page, pages: totalPages })}
            </span>
            <Button
              type='button'
              variant='outline'
              size='sm'
              disabled={page >= totalPages || invitationsQuery.isFetching}
              onClick={() => setPage((current) => current + 1)}
            >
              {t('Next')}
            </Button>
          </div>
        </div>
      </CardContent>
    </Card>
  )
}

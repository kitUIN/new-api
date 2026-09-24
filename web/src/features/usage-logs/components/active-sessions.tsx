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
import dayjs from 'dayjs'
import { useQuery } from '@tanstack/react-query'
import {
  ArrowLeft01Icon,
  ArrowRight01Icon,
  RefreshIcon,
  PencilEdit02Icon,
} from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useTranslation } from 'react-i18next'
import { useAuthStore } from '@/stores/auth-store'
import { useIsAdmin } from '@/hooks/use-admin'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Empty, EmptyHeader, EmptyTitle } from '@/components/ui/empty'
import { Input } from '@/components/ui/input'
import { Spinner } from '@/components/ui/spinner'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { getRelaySessions, type RelaySession } from '../session-api'
import { SessionGroupDialog } from './dialogs/session-group-dialog'

export function ActiveSessions() {
  const { t } = useTranslation()
  const isAdmin = useIsAdmin()
  const userID = useAuthStore((state) => state.auth.user?.id)
  const [page, setPage] = useState(1)
  const [searchInput, setSearchInput] = useState('')
  const [search, setSearch] = useState('')
  const [selected, setSelected] = useState<RelaySession | null>(null)
  const query = useQuery({
    queryKey: ['relay-sessions', userID, isAdmin, page, search],
    queryFn: () => getRelaySessions(isAdmin, page, search),
    refetchInterval: 15000,
  })
  const sessions = query.data?.items ?? []
  const total = query.data?.total ?? 0
  const pages = Math.max(1, Math.ceil(total / 20))

  return (
    <div className='flex min-w-0 flex-col gap-4'>
      <div className='flex flex-wrap items-center justify-between gap-2'>
        <form
          className='flex min-w-0 flex-1 items-center gap-2'
          onSubmit={(event) => {
            event.preventDefault()
            setPage(1)
            setSearch(searchInput.trim())
          }}
        >
          <Input
            aria-label={t('usageLogs.sessions.search')}
            placeholder={t('usageLogs.sessions.search')}
            value={searchInput}
            onChange={(event) => setSearchInput(event.target.value)}
            className='min-w-0 sm:max-w-sm'
          />
          <Button type='submit' variant='outline'>
            {t('Search')}
          </Button>
        </form>
        <Tooltip>
          <TooltipTrigger
            render={
              <Button
                variant='outline'
                size='icon'
                aria-label={t('Refresh')}
                disabled={query.isFetching}
                onClick={() => void query.refetch()}
              />
            }
          >
            <HugeiconsIcon icon={RefreshIcon} />
          </TooltipTrigger>
          <TooltipContent>{t('Refresh')}</TooltipContent>
        </Tooltip>
      </div>
      <div className='text-muted-foreground flex flex-wrap items-center gap-2 text-sm'>
        <span>{t('usageLogs.sessions.count', { count: total })}</span>
        <Badge variant='outline'>{t('usageLogs.sessions.activeWindow')}</Badge>
      </div>
      {query.isError && (
        <p role='alert' className='text-destructive text-sm'>
          {query.error.message || t('usageLogs.sessions.loadFailed')}
        </p>
      )}
      {query.isPending && (
        <div className='flex h-40 items-center justify-center'>
          <Spinner />
        </div>
      )}
      {!query.isPending && !query.isError && sessions.length === 0 && (
        <Empty>
          <EmptyHeader>
            <EmptyTitle>{t('usageLogs.sessions.empty')}</EmptyTitle>
          </EmptyHeader>
        </Empty>
      )}
      {sessions.length > 0 && (
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{t('usageLogs.sessions.session')}</TableHead>
              {isAdmin && (
                <TableHead className='hidden md:table-cell'>
                  {t('Username')}
                </TableHead>
              )}
              <TableHead className='hidden md:table-cell'>
                {t('API Key')}
              </TableHead>
              <TableHead className='hidden md:table-cell'>
                {t('Model')}
              </TableHead>
              <TableHead>{t('usageLogs.sessions.currentGroup')}</TableHead>
              <TableHead className='hidden md:table-cell'>
                {t('usageLogs.sessions.lastSeen')}
              </TableHead>
              {isAdmin && (
                <TableHead className='text-right'>{t('Actions')}</TableHead>
              )}
            </TableRow>
          </TableHeader>
          <TableBody>
            {sessions.map((session) => (
              <TableRow key={session.id}>
                <TableCell>
                  <span
                    className='block max-w-36 truncate font-mono text-xs md:max-w-64'
                    title={session.session_key}
                  >
                    {session.session_key}
                  </span>
                  <div className='text-muted-foreground mt-1 flex max-w-36 flex-col gap-1 text-xs md:hidden'>
                    {isAdmin && (
                      <span className='truncate'>
                        {session.username || `#${session.user_id}`}
                      </span>
                    )}
                    <span className='truncate'>
                      {session.token_name || `#${session.token_id}`}
                    </span>
                    <span className='truncate'>{session.model_name}</span>
                    <span>
                      {dayjs
                        .unix(session.last_seen_at)
                        .format('MM-DD HH:mm:ss')}
                    </span>
                  </div>
                </TableCell>
                {isAdmin && (
                  <TableCell className='hidden md:table-cell'>
                    <span
                      className='block max-w-40 truncate'
                      title={session.username}
                    >
                      {session.username || `#${session.user_id}`}
                    </span>
                  </TableCell>
                )}
                <TableCell className='hidden md:table-cell'>
                  <span
                    className='block max-w-40 truncate'
                    title={session.token_name}
                  >
                    {session.token_name || `#${session.token_id}`}
                  </span>
                </TableCell>
                <TableCell className='hidden md:table-cell'>
                  <span
                    className='block max-w-48 truncate'
                    title={session.model_name}
                  >
                    {session.model_name}
                  </span>
                </TableCell>
                <TableCell>
                  <div className='flex flex-col items-start gap-2 md:flex-row md:items-center'>
                    <span
                      className='max-w-20 truncate md:max-w-40'
                      title={session.override_group || session.last_group}
                    >
                      {session.override_group || session.last_group}
                    </span>
                    {session.override_group && (
                      <Badge variant='secondary'>
                        {t('usageLogs.sessions.manual')}
                      </Badge>
                    )}
                  </div>
                </TableCell>
                <TableCell className='hidden md:table-cell'>
                  {dayjs.unix(session.last_seen_at).format('MM-DD HH:mm:ss')}
                </TableCell>
                {isAdmin && (
                  <TableCell className='text-right'>
                    <Tooltip>
                      <TooltipTrigger
                        render={
                          <Button
                            variant='ghost'
                            size='icon'
                            aria-label={t('usageLogs.sessions.changeGroup')}
                            onClick={() => setSelected(session)}
                          />
                        }
                      >
                        <HugeiconsIcon icon={PencilEdit02Icon} />
                      </TooltipTrigger>
                      <TooltipContent>
                        {t('usageLogs.sessions.changeGroup')}
                      </TooltipContent>
                    </Tooltip>
                  </TableCell>
                )}
              </TableRow>
            ))}
          </TableBody>
        </Table>
      )}
      <div className='flex items-center justify-end gap-2'>
        <Button
          variant='outline'
          size='icon'
          aria-label={t('Previous page')}
          disabled={page <= 1 || query.isFetching}
          onClick={() => setPage((value) => value - 1)}
        >
          <HugeiconsIcon icon={ArrowLeft01Icon} />
        </Button>
        <span className='min-w-16 text-center text-sm tabular-nums'>
          {page} / {pages}
        </span>
        <Button
          variant='outline'
          size='icon'
          aria-label={t('Next page')}
          disabled={page >= pages || query.isFetching}
          onClick={() => setPage((value) => value + 1)}
        >
          <HugeiconsIcon icon={ArrowRight01Icon} />
        </Button>
      </div>
      {isAdmin && selected && (
        <SessionGroupDialog
          key={selected.id}
          session={selected}
          onClose={() => setSelected(null)}
        />
      )}
    </div>
  )
}

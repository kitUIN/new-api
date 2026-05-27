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
  ChevronDownIcon,
  ImageIcon,
  ListIcon,
  PlusIcon,
  Trash2Icon,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from '@/components/ui/popover'
import { Spinner } from '@/components/ui/spinner'
import type { DrawingSession } from '../types'

type SessionSelectorProps = {
  sessions: DrawingSession[]
  activeSessionId: string | null
  loading: boolean
  onCreate: () => void
  onDelete: (session: DrawingSession) => void
  onSelect: (sessionId: string) => void
}

export function SessionSelector(props: SessionSelectorProps) {
  const { t } = useTranslation()

  return (
    <Popover>
      <PopoverTrigger
        render={
          <Button
            aria-label={t('Select session')}
            className='shrink-0 gap-1'
            type='button'
            variant='outline'
          />
        }
      >
        <ListIcon className='size-4' />
        <ChevronDownIcon className='size-3.5' />
      </PopoverTrigger>
      <PopoverContent
        align='start'
        className='w-[min(calc(100vw-2rem),22rem)] p-0'
      >
        <div className='flex items-center justify-between border-b px-3 py-2'>
          <span className='text-sm font-medium'>{t('Sessions')}</span>
          <Button
            aria-label={t('New session')}
            onClick={props.onCreate}
            size='icon-sm'
            type='button'
            variant='ghost'
          >
            <PlusIcon className='size-4' />
          </Button>
        </div>

        <div className='max-h-[min(60dvh,26rem)] overflow-auto p-2'>
          {props.loading ? (
            <div className='flex justify-center py-8'>
              <Spinner />
            </div>
          ) : props.sessions.length === 0 ? (
            <p className='text-muted-foreground py-8 text-center text-sm'>
              {t('No sessions yet')}
            </p>
          ) : (
            <div className='space-y-1'>
              {props.sessions.map((session) => {
                const isActive = props.activeSessionId === session.session_id
                return (
                  <button
                    className={cn(
                      'hover:bg-muted flex w-full items-center gap-2 rounded-lg px-2 py-2 text-left text-sm transition',
                      isActive && 'bg-primary/10 text-primary'
                    )}
                    key={session.session_id}
                    onClick={() => props.onSelect(session.session_id)}
                    type='button'
                  >
                    <ImageIcon className='size-4 shrink-0 opacity-70' />
                    <span className='min-w-0 flex-1 truncate'>
                      {session.title || t('Untitled session')}
                    </span>
                    <span className='bg-muted text-muted-foreground inline-flex h-6 min-w-9 shrink-0 items-center justify-center gap-1 rounded-md px-1.5 text-xs'>
                      <ImageIcon className='size-3' />
                      {Number(session.image_count || 0)}
                    </span>
                    <Button
                      aria-label={t('Delete session')}
                      className='text-destructive hover:text-destructive'
                      onClick={(event) => {
                        event.stopPropagation()
                        props.onDelete(session)
                      }}
                      size='icon-sm'
                      type='button'
                      variant='ghost'
                    >
                      <Trash2Icon className='size-3.5' />
                    </Button>
                  </button>
                )
              })}
            </div>
          )}
        </div>
      </PopoverContent>
    </Popover>
  )
}

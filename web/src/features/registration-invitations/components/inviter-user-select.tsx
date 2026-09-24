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
import { useDebounce } from '@/hooks'
import { ChevronsUpDown, Loader2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import {
  getQQAvatarUrl,
  getUserAvatarFallback,
  getUserAvatarStyle,
} from '@/lib/avatar'
import { Avatar, AvatarFallback, AvatarImage } from '@/components/ui/avatar'
import { Button } from '@/components/ui/button'
import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
} from '@/components/ui/command'
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from '@/components/ui/popover'
import { searchUsers } from '@/features/users/api'
import { isUserDeleted } from '@/features/users/constants'
import type { User } from '@/features/users/types'

const USER_PAGE_SIZE = 20

interface InviterUserSelectProps {
  value: string
  onValueChange: (value: string) => void
  onBlur?: () => void
  disabled?: boolean
  id?: string
  'aria-describedby'?: string
  'aria-invalid'?: boolean
  'data-slot'?: string
}

function UserAvatar({ user }: { user: User }) {
  const avatarName = user.display_name || user.username
  const avatarUrl = getQQAvatarUrl(user.qq_id)

  return (
    <Avatar size='sm'>
      {avatarUrl && <AvatarImage src={avatarUrl} alt={avatarName} />}
      <AvatarFallback
        className='font-semibold text-white'
        style={getUserAvatarStyle(avatarName)}
      >
        {getUserAvatarFallback(avatarName)}
      </AvatarFallback>
    </Avatar>
  )
}

function UserIdentity({ user }: { user: User }) {
  const nickname = user.display_name || user.username

  return (
    <span className='flex min-w-0 flex-col text-left'>
      <span className='truncate text-sm font-medium'>{nickname}</span>
      <span className='text-muted-foreground truncate text-xs'>
        {user.username} · ID {user.id}
      </span>
    </span>
  )
}

export function InviterUserSelect({
  value,
  onValueChange,
  onBlur,
  disabled,
  id,
  'aria-describedby': ariaDescribedBy,
  'aria-invalid': ariaInvalid,
  'data-slot': dataSlot,
}: InviterUserSelectProps) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  const [search, setSearch] = useState('')
  const [selectedUser, setSelectedUser] = useState<User>()
  const debouncedSearch = useDebounce(search.trim(), 300)

  const usersQuery = useQuery({
    queryKey: ['invitation-inviter-users', debouncedSearch],
    queryFn: async () => {
      const result = await searchUsers({
        keyword: debouncedSearch,
        p: 1,
        page_size: USER_PAGE_SIZE,
      })
      if (!result.success) {
        throw new Error(result.message || 'Failed to load users')
      }
      return (result.data?.items || []).filter((user) => !isUserDeleted(user))
    },
    enabled: open,
    staleTime: 30_000,
  })

  const users = usersQuery.data || []
  const selectedFromResults = users.find((user) => String(user.id) === value)
  const selected = value ? selectedFromResults || selectedUser : undefined

  const handleOpenChange = (nextOpen: boolean) => {
    setOpen(nextOpen)
    if (!nextOpen) setSearch('')
  }

  const handleSelect = (user: User) => {
    setSelectedUser(user)
    onValueChange(String(user.id))
    setOpen(false)
    setSearch('')
  }

  return (
    <Popover open={open} onOpenChange={handleOpenChange}>
      <PopoverTrigger
        render={
          <Button
            type='button'
            variant='outline'
            role='combobox'
            aria-expanded={open}
            aria-describedby={ariaDescribedBy}
            aria-invalid={ariaInvalid}
            data-slot={dataSlot}
            id={id}
            disabled={disabled}
            onBlur={onBlur}
            className='h-10 w-full justify-between gap-2 px-3 font-normal'
          />
        }
      >
        {selected ? (
          <span className='flex min-w-0 items-center gap-2'>
            <UserAvatar user={selected} />
            <UserIdentity user={selected} />
          </span>
        ) : (
          <span className='text-muted-foreground truncate'>
            {t('Select user')}
          </span>
        )}
        <ChevronsUpDown className='size-4 shrink-0 opacity-50' />
      </PopoverTrigger>
      <PopoverContent
        align='start'
        className='w-[var(--anchor-width)] overflow-hidden p-0'
      >
        <Command shouldFilter={false}>
          <CommandInput
            value={search}
            onValueChange={setSearch}
            placeholder={t('Search...')}
          />
          <CommandList>
            {usersQuery.isLoading || usersQuery.isFetching ? (
              <div className='text-muted-foreground flex items-center justify-center gap-2 py-6 text-sm'>
                <Loader2 className='size-4 animate-spin' />
                {t('Loading...')}
              </div>
            ) : usersQuery.isError ? (
              <div className='text-destructive py-6 text-center text-sm'>
                {t('Failed to load users')}
              </div>
            ) : (
              <>
                <CommandEmpty>{t('No users found')}</CommandEmpty>
                <CommandGroup>
                  {users.map((user) => (
                    <CommandItem
                      key={user.id}
                      value={`${user.display_name} ${user.username} ${user.id}`}
                      data-checked={value === String(user.id)}
                      onSelect={() => handleSelect(user)}
                      className='gap-2 py-2'
                    >
                      <UserAvatar user={user} />
                      <UserIdentity user={user} />
                    </CommandItem>
                  ))}
                </CommandGroup>
              </>
            )}
          </CommandList>
        </Command>
      </PopoverContent>
    </Popover>
  )
}

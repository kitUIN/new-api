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
import { useId } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { MultiSelect } from '@/components/multi-select'
import { CHANNEL_STATUS_CONFIG } from '@/features/channels/constants'
import { getGroupChannelOptions, setGroupChannels } from '../api'
import { useSystemOptions } from '../hooks/use-system-options'
import type { GroupBoundChannel } from '../types'
import { safeJsonParse } from '../utils/json-parser'

type Props = {
  group: string
  channels: GroupBoundChannel[]
  isLoading: boolean
  isError: boolean
  disabled: boolean
  draft?: string[]
  onDraftChange: (value: string[] | undefined) => void
}

export function GroupChannelsEditor(props: Props) {
  const { t } = useTranslation()
  const id = useId()
  const queryClient = useQueryClient()
  const optionsQuery = useQuery({
    queryKey: ['group-channel-options'],
    queryFn: getGroupChannelOptions,
  })
  const { data: settings } = useSystemOptions()
  const savedRatios = safeJsonParse<Record<string, number>>(
    settings?.data.find((option) => option.key === 'GroupRatio')?.value ?? '{}',
    { fallback: {}, silent: true }
  )
  const isSaved = Object.prototype.hasOwnProperty.call(savedRatios, props.group)
  const mutation = useMutation({
    mutationFn: (ids: string[]) =>
      setGroupChannels(props.group, ids.map(Number)),
    onSuccess: async () => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['channel-group-bindings'] }),
        queryClient.invalidateQueries({ queryKey: ['channels'] }),
        queryClient.invalidateQueries({ queryKey: ['pricing'] }),
        queryClient.invalidateQueries({ queryKey: ['groups'] }),
      ])
      props.onDraftChange(undefined)
      toast.success(t('Channel selection saved'))
    },
    onError: () => toast.error(t('Failed to save channel selection')),
  })
  const loading = props.isLoading || optionsQuery.isPending
  const failed = props.isError || optionsQuery.isError
  const disabled =
    props.disabled || !isSaved || loading || failed || mutation.isPending
  const selected =
    props.draft ?? props.channels.map((channel) => String(channel.id))

  return (
    <div className='space-y-3'>
      <Label htmlFor={id}>{t('Select channels')}</Label>
      <MultiSelect
        id={id}
        options={(optionsQuery.data ?? []).map((channel) => {
          const status =
            CHANNEL_STATUS_CONFIG[
              channel.status as keyof typeof CHANNEL_STATUS_CONFIG
            ] ?? CHANNEL_STATUS_CONFIG[0]
          return {
            value: String(channel.id),
            label: `${channel.name} #${channel.id} · ${t(status.label)}`,
          }
        })}
        selected={selected}
        onChange={props.onDraftChange}
        disabled={disabled}
        maxVisibleChips={5}
        placeholder={t('Search and select channels')}
      />
      {loading && (
        <p className='text-muted-foreground text-sm'>{t('Loading...')}</p>
      )}
      {failed && (
        <p role='alert' className='text-destructive text-sm'>
          {t('Failed to load channels')}
        </p>
      )}
      {props.disabled && (
        <p className='text-muted-foreground text-sm'>
          {t('Channels can only be assigned to billing groups.')}
        </p>
      )}
      {!isSaved && (
        <p className='text-muted-foreground text-sm'>
          {t('Save the group before assigning channels.')}
        </p>
      )}
      <p className='text-muted-foreground text-xs'>
        {t(
          'Save channel selection separately. Other group memberships are preserved.'
        )}
      </p>
      <Button
        type='button'
        variant='outline'
        disabled={disabled || props.draft === undefined}
        onClick={() => mutation.mutate(selected)}
      >
        {mutation.isPending ? t('Saving...') : t('Save channel selection')}
      </Button>
    </div>
  )
}

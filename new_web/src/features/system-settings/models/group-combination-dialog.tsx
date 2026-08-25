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
import { useEffect, useMemo, useState } from 'react'
import { Plus, Trash2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import type { GroupBoundChannel } from '../types'

export type GroupCombinationRoutes = Record<string, number>

type RouteDraft = {
  id: number
  model: string
  channelId: string
}

type GroupCombinationDialogProps = {
  open: boolean
  groupName: string
  routes: GroupCombinationRoutes
  channels: GroupBoundChannel[]
  onOpenChange: (open: boolean) => void
  onSave: (routes: GroupCombinationRoutes) => void
}

let nextRouteID = 1

function createRouteDraft(model = '', channelID = ''): RouteDraft {
  const route = { id: nextRouteID, model, channelId: channelID }
  nextRouteID += 1
  return route
}

export function GroupCombinationDialog(props: GroupCombinationDialogProps) {
  const { t } = useTranslation()
  const [drafts, setDrafts] = useState<RouteDraft[]>([])

  const sortedChannels = useMemo(
    () => [...props.channels].sort((left, right) => left.id - right.id),
    [props.channels]
  )
  const channelByID = useMemo(
    () => new Map(sortedChannels.map((channel) => [channel.id, channel])),
    [sortedChannels]
  )
  const exposedModels = useMemo(
    () =>
      Array.from(
        new Set(sortedChannels.flatMap((channel) => channel.models || []))
      ).sort(),
    [sortedChannels]
  )

  useEffect(() => {
    if (!props.open) return
    setDrafts(
      Object.entries(props.routes)
        .sort(([left], [right]) => left.localeCompare(right))
        .map(([model, channelID]) => createRouteDraft(model, String(channelID)))
    )
  }, [props.open, props.routes])

  const normalizedModels = drafts.map((draft) => draft.model.trim())
  const duplicateModels = new Set(
    normalizedModels.filter(
      (model, index) => model && normalizedModels.indexOf(model) !== index
    )
  )
  const invalidRouteIDs = new Set(
    drafts
      .filter((draft) => {
        const model = draft.model.trim()
        const channelID = Number(draft.channelId)
        const channel = channelByID.get(channelID)
        return (
          !model ||
          !channel ||
          channel.status !== 1 ||
          !(channel.models || []).includes(model) ||
          duplicateModels.has(model)
        )
      })
      .map((draft) => draft.id)
  )
  const canSave = drafts.length > 0 && invalidRouteIDs.size === 0

  const updateDraft = (
    id: number,
    field: 'model' | 'channelId',
    value: string
  ) => {
    setDrafts((current) =>
      current.map((draft) =>
        draft.id === id ? { ...draft, [field]: value } : draft
      )
    )
  }

  const handleChannelChange = (draft: RouteDraft, value: string) => {
    const channel = channelByID.get(Number(value))
    const nextModel =
      draft.model.trim() || channel?.models?.length !== 1
        ? draft.model
        : channel.models[0]
    setDrafts((current) =>
      current.map((item) =>
        item.id === draft.id
          ? { ...item, channelId: value, model: nextModel }
          : item
      )
    )
  }

  const handleSave = () => {
    if (!canSave) return
    const routes: GroupCombinationRoutes = {}
    for (const draft of drafts) {
      routes[draft.model.trim()] = Number(draft.channelId)
    }
    props.onSave(routes)
  }

  return (
    <Dialog open={props.open} onOpenChange={props.onOpenChange}>
      <DialogContent className='sm:max-w-3xl'>
        <DialogHeader>
          <DialogTitle>{t('Combination routes')}</DialogTitle>
          <DialogDescription>
            {t('{{group}} model routes', { group: props.groupName })}
          </DialogDescription>
        </DialogHeader>

        <datalist id='group-combination-models'>
          {exposedModels.map((model) => (
            <option key={model} value={model} />
          ))}
        </datalist>

        <div className='max-h-[55vh] space-y-3 overflow-y-auto pr-1'>
          {drafts.map((draft) => {
            const channel = channelByID.get(Number(draft.channelId))
            const model = draft.model.trim()
            const duplicate = duplicateModels.has(model)
            const unsupported =
              !!channel && !!model && !(channel.models || []).includes(model)

            return (
              <div
                key={draft.id}
                className='grid grid-cols-1 gap-2 border-b pb-3 last:border-b-0 sm:grid-cols-[minmax(0,1fr)_minmax(0,1.2fr)_2.25rem] sm:items-start'
              >
                <div className='space-y-1'>
                  <Input
                    list='group-combination-models'
                    value={draft.model}
                    placeholder={t('Model name')}
                    aria-invalid={invalidRouteIDs.has(draft.id)}
                    onChange={(event) =>
                      updateDraft(draft.id, 'model', event.target.value)
                    }
                  />
                  {duplicate && (
                    <p className='text-destructive text-xs'>
                      {t('Duplicate model route')}
                    </p>
                  )}
                  {unsupported && !duplicate && (
                    <p className='text-destructive text-xs'>
                      {t('Channel does not expose this model')}
                    </p>
                  )}
                </div>

                <Select
                  items={sortedChannels.map((item) => ({
                    value: String(item.id),
                    label: `#${item.id} ${item.name}`,
                  }))}
                  value={draft.channelId}
                  onValueChange={(value) => {
                    if (value === null) return
                    handleChannelChange(draft, value)
                  }}
                >
                  <SelectTrigger className='w-full' aria-invalid={!channel}>
                    <SelectValue placeholder={t('Select channel')} />
                  </SelectTrigger>
                  <SelectContent alignItemWithTrigger={false}>
                    <SelectGroup>
                      {sortedChannels.map((item) => (
                        <SelectItem
                          key={item.id}
                          value={String(item.id)}
                          disabled={item.status !== 1}
                        >
                          {`#${item.id} ${item.name}`}
                        </SelectItem>
                      ))}
                    </SelectGroup>
                  </SelectContent>
                </Select>

                <Button
                  type='button'
                  variant='ghost'
                  size='icon'
                  aria-label={t('Delete route')}
                  onClick={() =>
                    setDrafts((current) =>
                      current.filter((item) => item.id !== draft.id)
                    )
                  }
                >
                  <Trash2 className='h-4 w-4' />
                </Button>
              </div>
            )
          })}

          {drafts.length === 0 && (
            <p className='text-muted-foreground py-6 text-center text-sm'>
              {t('No model routes')}
            </p>
          )}
        </div>

        <DialogFooter className='gap-2 sm:justify-between'>
          <Button
            type='button'
            variant='outline'
            onClick={() =>
              setDrafts((current) => [...current, createRouteDraft()])
            }
          >
            <Plus className='mr-2 h-4 w-4' />
            {t('Add route')}
          </Button>
          <Button type='button' disabled={!canSave} onClick={handleSave}>
            {t('Save routes')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

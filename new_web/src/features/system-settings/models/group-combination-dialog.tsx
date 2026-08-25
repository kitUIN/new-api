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
import { ArrowDown, ArrowUp, Plus, Trash2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { MultiSelect } from '@/components/multi-select'

export type GroupCombinationMember = {
  group: string
  models: string[]
}

export type GroupCombinationMembers = GroupCombinationMember[]

export type GroupCombinationOption = {
  name: string
  ratio: number
  models: string[]
}

type MemberGroupDraft = {
  id: number
  group: string
  models: string[]
}

type GroupCombinationDialogProps = {
  open: boolean
  groupName: string
  memberGroups: GroupCombinationMembers
  groups: GroupCombinationOption[]
  onOpenChange: (open: boolean) => void
  onSave: (memberGroups: GroupCombinationMembers) => void
}

let nextDraftID = 1

function createMemberGroupDraft(
  group = '',
  models: string[] = []
): MemberGroupDraft {
  const draft = { id: nextDraftID, group, models }
  nextDraftID += 1
  return draft
}

export function normalizeGroupCombinationMembers(
  value: unknown
): GroupCombinationMembers {
  if (!Array.isArray(value)) return []
  return value.flatMap((member) => {
    if (!member || typeof member !== 'object' || Array.isArray(member)) {
      return []
    }
    const group = (member as { group?: unknown }).group
    const models = (member as { models?: unknown }).models
    if (typeof group !== 'string' || !Array.isArray(models)) return []
    return [
      {
        group,
        models: models.filter(
          (model): model is string =>
            typeof model === 'string' && model.trim() !== ''
        ),
      },
    ]
  })
}

export function GroupCombinationDialog(props: GroupCombinationDialogProps) {
  const { t } = useTranslation()
  const [drafts, setDrafts] = useState<MemberGroupDraft[]>([])

  const sortedGroups = useMemo(
    () =>
      [...props.groups].sort((left, right) =>
        left.name.localeCompare(right.name)
      ),
    [props.groups]
  )
  const groupByName = useMemo(
    () => new Map(sortedGroups.map((group) => [group.name, group])),
    [sortedGroups]
  )

  useEffect(() => {
    if (!props.open) return
    setDrafts(
      normalizeGroupCombinationMembers(props.memberGroups).map((member) =>
        createMemberGroupDraft(member.group, member.models)
      )
    )
  }, [props.memberGroups, props.open])

  const duplicateGroups = useMemo(() => {
    const seen = new Set<string>()
    const duplicates = new Set<string>()
    for (const draft of drafts) {
      if (!draft.group) continue
      if (seen.has(draft.group)) duplicates.add(draft.group)
      seen.add(draft.group)
    }
    return duplicates
  }, [drafts])

  const canSave =
    drafts.length >= 2 &&
    drafts.every(
      (draft) =>
        groupByName.has(draft.group) &&
        !duplicateGroups.has(draft.group) &&
        draft.models.length > 0 &&
        draft.models.every((model) =>
          groupByName.get(draft.group)?.models.includes(model)
        )
    )

  const updateDraftGroup = (id: number, group: string) => {
    const supportedModels = new Set(groupByName.get(group)?.models ?? [])
    setDrafts((current) =>
      current.map((draft) =>
        draft.id === id
          ? {
              ...draft,
              group,
              models: draft.models.filter((model) =>
                supportedModels.has(model)
              ),
            }
          : draft
      )
    )
  }

  const updateDraftModels = (id: number, models: string[]) => {
    setDrafts((current) =>
      current.map((draft) => (draft.id === id ? { ...draft, models } : draft))
    )
  }

  const moveDraft = (index: number, direction: -1 | 1) => {
    setDrafts((current) => {
      const targetIndex = index + direction
      if (targetIndex < 0 || targetIndex >= current.length) return current
      const next = [...current]
      ;[next[index], next[targetIndex]] = [next[targetIndex], next[index]]
      return next
    })
  }

  const addDraft = () => {
    const selectedGroups = new Set(drafts.map((draft) => draft.group))
    const nextGroup = sortedGroups.find(
      (group) => !selectedGroups.has(group.name)
    )
    setDrafts((current) => [
      ...current,
      createMemberGroupDraft(nextGroup?.name ?? ''),
    ])
  }

  return (
    <Dialog open={props.open} onOpenChange={props.onOpenChange}>
      <DialogContent className='sm:max-w-2xl'>
        <DialogHeader>
          <DialogTitle>{t('Member groups')}</DialogTitle>
          <DialogDescription>
            {t('{{group}} member group and model priority', {
              group: props.groupName,
            })}
          </DialogDescription>
        </DialogHeader>

        <div className='max-h-[55vh] space-y-2 overflow-y-auto pr-1'>
          {drafts.map((draft, index) => {
            const selectedGroup = groupByName.get(draft.group)
            return (
              <div
                key={draft.id}
                className='grid min-w-0 grid-cols-[2.5rem_minmax(0,1fr)_auto] items-center gap-2 border-b pb-3 last:border-b-0 sm:grid-cols-[3rem_minmax(0,0.8fr)_5rem_minmax(0,1.2fr)_6.75rem] sm:gap-3'
              >
                <span className='text-muted-foreground text-xs font-medium'>
                  #{index + 1}
                </span>

                <Select
                  items={sortedGroups.map((group) => ({
                    value: group.name,
                    label:
                      group.name === props.groupName
                        ? `${group.name} (${t('Original group')})`
                        : group.name,
                  }))}
                  value={draft.group}
                  onValueChange={(value) => {
                    if (value !== null) updateDraftGroup(draft.id, value)
                  }}
                >
                  <SelectTrigger
                    className='w-full min-w-0'
                    aria-invalid={
                      !selectedGroup || duplicateGroups.has(draft.group)
                    }
                  >
                    <SelectValue placeholder={t('Select group')} />
                  </SelectTrigger>
                  <SelectContent alignItemWithTrigger={false}>
                    <SelectGroup>
                      {sortedGroups.map((group) => (
                        <SelectItem key={group.name} value={group.name}>
                          {group.name === props.groupName
                            ? `${group.name} (${t('Original group')})`
                            : group.name}
                        </SelectItem>
                      ))}
                    </SelectGroup>
                  </SelectContent>
                </Select>

                <Badge
                  variant='secondary'
                  className='hidden justify-center sm:flex'
                >
                  {selectedGroup ? `${selectedGroup.ratio}x` : '-'}
                </Badge>

                <MultiSelect
                  className='col-start-2 col-end-4 min-w-0 sm:col-auto'
                  options={(selectedGroup?.models ?? []).map((model) => ({
                    label: model,
                    value: model,
                  }))}
                  selected={draft.models}
                  onChange={(models) => updateDraftModels(draft.id, models)}
                  placeholder={t('Select models')}
                  disabled={!selectedGroup}
                  maxVisibleChips={2}
                />

                <div className='col-start-3 row-start-1 flex items-center justify-end sm:col-auto sm:row-auto'>
                  <Button
                    type='button'
                    variant='ghost'
                    size='icon'
                    disabled={index === 0}
                    title={t('Move group up')}
                    aria-label={t('Move group up')}
                    onClick={() => moveDraft(index, -1)}
                  >
                    <ArrowUp className='h-4 w-4' />
                  </Button>
                  <Button
                    type='button'
                    variant='ghost'
                    size='icon'
                    disabled={index === drafts.length - 1}
                    title={t('Move group down')}
                    aria-label={t('Move group down')}
                    onClick={() => moveDraft(index, 1)}
                  >
                    <ArrowDown className='h-4 w-4' />
                  </Button>
                  <Button
                    type='button'
                    variant='ghost'
                    size='icon'
                    title={t('Remove group')}
                    aria-label={t('Remove group')}
                    onClick={() =>
                      setDrafts((current) =>
                        current.filter((item) => item.id !== draft.id)
                      )
                    }
                  >
                    <Trash2 className='h-4 w-4' />
                  </Button>
                </div>
              </div>
            )
          })}

          {drafts.length === 0 && (
            <p className='text-muted-foreground py-6 text-center text-sm'>
              {t('No member groups')}
            </p>
          )}
        </div>

        <DialogFooter className='gap-2 sm:justify-between'>
          <Button
            type='button'
            variant='outline'
            disabled={drafts.length >= sortedGroups.length}
            onClick={addDraft}
          >
            <Plus className='mr-2 h-4 w-4' />
            {t('Add group')}
          </Button>
          <Button
            type='button'
            disabled={!canSave}
            onClick={() =>
              props.onSave(
                drafts.map((draft) => ({
                  group: draft.group,
                  models: draft.models,
                }))
              )
            }
          >
            {t('Save')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

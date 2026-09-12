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
import { z } from 'zod'
import { useController, useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Field, FieldGroup, FieldLabel } from '@/components/ui/field'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Spinner } from '@/components/ui/spinner'
import {
  getRelaySessionGroups,
  updateRelaySessionGroup,
  type RelaySession,
} from '../../session-api'

interface SessionGroupDialogProps {
  session: RelaySession
  onClose: () => void
}

const sessionGroupSchema = z.object({ group: z.string().min(1) })

export function SessionGroupDialog(props: SessionGroupDialogProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const form = useForm<z.infer<typeof sessionGroupSchema>>({
    resolver: zodResolver(sessionGroupSchema),
    defaultValues: {
      group: props.session.override_group || props.session.last_group,
    },
  })
  const groupField = useController({ name: 'group', control: form.control })
  const group = groupField.field.value
  const groups = useQuery({
    queryKey: ['relay-session-groups', props.session.id],
    queryFn: () => getRelaySessionGroups(props.session.id),
  })
  const mutation = useMutation({
    mutationFn: (selectedGroup: string) =>
      updateRelaySessionGroup(props.session.id, selectedGroup),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ['relay-sessions'] })
      toast.success(t('usageLogs.sessions.updated'))
      props.onClose()
    },
  })
  const items = (groups.data ?? []).map((value) => ({ label: value, value }))
  const canSave = Boolean(group && groups.data?.includes(group))

  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open && !mutation.isPending) props.onClose()
      }}
    >
      <DialogContent className='sm:max-w-md'>
        <DialogHeader>
          <DialogTitle>{t('usageLogs.sessions.changeGroup')}</DialogTitle>
          <DialogDescription className='break-all'>
            {props.session.username} / {props.session.session_key}
          </DialogDescription>
        </DialogHeader>
        <form
          id='session-group-form'
          onSubmit={form.handleSubmit((values) =>
            mutation.mutate(values.group)
          )}
        >
          <FieldGroup>
            <Field>
              <FieldLabel htmlFor='session-group'>{t('Group')}</FieldLabel>
              <Select
                items={items}
                value={group}
                onValueChange={(value) => {
                  if (value !== null) groupField.field.onChange(value)
                }}
                disabled={groups.isPending || mutation.isPending}
              >
                <SelectTrigger id='session-group' className='w-full'>
                  <SelectValue placeholder={t('Select a group')} />
                </SelectTrigger>
                <SelectContent>
                  <SelectGroup>
                    {items.map((item) => (
                      <SelectItem key={item.value} value={item.value}>
                        {item.label}
                      </SelectItem>
                    ))}
                  </SelectGroup>
                </SelectContent>
              </Select>
            </Field>
          </FieldGroup>
        </form>
        {groups.isPending && <Spinner />}
        {groups.isError && (
          <p role='alert' className='text-destructive text-sm'>
            {groups.error.message || t('usageLogs.sessions.loadFailed')}
          </p>
        )}
        {groups.data?.length === 0 && (
          <p className='text-muted-foreground text-sm'>
            {t('usageLogs.sessions.noGroups')}
          </p>
        )}
        {mutation.isError && (
          <p role='alert' className='text-destructive text-sm'>
            {mutation.error.message || t('Failed to update group')}
          </p>
        )}
        <DialogFooter className='flex-wrap'>
          {props.session.override_group && (
            <Button
              variant='outline'
              disabled={mutation.isPending}
              onClick={() => mutation.mutate('')}
            >
              {t('usageLogs.sessions.resetGroup')}
            </Button>
          )}
          <Button
            variant='outline'
            disabled={mutation.isPending}
            onClick={props.onClose}
          >
            {t('Cancel')}
          </Button>
          <Button
            type='submit'
            form='session-group-form'
            disabled={!canSave || mutation.isPending}
          >
            {mutation.isPending && <Spinner data-icon='inline-start' />}
            {t('Confirm')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

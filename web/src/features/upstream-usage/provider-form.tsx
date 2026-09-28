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
import { Controller, useFieldArray, useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { handleServerError } from '@/lib/handle-server-error'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { MultiSelect } from '@/components/multi-select'
import { getGroups } from '@/features/users/api'
import { saveUpstreamUsage } from './api'
import { usageProviderSchema } from './schema'
import type { UsageProvider, UsageProviderInput } from './types'

export function ProviderForm(props: {
  provider?: UsageProvider
  onClose: () => void
}) {
  const { t } = useTranslation()
  const prefix = useId()
  const client = useQueryClient()
  const form = useForm<UsageProviderInput>({
    resolver: zodResolver(usageProviderSchema),
    defaultValues: {
      base_url: props.provider?.base_url ?? '',
      access_token: '',
      refresh_token: '',
      interval_minutes: props.provider?.interval_minutes ?? 5,
      accounts:
        props.provider?.accounts.map((a) => ({
          account_id: a.account_id,
          groups: a.groups,
        })) ?? [],
    },
  })
  const accounts = useFieldArray({ control: form.control, name: 'accounts' })
  const groups = useQuery({
    queryKey: ['upstream-usage-groups'],
    queryFn: () => getGroups(),
  })
  const save = useMutation({
    mutationFn: (value: UsageProviderInput) =>
      saveUpstreamUsage(props.provider?.id, value),
    onSuccess: () => {
      void client.invalidateQueries({ queryKey: ['upstream-usage'] })
      void client.invalidateQueries({ queryKey: ['group-upstream-usage'] })
      toast.success(t('upstreamUsage.saved'))
      props.onClose()
    },
    onError: handleServerError,
  })
  const submit = form.handleSubmit((value) => {
    if (
      !props.provider ||
      value.base_url.replace(/\/+$/, '') !== props.provider.base_url
    ) {
      if (!value.access_token || !value.refresh_token) {
        form.setError('root', { message: t('upstreamUsage.tokensRequired') })
        return
      }
    }
    save.mutate(value)
  })
  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open && !save.isPending) props.onClose()
      }}
    >
      <DialogContent className='max-h-[90vh] overflow-y-auto sm:max-w-2xl'>
        <DialogHeader>
          <DialogTitle>{t('upstreamUsage.configure')}</DialogTitle>
          <DialogDescription>
            {t('upstreamUsage.credentialsHint')}
          </DialogDescription>
        </DialogHeader>
        <form onSubmit={submit} className='space-y-4'>
          <fieldset disabled={save.isPending} className='space-y-4'>
            <div className='space-y-1'>
              <label htmlFor={`${prefix}-url`}>Base URL</label>
              <Input
                id={`${prefix}-url`}
                placeholder='https://sub2api.example.com'
                {...form.register('base_url')}
              />
            </div>
            <div className='grid gap-3 sm:grid-cols-2'>
              <div className='space-y-1'>
                <label htmlFor={`${prefix}-access`}>access_token</label>
                <Input
                  id={`${prefix}-access`}
                  type='password'
                  autoComplete='new-password'
                  {...form.register('access_token')}
                />
              </div>
              <div className='space-y-1'>
                <label htmlFor={`${prefix}-refresh`}>refresh_token</label>
                <Input
                  id={`${prefix}-refresh`}
                  type='password'
                  autoComplete='new-password'
                  {...form.register('refresh_token')}
                />
              </div>
            </div>
            <div className='space-y-1'>
              <label htmlFor={`${prefix}-interval`}>
                {t('upstreamUsage.interval')}
              </label>
              <Input
                id={`${prefix}-interval`}
                type='number'
                min={1}
                max={1440}
                {...form.register('interval_minutes', { valueAsNumber: true })}
              />
            </div>
            <div className='flex items-center justify-between'>
              <h3 className='font-medium'>{t('upstreamUsage.accounts')}</h3>
              <Button
                type='button'
                variant='outline'
                onClick={() => accounts.append({ account_id: 1, groups: [] })}
              >
                {t('upstreamUsage.addAccount')}
              </Button>
            </div>
            {accounts.fields.map((field, index) => (
              <div key={field.id} className='space-y-3 rounded-lg border p-3'>
                <div className='flex items-end gap-3'>
                  <div className='flex-1 space-y-1'>
                    <label htmlFor={`${prefix}-account-${field.id}`}>
                      account_id
                    </label>
                    <Input
                      id={`${prefix}-account-${field.id}`}
                      type='number'
                      min={1}
                      {...form.register(`accounts.${index}.account_id`, {
                        valueAsNumber: true,
                      })}
                    />
                  </div>
                  <Button
                    type='button'
                    variant='outline'
                    onClick={() => accounts.remove(index)}
                  >
                    {t('upstreamUsage.removeAccount')}
                  </Button>
                </div>
                <label
                  htmlFor={`${prefix}-groups-${field.id}`}
                  className='text-sm'
                >
                  {t('upstreamUsage.bindGroups')}
                </label>
                <Controller
                  control={form.control}
                  name={`accounts.${index}.groups`}
                  render={({ field: groupField }) => (
                    <MultiSelect
                      id={`${prefix}-groups-${field.id}`}
                      options={[
                        ...new Set([
                          ...(groups.data?.data ?? []),
                          ...groupField.value,
                        ]),
                      ].map((group) => ({ label: group, value: group }))}
                      selected={groupField.value}
                      onChange={groupField.onChange}
                      placeholder={t('upstreamUsage.bindGroups')}
                    />
                  )}
                />
              </div>
            ))}
            {Object.keys(form.formState.errors).length > 0 && (
              <p role='alert' className='text-destructive text-sm'>
                {form.formState.errors.root?.message ??
                  t('upstreamUsage.invalidForm')}
              </p>
            )}
            <div className='flex justify-end gap-2'>
              <Button type='button' variant='outline' onClick={props.onClose}>
                {t('Cancel')}
              </Button>
              <Button type='submit' disabled={save.isPending}>
                {t('Save')}
              </Button>
            </div>
          </fieldset>
        </form>
      </DialogContent>
    </Dialog>
  )
}

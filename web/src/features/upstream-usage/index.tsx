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
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { handleServerError } from '@/lib/handle-server-error'
import { Button } from '@/components/ui/button'
import { ConfirmDialog } from '@/components/confirm-dialog'
import {
  deleteUpstreamUsage,
  getUpstreamUsage,
  refreshUpstreamUsage,
} from './api'
import { ProviderForm } from './provider-form'
import type { UsageProvider } from './types'
import { UsageBattery } from './usage-battery'
import { UsageDetails } from './usage-details'

export function UpstreamUsage() {
  const { t } = useTranslation()
  const client = useQueryClient()
  const [editing, setEditing] = useState<UsageProvider | 'new' | null>(null)
  const [deleting, setDeleting] = useState<UsageProvider | null>(null)
  const query = useQuery({
    queryKey: ['upstream-usage'],
    queryFn: getUpstreamUsage,
    refetchInterval: 30_000,
  })
  const refresh = useMutation({
    mutationFn: refreshUpstreamUsage,
    onSuccess: (data) => {
      client.setQueryData(['upstream-usage'], data)
      void client.invalidateQueries({ queryKey: ['group-upstream-usage'] })
    },
    onError: handleServerError,
  })
  const remove = useMutation({
    mutationFn: deleteUpstreamUsage,
    onSuccess: () => {
      setDeleting(null)
      void client.invalidateQueries({ queryKey: ['upstream-usage'] })
      void client.invalidateQueries({ queryKey: ['group-upstream-usage'] })
    },
    onError: handleServerError,
  })
  return (
    <div className='space-y-4'>
      <div className='flex flex-wrap items-center justify-between gap-3'>
        <p className='text-muted-foreground text-sm'>
          {t('upstreamUsage.description')}
        </p>
        <Button onClick={() => setEditing('new')}>
          {t('upstreamUsage.addProvider')}
        </Button>
      </div>
      {query.isLoading && <p>{t('Loading...')}</p>}
      {query.isError && (
        <div role='alert'>
          {t('upstreamUsage.loadFailed')}{' '}
          <Button variant='outline' onClick={() => void query.refetch()}>
            {t('Retry')}
          </Button>
        </div>
      )}
      {query.data?.length === 0 && (
        <p className='text-muted-foreground rounded-lg border p-8 text-center'>
          {t('upstreamUsage.empty')}
        </p>
      )}
      {query.data?.map((provider) => (
        <section key={provider.id} className='bg-card rounded-lg border'>
          <header className='flex flex-wrap items-center justify-between gap-3 border-b p-4'>
            <div className='min-w-0'>
              <h3 className='truncate font-medium'>{provider.base_url}</h3>
              <p className='text-muted-foreground text-xs'>
                {t('upstreamUsage.everyMinutes', {
                  minutes: provider.interval_minutes,
                })}
              </p>
            </div>
            <div className='flex gap-2'>
              <Button
                variant='outline'
                disabled={refresh.isPending}
                onClick={() => refresh.mutate(provider.id)}
              >
                {t('upstreamUsage.refresh')}
              </Button>
              <Button variant='outline' onClick={() => setEditing(provider)}>
                {t('Edit')}
              </Button>
              <Button
                variant='destructive'
                onClick={() => setDeleting(provider)}
              >
                {t('Delete')}
              </Button>
            </div>
          </header>
          <div className='grid gap-3 p-4 md:grid-cols-2 xl:grid-cols-3'>
            {!provider.accounts.length && (
              <p className='text-muted-foreground text-sm'>
                {t('upstreamUsage.noAccounts')}
              </p>
            )}
            {provider.accounts.map((account) => (
              <div key={account.id} className='rounded-lg border p-3'>
                <div className='flex items-center justify-between gap-2'>
                  <span className='text-muted-foreground truncate text-xs'>
                    {account.groups.join(', ') || t('upstreamUsage.noBinding')}
                  </span>
                  <UsageBattery
                    accounts={[{ ...account, base_url: provider.base_url }]}
                  />
                </div>
                <UsageDetails account={account} />
              </div>
            ))}
          </div>
        </section>
      ))}
      {editing !== null && (
        <ProviderForm
          provider={editing === 'new' ? undefined : editing}
          onClose={() => setEditing(null)}
        />
      )}
      <ConfirmDialog
        open={deleting !== null}
        onOpenChange={(open) => {
          if (!open) setDeleting(null)
        }}
        title={t('upstreamUsage.deleteTitle')}
        desc={t('upstreamUsage.deleteDescription', { url: deleting?.base_url })}
        destructive
        isLoading={remove.isPending}
        handleConfirm={() => {
          if (deleting) remove.mutate(deleting.id)
        }}
      />
    </div>
  )
}

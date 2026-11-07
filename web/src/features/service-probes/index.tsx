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
import { useQuery, useQueryClient } from '@tanstack/react-query'
import {
  Clock01Icon,
  InformationCircleIcon,
  RefreshIcon,
  ServerStack01Icon,
  Shield01Icon,
} from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useTranslation } from 'react-i18next'
import { useAuthStore } from '@/stores/auth-store'
import { ROLE } from '@/lib/roles'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyTitle,
} from '@/components/ui/empty'
import { Input } from '@/components/ui/input'
import { Skeleton } from '@/components/ui/skeleton'
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'
import { SectionPageLayout } from '@/components/layout'
import { getProbes } from './api'
import { BeeCard } from './bee-card'
import { ProbeHint } from './hint'
import { formatTime } from './lib'
import type { ProbeWindow } from './types'

export function ServiceProbes() {
  const { t, i18n } = useTranslation()
  const user = useAuthStore((state) => state.auth.user)
  const queryClient = useQueryClient()
  const [window, setWindow] = useState<ProbeWindow>('24h')
  const [search, setSearch] = useState('')
  const query = useQuery({
    queryKey: ['service-probes', user?.id, 'overview'],
    queryFn: ({ signal }) => getProbes(signal),
    enabled: !!user && user.role >= ROLE.ADMIN,
    refetchInterval: 30_000,
    retry: false,
    gcTime: 0,
  })
  if (!user || user.role < ROLE.ADMIN) return null
  const bees = query.data?.quotas?.bees ?? []
  const filtered = bees.filter((bee) =>
    [bee.name, bee.id, bee.region, bee.ip].some((value) =>
      value?.toLowerCase().includes(search.trim().toLowerCase())
    )
  )
  const online = bees.filter((bee) => bee.online).length
  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>
        {t('serviceProbe.title')}
      </SectionPageLayout.Title>
      <SectionPageLayout.Actions>
        <ProbeHint
          label={t('serviceProbe.adminOnly')}
          className='text-muted-foreground'
        >
          <HugeiconsIcon icon={Shield01Icon} size={16} aria-hidden='true' />
        </ProbeHint>
        <Button
          size='icon-sm'
          variant='ghost'
          aria-label={t('serviceProbe.refresh')}
          title={t('serviceProbe.refresh')}
          disabled={query.isFetching}
          onClick={() =>
            void queryClient.invalidateQueries({
              queryKey: ['service-probes', user.id],
            })
          }
        >
          <HugeiconsIcon icon={RefreshIcon} aria-hidden='true' />
        </Button>
      </SectionPageLayout.Actions>
      <SectionPageLayout.Content>
        <div className='flex flex-col gap-3'>
          <div className='flex flex-wrap items-center justify-between gap-2'>
            <Input
              className='w-full sm:w-64'
              value={search}
              onChange={(event) => setSearch(event.target.value)}
              placeholder={t('serviceProbe.search')}
              aria-label={t('serviceProbe.search')}
            />
            <div className='flex items-center gap-2'>
              <ToggleGroup
                variant='outline'
                size='sm'
                value={[window]}
                onValueChange={(values) => {
                  if (values[0]) setWindow(values[0] as ProbeWindow)
                }}
                aria-label={t('serviceProbe.window')}
              >
                <ToggleGroupItem value='1h' aria-label={t('serviceProbe.hour')}>
                  1h
                </ToggleGroupItem>
                <ToggleGroupItem value='24h' aria-label={t('serviceProbe.day')}>
                  24h
                </ToggleGroupItem>
                <ToggleGroupItem value='7d' aria-label={t('serviceProbe.week')}>
                  7d
                </ToggleGroupItem>
              </ToggleGroup>
              <ProbeHint
                label={`${t('serviceProbe.refreshHint')} ${t('serviceProbe.historyHint')}`}
                className='text-muted-foreground'
              >
                <HugeiconsIcon
                  icon={InformationCircleIcon}
                  size={16}
                  aria-hidden='true'
                />
              </ProbeHint>
            </div>
          </div>
          {query.isError && (
            <Alert variant='destructive'>
              <AlertTitle>{t('serviceProbe.loadFailed')}</AlertTitle>
              <AlertDescription>
                {t(query.error.message, {
                  defaultValue: t('serviceProbe.connectionFailed'),
                })}
              </AlertDescription>
            </Alert>
          )}
          {query.isPending && (
            <div className='grid grid-cols-[repeat(auto-fill,minmax(min(100%,22rem),1fr))] gap-3'>
              {[0, 1, 2].map((key) => (
                <Skeleton key={key} className='h-64 rounded-xl' />
              ))}
            </div>
          )}
          {query.data?.configured === false && (
            <Empty>
              <EmptyHeader>
                <EmptyTitle>{t('serviceProbe.notConfigured')}</EmptyTitle>
                <EmptyDescription>
                  {t('serviceProbe.setupHint')}
                </EmptyDescription>
              </EmptyHeader>
            </Empty>
          )}
          {query.data?.configured && (
            <>
              <div className='text-muted-foreground flex flex-wrap items-center justify-between gap-2 text-xs tabular-nums'>
                <div className='flex items-center gap-3'>
                  <ProbeHint
                    label={t('serviceProbe.beeCount', { count: bees.length })}
                    className='flex items-center gap-1'
                  >
                    <HugeiconsIcon
                      icon={ServerStack01Icon}
                      size={14}
                      aria-hidden='true'
                    />
                    {bees.length}
                  </ProbeHint>
                  <ProbeHint
                    label={t('serviceProbe.online')}
                    className='flex items-center gap-1'
                  >
                    <span className='bg-success size-1.5 rounded-full' />
                    {query.isError ? '—' : online}
                  </ProbeHint>
                  <ProbeHint
                    label={t('serviceProbe.offline')}
                    className='flex items-center gap-1'
                  >
                    <span className='bg-destructive size-1.5 rounded-full' />
                    {query.isError ? '—' : bees.length - online}
                  </ProbeHint>
                  <ProbeHint label={t('serviceProbe.unknown')}>
                    <span className='bg-muted-foreground/40 block size-1.5 rounded-full' />
                  </ProbeHint>
                </div>
                <ProbeHint
                  label={t('serviceProbe.updatedAt', {
                    time: formatTime(query.data.quotas?.asOf, i18n.language),
                  })}
                  className='flex items-center gap-1'
                >
                  <HugeiconsIcon
                    icon={Clock01Icon}
                    size={12}
                    aria-hidden='true'
                  />
                  {query.data.quotas?.asOf
                    ? new Date(query.data.quotas.asOf).toLocaleTimeString(
                        i18n.language
                      )
                    : '—'}
                </ProbeHint>
              </div>
              {filtered.length === 0 ? (
                <Empty>
                  <EmptyHeader>
                    <EmptyTitle>{t('serviceProbe.noBees')}</EmptyTitle>
                    <EmptyDescription>
                      {t('serviceProbe.noBeesHint')}
                    </EmptyDescription>
                  </EmptyHeader>
                </Empty>
              ) : (
                <div className='grid grid-cols-[repeat(auto-fill,minmax(min(100%,22rem),1fr))] items-start gap-3'>
                  {filtered.map((bee) => (
                    <BeeCard
                      key={bee.id}
                      bee={bee}
                      window={window}
                      userId={user.id}
                      stale={query.isError}
                    />
                  ))}
                </div>
              )}
            </>
          )}
        </div>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}

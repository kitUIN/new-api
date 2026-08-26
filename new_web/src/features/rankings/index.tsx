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
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useNavigate, useSearch } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Skeleton } from '@/components/ui/skeleton'
import { PublicLayout } from '@/components/layout'
import { PageTransition } from '@/components/page-transition'
import { updateRankingPrivacy } from './api'
import {
  GroupsRankingSection,
  ModelsSection,
  RankingsHero,
  UsersRankingSection,
} from './components'
import { useRankings } from './hooks/use-rankings'
import type {
  RankingCustomRange,
  RankingPeriod,
  UserRankingMetric,
} from './types'

const VALID_PERIODS: RankingPeriod[] = [
  'today',
  'yesterday',
  'week',
  'last_week',
  'month',
  'last_month',
  'year',
  'all',
  'custom',
]
const VALID_USER_METRICS: UserRankingMetric[] = ['tokens', 'quota']

export function Rankings() {
  const { t } = useTranslation()
  const search = useSearch({ from: '/rankings/' })
  const navigate = useNavigate()
  const queryClient = useQueryClient()

  const period: RankingPeriod = VALID_PERIODS.includes(
    search.period as RankingPeriod
  )
    ? (search.period as RankingPeriod)
    : 'week'
  const initialUserMetric: UserRankingMetric = VALID_USER_METRICS.includes(
    search.user_metric as UserRankingMetric
  )
    ? (search.user_metric as UserRankingMetric)
    : 'tokens'
  const [userMetric, setUserMetric] =
    useState<UserRankingMetric>(initialUserMetric)

  const customRange: RankingCustomRange | undefined =
    period === 'custom' &&
    search.start_time &&
    search.end_time &&
    search.start_time <= search.end_time
      ? { start_time: search.start_time, end_time: search.end_time }
      : undefined

  const rankingsQuery = useRankings(period, userMetric, customRange)
  const isRankingsLoading =
    rankingsQuery.isLoading || rankingsQuery.isPlaceholderData
  const snapshot = isRankingsLoading ? undefined : rankingsQuery.data?.data
  const rankingPrivacyMutation = useMutation({
    mutationFn: updateRankingPrivacy,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['rankings'] })
    },
    onError: () => {
      toast.error(t('Failed to update settings'))
    },
  })

  const handlePeriodChange = (next: RankingPeriod) => {
    if (next === 'custom') {
      const end = new Date()
      const start = new Date(end)
      start.setDate(start.getDate() - 6)
      start.setHours(0, 0, 0, 0)
      navigate({
        to: '/rankings',
        search: (prev) => ({
          ...prev,
          period: next,
          start_time:
            customRange?.start_time ?? Math.floor(start.getTime() / 1000),
          end_time: customRange?.end_time ?? Math.floor(end.getTime() / 1000),
        }),
      })
      return
    }
    navigate({
      to: '/rankings',
      search: (prev) => ({
        ...prev,
        period: next,
        start_time: undefined,
        end_time: undefined,
      }),
    })
  }

  const handleCustomRangeChange = (range: { start?: Date; end?: Date }) => {
    if (!range.start || !range.end) {
      toast.error(t('Please select a complete time range'))
      return
    }
    const startTime = Math.floor(range.start.getTime() / 1000)
    const endTime = Math.min(
      Math.floor(range.end.getTime() / 1000),
      Math.floor(Date.now() / 1000)
    )
    if (startTime > endTime) {
      toast.error(t('Start time must not be after end time'))
      return
    }
    navigate({
      to: '/rankings',
      search: (prev) => ({
        ...prev,
        period: 'custom',
        start_time: startTime,
        end_time: endTime,
      }),
    })
  }

  const handleUserMetricChange = (next: UserRankingMetric) => {
    setUserMetric(next)
  }

  return (
    <PublicLayout showMainContainer={false}>
      <div className='relative'>
        <div
          aria-hidden
          className='pointer-events-none absolute inset-x-0 top-0 h-[600px] opacity-20 dark:opacity-[0.10]'
          style={{
            background: [
              'radial-gradient(ellipse 60% 50% at 20% 20%, oklch(0.72 0.18 250 / 80%) 0%, transparent 70%)',
              'radial-gradient(ellipse 50% 40% at 80% 15%, oklch(0.65 0.15 200 / 60%) 0%, transparent 70%)',
              'radial-gradient(ellipse 40% 35% at 50% 70%, oklch(0.70 0.12 280 / 40%) 0%, transparent 70%)',
            ].join(', '),
            maskImage:
              'linear-gradient(to bottom, black 40%, transparent 100%)',
            WebkitMaskImage:
              'linear-gradient(to bottom, black 40%, transparent 100%)',
          }}
        />
        <PageTransition className='relative mx-auto w-full max-w-[1280px] space-y-8 px-3 pt-16 pb-10 sm:px-6 sm:pt-20 sm:pb-12 xl:px-8'>
          <RankingsHero
            period={period}
            onPeriodChange={handlePeriodChange}
            customStart={
              customRange ? new Date(customRange.start_time * 1000) : undefined
            }
            customEnd={
              customRange ? new Date(customRange.end_time * 1000) : undefined
            }
            onCustomRangeChange={handleCustomRangeChange}
          />

          {isRankingsLoading ? (
            <RankingsLoading />
          ) : !snapshot ? (
            <RankingsError
              message={
                rankingsQuery.error instanceof Error
                  ? rankingsQuery.error.message
                  : t('Unable to load rankings data')
              }
            />
          ) : (
            <>
              <ModelsSection
                history={snapshot.models_history}
                rows={snapshot.models}
                period={period}
              />

              <UsersRankingSection
                rows={snapshot.users}
                self={snapshot.self_user}
                metric={userMetric}
                onMetricChange={handleUserMetricChange}
                onPrivacyChange={(next) => rankingPrivacyMutation.mutate(next)}
                isPrivacyUpdating={rankingPrivacyMutation.isPending}
              />

              <GroupsRankingSection rows={snapshot.groups} />
            </>
          )}
        </PageTransition>
      </div>
    </PublicLayout>
  )
}

function RankingsLoading() {
  return (
    <div className='space-y-6'>
      <Skeleton className='h-[420px] w-full rounded-xl' />
      <Skeleton className='h-[360px] w-full rounded-xl' />
      <Skeleton className='h-[180px] w-full rounded-xl' />
    </div>
  )
}

function RankingsError(props: { message: string }) {
  const { t } = useTranslation()
  return (
    <div className='bg-card rounded-xl border border-dashed px-6 py-12 text-center'>
      <h2 className='text-foreground text-base font-semibold'>
        {t('Unable to load rankings')}
      </h2>
      <p className='text-muted-foreground mx-auto mt-2 max-w-md text-sm'>
        {props.message}
      </p>
    </div>
  )
}

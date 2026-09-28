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
import { useMemo, useState } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { SectionPageLayout } from '@/components/layout'
import { getBillingSummary } from './api'
import { CostDialog } from './components/cost-dialog'
import { CostTable } from './components/cost-table'
import { GroupCostsTable } from './components/group-costs-table'
import { TopUpsTable } from './components/topups-table'
import { auditMoney, auditTime, currentAuditMonth } from './lib'
import type { CostAction } from './types'

export function BillingAudit() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [month, setMonth] = useState(currentAuditMonth)
  const [action, setAction] = useState<CostAction | null>(null)
  const [excludedGroups, setExcludedGroups] = useState<Set<string>>(new Set())
  const query = useQuery({
    queryKey: ['billing-audit', 'summary', month],
    queryFn: () => getBillingSummary(month),
  })
  const data = query.data
  const runningAmount = useMemo(
    () =>
      (data?.groups ?? []).reduce(
        (total, group) =>
          excludedGroups.has(group.group)
            ? total
            : total + Number(group.estimated_cost),
        0
      ),
    [data?.groups, excludedGroups]
  )
  const cards = data
    ? [
        {
          label: t('billingAudit.netReceived'),
          value: data.net_recharge,
          hint: t('billingAudit.receivedAndRefund', {
            received: auditMoney(data.recharge_amount),
            refund: auditMoney(data.refund_amount),
          }),
        },
        {
          label: t('billingAudit.estimated'),
          value: runningAmount,
          hint: t('billingAudit.estimatedHint'),
        },
        {
          label: t('billingAudit.actual'),
          value: data.actual_cost,
          hint: t('billingAudit.actualHint'),
        },
        {
          label: t('billingAudit.balance'),
          value: data.current_balance,
          hint: t('billingAudit.balanceHint'),
        },
      ]
    : []
  return (
    <>
      <SectionPageLayout>
        <SectionPageLayout.Title>
          {t('billingAudit.title')}
        </SectionPageLayout.Title>
        <SectionPageLayout.Actions>
          <Label htmlFor='audit-month'>{t('billingAudit.month')}</Label>
          <Input
            id='audit-month'
            type='month'
            min='2000-01'
            max='9998-12'
            className='w-44'
            value={month}
            onChange={(event) => {
              const value = event.target.value
              if (
                /^(?:20\d{2}|[3-9]\d{3})-(0[1-9]|1[0-2])$/.test(value) &&
                value <= '9998-12'
              )
                setMonth(value)
            }}
          />
          <Button
            variant='outline'
            disabled={query.isFetching}
            onClick={() =>
              void queryClient.invalidateQueries({
                queryKey: ['billing-audit'],
              })
            }
          >
            {t('Refresh')}
          </Button>
        </SectionPageLayout.Actions>
        <SectionPageLayout.Content>
          <div className='space-y-4'>
            <p className='text-muted-foreground text-sm'>
              {t('billingAudit.description')}
            </p>
            {query.isPending && <p role='status'>{t('Loading...')}</p>}
            {query.isError && (
              <p role='alert' className='text-destructive'>
                {t('billingAudit.loadFailed')}{' '}
                <Button variant='outline' onClick={() => void query.refetch()}>
                  {t('Retry')}
                </Button>
              </p>
            )}
            {data && (
              <>
                <div className='grid gap-3 sm:grid-cols-2 xl:grid-cols-4'>
                  {cards.map((card) => (
                    <article key={card.label} className='rounded-xl border p-4'>
                      <h3 className='text-muted-foreground text-sm'>
                        {card.label}
                      </h3>
                      <p className='my-2 text-2xl font-semibold tabular-nums'>
                        {auditMoney(card.value)}
                      </p>
                      <p className='text-muted-foreground text-xs'>
                        {card.hint}
                      </p>
                    </article>
                  ))}
                </div>
                <p className='text-muted-foreground text-xs'>
                  {t('billingAudit.asOf', { time: auditTime(data.queried_at) })}
                </p>
                <CostTable costs={data.costs} onAction={setAction} />
                <GroupCostsTable
                  groups={data.groups}
                  excludedGroups={excludedGroups}
                  onExclude={(group, excluded) => {
                    setExcludedGroups((previous) => {
                      const next = new Set(previous)
                      if (excluded) next.add(group)
                      else next.delete(group)
                      return next
                    })
                  }}
                  onReset={() => setExcludedGroups(new Set())}
                />
              </>
            )}
            <TopUpsTable key={month} month={month} />
          </div>
        </SectionPageLayout.Content>
      </SectionPageLayout>
      {action && (
        <CostDialog
          action={action}
          month={month}
          onClose={() => setAction(null)}
        />
      )}
    </>
  )
}

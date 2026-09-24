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
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { getBillingTopUps } from '../api'
import { auditMoney, auditTime } from '../lib'
import type { BillingTopUp } from '../types'
import { BillingUser } from './billing-user'
import { UserTopUpsTable } from './user-topups-table'

function manualAmount(row: BillingTopUp): string {
  if (row.manual_amount !== undefined) return auditMoney(row.manual_amount)
  return row.original_amount ?? '—'
}

export function TopUpsTable(props: { month: string }) {
  const { t } = useTranslation()
  return (
    <section className='rounded-xl border p-4'>
      <Tabs defaultValue='orders'>
        <div className='flex flex-wrap items-center justify-between gap-3'>
          <h3 className='font-semibold'>{t('billingAudit.topups')}</h3>
          <TabsList aria-label={t('billingAudit.topups')}>
            <TabsTrigger value='orders'>
              {t('billingAudit.orderView')}
            </TabsTrigger>
            <TabsTrigger value='users'>
              {t('billingAudit.userView')}
            </TabsTrigger>
          </TabsList>
        </div>
        <TabsContent value='orders'>
          <TopUpOrdersTable key={props.month} month={props.month} />
        </TabsContent>
        <TabsContent value='users'>
          <UserTopUpsTable key={props.month} month={props.month} />
        </TabsContent>
      </Tabs>
    </section>
  )
}

function TopUpOrdersTable(props: { month: string }) {
  const { t } = useTranslation()
  const [page, setPage] = useState(1)
  const query = useQuery({
    queryKey: ['billing-audit', 'topups', props.month, page],
    queryFn: () => getBillingTopUps(props.month, page),
  })
  const pages = Math.max(1, Math.ceil((query.data?.total ?? 0) / 20))
  return (
    <div>
      <p className='text-muted-foreground my-2 text-sm'>
        {t('billingAudit.refundHint')}
      </p>
      {query.isPending && <p role='status'>{t('Loading...')}</p>}
      {query.isError && (
        <div role='alert'>
          {t('billingAudit.loadFailed')}{' '}
          <Button variant='outline' onClick={() => void query.refetch()}>
            {t('Retry')}
          </Button>
        </div>
      )}
      {query.data && (
        <>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t('billingAudit.order')}</TableHead>
                <TableHead>{t('User')}</TableHead>
                <TableHead>{t('billingAudit.paymentMethod')}</TableHead>
                <TableHead>{t('billingAudit.paidAt')}</TableHead>
                <TableHead className='text-right'>
                  {t('billingAudit.received')}
                </TableHead>
                <TableHead className='text-right'>
                  {t('billingAudit.refunded')}
                </TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {query.data.items.map((row) => (
                <TableRow key={row.id}>
                  <TableCell className='font-mono text-xs'>
                    {row.trade_no}
                  </TableCell>
                  <TableCell>
                    <BillingUser userId={row.user_id} user={row.user} />
                  </TableCell>
                  <TableCell>
                    {row.payment_method === 'admin_recharge' &&
                      t('billingAudit.adminRecharge')}
                    {row.payment_method === 'admin_refund' &&
                      t('billingAudit.adminRefund')}
                    {row.payment_method !== 'admin_recharge' &&
                      row.payment_method !== 'admin_refund' &&
                      row.payment_method}
                  </TableCell>
                  <TableCell>{auditTime(row.billing_time)}</TableCell>
                  <TableCell className='text-right tabular-nums'>
                    {row.payment_method === 'admin_recharge'
                      ? manualAmount(row)
                      : auditMoney(row.money)}
                  </TableCell>
                  <TableCell className='text-right tabular-nums'>
                    {row.payment_method === 'admin_refund'
                      ? manualAmount(row)
                      : auditMoney(row.refunded_cents / 100)}
                  </TableCell>
                </TableRow>
              ))}
              {!query.data.items.length && (
                <TableRow>
                  <TableCell
                    colSpan={6}
                    className='text-muted-foreground py-8 text-center'
                  >
                    {t('billingAudit.noTopups')}
                  </TableCell>
                </TableRow>
              )}
            </TableBody>
          </Table>
          <div className='mt-3 flex items-center justify-end gap-3 text-sm'>
            <span>
              {t('billingAudit.page', { page, pages, count: query.data.total })}
            </span>
            <Button
              size='sm'
              variant='outline'
              disabled={page <= 1}
              onClick={() => setPage(page - 1)}
            >
              {t('Previous')}
            </Button>
            <Button
              size='sm'
              variant='outline'
              disabled={page >= pages}
              onClick={() => setPage(page + 1)}
            >
              {t('Next')}
            </Button>
          </div>
        </>
      )}
    </div>
  )
}

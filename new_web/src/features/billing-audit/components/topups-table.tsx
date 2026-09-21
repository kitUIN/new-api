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
import { getBillingTopUps } from '../api'
import { auditMoney, auditTime } from '../lib'

export function TopUpsTable(props: { month: string }) {
  const { t } = useTranslation()
  const [page, setPage] = useState(1)
  const query = useQuery({
    queryKey: ['billing-audit', 'topups', props.month, page],
    queryFn: () => getBillingTopUps(props.month, page),
  })
  const pages = Math.max(1, Math.ceil((query.data?.total ?? 0) / 20))
  return (
    <section className='rounded-xl border p-4'>
      <h3 className='font-semibold'>{t('billingAudit.topups')}</h3>
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
                <TableHead>{t('billingAudit.user')}</TableHead>
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
                  <TableCell>#{row.user_id}</TableCell>
                  <TableCell>{row.payment_method}</TableCell>
                  <TableCell>{auditTime(row.complete_time)}</TableCell>
                  <TableCell className='text-right tabular-nums'>
                    {auditMoney(row.money)}
                  </TableCell>
                  <TableCell className='text-right tabular-nums'>
                    {auditMoney(row.refunded_cents / 100)}
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
    </section>
  )
}

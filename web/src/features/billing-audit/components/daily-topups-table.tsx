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
import { getBillingDailyTopUps } from '../api'
import { auditMoney } from '../lib'

export function DailyTopUpsTable(props: { month: string }) {
  const { t } = useTranslation()
  const [page, setPage] = useState(1)
  const query = useQuery({
    queryKey: ['billing-audit', 'daily-topups', props.month, page],
    queryFn: () => getBillingDailyTopUps(props.month, page),
  })
  const pages = Math.max(1, Math.ceil((query.data?.total ?? 0) / 20))
  return (
    <div>
      <p className='text-muted-foreground my-2 text-sm'>
        {t('billingAudit.dailyViewHint')}
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
                <TableHead>{t('billingAudit.billingDate')}</TableHead>
                <TableHead className='text-right'>
                  {t('billingAudit.recordCount')}
                </TableHead>
                <TableHead className='text-right'>
                  {t('billingAudit.totalReceived')}
                </TableHead>
                <TableHead className='text-right'>
                  {t('billingAudit.totalRefunded')}
                </TableHead>
                <TableHead className='text-right'>
                  {t('billingAudit.netRecharge')}
                </TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {query.data.items.map((row) => (
                <TableRow key={row.date}>
                  <TableCell className='tabular-nums'>
                    {row.date}
                    {row.unconverted_count > 0 && (
                      <p className='text-muted-foreground mt-1 text-xs'>
                        {t('billingAudit.unconvertedCount', {
                          count: row.unconverted_count,
                        })}
                      </p>
                    )}
                  </TableCell>
                  <TableCell className='text-right tabular-nums'>
                    {row.record_count}
                  </TableCell>
                  <TableCell className='text-right tabular-nums'>
                    {auditMoney(row.received, 6)}
                  </TableCell>
                  <TableCell className='text-right tabular-nums'>
                    {auditMoney(row.refunded, 6)}
                  </TableCell>
                  <TableCell className='text-right tabular-nums'>
                    {auditMoney(row.net_recharge, 6)}
                  </TableCell>
                </TableRow>
              ))}
              {!query.data.items.length && (
                <TableRow>
                  <TableCell
                    colSpan={5}
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
              {t('billingAudit.dailyPage', {
                page,
                pages,
                count: query.data.total,
              })}
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

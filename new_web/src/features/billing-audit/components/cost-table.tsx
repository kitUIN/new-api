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
import { auditMoney } from '../lib'
import type { BillingCost, CostAction } from '../types'

export function CostTable(props: {
  costs: BillingCost[]
  onAction: (action: CostAction) => void
}) {
  const { t } = useTranslation()
  return (
    <section className='rounded-xl border p-4'>
      <div className='mb-3 flex flex-wrap items-center justify-between gap-2'>
        <h3 className='font-semibold'>{t('billingAudit.fixedCosts')}</h3>
        <Button size='sm' onClick={() => props.onAction({ kind: 'create' })}>
          {t('billingAudit.addCost')}
        </Button>
      </div>
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>{t('billingAudit.costName')}</TableHead>
            <TableHead>{t('billingAudit.type')}</TableHead>
            <TableHead>{t('billingAudit.remark')}</TableHead>
            <TableHead className='text-right'>
              {t('billingAudit.amountUSD')}
            </TableHead>
            <TableHead className='text-right'>{t('Actions')}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {props.costs.map((cost) => (
            <TableRow key={cost.id}>
              <TableCell>{cost.name}</TableCell>
              <TableCell>
                {cost.recurring
                  ? t('billingAudit.recurring')
                  : t('billingAudit.oneTime')}
                {cost.exception && (
                  <span className='text-muted-foreground ml-2'>
                    {t('billingAudit.exception')}
                  </span>
                )}
              </TableCell>
              <TableCell className='max-w-64 whitespace-normal'>
                {cost.remark || '—'}
              </TableCell>
              <TableCell className='text-right tabular-nums'>
                {auditMoney(cost.amount_cents / 100)}
              </TableCell>
              <TableCell>
                <div className='flex justify-end gap-2'>
                  <Button
                    size='sm'
                    variant='outline'
                    onClick={() => props.onAction({ kind: 'edit', cost })}
                  >
                    {t('Edit')}
                  </Button>
                  <Button
                    size='sm'
                    variant='outline'
                    onClick={() => props.onAction({ kind: 'delete', cost })}
                  >
                    {cost.recurring
                      ? t('billingAudit.deleteOrStop')
                      : t('Delete')}
                  </Button>
                </div>
              </TableCell>
            </TableRow>
          ))}
          {!props.costs.length && (
            <TableRow>
              <TableCell
                colSpan={5}
                className='text-muted-foreground py-8 text-center'
              >
                {t('billingAudit.noCosts')}
              </TableCell>
            </TableRow>
          )}
        </TableBody>
      </Table>
    </section>
  )
}

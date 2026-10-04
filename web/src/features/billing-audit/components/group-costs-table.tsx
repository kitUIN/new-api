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
import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { auditMoney } from '../lib'
import type { BillingSummary } from '../types'

interface GroupCostsTableProps {
  groups: BillingSummary['groups']
  excludedGroups: ReadonlySet<string>
  saving: boolean
  onExclude: (group: string, excluded: boolean) => void
  onReset: () => void
}

export function GroupCostsTable(props: GroupCostsTableProps) {
  const { t } = useTranslation()
  const sortedGroups = useMemo(
    () =>
      [...props.groups].sort(
        (a, b) => Number(b.estimated_cost) - Number(a.estimated_cost)
      ),
    [props.groups]
  )
  return (
    <section className='rounded-xl border p-4'>
      <div className='mb-3 flex flex-wrap items-center justify-between gap-3'>
        <h3 className='font-semibold'>{t('billingAudit.groupCosts')}</h3>
        <Button
          variant='outline'
          size='sm'
          disabled={props.saving || props.excludedGroups.size === 0}
          onClick={props.onReset}
        >
          {t('billingAudit.includeAllGroups')}
        </Button>
      </div>
      <p className='text-muted-foreground mb-3 text-sm'>
        {t('billingAudit.excludeGroupsHint')}
      </p>
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>{t('Group')}</TableHead>
            <TableHead>{t('billingAudit.excludeGroup')}</TableHead>
            <TableHead className='text-right'>
              {t('billingAudit.estimated')}
            </TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {sortedGroups.map((group) => (
            <TableRow key={group.group}>
              <TableCell>
                {group.group || t('billingAudit.unknownGroup')}
              </TableCell>
              <TableCell>
                <Checkbox
                  disabled={props.saving}
                  checked={props.excludedGroups.has(group.group)}
                  onCheckedChange={(checked) =>
                    props.onExclude(group.group, checked)
                  }
                  aria-label={t('billingAudit.excludeNamedGroup', {
                    group: group.group || t('billingAudit.unknownGroup'),
                  })}
                />
              </TableCell>
              <TableCell className='text-right tabular-nums'>
                {auditMoney(group.estimated_cost, 6)}
              </TableCell>
            </TableRow>
          ))}
          {!props.groups.length && (
            <TableRow>
              <TableCell
                colSpan={3}
                className='text-muted-foreground py-8 text-center'
              >
                {t('billingAudit.noGroups')}
              </TableCell>
            </TableRow>
          )}
        </TableBody>
      </Table>
    </section>
  )
}

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
import { memo } from 'react'
import { flexRender, type ColumnDef, type Row } from '@tanstack/react-table'
import { cn } from '@/lib/utils'
import { TableCell, TableRow } from '@/components/ui/table'
import { getUsageLogRowClassName } from '../lib/row-style'
import type { LogCategory } from '../types'

interface UsageLogTableRowProps {
  row: Row<Record<string, unknown>>
  category: LogCategory
  columns: ColumnDef<Record<string, unknown>>[]
  visibleColumns: string
}

export const UsageLogTableRow = memo(
  function UsageLogTableRow(props: UsageLogTableRowProps) {
    return (
      <TableRow
        className={cn(
          'usage-log-row transition-colors',
          getUsageLogRowClassName(props.row.original, props.category)
        )}
      >
        {props.row.getVisibleCells().map((cell) => (
          <TableCell
            key={cell.id}
            className={props.category === 'common' ? 'py-2' : 'py-3.5'}
          >
            {flexRender(cell.column.columnDef.cell, cell.getContext())}
          </TableCell>
        ))}
      </TableRow>
    )
  },
  (previous, next) =>
    previous.row.original === next.row.original &&
    previous.columns === next.columns &&
    previous.category === next.category &&
    previous.visibleColumns === next.visibleColumns
)

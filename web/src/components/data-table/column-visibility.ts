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
import type { Column, Table } from '@tanstack/react-table'

export function getViewOptionColumns<TData>(
  table: Table<TData>
): Column<TData, unknown>[] {
  return table
    .getAllLeafColumns()
    .filter(
      (column) =>
        column.getCanHide() &&
        (column.accessorFn !== undefined || !!column.columnDef.meta?.label)
    )
}

export function isColumnVisible<TData>(
  table: Table<TData>,
  column: Column<TData, unknown>,
  isMobile: boolean
): boolean {
  if (!column.getIsVisible()) return false
  if (!isMobile || !column.columnDef.meta?.mobileHidden) return true

  // Mobile defaults must yield to an explicit choice in the column menu.
  return table.getState().columnVisibility[column.id] === true
}

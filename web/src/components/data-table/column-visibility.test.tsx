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
import {
  createTable,
  getCoreRowModel,
  type ColumnDef,
} from '@tanstack/react-table'
import { createInstance } from 'i18next'
import assert from 'node:assert/strict'
import { test } from 'node:test'
import { renderToStaticMarkup } from 'react-dom/server'
import { I18nextProvider } from 'react-i18next'
import { getViewOptionColumns, isColumnVisible } from './column-visibility'
import { MobileCardList } from './mobile-card-list'

type LogRow = { model: string; session: string }

function createLogTable(compact = true) {
  const columns: ColumnDef<LogRow>[] = [
    { id: 'select', enableHiding: false },
    { id: 'time', enableHiding: false, meta: { label: 'Time' } },
    {
      id: 'request',
      header: 'Request',
      columns: [
        {
          accessorKey: 'model',
          meta: { label: 'Model', mobileTitle: compact },
        },
        {
          id: 'session_key',
          cell: ({ row }) => row.original.session,
          meta: { label: 'Session Key', mobileHidden: true },
        },
      ],
    },
    { id: 'actions' },
  ]
  const table = createTable({
    data: [{ model: 'test-model', session: 'complete-session-key' }],
    columns,
    state: { columnVisibility: {} },
    onStateChange: () => {},
    onColumnVisibilityChange: (updater) => {
      table.setOptions((options) => ({
        ...options,
        state: {
          ...options.state,
          columnVisibility:
            typeof updater === 'function'
              ? updater(table.getState().columnVisibility)
              : updater,
        },
      }))
    },
    getCoreRowModel: getCoreRowModel(),
    renderFallbackValue: null,
  })
  table.setOptions((options) => ({ ...options, state: table.initialState }))
  return table
}

test('column options include computed leaf columns and allow hiding and restoring them', () => {
  const table = createLogTable()
  assert.deepEqual(
    getViewOptionColumns(table).map((column) => column.id),
    ['model', 'session_key']
  )

  const session = table.getColumn('session_key')!
  session.toggleVisibility(false)
  assert.ok(
    !table
      .getRowModel()
      .rows[0].getVisibleCells()
      .some((cell) => cell.column.id === session.id)
  )
  session.toggleVisibility(true)
  assert.ok(
    table
      .getRowModel()
      .rows[0].getVisibleCells()
      .some((cell) => cell.column.id === session.id)
  )
})

for (const compact of [true, false]) {
  test(`mobile column selection matches rendered ${compact ? 'compact' : 'fallback'} rows`, async () => {
    const table = createLogTable(compact)
    const session = table.getColumn('session_key')!
    const translations = createInstance()
    await translations.init({ lng: 'en', resources: {} })
    const render = () =>
      renderToStaticMarkup(
        <I18nextProvider i18n={translations}>
          <MobileCardList table={table} />
        </I18nextProvider>
      )

    assert.equal(isColumnVisible(table, session, false), true)
    assert.equal(isColumnVisible(table, session, true), false)
    assert.ok(!render().includes('complete-session-key'))

    session.toggleVisibility(true)
    assert.equal(isColumnVisible(table, session, true), true)
    assert.ok(render().includes('complete-session-key'))

    session.toggleVisibility(false)
    assert.equal(isColumnVisible(table, session, true), false)
    assert.ok(!render().includes('complete-session-key'))
  })
}

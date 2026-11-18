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
import { useEffect, useRef, useState } from 'react'
import { hashKey, useQuery } from '@tanstack/react-query'
import { getRouteApi } from '@tanstack/react-router'
import {
  type ColumnDef,
  getCoreRowModel,
  getFacetedRowModel,
  getFacetedUniqueValues,
  getFilteredRowModel,
  getPaginationRowModel,
  useReactTable,
} from '@tanstack/react-table'
import { useMediaQuery } from '@/hooks'
import '@/styles/usage-log-row.css'
import { ArrowUp, Download, Loader2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { cn } from '@/lib/utils'
import { useIsAdmin } from '@/hooks/use-admin'
import { useTableUrlState } from '@/hooks/use-table-url-state'
import { Button } from '@/components/ui/button'
import { DataTablePage } from '@/components/data-table'
import { refreshLogsByIDs } from '../api'
import { DEFAULT_LOGS_DATA, LOG_TYPE_ALL_VALUE } from '../constants'
import { useLiveUsageLogs } from '../hooks/use-live-usage-logs'
import { useLogReading } from '../hooks/use-log-reading'
import { useLogSnapshot } from '../hooks/use-log-snapshot'
import { useColumnsByCategory } from '../lib/columns'
import { exportUsageLogsToExcel } from '../lib/export'
import { shareLogPage, type LogPage } from '../lib/live-snapshot'
import { getUsageLogRowClassName } from '../lib/row-style'
import { fetchLogsByCategory } from '../lib/utils'
import type { LogCategory, UsageLog } from '../types'
import { CommonLogsFilterBar } from './common-logs-filter-bar'
import { TaskLogsFilterBar } from './task-logs-filter-bar'
import { UsageLogTableRow } from './usage-log-table-row'
import { useUsageLogsContext } from './usage-logs-provider'

const route = getRouteApi('/_authenticated/usage-logs/$section')

function deserializeLogTypeFilter(value: unknown): unknown[] {
  const values = Array.isArray(value) ? value : value ? [value] : []
  return values.filter((item) => String(item) !== LOG_TYPE_ALL_VALUE)
}

interface UsageLogsTableProps {
  logCategory: LogCategory
}

export function UsageLogsTable({ logCategory }: UsageLogsTableProps) {
  const { t } = useTranslation()
  const isAdmin = useIsAdmin()
  const { sensitiveVisible, autoRefresh, manualRefreshVersion } =
    useUsageLogsContext()
  const isCommon = logCategory === 'common'
  const containerRef = useRef<HTMLDivElement>(null)
  const displayedRows = useRef<{
    scope: string
    items: UsageLog[]
    frozen: boolean
  }>({ scope: '', items: [], frozen: false })
  const { reading, scrollToTop } = useLogReading(containerRef, isCommon)
  const navigate = route.useNavigate()
  useLiveUsageLogs(logCategory === 'common' && autoRefresh, isAdmin)
  const isMobile = useMediaQuery('(max-width: 640px)')
  const searchParams = route.useSearch()
  const [isExporting, setIsExporting] = useState(false)
  const [exportProgress, setExportProgress] = useState<{
    current: number
    total: number
  } | null>(null)

  const {
    columnFilters,
    onColumnFiltersChange,
    pagination,
    onPaginationChange,
    ensurePageInRange,
  } = useTableUrlState({
    search: route.useSearch(),
    navigate: route.useNavigate(),
    pagination: { defaultPage: 1, defaultPageSize: isMobile ? 20 : 100 },
    globalFilter: { enabled: false },
    columnFilters: [
      {
        columnId: 'created_at',
        searchKey: 'type',
        type: 'array' as const,
        deserialize: deserializeLogTypeFilter,
      },
      { columnId: 'model_name', searchKey: 'model', type: 'string' as const },
      { columnId: 'token_name', searchKey: 'token', type: 'string' as const },
      { columnId: 'group', searchKey: 'group', type: 'string' as const },
      ...(isAdmin
        ? [
            {
              columnId: 'channel',
              searchKey: 'channel',
              type: 'string' as const,
            },
            {
              columnId: 'username',
              searchKey: 'username',
              type: 'string' as const,
            },
            {
              columnId: 'session_key',
              searchKey: 'sessionKey',
              type: 'string' as const,
            },
          ]
        : []),
    ],
  })

  const queryKey = [
    'logs',
    logCategory,
    isAdmin,
    pagination.pageIndex + 1,
    pagination.pageSize,
    columnFilters,
    searchParams,
    t,
  ]
  const scope = hashKey(queryKey)
  // Pinned rows supplement this page; they must not change its cache identity
  // whenever a background response updates the visible snapshot.
  // eslint-disable-next-line @tanstack/query/exhaustive-deps
  const query = useQuery<LogPage>({
    refetchOnWindowFocus:
      import.meta.env.PROD && (logCategory !== 'common' || autoRefresh),
    refetchOnReconnect: logCategory !== 'common' || autoRefresh,
    queryKey,
    structuralSharing: (previous, next) =>
      shareLogPage(previous as LogPage | undefined, next as LogPage),
    queryFn: async ({ queryKey: requestKey }) => {
      const requestScope = hashKey(requestKey)
      const result = await fetchLogsByCategory({
        logCategory,
        isAdmin,
        page: pagination.pageIndex + 1,
        pageSize: pagination.pageSize,
        searchParams,
        columnFilters,
      })

      if (!result?.success) {
        throw new Error(result?.message || t('Failed to load logs'))
      }

      const displayed = displayedRows.current
      const page = result.data || {
        ...DEFAULT_LOGS_DATA,
        page: pagination.pageIndex + 1,
        page_size: pagination.pageSize,
      }
      if (
        logCategory !== 'common' ||
        displayed.scope !== requestScope ||
        !displayed.frozen
      )
        return page
      const returnedIDs = new Set(page.items.map((item) => item.id))
      const missingIDs = displayed.items
        .filter((item) => !returnedIDs.has(item.id))
        .map((item) => item.id)
      // Refresh pinned rows that no longer occur in this offset page.
      const updates: UsageLog[] = []
      for (let offset = 0; offset < missingIDs.length; offset += 1000) {
        updates.push(
          ...(await refreshLogsByIDs(
            missingIDs.slice(offset, offset + 1000),
            isAdmin
          ))
        )
      }
      return { ...page, updates }
    },
    placeholderData: (previousData, previousQuery) => {
      if (previousQuery?.queryKey[1] === logCategory) {
        return previousData
      }
      return undefined
    },
  })

  const snapshot = useLogSnapshot({
    scope,
    data: query.data,
    updatedAt: query.dataUpdatedAt,
    manualVersion: manualRefreshVersion,
    hold: isCommon && (reading || pagination.pageIndex > 0),
    placeholder: query.isPlaceholderData,
  })
  const data = isCommon ? snapshot.data : query.data
  useEffect(() => {
    displayedRows.current = {
      scope,
      items:
        isCommon && !query.isPlaceholderData
          ? ((data?.items as UsageLog[]) ?? [])
          : [],
      frozen: reading || pagination.pageIndex > 0 || snapshot.hasUpdates,
    }
  }, [
    scope,
    data,
    isCommon,
    query.isPlaceholderData,
    reading,
    pagination.pageIndex,
    snapshot.hasUpdates,
  ])
  const logs = data?.items || DEFAULT_LOGS_DATA.items
  const columns = useColumnsByCategory(logCategory, isAdmin)
  const isLoadingData = query.isLoading

  const table = useReactTable({
    data: logs as Record<string, unknown>[],
    columns: columns as ColumnDef<Record<string, unknown>>[],
    state: {
      columnFilters,
      pagination,
    },
    enableRowSelection: false,
    getRowId: (row) => String(row.id),
    onPaginationChange,
    onColumnFiltersChange,
    getCoreRowModel: getCoreRowModel(),
    getFilteredRowModel: getFilteredRowModel(),
    getPaginationRowModel: getPaginationRowModel(),
    getFacetedRowModel: getFacetedRowModel(),
    getFacetedUniqueValues: getFacetedUniqueValues(),
    manualPagination: true,
    manualFiltering: isCommon,
    pageCount: Math.ceil((data?.total || 0) / pagination.pageSize),
  })

  const pageCount = table.getPageCount()
  useEffect(() => {
    ensurePageInRange(pageCount)
  }, [pageCount, ensurePageInRange])

  const visibleColumns = table
    .getVisibleLeafColumns()
    .map((column) => column.id)
    .join('|')
  const exportColumns = table.getVisibleLeafColumns().map((column) => ({
    id: column.id,
    label: column.columnDef.meta?.label || column.id,
  }))

  const handleExport = async () => {
    setIsExporting(true)
    setExportProgress({ current: 0, total: data?.total || 0 })
    try {
      const result = await exportUsageLogsToExcel({
        logCategory,
        isAdmin,
        searchParams,
        columnFilters,
        columns: exportColumns,
        sensitiveVisible,
        t,
        onProgress: (current, total) => setExportProgress({ current, total }),
      })
      if (result.count === 0) {
        toast.info(t('No logs to export.'))
        return
      }
      toast.success(
        t('Exported {{count}} log entries.', { count: result.count })
      )
    } catch (error) {
      const message = error instanceof Error ? error.message : ''
      toast.error(message || t('Failed to export logs'))
    } finally {
      setIsExporting(false)
      setExportProgress(null)
    }
  }

  const exportPercentage =
    exportProgress && exportProgress.total > 0
      ? Math.min(
          100,
          Math.round((exportProgress.current / exportProgress.total) * 100)
        )
      : 0

  const exportAction = (
    <Button
      type='button'
      variant='outline'
      onClick={handleExport}
      disabled={
        isExporting || (data?.total || 0) === 0 || exportColumns.length === 0
      }
      aria-label={
        isExporting && exportProgress
          ? `${t('Exporting...')} ${exportProgress.current}/${exportProgress.total}`
          : t('Export Excel')
      }
      title={t('Export Excel')}
      className='relative overflow-hidden'
    >
      {isExporting && (
        <span
          aria-hidden='true'
          className='bg-primary/20 pointer-events-none absolute inset-y-0 left-0 transition-[width] duration-200'
          style={{ width: `${exportPercentage}%` }}
        />
      )}
      {isExporting ? (
        <Loader2 className='relative z-10 animate-spin' />
      ) : (
        <Download aria-hidden='true' className='relative z-10' />
      )}
      <span className='relative z-10'>{t('Export Excel')}</span>
    </Button>
  )

  const showLatest = () => {
    snapshot.acceptLatest()
    scrollToTop()
    if (pagination.pageIndex > 0) {
      void navigate({ search: (previous) => ({ ...previous, page: 1 }) })
    }
  }

  return (
    <div ref={containerRef} className='usage-log-table'>
      <DataTablePage
        table={table}
        columns={columns as ColumnDef<Record<string, unknown>>[]}
        isLoading={isLoadingData}
        isFetching={!isCommon && query.isFetching}
        emptyTitle={t('No Logs Found')}
        emptyDescription={t(
          'No usage logs available. Logs will appear here once API calls are made.'
        )}
        skeletonKeyPrefix='usage-log-skeleton'
        tableClassName='overflow-x-auto'
        tableHeaderClassName='bg-muted/30 sticky top-0 z-10'
        getRowClassName={(row) =>
          cn(
            'usage-log-row',
            getUsageLogRowClassName(row.original, logCategory)
          )
        }
        toolbar={
          isCommon ? (
            <CommonLogsFilterBar table={table} actions={exportAction} />
          ) : (
            <TaskLogsFilterBar
              table={table}
              logCategory={logCategory}
              actions={exportAction}
            />
          )
        }
        renderRow={(row) => (
          <UsageLogTableRow
            key={row.id}
            row={row}
            category={logCategory}
            columns={columns as ColumnDef<Record<string, unknown>>[]}
            visibleColumns={visibleColumns}
          />
        )}
      />
      {isCommon && snapshot.hasUpdates && (
        <div className='pointer-events-none fixed inset-x-0 bottom-20 z-30 flex justify-center px-3'>
          <Button
            type='button'
            onClick={showLatest}
            className='pointer-events-auto max-w-full shadow-lg'
          >
            <ArrowUp aria-hidden='true' />
            {snapshot.pendingCount > 0
              ? t('usageLogs.newLogs', { count: snapshot.pendingCount })
              : t('usageLogs.updatesAvailable')}
          </Button>
        </div>
      )}
    </div>
  )
}

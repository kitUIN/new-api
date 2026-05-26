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
'use client'

import * as React from 'react'
import { cn } from '@/lib/utils'

type TableProps = React.ComponentPropsWithoutRef<'table'> &
  React.RefAttributes<HTMLTableElement>

type TableHorizontalScrollMetrics = {
  scrollWidth: number
  clientWidth: number
}

type TableHorizontalScrollContextValue = {
  metrics: TableHorizontalScrollMetrics
  setMetrics: React.Dispatch<React.SetStateAction<TableHorizontalScrollMetrics>>
  tableScrollContainerRef: React.RefObject<HTMLDivElement | null>
  registerScrollbar: (node: HTMLDivElement) => () => void
  syncFromTable: (source: HTMLDivElement) => void
  syncFromScrollbar: (source: HTMLDivElement) => void
}

type TableStickyColumnSide = 'left' | 'right' | false | undefined

const TableHorizontalScrollContext =
  React.createContext<TableHorizontalScrollContextValue | null>(null)

function getTableStickyColumnClass(
  side: TableStickyColumnSide,
  variant: 'header' | 'cell'
): string | undefined {
  if (!side) return undefined

  return cn(
    'sticky',
    side === 'right' &&
      "-right-px border-r border-r-background shadow-[-10px_0_12px_-12px_rgb(0_0_0_/_0.45)] before:absolute before:inset-y-0 before:left-0 before:z-10 before:w-0.5 before:bg-border before:content-['']",
    side === 'left' &&
      "left-0 shadow-[10px_0_12px_-12px_rgb(0_0_0_/_0.45)] after:absolute after:inset-y-0 after:right-0 after:z-10 after:w-0.5 after:bg-border after:content-['']",
    variant === 'header' ? 'bg-muted z-30' : 'bg-background z-20',
    variant === 'cell' &&
      'group-hover/row:bg-muted group-data-[state=selected]/row:bg-muted'
  )
}

function TableHorizontalScrollProvider({
  children,
}: {
  children: React.ReactNode
}) {
  const tableScrollContainerRef = React.useRef<HTMLDivElement>(null)
  const scrollbarRefs = React.useRef(new Set<HTMLDivElement>())
  const isSyncingScrollRef = React.useRef(false)
  const [metrics, setMetrics] = React.useState<TableHorizontalScrollMetrics>({
    scrollWidth: 0,
    clientWidth: 0,
  })

  const syncScrollbarRefs = React.useCallback(
    (scrollLeft: number, exclude?: HTMLDivElement) => {
      scrollbarRefs.current.forEach((scrollbar) => {
        if (scrollbar !== exclude) {
          scrollbar.scrollLeft = scrollLeft
        }
      })
    },
    []
  )

  const releaseSyncLock = React.useCallback(() => {
    window.requestAnimationFrame(() => {
      isSyncingScrollRef.current = false
    })
  }, [])

  const registerScrollbar = React.useCallback(
    (node: HTMLDivElement) => {
      scrollbarRefs.current.add(node)
      if (tableScrollContainerRef.current) {
        node.scrollLeft = tableScrollContainerRef.current.scrollLeft
      }

      return () => {
        scrollbarRefs.current.delete(node)
      }
    },
    []
  )

  const syncFromTable = React.useCallback(
    (source: HTMLDivElement) => {
      if (isSyncingScrollRef.current) return

      isSyncingScrollRef.current = true
      syncScrollbarRefs(source.scrollLeft)
      releaseSyncLock()
    },
    [releaseSyncLock, syncScrollbarRefs]
  )

  const syncFromScrollbar = React.useCallback(
    (source: HTMLDivElement) => {
      const scrollContainer = tableScrollContainerRef.current
      if (!scrollContainer || isSyncingScrollRef.current) return

      isSyncingScrollRef.current = true
      scrollContainer.scrollLeft = source.scrollLeft
      syncScrollbarRefs(source.scrollLeft, source)
      releaseSyncLock()
    },
    [releaseSyncLock, syncScrollbarRefs]
  )

  const value = React.useMemo<TableHorizontalScrollContextValue>(
    () => ({
      metrics,
      setMetrics,
      tableScrollContainerRef,
      registerScrollbar,
      syncFromTable,
      syncFromScrollbar,
    }),
    [metrics, registerScrollbar, syncFromScrollbar, syncFromTable]
  )

  return (
    <TableHorizontalScrollContext.Provider value={value}>
      {children}
    </TableHorizontalScrollContext.Provider>
  )
}

function TableHorizontalScrollbar({
  className,
  ...props
}: React.ComponentPropsWithoutRef<'div'>) {
  const horizontalScrollContext = React.useContext(TableHorizontalScrollContext)
  const cleanupRef = React.useRef<(() => void) | null>(null)

  const setScrollbarRef = React.useCallback(
    (node: HTMLDivElement | null) => {
      cleanupRef.current?.()
      cleanupRef.current = null

      if (node && horizontalScrollContext) {
        cleanupRef.current = horizontalScrollContext.registerScrollbar(node)
      }
    },
    [horizontalScrollContext]
  )

  React.useEffect(() => {
    return () => {
      cleanupRef.current?.()
      cleanupRef.current = null
    }
  }, [])

  if (!horizontalScrollContext) return null

  const showHorizontalScrollbar =
    horizontalScrollContext.metrics.scrollWidth >
    horizontalScrollContext.metrics.clientWidth + 1

  if (!showHorizontalScrollbar) return null

  return (
    <div
      {...props}
      data-slot='table-horizontal-scrollbar'
      ref={setScrollbarRef}
      className={cn(
        'bg-background/95 h-3 w-full overflow-x-auto overflow-y-hidden',
        className
      )}
      onScroll={(event) =>
        horizontalScrollContext.syncFromScrollbar(event.currentTarget)
      }
    >
      <div
        style={{
          width: horizontalScrollContext.metrics.scrollWidth,
          height: 1,
        }}
      />
    </div>
  )
}

function Table(props: TableProps) {
  const horizontalScrollContext = React.useContext(TableHorizontalScrollContext)

  if (!horizontalScrollContext) {
    return (
      <TableHorizontalScrollProvider>
        <TableContent {...props} />
        <TableHorizontalScrollbar className='mt-1 border-t' />
      </TableHorizontalScrollProvider>
    )
  }

  return <TableContent {...props} />
}

function TableContent({ className, ref, ...props }: TableProps) {
  const horizontalScrollContext = React.useContext(TableHorizontalScrollContext)
  const tableRef = React.useRef<HTMLTableElement>(null)
  const frameRef = React.useRef<number | null>(null)
  const scrollContainerRef = horizontalScrollContext?.tableScrollContainerRef
  const setMetrics = horizontalScrollContext?.setMetrics
  const syncFromTable = horizontalScrollContext?.syncFromTable

  const setTableRef = React.useCallback(
    (node: HTMLTableElement | null) => {
      tableRef.current = node
      if (typeof ref === 'function') {
        ref(node)
      } else if (ref) {
        ref.current = node
      }
    },
    [ref]
  )

  const measureScroll = React.useCallback(() => {
    const scrollContainer = scrollContainerRef?.current
    if (!scrollContainer || !setMetrics) return

    const nextScrollWidth = scrollContainer.scrollWidth
    const nextClientWidth = scrollContainer.clientWidth

    setMetrics((current) => {
      if (
        current.scrollWidth === nextScrollWidth &&
        current.clientWidth === nextClientWidth
      ) {
        return current
      }

      return {
        scrollWidth: nextScrollWidth,
        clientWidth: nextClientWidth,
      }
    })
  }, [scrollContainerRef, setMetrics])

  const scheduleMeasure = React.useCallback(() => {
    if (frameRef.current != null) {
      window.cancelAnimationFrame(frameRef.current)
    }
    frameRef.current = window.requestAnimationFrame(() => {
      frameRef.current = null
      measureScroll()
    })
  }, [measureScroll])

  React.useLayoutEffect(() => {
    const scrollContainer = scrollContainerRef?.current
    const table = tableRef.current
    if (!scrollContainer || !table) return

    measureScroll()

    const resizeObserver = new ResizeObserver(scheduleMeasure)
    resizeObserver.observe(scrollContainer)
    resizeObserver.observe(table)

    const mutationObserver = new MutationObserver(scheduleMeasure)
    mutationObserver.observe(table, {
      attributes: true,
      childList: true,
      characterData: true,
      subtree: true,
    })

    window.addEventListener('resize', scheduleMeasure)

    return () => {
      resizeObserver.disconnect()
      mutationObserver.disconnect()
      window.removeEventListener('resize', scheduleMeasure)
      if (frameRef.current != null) {
        window.cancelAnimationFrame(frameRef.current)
        frameRef.current = null
      }
    }
  }, [measureScroll, scheduleMeasure, scrollContainerRef])

  const handleTableScroll = React.useCallback(
    (event: React.UIEvent<HTMLDivElement>) => {
      syncFromTable?.(event.currentTarget)
    },
    [syncFromTable]
  )

  return (
    <div data-slot='table-frame' className='relative w-full'>
      <div
        data-slot='table-container'
        ref={scrollContainerRef}
        className='no-scrollbar relative w-full overflow-x-auto overflow-y-hidden'
        onScroll={handleTableScroll}
      >
        <table
          data-slot='table'
          ref={setTableRef}
          className={cn(
            'w-full caption-bottom text-sm tabular-nums [&_td]:text-sm [&_td_*]:text-sm [&_th]:text-sm [&_th_*]:text-sm',
            className
          )}
          {...props}
        />
      </div>
    </div>
  )
}

function TableHeader({ className, ...props }: React.ComponentProps<'thead'>) {
  return (
    <thead
      data-slot='table-header'
      className={cn('[&_tr]:border-b', className)}
      {...props}
    />
  )
}

function TableBody({ className, ...props }: React.ComponentProps<'tbody'>) {
  return (
    <tbody
      data-slot='table-body'
      className={cn('[&_tr:last-child]:border-0', className)}
      {...props}
    />
  )
}

function TableFooter({ className, ...props }: React.ComponentProps<'tfoot'>) {
  return (
    <tfoot
      data-slot='table-footer'
      className={cn(
        'bg-muted/50 border-t font-medium [&>tr]:last:border-b-0',
        className
      )}
      {...props}
    />
  )
}

function TableRow({ className, ...props }: React.ComponentProps<'tr'>) {
  return (
    <tr
      data-slot='table-row'
      className={cn(
        'group/row hover:bg-muted/50 has-aria-expanded:bg-muted/50 data-[state=selected]:bg-muted border-b transition-colors',
        className
      )}
      {...props}
    />
  )
}

function TableHead({ className, ...props }: React.ComponentProps<'th'>) {
  return (
    <th
      data-slot='table-head'
      className={cn(
        'text-foreground h-10 px-2 text-left align-middle font-medium whitespace-nowrap [&:has([role=checkbox])]:pr-0',
        className
      )}
      {...props}
    />
  )
}

function TableCell({ className, ...props }: React.ComponentProps<'td'>) {
  return (
    <td
      data-slot='table-cell'
      className={cn(
        'p-2 align-middle whitespace-nowrap [&:has([role=checkbox])]:pr-0',
        className
      )}
      {...props}
    />
  )
}

function TableCaption({
  className,
  ...props
}: React.ComponentProps<'caption'>) {
  return (
    <caption
      data-slot='table-caption'
      className={cn('text-muted-foreground mt-4 text-sm', className)}
      {...props}
    />
  )
}

export {
  Table,
  TableHeader,
  TableBody,
  TableFooter,
  TableHead,
  TableRow,
  TableCell,
  TableCaption,
  TableHorizontalScrollbar,
  TableHorizontalScrollProvider,
  getTableStickyColumnClass,
}

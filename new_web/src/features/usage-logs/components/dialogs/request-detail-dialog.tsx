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
import { useQuery } from '@tanstack/react-query'
import { FileText } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { cn } from '@/lib/utils'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Spinner } from '@/components/ui/spinner'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { CopyButton } from '@/components/copy-button'
import { getRequestDetailByRequestId } from '../../api'

interface RequestDetailDialogProps {
  requestId: string | null
  open: boolean
  onOpenChange: (open: boolean) => void
}

interface DetailPaneProps {
  label: string
  value?: string
  emptyText: string
}

function formatMaybeJson(value: string): string {
  if (!value.trim()) return value
  try {
    return JSON.stringify(JSON.parse(value), null, 2)
  } catch {
    return value
  }
}

function DetailPane(props: DetailPaneProps) {
  const { t } = useTranslation()
  const displayValue = props.value ? formatMaybeJson(props.value) : ''

  return (
    <div className='space-y-2'>
      <div className='flex items-center justify-between gap-2'>
        <span className='text-muted-foreground text-xs font-medium'>
          {props.label}
        </span>
        {displayValue && (
          <CopyButton
            value={displayValue}
            size='icon'
            tooltip={t('Copy to clipboard')}
            className='size-6'
          />
        )}
      </div>
      <ScrollArea className='bg-muted/30 h-[48vh] rounded-md border'>
        <pre
          className={cn(
            'min-h-full p-3 font-mono text-xs leading-relaxed break-words whitespace-pre-wrap',
            !displayValue && 'text-muted-foreground'
          )}
        >
          {displayValue || props.emptyText}
        </pre>
      </ScrollArea>
    </div>
  )
}

export function RequestDetailDialog(props: RequestDetailDialogProps) {
  const { t } = useTranslation()

  const detailQuery = useQuery({
    queryKey: ['request-detail', props.requestId],
    queryFn: () => getRequestDetailByRequestId(props.requestId || ''),
    enabled: props.open && !!props.requestId,
  })

  const detail = detailQuery.data?.data
  const emptyText = t('No data')

  return (
    <Dialog open={props.open} onOpenChange={props.onOpenChange}>
      <DialogContent className='min-w-0 overflow-hidden sm:max-w-4xl'>
        <DialogHeader>
          <DialogTitle className='flex items-center gap-2 text-base'>
            <FileText className='size-4' aria-hidden='true' />
            {t('Request Details')}
          </DialogTitle>
          <DialogDescription className='break-all'>
            {props.requestId
              ? `${t('Request ID')}: ${props.requestId}`
              : t('View upstream request and response payloads')}
          </DialogDescription>
        </DialogHeader>

        {detailQuery.isLoading || detailQuery.isFetching ? (
          <div className='flex h-64 items-center justify-center'>
            <Spinner className='text-primary size-6' />
          </div>
        ) : !detail ? (
          <div className='text-muted-foreground flex h-40 items-center justify-center rounded-md border border-dashed text-sm'>
            {detailQuery.data?.message || t('No request detail found')}
          </div>
        ) : (
          <Tabs defaultValue='request_headers' className='min-w-0'>
            <TabsList className='max-w-full flex-wrap justify-start group-data-horizontal/tabs:h-auto'>
              <TabsTrigger value='request_headers'>
                {t('Request Headers')}
              </TabsTrigger>
              <TabsTrigger value='request_body'>
                {t('Request Body')}
              </TabsTrigger>
              <TabsTrigger value='response_headers'>
                {t('Response Headers')}
              </TabsTrigger>
              <TabsTrigger value='response_body'>
                {t('Response Body')}
              </TabsTrigger>
            </TabsList>
            <TabsContent value='request_headers'>
              <DetailPane
                label={t('Request Headers')}
                value={detail.request_headers}
                emptyText={emptyText}
              />
            </TabsContent>
            <TabsContent value='request_body'>
              <DetailPane
                label={t('Request Body')}
                value={detail.request_body}
                emptyText={emptyText}
              />
            </TabsContent>
            <TabsContent value='response_headers'>
              <DetailPane
                label={t('Response Headers')}
                value={detail.response_headers}
                emptyText={emptyText}
              />
            </TabsContent>
            <TabsContent value='response_body'>
              <DetailPane
                label={t('Response Body')}
                value={detail.response_body}
                emptyText={emptyText}
              />
            </TabsContent>
          </Tabs>
        )}
      </DialogContent>
    </Dialog>
  )
}

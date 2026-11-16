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
import { ImageIcon } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { useStatus } from '@/hooks/use-status'
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { formatBodySize, getRawRequestBodySize } from '../lib/request-body'
import type { LogOtherData } from '../types'
import { RequestClientIcon } from './request-client-icon'

export function RequestBodyCell(props: { other: LogOtherData | null }) {
  const { t } = useTranslation()
  const { status } = useStatus()
  const client = props.other?.client_request_body
  const upstream = props.other?.upstream_request_body
  const clientInfo = status?.request_clients?.find(
    (item) => item.id === props.other?.request_client
  )
  if (!client && !upstream) {
    return (
      <span className='inline-flex items-center gap-1.5'>
        <span className='inline-flex w-5 shrink-0 items-center'>
          <RequestClientIcon client={clientInfo} />
        </span>
        <span className='text-muted-foreground/40 text-xs'>—</span>
      </span>
    )
  }
  const rawSize = getRawRequestBodySize(client)
  const imageCount = client?.image_count
  const rows = [
    { label: t('客户端 → 网关'), info: client },
    { label: t('网关 → 上游'), info: upstream },
  ]

  return (
    <span className='inline-flex items-center gap-1.5 whitespace-nowrap'>
      <span className='inline-flex w-5 shrink-0 items-center'>
        <RequestClientIcon client={clientInfo} />
      </span>
      <TooltipProvider delay={300}>
        <Tooltip>
          <TooltipTrigger
            render={<button type='button' />}
            className='inline-flex cursor-help items-center gap-1.5 font-mono text-xs whitespace-nowrap tabular-nums'
            aria-label={`${t('原始大小')}: ${formatBodySize(rawSize, true) ?? t('未记录')}`}
          >
            {formatBodySize(rawSize) ?? t('未记录')}
          </TooltipTrigger>
          <TooltipContent className='w-fit max-w-[calc(100vw-2rem)]'>
            <div className='space-y-2 text-xs'>
              <div className='flex justify-between gap-4'>
                <span>{t('原始大小')}</span>
                <span className='font-mono tabular-nums'>
                  {formatBodySize(rawSize, true) ?? t('未记录')}
                </span>
              </div>
              {rows.map((row) => (
                <div
                  key={row.label}
                  className='space-y-1 border-t border-current/20 pt-1'
                >
                  <p className='font-medium'>{row.label}</p>
                  <div className='flex justify-between gap-4'>
                    <span>{t('请求体大小')}</span>
                    <span className='font-mono tabular-nums'>
                      {formatBodySize(row.info?.size, true) ?? t('未记录')}
                    </span>
                  </div>
                  <div className='flex justify-between gap-4'>
                    <span>{t('编码模式')}</span>
                    <span>
                      {row.info?.encoding === 'identity'
                        ? `${t('未压缩')} (identity)`
                        : row.info?.encoding || t('未记录')}
                    </span>
                  </div>
                </div>
              ))}
              <p className='max-w-80 opacity-70'>
                {t('记录编码后的请求体大小，不含请求头和传输协议开销。')}
              </p>
            </div>
          </TooltipContent>
        </Tooltip>
        {imageCount != null && imageCount > 0 && (
          <Tooltip>
            <TooltipTrigger
              render={<button type='button' />}
              className='text-muted-foreground/60 inline-flex shrink-0 cursor-help'
              aria-label={`${t('图片数量')}: ${imageCount}`}
            >
              <ImageIcon className='size-3' aria-hidden='true' />
            </TooltipTrigger>
            <TooltipContent>
              {t('图片数量')}: {imageCount}
            </TooltipContent>
          </Tooltip>
        )}
      </TooltipProvider>
    </span>
  )
}

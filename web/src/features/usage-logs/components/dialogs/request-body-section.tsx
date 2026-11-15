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
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import type { LogOtherData } from '../../types'

function formatBodySize(size: number | undefined): string | null {
  if (size === undefined || !Number.isFinite(size) || size < 0) return null
  const bytes = `${size.toLocaleString()} B`
  if (size < 1024) return bytes
  const unit = size >= 1024 * 1024 ? 'MiB' : 'KiB'
  const divisor = unit === 'MiB' ? 1024 * 1024 : 1024
  return `${(size / divisor).toFixed(2)} ${unit} (${bytes})`
}

export function RequestBodySection(props: { other: LogOtherData | null }) {
  const { t } = useTranslation()
  if (
    !props.other ||
    (!props.other.client_request_body && !props.other.upstream_request_body)
  ) {
    return null
  }
  const rows = [
    { label: t('客户端传入'), info: props.other.client_request_body },
    { label: t('发送给上游'), info: props.other.upstream_request_body },
  ]

  return (
    <section
      className='flex min-w-0 flex-col gap-1.5'
      aria-label={t('请求体传输')}
    >
      <h3 className='text-xs font-semibold'>{t('请求体传输')}</h3>
      <div className='overflow-hidden rounded-md border'>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{t('传输方向')}</TableHead>
              <TableHead>{t('请求体大小')}</TableHead>
              <TableHead>{t('编码模式')}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {rows.map((row) => (
              <TableRow key={row.label}>
                <TableCell>{row.label}</TableCell>
                <TableCell>
                  {formatBodySize(row.info?.size) ?? t('未记录')}
                </TableCell>
                <TableCell>
                  {row.info?.encoding === 'identity'
                    ? `${t('未压缩')} (identity)`
                    : (row.info?.encoding ?? t('未记录'))}
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>
      <p className='text-muted-foreground text-xs'>
        {t('记录编码后的请求体大小，不含请求头和传输协议开销。')}
      </p>
    </section>
  )
}

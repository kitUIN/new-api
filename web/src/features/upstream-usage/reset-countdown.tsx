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
import { useEffect, useState } from 'react'
import { RotateCcw } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import dayjs from '@/lib/dayjs'
import { formatResetCountdown, nextUsageReset } from './lib'
import type { UsageAccount } from './types'

export function ResetCountdown(props: { accounts: UsageAccount[] }) {
  const { t } = useTranslation()
  const [now, setNow] = useState(() => Date.now())
  useEffect(() => {
    const timer = setInterval(() => setNow(Date.now()), 30000)
    return () => clearInterval(timer)
  }, [])
  const resetAt = nextUsageReset(props.accounts)
  const label = resetAt === null ? '—' : formatResetCountdown(resetAt, now)
  return (
    <span
      className='inline-flex h-6 items-center gap-1.5'
      title={`${t('upstreamUsage.resetsAt')}: ${resetAt === null ? '—' : dayjs(resetAt).format('YYYY-MM-DD HH:mm:ss')}`}
    >
      <RotateCcw
        className='text-muted-foreground size-3.5 shrink-0'
        strokeWidth={1.5}
        aria-hidden='true'
      />
      <span className='sr-only'>
        {t('upstreamUsage.details')} · {t('upstreamUsage.resetsAt')}:{' '}
      </span>
      <span className='text-foreground text-xs font-bold whitespace-nowrap tabular-nums'>
        {label}
      </span>
    </span>
  )
}

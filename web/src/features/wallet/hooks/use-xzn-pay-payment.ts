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
import { useCallback, useState } from 'react'
import i18next from 'i18next'
import { toast } from 'sonner'
import { isApiSuccess, requestXznPayPayment } from '../api'

function getSafePaymentUrl(data: unknown): string | null {
  if (!data || typeof data !== 'object' || !('payment_url' in data)) {
    return null
  }
  const value = data.payment_url
  if (typeof value !== 'string') {
    return null
  }
  try {
    const parsed = new URL(value)
    return parsed.protocol === 'http:' || parsed.protocol === 'https:'
      ? value
      : null
  } catch {
    return null
  }
}

export function useXznPayPayment() {
  const [processing, setProcessing] = useState(false)

  const processXznPayPayment = useCallback(
    async (topupAmount: number, payMethodIndex: number) => {
      setProcessing(true)
      try {
        const response = await requestXznPayPayment({
          amount: Math.floor(topupAmount),
          pay_method_index: payMethodIndex,
        })
        if (isApiSuccess(response)) {
          const paymentUrl = getSafePaymentUrl(response.data)
          if (paymentUrl) {
            window.open(paymentUrl, '_blank', 'noopener,noreferrer')
            toast.success(i18next.t('Redirecting to payment page...'))
            return true
          }
        }
        const dataMessage =
          typeof response.data === 'string' ? response.data : undefined
        toast.error(
          dataMessage || response.message || i18next.t('Payment request failed')
        )
        return false
      } catch {
        toast.error(i18next.t('Payment request failed'))
        return false
      } finally {
        setProcessing(false)
      }
    },
    []
  )

  return { processing, processXznPayPayment }
}

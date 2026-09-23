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
import { api } from '@/lib/api'
import type {
  BillingSummary,
  BillingTopUp,
  BillingUserTopUp,
  CostAction,
  CostInput,
} from './types'

interface Response<T> {
  success: boolean
  message?: string
  data: T
}

function unwrap<T>(response: Response<T>): T {
  if (!response.success) throw new Error(response.message || 'Operation failed')
  return response.data
}

export async function getBillingSummary(month: string) {
  const result = await api.get<Response<BillingSummary>>('/api/billing-audit', {
    params: { month },
  })
  return unwrap(result.data)
}

export async function getBillingTopUps(month: string, page: number) {
  const result = await api.get<
    Response<{ items: BillingTopUp[]; total: number }>
  >('/api/billing-audit/topups', { params: { month, page, page_size: 20 } })
  return unwrap(result.data)
}

export async function getBillingUserTopUps(month: string, page: number) {
  const result = await api.get<
    Response<{ items: BillingUserTopUp[]; total: number }>
  >('/api/billing-audit/topups', {
    params: { month, page, page_size: 20, view: 'users' },
  })
  return unwrap(result.data)
}

export async function saveBillingCost(action: CostAction, input: CostInput) {
  const path = '/api/billing-audit/costs'
  if (action.kind === 'create') {
    return unwrap((await api.post<Response<unknown>>(path, input)).data)
  }
  if (!action.cost) throw new Error('Missing cost')
  // Dates define the immutable billing anchor; mutations address a whole cycle.
  const update = {
    month: input.month,
    name: input.name,
    amount: input.amount,
    remark: input.remark,
    recurring: input.recurring,
    scope: action.kind === 'delete-all' ? 'all' : input.scope,
  }
  if (action.kind === 'delete' || action.kind === 'delete-all') {
    return unwrap(
      (
        await api.delete<Response<unknown>>(`${path}/${action.cost.id}`, {
          data: update,
        })
      ).data
    )
  }
  return unwrap(
    (await api.put<Response<unknown>>(`${path}/${action.cost.id}`, update)).data
  )
}

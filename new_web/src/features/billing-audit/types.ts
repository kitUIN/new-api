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
export interface BillingCost {
  id: number
  month: string
  start_month: string
  recurring: boolean
  exception: boolean
  name: string
  amount_cents: number
  remark: string
  allocation: 'month' | 'subscription'
  cycle_month: string
  period_start: string
  period_end: string
  period_amount_cents: number
  allocated_days: number
  period_days: number
}

export interface BillingSummary {
  month: string
  currency: string
  timezone: string
  queried_at: number
  recharge_amount: string
  refund_amount: string
  net_recharge: string
  estimated_cost: string
  actual_cost: string
  total_cost: string
  current_balance: string
  groups: { group: string; estimated_cost: string }[]
  costs: BillingCost[]
}

export interface BillingTopUp {
  id: number
  user_id: number
  trade_no: string
  payment_method: string
  complete_time: number
  billing_time: number
  money: number
  refunded_cents: number
  status: string
  manual_amount?: string
  original_amount?: string
  user: BillingAuditUser
}

export interface BillingAuditUser {
  username: string
  display_name: string
  qq_id: string
}

export interface BillingUserTopUp {
  user_id: number
  user: BillingAuditUser
  received: string
  refunded: string
  unconverted_count: number
}

export interface CostInput {
  month: string
  name: string
  amount: string
  remark: string
  recurring: boolean
  scope: 'month' | 'future' | 'all'
  allocation: 'month' | 'subscription'
  start_date: string
  end_date: string
}

export interface CostAction {
  kind: 'create' | 'edit' | 'delete' | 'delete-all'
  cost?: BillingCost
}

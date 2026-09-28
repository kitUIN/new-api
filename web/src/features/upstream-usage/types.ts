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
export type UsageWindow = {
  utilization: number
  resets_at: string
  remaining_seconds: number
  window_stats: {
    requests: number
    tokens: number
    cost: number
    standard_cost: number
  } | null
}

export type UsageAccount = {
  id: number
  account_id: number
  groups: string[]
  fetched_at: number
  last_error: string
  available: boolean
  usage: {
    updated_at: string
    five_hour: UsageWindow | null
    seven_day: UsageWindow | null
  } | null
}

export type UsageProvider = {
  id: number
  base_url: string
  interval_minutes: number
  last_polled_at: number
  accounts: UsageAccount[]
}

export type UsageProviderInput = {
  base_url: string
  access_token: string
  refresh_token: string
  interval_minutes: number
  accounts: { account_id: number; groups: string[] }[]
}

export type GroupUsageAccount = UsageAccount & { base_url?: string }

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
export type PerformanceSeriesPoint = {
  ts: number
  avg_ttft_ms: number
  avg_latency_ms: number
  success_rate: number
  avg_tps: number
}

export type PerformanceGroup = {
  group: string
  avg_ttft_ms: number
  avg_latency_ms: number
  success_rate: number
  avg_tps: number
  series: PerformanceSeriesPoint[]
}

export type PerformanceMetricsData = {
  success: boolean
  message?: string
  data: {
    model_name: string
    series_schema?: string
    groups: PerformanceGroup[]
  }
}

export type PerfModelSummary = {
  model_name: string
  avg_latency_ms: number
  success_rate: number
  avg_tps: number
  request_count?: number
}

export type PerfSummaryAllData = {
  success: boolean
  message?: string
  data: {
    models: PerfModelSummary[]
  }
}

export type PerfGroupHealthStatus = 'empty' | 'ok' | 'warning' | 'error'

export type PerfGroupHealthBucket = {
  ts: number
  end_ts: number
  request_count: number
  success_count: number
  test_request_count?: number
  non_test_request_count?: number
  success_rate: number
  avg_ttft_ms: number
  avg_latency_ms: number
  avg_tps: number
  status: PerfGroupHealthStatus
}

export type PerfGroupHealth = {
  group: string
  ratio: number
  provider_count: number
  balance_level: 0 | 1 | 2
  juice: string
  juice_updated_time: number
  request_count: number
  success_rate: number
  avg_ttft_ms: number
  avg_latency_ms: number
  avg_tps: number
  recent_request_count?: number
  recent_success_rate?: number
  recent_window_minutes?: number
  buckets: PerfGroupHealthBucket[]
}

export type PerfGroupHealthData = {
  success: boolean
  message?: string
  data: {
    window_hours: number
    interval_minutes: number
    bucket_count: number
    series_schema?: string
    groups: PerfGroupHealth[]
  }
}

export type GroupRatioHistoryPoint = {
  ts: number
  ratio: number
  source?: string
}

export type GroupRatioHistorySeries = {
  group: string
  points: GroupRatioHistoryPoint[]
}

export type GroupRatioHistoryData = {
  success: boolean
  message?: string
  data: {
    start_ts: number
    end_ts: number
    groups: GroupRatioHistorySeries[]
  }
}

export type GroupJuiceHistoryPoint = {
  ts: number
  juice: string
  channel_id?: number
  source?: string
}

export type GroupJuiceHistorySeries = {
  group: string
  points: GroupJuiceHistoryPoint[]
}

export type GroupJuiceHistoryData = {
  success: boolean
  message?: string
  data: {
    start_ts: number
    end_ts: number
    groups: GroupJuiceHistorySeries[]
  }
}

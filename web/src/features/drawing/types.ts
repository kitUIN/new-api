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
export type DrawingStatus = 'pending' | 'processing' | 'success' | 'failure'

export type DrawingSession = {
  id?: number
  session_id: string
  user_id?: number
  title?: string
  image_count?: number
  created_at?: number
  updated_at?: number
}

export type DrawingMessage = {
  id: number
  session_id: string
  task_id: string | null
  prompt: string
  model: string
  size: string
  quality: string
  status: DrawingStatus
  fail_reason?: string
  image_urls?: string[] | string | null
  result_data?: DrawingImageResult[] | string | null
  created_at: number
  optimistic?: boolean
}

export type DrawingMessagePage = {
  message: DrawingMessage | null
  current_index: number
  total: number
  has_prev: boolean
  has_next: boolean
}

export type DrawingGenerateRequest = {
  prompt: string
  model: string
  size: string
  quality: string
  images: string[]
}

export type DrawingTaskStatus = {
  task_id: string
  status: 'SUBMITTED' | 'IN_PROGRESS' | 'SUCCESS' | 'FAILURE' | string
  progress?: string
  fail_reason?: string
  result_data?: DrawingImageResult[] | string | null
}

export type DrawingImageResult = {
  url?: string
  b64_json?: string
  revised_prompt?: string
  [key: string]: unknown
}

export type DrawingMessageImages = {
  image_urls?: string[] | string | null
  result_data?: DrawingImageResult[] | string | null
}

export type DrawingBalanceInfo = {
  balanceText: string
  balanceUSD: number
  availableGenerationsText: string
  modelName: string
  priceText: string
  priceUnavailable: string
  pricingLoading: boolean
  tone: 'success' | 'warning' | 'danger'
  usedGroup: string
}

export type ApiEnvelope<T> = {
  success: boolean
  message?: string
  data: T
}

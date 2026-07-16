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
import { z } from 'zod'

// ============================================================================
// Channel Schema & Types
// ============================================================================

export const channelInfoSchema = z.object({
  is_multi_key: z.boolean().default(false),
  multi_key_size: z.number().default(0),
  multi_key_status_list: z.record(z.string(), z.number()).optional(),
  multi_key_disabled_reason: z.record(z.string(), z.string()).optional(),
  multi_key_disabled_time: z.record(z.string(), z.number()).optional(),
  multi_key_polling_index: z.number().default(0),
  multi_key_mode: z.enum(['random', 'polling']).default('random'),
})

export type ChannelInfo = z.infer<typeof channelInfoSchema>

export const channelProviderSummarySchema = z.object({
  id: z.number(),
  name: z.string(),
  base_url: z.string(),
  status: z.number(),
  balance: z.number().optional(),
  balance_updated_time: z.number().optional(),
  settings: z.string().optional(),
})

export const channelSchema = z.object({
  id: z.number(),
  provider_id: z.number().default(0),
  provider: channelProviderSummarySchema.optional(),
  type: z.number(),
  key: z.string(),
  openai_organization: z.string().nullish(),
  test_model: z.string().nullish(),
  status: z.number(), // 1: enabled, 0: manual disabled, 2: auto disabled
  name: z.string(),
  weight: z.number().nullish(),
  created_time: z.number(),
  test_time: z.number(),
  response_time: z.number(), // in milliseconds
  base_url: z.string().nullish(),
  other: z.string().default(''),
  balance: z.number().default(0), // in USD
  balance_updated_time: z.number(),
  models: z.string().default(''),
  group: z.string().default('default'),
  used_quota: z.number().default(0),
  model_mapping: z.string().nullish(),
  status_code_mapping: z.string().nullish(),
  priority: z.number().nullish(),
  auto_ban: z.number().nullish(),
  other_info: z.string().default(''),
  tag: z.string().nullish(),
  setting: z.string().nullish(),
  param_override: z.string().nullish(),
  header_override: z.string().nullish(),
  remark: z.string().default(''),
  max_input_tokens: z.number().default(0),
  channel_info: channelInfoSchema.default({
    is_multi_key: false,
    multi_key_size: 0,
    multi_key_polling_index: 0,
    multi_key_mode: 'random',
  }),
  settings: z.string().default('{}'), // other_settings JSON
})

export type Channel = z.infer<typeof channelSchema>

export type ProviderRow = {
  id: string
  key: string
  is_provider: true
  provider_id: number
  name: string
  base_url: string
  status: number
  group: string
  settings?: string
  balance?: number
  balance_updated_time?: number
  used_quota: number
  response_time: number
  priority: number | string | null
  weight: number | string | null
  type?: number
  created_time?: number
  test_time?: number
  models?: string
  channel_info?: ChannelInfo
  channel_count: number
  enabled_count: number
  children: Channel[]
}

export type ChannelRow = Channel | ProviderRow

export type ChannelProvider = z.infer<typeof channelProviderSummarySchema> & {
  created_time?: number
  updated_time?: number
  remark?: string
}

// ============================================================================
// Channel Settings Types
// ============================================================================

export interface ChannelSettings {
  force_format?: boolean
  thinking_to_content?: boolean
  proxy?: string
  pass_through_body_enabled?: boolean
  system_prompt?: string
  system_prompt_override?: boolean
}

export type QueryRequestConfig = {
  url?: string
  method?: string
  headers?: Record<string, string>
  body?: string
}

export type BalanceQueryExtractorConfig = {
  plan_name_path?: string
  remaining_path?: string
  used_path?: string
  total_path?: string
  unit_path?: string
  unit?: string
  divisor?: number
  success_path?: string
  success_value?: string
  success_optional?: boolean
  message_path?: string
}

export type BalanceQueryResult = {
  is_valid: boolean
  invalid_message?: string
  plan_name?: string
  remaining?: number
  used?: number
  total?: number
  unit?: string
  checked_at?: number
}

export type BalanceQueryConfig = {
  enabled?: boolean
  template?: string
  interval_seconds?: number
  source_channel_id?: number
  access_token?: string
  refresh_token?: string
  user_id?: string
  request?: QueryRequestConfig
  extractor?: BalanceQueryExtractorConfig
  last_result?: BalanceQueryResult | null
  last_check_time?: number
  last_error?: string
}

export type GroupQueryExtractorConfig = {
  data_path?: string
  desc_path?: string
  ratio_path?: string
  success_path?: string
  success_value?: string
  success_optional?: boolean
  message_path?: string
}

export type GroupQueryItem = {
  desc?: string
  ratio?: number
}

export type GroupQueryConfig = {
  enabled?: boolean
  template?: string
  interval_seconds?: number
  source_channel_id?: number
  access_token?: string
  refresh_token?: string
  user_id?: string
  request?: QueryRequestConfig
  extractor?: GroupQueryExtractorConfig
  last_result?: Record<string, GroupQueryItem> | null
  last_check_time?: number
  last_error?: string
}

export interface ChannelOtherSettings {
  azure_responses_version?: string
  vertex_key_type?: 'json' | 'api_key'
  openrouter_enterprise?: boolean
  aws_key_type?: 'ak_sk' | 'api_key'
  allow_service_tier?: boolean
  disable_store?: boolean
  allow_safety_identifier?: boolean
  allow_include_obfuscation?: boolean
  disable_responses_image_generation_tool_filter?: boolean
  auto_test_enabled?: boolean
  allow_inference_geo?: boolean
  allow_speed?: boolean
  claude_beta_query?: boolean
  upstream_model_update_check_enabled?: boolean
  upstream_model_update_auto_sync_enabled?: boolean
  upstream_model_update_ignored_models?: string[]
  upstream_model_update_last_check_time?: number
  upstream_model_update_last_detected_models?: string[]
  balance_query?: BalanceQueryConfig
  group_query?: GroupQueryConfig
}

// ============================================================================
// API Response Types
// ============================================================================

export interface GetChannelsResponse {
  success: boolean
  message?: string
  data?: {
    items: ChannelRow[]
    total: number
    page: number
    page_size: number
    type_counts?: Record<string, number>
  }
}

export interface SearchChannelsResponse {
  success: boolean
  message?: string
  data?: {
    items: ChannelRow[]
    total: number
    type_counts?: Record<string, number>
  }
}

export interface GetChannelResponse {
  success: boolean
  message?: string
  data?: Channel
}

export interface ChannelTestResponse {
  success: boolean
  message?: string
  error_code?: string
  /** Test duration returned by the channel test endpoint, in seconds. */
  time?: number
  data?: {
    /** Backward-compatible response time returned by older deployments, in milliseconds. */
    response_time?: number
    error?: string
  }
}

export interface ChannelBalanceResponse {
  success: boolean
  message?: string
  balance?: number
  currency?: string
}

export interface FetchModelsResponse {
  success: boolean
  message?: string
  data?: string[]
}

export interface CopyChannelResponse {
  success: boolean
  message?: string
  data?: {
    id: number
  }
}

export interface QueryInstance {
  id: number
  name?: string
  type?: number
  template?: string
  interval_seconds?: number
  last_check_time?: number
  last_error?: string
  last_result?: BalanceQueryResult | Record<string, GroupQueryItem> | null
}

export interface QueryInstancesResponse {
  success: boolean
  message?: string
  data?: QueryInstance[]
}

export interface ChannelProvidersResponse {
  success: boolean
  message?: string
  data?: {
    items: ChannelProvider[]
    total: number
    page: number
    page_size: number
  }
}

// ============================================================================
// Multi-Key Management Types
// ============================================================================

export interface KeyStatus {
  index: number
  status: number // 1: enabled, 2: manual disabled, 3: auto disabled
  disabled_time?: number
  reason?: string
  key_preview?: string
}

export type MultiKeyConfirmAction = {
  type:
    | 'enable'
    | 'disable'
    | 'delete'
    | 'enable-all'
    | 'disable-all'
    | 'delete-disabled'
  keyIndex?: number
}

export interface MultiKeyStatusResponse {
  success: boolean
  message?: string
  data?: {
    keys: KeyStatus[]
    total: number
    page: number
    page_size: number
    total_pages: number
    enabled_count: number
    manual_disabled_count: number
    auto_disabled_count: number
  }
}

// ============================================================================
// API Request Parameters
// ============================================================================

export type ChannelSortBy =
  | 'id'
  | 'name'
  | 'priority'
  | 'balance'
  | 'response_time'
  | 'test_time'

export type ChannelSortOrder = 'asc' | 'desc'

export interface GetChannelsParams {
  p?: number
  page_size?: number
  status?: string // 'enabled', 'disabled', or empty for all
  type?: number
  group?: string
  id_sort?: boolean
  tag_mode?: boolean
  provider_mode?: boolean
  sort_by?: ChannelSortBy
  sort_order?: ChannelSortOrder
}

export interface SearchChannelsParams {
  keyword?: string
  group?: string
  model?: string
  status?: string
  type?: number
  id_sort?: boolean
  tag_mode?: boolean
  provider_mode?: boolean
  sort_by?: ChannelSortBy
  sort_order?: ChannelSortOrder
  p?: number
  page_size?: number
}

export interface ChannelTestParams {
  test_model?: string
}

export interface CopyChannelParams {
  suffix?: string
  reset_balance?: boolean
}

export interface MultiKeyManageParams {
  channel_id: number
  action:
    | 'get_key_status'
    | 'disable_key'
    | 'enable_key'
    | 'enable_all_keys'
    | 'disable_all_keys'
    | 'delete_key'
    | 'delete_disabled_keys'
  key_index?: number
  page?: number
  page_size?: number
  status?: number // 1=enabled, 2=manual_disabled, 3=auto_disabled
}

export interface BatchDeleteParams {
  ids: number[]
}

export interface BatchSetTagParams {
  ids: number[]
  tag: string | null
}

export interface TagOperationParams {
  tag: string
  new_tag?: string
  priority?: number
  weight?: number
  model_mapping?: string
  models?: string
  groups?: string
}

// ============================================================================
// Form Data Types
// ============================================================================

export interface ChannelFormData {
  provider_id?: number
  name: string
  type: number
  base_url: string
  key: string
  openai_organization?: string
  models: string
  group: string
  model_mapping?: string
  priority?: number
  weight?: number
  test_model?: string
  auto_ban?: number
  auto_test_enabled?: boolean
  status: number
  status_code_mapping?: string
  tag?: string
  remark?: string
  setting?: string
  param_override?: string
  header_override?: string
  settings?: string
  other?: string
  // Multi-key specific
  multi_key_mode?: 'single' | 'batch' | 'multi_to_single'
  multi_key_type?: 'random' | 'polling'
  batch_add_set_key_prefix_2_name?: boolean
  // Balance query settings (stored in settings JSON)
  balance_query_enabled?: boolean
  balance_query_template?: string
  balance_query_interval_seconds?: number
  balance_query_source_channel_id?: number
  balance_query_access_token?: string
  balance_query_refresh_token?: string
  balance_query_user_id?: string
  balance_query_request_url?: string
  balance_query_request_method?: string
  balance_query_request_headers?: string
  balance_query_request_body?: string
  balance_query_plan_name_path?: string
  balance_query_remaining_path?: string
  balance_query_used_path?: string
  balance_query_total_path?: string
  balance_query_unit_path?: string
  balance_query_unit?: string
  balance_query_divisor?: number
  balance_query_success_path?: string
  balance_query_success_value?: string
  balance_query_success_optional?: boolean
  balance_query_message_path?: string
  balance_query_last_check_time?: number
  balance_query_last_result?: BalanceQueryResult | null
  balance_query_last_error?: string
  // Group query settings (stored in settings JSON)
  group_query_enabled?: boolean
  group_query_template?: string
  group_query_interval_seconds?: number
  group_query_source_channel_id?: number
  group_query_access_token?: string
  group_query_refresh_token?: string
  group_query_user_id?: string
  group_query_request_url?: string
  group_query_request_method?: string
  group_query_request_headers?: string
  group_query_request_body?: string
  group_query_data_path?: string
  group_query_desc_path?: string
  group_query_ratio_path?: string
  group_query_success_path?: string
  group_query_success_value?: string
  group_query_success_optional?: boolean
  group_query_message_path?: string
  group_query_last_check_time?: number
  group_query_last_result?: Record<string, GroupQueryItem> | null
  group_query_last_error?: string
}

// ============================================================================
// Add Channel Request (special structure)
// ============================================================================

export interface AddChannelRequest {
  mode: 'single' | 'batch' | 'multi_to_single'
  multi_key_mode?: 'random' | 'polling'
  batch_add_set_key_prefix_2_name?: boolean
  channel: Partial<Channel> & { provider_id?: number }
}

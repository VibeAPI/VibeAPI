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
  ConfirmPaymentComplianceResponse,
  FetchUpstreamRatiosRequest,
  LogCleanupTask,
  SystemOptionsResponse,
  SystemTaskListResponse,
  SystemTaskResponse,
  UpdateOptionRequest,
  UpdateOptionResponse,
  UpstreamChannelsResponse,
  UpstreamRatiosResponse,
} from './types'

export async function getSystemOptions() {
  const res = await api.get<SystemOptionsResponse>('/api/option/')
  return res.data
}

export async function updateSystemOption(request: UpdateOptionRequest) {
  const res = await api.put<UpdateOptionResponse>('/api/option/', request)
  return res.data
}

export type UpstreamBalanceAccountSetting = {
  id: string
  name: string
  base_url: string
  token: string
  has_token: boolean
}

export type UpstreamBalanceSettings = {
  enabled: boolean
  visibility: 'all' | 'admin'
  refresh_interval_seconds: number
  accounts: UpstreamBalanceAccountSetting[]
}

export async function getUpstreamBalanceSettings() {
  const res = await api.get<{
    success: boolean
    message: string
    data?: UpstreamBalanceSettings
  }>('/api/option/upstream-balances', {
    skipBusinessError: true,
    skipErrorHandler: true,
  })
  if (!res.data.success) {
    throw new Error(res.data.message || 'Failed to load settings')
  }
  return res.data
}

export async function updateUpstreamBalanceSettings(
  request: UpstreamBalanceSettings
) {
  const res = await api.put<UpdateOptionResponse>(
    '/api/option/upstream-balances',
    request,
    {
      skipBusinessError: true,
      skipErrorHandler: true,
    }
  )
  return res.data
}

export type PromptAuditEndpointSetting = {
  url: string
  api_key: string
  has_api_key: boolean
  model: string
  system_prompt: string
  timeout_seconds: number
}

export type PromptAuditSettings = {
  enabled: boolean
  mode: 'downgrade' | 'reject'
  protected_channel_ids: number[]
  audience_user_ids: number[]
  content_scope: 'latest' | 'latest_tools' | 'all'
  max_characters: number
  main_threshold: number
  allow_private_endpoints: boolean
  reject_message: string
  version: number
  tested_version: number
  main_tested_version: number
  main: PromptAuditEndpointSetting
  restricted_users: PromptAuditBlacklistUser[]
  protected_channel_count: number
}

export async function getPromptAuditSettings() {
  const res = await api.get<{
    success: boolean
    message: string
    data?: Omit<
      PromptAuditSettings,
      | 'protected_channel_ids'
      | 'audience_user_ids'
      | 'restricted_users'
      | 'protected_channel_count'
    > & {
      protected_channel_ids?: number[] | null
      audience_user_ids?: number[] | null
      restricted_users?: PromptAuditBlacklistUser[] | null
      protected_channel_count?: number | null
    }
  }>('/api/option/prompt-audit', {
    skipBusinessError: true,
    skipErrorHandler: true,
  })
  if (!res.data.success || !res.data.data) {
    throw new Error(res.data.message || 'Failed to load prompt audit settings')
  }
  return {
    ...res.data.data,
    protected_channel_ids: res.data.data.protected_channel_ids ?? [],
    audience_user_ids: res.data.data.audience_user_ids ?? [],
    restricted_users: res.data.data.restricted_users ?? [],
    protected_channel_count: res.data.data.protected_channel_count ?? 0,
  }
}

export async function updatePromptAuditSettings(
  request: Omit<
    PromptAuditSettings,
    'restricted_users' | 'protected_channel_count'
  >
) {
  const res = await api.put<UpdateOptionResponse>(
    '/api/option/prompt-audit',
    request,
    { skipBusinessError: true, skipErrorHandler: true }
  )
  return res.data
}

export async function testPromptAuditSettings(
  content: string
) {
  const res = await api.post<UpdateOptionResponse>(
    '/api/option/prompt-audit/test',
    { stage: 'main', content },
    { skipBusinessError: true, skipErrorHandler: true }
  )
  return res.data
}

export async function setPromptAuditEnabled(enabled: boolean) {
  const res = await api.post<UpdateOptionResponse>(
    `/api/option/prompt-audit/${enabled ? 'enable' : 'disable'}`,
    undefined,
    { skipBusinessError: true, skipErrorHandler: true }
  )
  return res.data
}

export type PromptAuditBlacklistUser = {
  user_id: number
  username: string
  display_name: string
  email: string
  remark: string
  channel_count: number
}

export async function removePromptAuditBlacklistUser(userId: number) {
  const res = await api.delete<UpdateOptionResponse>(
    `/api/option/prompt-audit/restricted-users/${userId}`
  )
  return res.data
}

export async function confirmPaymentCompliance() {
  const res = await api.post<ConfirmPaymentComplianceResponse>(
    '/api/option/payment_compliance',
    { confirmed: true }
  )
  return res.data
}

export async function startLogCleanupTask(targetTimestamp: number) {
  const res = await api.post<SystemTaskResponse<LogCleanupTask>>(
    '/api/system-task/log-cleanup',
    null,
    {
      params: { target_timestamp: targetTimestamp },
    }
  )
  return res.data
}

export async function getCurrentLogCleanupTask() {
  const res = await api.get<SystemTaskResponse<LogCleanupTask | null>>(
    '/api/system-task/current',
    {
      params: { type: 'log_cleanup' },
    }
  )
  return res.data
}

export async function getSystemTask(taskId: string) {
  const res = await api.get<SystemTaskResponse<LogCleanupTask>>(
    `/api/system-task/${taskId}`
  )
  return res.data
}

export async function listSystemTasks(limit = 20) {
  const res = await api.get<SystemTaskListResponse>('/api/system-task/list', {
    params: { limit },
  })
  return res.data
}

export async function resetModelRatios() {
  const res = await api.post<UpdateOptionResponse>(
    '/api/option/rest_model_ratio'
  )
  return res.data
}

export async function getUpstreamChannels() {
  const res = await api.get<UpstreamChannelsResponse>(
    '/api/ratio_sync/channels'
  )
  return res.data
}

export async function fetchUpstreamRatios(request: FetchUpstreamRatiosRequest) {
  const res = await api.post<UpstreamRatiosResponse>(
    '/api/ratio_sync/fetch',
    request
  )
  return res.data
}

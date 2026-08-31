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
  audience_mode: 'all' | 'whitelist' | 'blacklist'
  audience_user_ids: number[]
  content_scope: 'latest' | 'latest_tools' | 'all'
  max_characters: number
  main_threshold: number
  review_threshold: number
  review_enabled: boolean
  required_valid_votes: number
  required_flagged_votes: number
  review_total_timeout_seconds: number
  allow_private_endpoints: boolean
  first_restriction_hours: number
  second_restriction_hours: number
  violation_reset_days: number
  dedupe_minutes: number
  retention_days: number
  reject_message: string
  appeal_contact: string
  version: number
  tested_version: number
  main_tested_version: number
  review_tested_version: number
  main: PromptAuditEndpointSetting
  review: PromptAuditEndpointSetting
}

export async function getPromptAuditSettings() {
  const res = await api.get<{
    success: boolean
    message: string
    data?: PromptAuditSettings
  }>('/api/option/prompt-audit', {
    skipBusinessError: true,
    skipErrorHandler: true,
  })
  if (!res.data.success || !res.data.data) {
    throw new Error(res.data.message || 'Failed to load prompt audit settings')
  }
  return res.data.data
}

export async function updatePromptAuditSettings(request: PromptAuditSettings) {
  const res = await api.put<UpdateOptionResponse>(
    '/api/option/prompt-audit',
    request,
    { skipBusinessError: true, skipErrorHandler: true }
  )
  return res.data
}

export async function testPromptAuditSettings(
  stage: 'main' | 'review',
  content: string
) {
  const res = await api.post<UpdateOptionResponse>(
    '/api/option/prompt-audit/test',
    { stage, content },
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

export type PromptAuditEvent = {
  event_id: string
  user_id: number
  request_id: string
  status: 'pending' | 'safe' | 'violation' | 'failed'
  categories: string
  main_confidence: number
  valid_votes: number
  flagged_votes: number
  origin_channel_id: number
  final_channel_id: number
  content_truncated: boolean
  failure_type: string
  latency_ms: number
  email_status: string
  email_error: string
  created_at: number
}

export async function getPromptAuditEvents(status = '') {
  const res = await api.get<{
    success: boolean
    message: string
    data?: { items: PromptAuditEvent[]; total: number }
  }>('/api/option/prompt-audit/events', { params: { status, page_size: 50 } })
  return res.data
}

export async function clearPromptAuditRestriction(
  userId: number,
  resetCount: boolean
) {
  const res = await api.post<UpdateOptionResponse>(
    '/api/option/prompt-audit/restriction/clear',
    { user_id: userId, reset_count: resetCount }
  )
  return res.data
}

export async function resendPromptAuditEmail(eventId: string) {
  const res = await api.post<UpdateOptionResponse>(
    `/api/option/prompt-audit/events/${eventId}/resend-email`
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

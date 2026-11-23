import { api } from '@/lib/api'
import { requireServerSuccess } from '@/lib/server-error-message'

import type {
  RequestRule,
  RuleEndpoint,
  RuleTestResult,
} from './request-rules/types'

export type PolicyConfig = {
  options: Record<string, string>
  request_rule_presets?: RequestRule[]
}
export type PolicyDecision = { action: string; reason: string; source: string }
export type PolicyEvent = {
  attempt: number
  channel_id?: number
  group?: string
  rule?: string
  status?: number
  error_code?: string
  error_source?: string
  elapsed_ms: number
  decision: PolicyDecision
  health?: string
}
type Response<T> = { success: boolean; message?: string; data: T }
export async function getPolicyConfig() {
  const response = await api.get<Response<PolicyConfig>>(
    '/api/option/request_policy'
  )
  return requireServerSuccess(response.data).data
}
export async function savePolicyConfig(options: Record<string, string>) {
  const response = await api.patch<Response<PolicyConfig>>(
    '/api/option/request_policy',
    { options }
  )
  return requireServerSuccess(response.data).data
}

export type RuleTestRequest = {
  rules: string
  endpoint: RuleEndpoint
  model: string
  pass_through: boolean
  body: string
  headers: Record<string, string>
}

export async function testRequestRules(request: RuleTestRequest) {
  const response = await api.post<Response<RuleTestResult>>(
    '/api/option/request_policy/rules/test',
    request
  )
  return requireServerSuccess(response.data).data
}

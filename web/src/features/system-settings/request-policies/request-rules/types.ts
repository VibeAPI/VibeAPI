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
// Mirrors pkg/requestrule on the backend, which validates every saved rule.

export const RULE_ENDPOINT_VALUES = [
  'claude_messages',
  'openai_chat',
  'openai_responses',
  'gemini',
] as const

export type RuleEndpoint = (typeof RULE_ENDPOINT_VALUES)[number]

export const CONDITION_SOURCES = ['json', 'body', 'header'] as const

export type ConditionSource = (typeof CONDITION_SOURCES)[number]

export const CONDITION_OPS = [
  'regex',
  'not_regex',
  'exists',
  'not_exists',
] as const

export type ConditionOp = (typeof CONDITION_OPS)[number]

export type RuleMatchMode = 'all' | 'any'

export type RequestRuleCondition = {
  source: ConditionSource
  path: string
  op: ConditionOp
  value: string
}

export type RequestRuleResponse = {
  status: number
  code: string
  message: string
}

export type RequestRule = {
  id: string
  name: string
  description: string
  preset: boolean
  enabled: boolean
  endpoints: RuleEndpoint[]
  model_regex: string
  only_pass_through: boolean
  match: RuleMatchMode
  conditions: RequestRuleCondition[]
  response: RequestRuleResponse
}

export type RuleTraceResult =
  | 'disabled'
  | 'endpoint'
  | 'model'
  | 'pass_through'
  | 'conditions'
  | 'matched'
  | 'not_checked'

export type RuleTestResult = {
  matched: boolean
  rule?: { position: number; id: string; name: string }
  field?: string
  value?: string
  status?: number
  response?: unknown
  trace: Array<{
    position: number
    id: string
    name: string
    result: RuleTraceResult
  }>
}

// Endpoint names are API product names and stay untranslated.
export const RULE_ENDPOINTS: ReadonlyArray<{
  value: RuleEndpoint
  label: string
  path: string
}> = [
  { value: 'claude_messages', label: 'Claude Messages', path: '/v1/messages' },
  {
    value: 'openai_chat',
    label: 'OpenAI Chat',
    path: '/v1/chat/completions',
  },
  {
    value: 'openai_responses',
    label: 'OpenAI Responses',
    path: '/v1/responses',
  },
  { value: 'gemini', label: 'Gemini', path: '/v1beta/models/…' },
]

export const DEFAULT_RULE_ERROR_CODE = 'request_rule_blocked'
export const MAX_RULE_CONDITIONS = 10

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
// Mirrors operation_setting.UpstreamErrorRule on the backend, which validates
// every saved rule.

export const ERROR_RULE_ACTIONS = ['intercept', 'rewrite'] as const

export type ErrorRuleAction = (typeof ERROR_RULE_ACTIONS)[number]

export type ErrorRuleResponse = {
  status: number
  code: string
  message: string
}

export type UpstreamErrorRule = {
  name: string
  enabled: boolean
  // Comma-separated status codes and inclusive ranges; empty matches any.
  status_codes: string
  // Matched against the upstream error code and message, ignoring case.
  keywords: string[]
  action: ErrorRuleAction
  // Only rewrite rules answer with their own response.
  response?: ErrorRuleResponse
  // Ends the request with the rule's error at once: no retry and no
  // automatic disabling. Absent means the global settings apply.
  skip_retry?: boolean
}

// Intercepted errors get this site's own no-available-channel error. The
// message follows the caller's language; API callers usually get English.
export const INTERCEPT_RESPONSE: ErrorRuleResponse = {
  status: 503,
  code: 'model_not_found',
  message:
    'No available channel for model {{model}} under group {{group}} (distributor)',
}

// A new rewrite rule starts as "too many requests", which tells callers to
// slow down instead of retrying at once.
export const DEFAULT_REWRITE_RESPONSE: ErrorRuleResponse = {
  status: 429,
  code: 'rate_limit_exceeded',
  message: 'Too many requests, please retry later',
}

export const MAX_ERROR_RULE_KEYWORDS = 200
export const MAX_ERROR_RULE_KEYWORD_LENGTH = 200
export const MAX_ERROR_RULE_NAME_LENGTH = 60
export const MAX_ERROR_RULE_MESSAGE_LENGTH = 500

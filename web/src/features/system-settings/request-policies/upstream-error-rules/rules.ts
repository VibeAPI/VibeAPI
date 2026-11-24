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
import type { TFunction } from 'i18next'

import {
  DEFAULT_REWRITE_RESPONSE,
  INTERCEPT_RESPONSE,
  type ErrorRuleResponse,
  type UpstreamErrorRule,
} from './types'

type ParsedErrorRules =
  | { ok: true; rules: UpstreamErrorRule[] }
  | { ok: false; error: string }

function text(value: unknown): string {
  return typeof value === 'string' ? value : ''
}

// Fills the fields the editor relies on; the backend still validates each
// rule. An unknown action is an error rather than a silent default, since it
// would change what callers see.
function normalizeRule(value: unknown): UpstreamErrorRule | null {
  const item = (value ?? {}) as Record<string, unknown>
  if (item.action !== 'intercept' && item.action !== 'rewrite') return null
  const rule: UpstreamErrorRule = {
    name: text(item.name),
    enabled: item.enabled === true,
    status_codes: text(item.status_codes),
    keywords: Array.isArray(item.keywords)
      ? item.keywords.filter(
          (keyword): keyword is string => typeof keyword === 'string'
        )
      : [],
    action: item.action,
  }
  if (item.action === 'rewrite') {
    const response = (item.response ?? {}) as Record<string, unknown>
    rule.response = {
      status:
        typeof response.status === 'number'
          ? response.status
          : DEFAULT_REWRITE_RESPONSE.status,
      code: text(response.code) || DEFAULT_REWRITE_RESPONSE.code,
      message: text(response.message),
    }
  }
  if (item.skip_retry === true) rule.skip_retry = true
  return rule
}

export function parseErrorRulesJson(json: string): ParsedErrorRules {
  if (!json.trim()) return { ok: true, rules: [] }
  let value: unknown
  try {
    value = JSON.parse(json)
  } catch {
    return { ok: false, error: 'Rules JSON is not valid JSON' }
  }
  if (!Array.isArray(value)) {
    return { ok: false, error: 'Rules JSON must be an array of rules' }
  }
  const rules: UpstreamErrorRule[] = []
  for (const item of value) {
    const rule = normalizeRule(item)
    if (!rule) {
      return {
        ok: false,
        error: 'Each rule needs an action of intercept or rewrite',
      }
    }
    rules.push(rule)
  }
  return { ok: true, rules }
}

export function formatErrorRulesJson(rules: UpstreamErrorRule[]): string {
  return JSON.stringify(rules, null, 2)
}

// Stored JSON is shown formatted so the JSON tab stays readable.
export function formatStoredErrorRules(stored: string): string {
  const parsed = parseErrorRulesJson(stored)
  return parsed.ok ? formatErrorRulesJson(parsed.rules) : stored
}

// The option is saved as compact JSON, so comparing it ignores spacing.
export function compactErrorRulesJson(json: string): string | null {
  const parsed = parseErrorRulesJson(json)
  return parsed.ok ? JSON.stringify(parsed.rules) : null
}

export function createBlankErrorRule(): UpstreamErrorRule {
  return {
    name: '',
    enabled: true,
    status_codes: '',
    keywords: [],
    action: 'intercept',
    skip_retry: true,
  }
}

// The error a caller receives when the rule matches.
export function errorRuleResponse(rule: UpstreamErrorRule): ErrorRuleResponse {
  return rule.action === 'rewrite' && rule.response
    ? rule.response
    : INTERCEPT_RESPONSE
}

export function errorRuleDisplayName(
  t: TFunction,
  rule: UpstreamErrorRule,
  index: number
): string {
  return rule.name || t('Rule {{position}}', { position: index + 1 })
}

export function splitKeywords(value: string): string[] {
  return value
    .split(/\r?\n/)
    .map((keyword) => keyword.trim())
    .filter(Boolean)
}

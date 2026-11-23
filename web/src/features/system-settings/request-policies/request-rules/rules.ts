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
  CONDITION_OPS,
  CONDITION_SOURCES,
  DEFAULT_RULE_ERROR_CODE,
  RULE_ENDPOINT_VALUES,
  type RequestRule,
  type RequestRuleCondition,
} from './types'

type ParsedRules =
  | { ok: true; rules: RequestRule[] }
  | { ok: false; error: string }

function text(value: unknown): string {
  return typeof value === 'string' ? value : ''
}

// Fills fields the editor relies on; the backend still validates each rule.
function normalizeRule(value: unknown): RequestRule {
  const item = (value ?? {}) as Record<string, unknown>
  const response = (item.response ?? {}) as Record<string, unknown>
  const endpoints = Array.isArray(item.endpoints) ? item.endpoints : []
  const conditions: RequestRuleCondition[] = Array.isArray(item.conditions)
    ? item.conditions.map((entry: unknown) => {
        const condition = (entry ?? {}) as Record<string, unknown>
        return {
          source:
            CONDITION_SOURCES.find((s) => s === condition.source) ?? 'json',
          path: text(condition.path),
          op: CONDITION_OPS.find((o) => o === condition.op) ?? 'regex',
          value: text(condition.value),
        }
      })
    : []
  return {
    id: text(item.id),
    name: text(item.name),
    description: text(item.description),
    preset: item.preset === true,
    enabled: item.enabled === true,
    endpoints: RULE_ENDPOINT_VALUES.filter((e) => endpoints.includes(e)),
    model_regex: text(item.model_regex),
    only_pass_through: item.only_pass_through === true,
    match: item.match === 'any' ? 'any' : 'all',
    conditions,
    response: {
      status: typeof response.status === 'number' ? response.status : 400,
      code: text(response.code) || DEFAULT_RULE_ERROR_CODE,
      message: text(response.message),
    },
  }
}

export function parseRulesJson(json: string): ParsedRules {
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
  return { ok: true, rules: value.map(normalizeRule) }
}

export function formatRulesJson(rules: RequestRule[]): string {
  return JSON.stringify(rules, null, 2)
}

// The stored option is compact JSON; comparing parsed rules ignores spacing.
export function compactRulesJson(json: string): string | null {
  const parsed = parseRulesJson(json)
  return parsed.ok ? JSON.stringify(parsed.rules) : null
}

export function createRuleId(rules: RequestRule[]): string {
  const taken = new Set(rules.map((rule) => rule.id))
  const base = `rule_${Date.now().toString(36)}`
  let id = base
  for (let n = 2; taken.has(id); n++) {
    id = `${base}_${n}`
  }
  return id
}

export function createBlankRule(rules: RequestRule[]): RequestRule {
  return {
    id: createRuleId(rules),
    name: '',
    description: '',
    preset: false,
    enabled: true,
    endpoints: ['claude_messages'],
    model_regex: '',
    only_pass_through: false,
    match: 'all',
    conditions: [{ source: 'json', path: '', op: 'regex', value: '' }],
    response: { status: 400, code: DEFAULT_RULE_ERROR_CODE, message: '' },
  }
}

export type RuleTemplate = {
  key: string
  rule: Omit<RequestRule, 'id'>
}

function template(
  key: string,
  rule: Pick<
    RequestRule,
    'name' | 'description' | 'endpoints' | 'model_regex' | 'conditions'
  > &
    Partial<Pick<RequestRule, 'match'>> & { message: string }
): RuleTemplate {
  return {
    key,
    rule: {
      name: rule.name,
      description: rule.description,
      preset: false,
      enabled: true,
      endpoints: rule.endpoints,
      model_regex: rule.model_regex,
      only_pass_through: false,
      match: rule.match ?? 'all',
      conditions: rule.conditions,
      response: {
        status: 400,
        code: DEFAULT_RULE_ERROR_CODE,
        message: rule.message,
      },
    },
  }
}

// Starting points for upstream 400s seen in production logs. Models follow
// the ones that produced the errors; widen the regex when other models share
// the limit. Client-facing messages stay in English like other API errors.
export function ruleTemplates(t: TFunction): RuleTemplate[] {
  return [
    template('opus-tool-choice', {
      name: t('Forced tool use on claude-opus-5-5'),
      description: t(
        'tool_choice set to any or tool, which claude-opus-5-5 does not support.'
      ),
      endpoints: ['claude_messages'],
      model_regex: '^claude-opus-5-5',
      conditions: [
        {
          source: 'json',
          path: 'tool_choice.type',
          op: 'regex',
          value: '^(any|tool)$',
        },
      ],
      message:
        '{{model}} does not support forcing a tool call (tool_choice any or tool). Use tool_choice auto.',
    }),
    template('clear-thinking', {
      name: t('clear_thinking without thinking'),
      description: t(
        'Uses the clear_thinking context edit without turning on thinking.'
      ),
      endpoints: ['claude_messages'],
      model_regex: '',
      conditions: [
        {
          source: 'json',
          path: 'context_management.edits.#.type',
          op: 'regex',
          value: '^clear_thinking_',
        },
        {
          source: 'json',
          path: 'thinking.type',
          op: 'not_regex',
          value: '^(enabled|adaptive)$',
        },
      ],
      message:
        'The clear_thinking context edit requires thinking to be enabled or adaptive.',
    }),
    template('temperature-5-5', {
      name: t('temperature on Claude 5.5 models'),
      description: t(
        'Sets temperature, which is deprecated for Claude 5.5 models.'
      ),
      endpoints: ['claude_messages'],
      model_regex: '^claude-(opus|sonnet|haiku)-5-5',
      conditions: [
        { source: 'json', path: 'temperature', op: 'exists', value: '' },
      ],
      message:
        'temperature is deprecated for {{model}}. Remove it from the request.',
    }),
    template('prefill-sonnet-4-6', {
      name: t('Assistant prefill on claude-sonnet-4-6'),
      description: t(
        'The last message is from the assistant (prefill), which claude-sonnet-4-6 does not support.'
      ),
      endpoints: ['claude_messages'],
      model_regex: '^claude-sonnet-4-6',
      conditions: [
        {
          source: 'json',
          path: 'messages.-1.role',
          op: 'regex',
          value: '^assistant$',
        },
      ],
      message:
        '{{model}} does not support assistant message prefill. End the conversation with a user message.',
    }),
    template('adaptive-thinking', {
      name: t('Manual thinking on adaptive-only models'),
      description: t(
        'Sets thinking.type to enabled on models that only support adaptive thinking.'
      ),
      endpoints: ['claude_messages'],
      model_regex: '^claude-(opus-5-5|fable-5)',
      conditions: [
        {
          source: 'json',
          path: 'thinking.type',
          op: 'regex',
          value: '^enabled$',
        },
      ],
      message:
        '{{model}} requires adaptive thinking. Use thinking.type adaptive with output_config.effort.',
    }),
    template('gpt-6-effort-none', {
      name: t('Reasoning effort none on gpt-6'),
      description: t(
        'Sets reasoning effort to none, which the gpt-6 astra and sol models do not support.'
      ),
      endpoints: ['openai_chat', 'openai_responses'],
      model_regex: '^gpt-6(\\.\\d+)?-(astra|sol)',
      match: 'any',
      conditions: [
        {
          source: 'json',
          path: 'reasoning_effort',
          op: 'regex',
          value: '^none$',
        },
        {
          source: 'json',
          path: 'reasoning.effort',
          op: 'regex',
          value: '^none$',
        },
      ],
      message:
        '{{model}} does not support reasoning effort none. Use low or higher.',
    }),
  ]
}

// Preset names and descriptions are i18n keys until an admin edits them.
export function presetText(
  t: TFunction,
  value: string,
  presetValue: string | undefined
): string {
  return presetValue !== undefined && value === presetValue ? t(value) : value
}

export function ruleDisplayName(
  t: TFunction,
  rule: { id: string; name: string },
  presets: Map<string, RequestRule>
): string {
  return presetText(t, rule.name, presets.get(rule.id)?.name)
}

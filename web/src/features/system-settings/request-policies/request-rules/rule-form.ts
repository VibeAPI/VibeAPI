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
import * as z from 'zod'

import {
  CONDITION_OPS,
  CONDITION_SOURCES,
  MAX_RULE_CONDITIONS,
  RULE_ENDPOINT_VALUES,
  type ConditionOp,
  type RequestRule,
} from './types'

// Regex operators compare the values a condition finds; the others only check
// whether there are any.
export function isRegexOp(op: ConditionOp): boolean {
  return op === 'regex' || op === 'not_regex'
}

// Checks what the editor can check locally. Regex syntax follows Go (RE2),
// so the backend compiles every pattern when the rules are saved.
export function createRuleFormSchema(t: TFunction) {
  return z.object({
    name: z
      .string()
      .trim()
      .min(1, t('Name is required'))
      .max(60, t('Use at most {{count}} characters', { count: 60 })),
    description: z
      .string()
      .max(300, t('Use at most {{count}} characters', { count: 300 })),
    enabled: z.boolean(),
    endpoints: z
      .array(z.enum(RULE_ENDPOINT_VALUES))
      .min(1, t('Select at least one endpoint')),
    model_regex: z
      .string()
      .max(500, t('Use at most {{count}} characters', { count: 500 })),
    only_pass_through: z.boolean(),
    match: z.enum(['all', 'any']),
    conditions: z
      .array(
        z
          .object({
            source: z.enum(CONDITION_SOURCES),
            path: z.string(),
            op: z.enum(CONDITION_OPS),
            value: z.string(),
          })
          .superRefine((condition, ctx) => {
            if (condition.source !== 'body' && !condition.path.trim()) {
              ctx.addIssue({
                code: 'custom',
                path: ['path'],
                message: t('Path is required'),
              })
            }
            const isRegex = isRegexOp(condition.op)
            if (condition.source === 'body' && !isRegex) {
              ctx.addIssue({
                code: 'custom',
                path: ['op'],
                message: t('The request body can only be matched with a regex'),
              })
            }
            if (isRegex && !condition.value) {
              ctx.addIssue({
                code: 'custom',
                path: ['value'],
                message: t('Regex is required'),
              })
            }
          })
      )
      .min(1, t('Add at least one condition'))
      .max(MAX_RULE_CONDITIONS),
    status: z
      .number({ error: t('Enter a status code between 400 and 499') })
      .int()
      .min(400, t('Enter a status code between 400 and 499'))
      .max(499, t('Enter a status code between 400 and 499')),
    code: z
      .string()
      .regex(
        /^[A-Za-z0-9_.-]{1,64}$/,
        t('Use 1-64 letters, digits, dots, dashes or underscores')
      ),
    message: z
      .string()
      .trim()
      .min(1, t('Error message is required'))
      .max(500, t('Use at most {{count}} characters', { count: 500 })),
  })
}

export type RuleFormValues = z.infer<ReturnType<typeof createRuleFormSchema>>

export function ruleToFormValues(rule: RequestRule): RuleFormValues {
  return {
    name: rule.name,
    description: rule.description,
    enabled: rule.enabled,
    endpoints: rule.endpoints,
    model_regex: rule.model_regex,
    only_pass_through: rule.only_pass_through,
    match: rule.match,
    conditions: rule.conditions,
    status: rule.response.status,
    code: rule.response.code,
    message: rule.response.message,
  }
}

export function formValuesToRule(
  base: RequestRule,
  values: RuleFormValues
): RequestRule {
  return {
    ...base,
    name: values.name.trim(),
    description: values.description.trim(),
    enabled: values.enabled,
    endpoints: values.endpoints,
    model_regex: values.model_regex.trim(),
    only_pass_through: values.only_pass_through,
    match: values.match,
    conditions: values.conditions.map((condition) => ({
      source: condition.source,
      path: condition.source === 'body' ? '' : condition.path.trim(),
      op: condition.op,
      value: isRegexOp(condition.op) ? condition.value : '',
    })),
    response: {
      status: values.status,
      code: values.code,
      message: values.message.trim(),
    },
  }
}

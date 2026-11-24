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

import { parseHttpStatusCodeRules } from '@/lib/http-status-code-rules'

import { splitKeywords } from './rules'
import {
  DEFAULT_REWRITE_RESPONSE,
  ERROR_RULE_ACTIONS,
  MAX_ERROR_RULE_KEYWORD_LENGTH,
  MAX_ERROR_RULE_KEYWORDS,
  MAX_ERROR_RULE_MESSAGE_LENGTH,
  MAX_ERROR_RULE_NAME_LENGTH,
  type UpstreamErrorRule,
} from './types'

const ERROR_CODE_PATTERN = /^[A-Za-z0-9_.-]{1,64}$/

// Checks what the editor can check locally; the backend validates the saved
// rules again. Response fields only count for rewrite rules, so switching a
// rule to intercept never blocks on hidden fields.
export function createErrorRuleFormSchema(t: TFunction) {
  return z
    .object({
      name: z.string().max(
        MAX_ERROR_RULE_NAME_LENGTH,
        t('Use at most {{count}} characters', {
          count: MAX_ERROR_RULE_NAME_LENGTH,
        })
      ),
      enabled: z.boolean(),
      status_codes: z.string(),
      keywords: z.string(),
      action: z.enum(ERROR_RULE_ACTIONS),
      // Kept as text so an emptied number field is reported, not coerced.
      status: z.string(),
      code: z.string(),
      message: z.string(),
      skip_retry: z.boolean(),
    })
    .superRefine((values, ctx) => {
      const codes = parseHttpStatusCodeRules(values.status_codes)
      if (!codes.ok) {
        ctx.addIssue({
          code: 'custom',
          path: ['status_codes'],
          message: t('Invalid status code rules: {{tokens}}', {
            tokens: codes.invalidTokens.join(', '),
          }),
        })
      }
      const keywords = splitKeywords(values.keywords)
      if (!values.status_codes.trim() && keywords.length === 0) {
        ctx.addIssue({
          code: 'custom',
          path: ['keywords'],
          message: t('Enter status codes, keywords or both'),
        })
      }
      if (keywords.length > MAX_ERROR_RULE_KEYWORDS) {
        ctx.addIssue({
          code: 'custom',
          path: ['keywords'],
          message: t('Use at most {{count}} keywords', {
            count: MAX_ERROR_RULE_KEYWORDS,
          }),
        })
      }
      if (keywords.some((k) => k.length > MAX_ERROR_RULE_KEYWORD_LENGTH)) {
        ctx.addIssue({
          code: 'custom',
          path: ['keywords'],
          message: t('Each keyword can use at most {{count}} characters', {
            count: MAX_ERROR_RULE_KEYWORD_LENGTH,
          }),
        })
      }
      if (values.action !== 'rewrite') return
      const status = Number(values.status)
      if (
        !values.status.trim() ||
        !Number.isInteger(status) ||
        status < 400 ||
        status > 599
      ) {
        ctx.addIssue({
          code: 'custom',
          path: ['status'],
          message: t('Enter a status code between 400 and 599'),
        })
      }
      if (!ERROR_CODE_PATTERN.test(values.code.trim())) {
        ctx.addIssue({
          code: 'custom',
          path: ['code'],
          message: t('Use 1-64 letters, digits, dots, dashes or underscores'),
        })
      }
      const message = values.message.trim()
      if (!message) {
        ctx.addIssue({
          code: 'custom',
          path: ['message'],
          message: t('Error message is required'),
        })
      } else if (message.length > MAX_ERROR_RULE_MESSAGE_LENGTH) {
        ctx.addIssue({
          code: 'custom',
          path: ['message'],
          message: t('Use at most {{count}} characters', {
            count: MAX_ERROR_RULE_MESSAGE_LENGTH,
          }),
        })
      }
    })
}

export type ErrorRuleFormValues = z.infer<
  ReturnType<typeof createErrorRuleFormSchema>
>

export function errorRuleToFormValues(
  rule: UpstreamErrorRule
): ErrorRuleFormValues {
  const response = rule.response ?? DEFAULT_REWRITE_RESPONSE
  return {
    name: rule.name,
    enabled: rule.enabled,
    status_codes: rule.status_codes,
    keywords: rule.keywords.join('\n'),
    action: rule.action,
    status: String(response.status),
    code: response.code,
    message: response.message,
    skip_retry: rule.skip_retry === true,
  }
}

export function formValuesToErrorRule(
  values: ErrorRuleFormValues
): UpstreamErrorRule {
  const rule: UpstreamErrorRule = {
    name: values.name.trim(),
    enabled: values.enabled,
    status_codes: parseHttpStatusCodeRules(values.status_codes).normalized,
    keywords: splitKeywords(values.keywords),
    action: values.action,
  }
  if (values.action === 'rewrite') {
    rule.response = {
      status: Number(values.status),
      code: values.code.trim(),
      message: values.message.trim(),
    }
  }
  if (values.skip_retry) rule.skip_retry = true
  return rule
}

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

// A channel user rule keeps some of one user's requests off a channel. A rule
// with only user_id blocks every request from the user, which is how the
// channel stores it: in blacklist_user_ids, the list the prompt audit also
// edits. Rules narrowed by models, non_stream_only or rpm go to user_rules.
export type ChannelUserRule = {
  user_id: number
  models?: string[]
  non_stream_only?: boolean
  rpm?: number
}

export const MAX_CHANNEL_USER_RULES = 1000
export const MAX_CHANNEL_USER_RULE_RPM = 1000000

const RULE_FIELDS = new Set(['user_id', 'models', 'non_stream_only', 'rpm'])

export type ChannelUserRulesParseResult =
  | { rules: ChannelUserRule[]; error?: undefined }
  | { rules?: undefined; error: string }

const POSITIVE_USER_ID = 'Enter a positive user ID'
const RPM_RANGE = 'Enter a whole number from 0 to 1000000'

// One rule as the visual editor edits it. rpm 0 means every matching request
// skips the channel. Messages are English i18n keys.
export const channelUserRuleFormSchema = z.object({
  user_id: z.number().int(POSITIVE_USER_ID).min(1, POSITIVE_USER_ID),
  models: z.array(z.string()),
  non_stream_only: z.boolean(),
  rpm: z
    .number()
    .int(RPM_RANGE)
    .min(0, RPM_RANGE)
    .max(MAX_CHANNEL_USER_RULE_RPM, RPM_RANGE),
})

export type ChannelUserRuleFormValues = z.infer<
  typeof channelUserRuleFormSchema
>

/**
 * Writes rules as the user rules field text: a JSON list with one rule per
 * line, or an empty string when there are none.
 */
export function stringifyChannelUserRules(rules: readonly unknown[]): string {
  if (rules.length === 0) return ''
  return `[\n${rules.map((rule) => `  ${JSON.stringify(rule)}`).join(',\n')}\n]`
}

/**
 * Shows the channel's blacklist and user rules as one list, with the
 * blacklisted users first.
 */
export function formatChannelUserRules(
  blacklistUserIds: unknown,
  userRules: unknown
): string {
  return stringifyChannelUserRules([
    ...(Array.isArray(blacklistUserIds)
      ? blacklistUserIds.map((userId) => ({ user_id: userId }))
      : []),
    ...(Array.isArray(userRules) ? userRules : []),
  ])
}

/**
 * Parses the user rules field. An empty field means no rules. Errors are
 * English i18n keys.
 */
export function parseChannelUserRules(
  value: string | undefined
): ChannelUserRulesParseResult {
  if (!value?.trim()) return { rules: [] }
  let parsed: unknown
  try {
    parsed = JSON.parse(value)
  } catch {
    return { error: 'User rules must be a JSON array of rule objects' }
  }
  if (!Array.isArray(parsed)) {
    return { error: 'User rules must be a JSON array of rule objects' }
  }
  if (parsed.length > MAX_CHANNEL_USER_RULES) {
    return { error: 'User rules cannot contain more than 1000 rules' }
  }
  const rules: ChannelUserRule[] = []
  for (const item of parsed) {
    if (!item || typeof item !== 'object' || Array.isArray(item)) {
      return { error: 'User rules must be a JSON array of rule objects' }
    }
    const rule = item as Record<string, unknown>
    if (Object.keys(rule).some((key) => !RULE_FIELDS.has(key))) {
      return {
        error:
          'User rules only support user_id, models, non_stream_only and rpm',
      }
    }
    const userId = rule.user_id
    if (
      typeof userId !== 'number' ||
      !Number.isSafeInteger(userId) ||
      userId <= 0
    ) {
      return { error: 'Each user rule needs a positive user_id' }
    }
    const models = rule.models
    if (
      models !== undefined &&
      (!Array.isArray(models) ||
        models.some((model) => typeof model !== 'string' || !model.trim()))
    ) {
      return { error: 'models must be a list of non-empty model names' }
    }
    const nonStreamOnly = rule.non_stream_only
    if (nonStreamOnly !== undefined && typeof nonStreamOnly !== 'boolean') {
      return { error: 'non_stream_only must be true or false' }
    }
    const rpm = rule.rpm
    if (
      rpm !== undefined &&
      (typeof rpm !== 'number' ||
        !Number.isInteger(rpm) ||
        rpm < 0 ||
        rpm > MAX_CHANNEL_USER_RULE_RPM)
    ) {
      return { error: 'rpm must be a whole number from 0 to 1000000' }
    }
    rules.push({
      user_id: userId,
      ...(Array.isArray(models) && models.length > 0
        ? { models: models.map((model) => String(model).trim()) }
        : {}),
      ...(nonStreamOnly === true ? { non_stream_only: true } : {}),
      ...(typeof rpm === 'number' && rpm > 0 ? { rpm } : {}),
    })
  }
  return { rules }
}

/**
 * Splits parsed rules into the two settings fields: users blocked for every
 * request go to blacklist_user_ids, narrowed rules go to user_rules.
 */
export function splitChannelUserRules(rules: ChannelUserRule[]): {
  blacklistUserIds: number[]
  userRules: ChannelUserRule[]
} {
  const blacklistUserIds: number[] = []
  const userRules: ChannelUserRule[] = []
  for (const rule of rules) {
    const narrowed =
      (rule.models?.length ?? 0) > 0 ||
      rule.non_stream_only === true ||
      (rule.rpm ?? 0) > 0
    if (narrowed) {
      userRules.push(rule)
    } else if (!blacklistUserIds.includes(rule.user_id)) {
      blacklistUserIds.push(rule.user_id)
    }
  }
  return { blacklistUserIds, userRules }
}

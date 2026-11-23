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
import { assert, describe, expect, test } from 'vitest'

import { channelSchema } from '../../types'
import {
  channelFormSchema,
  transformChannelToFormDefaults,
  transformFormDataToUpdatePayload,
} from '../channel-form'
import { parseChannelUserRules } from '../channel-user-rules'

function channelWithSettings(settings: Record<string, unknown>) {
  return channelSchema.parse({
    id: 117,
    name: 'Pool channel',
    key: '',
    type: 14,
    models: 'claude-haiku-5-5,claude-opus-5-5',
    group: 'ccmax',
    status: 1,
    created_time: 0,
    test_time: 0,
    response_time: 0,
    balance_updated_time: 0,
    settings: JSON.stringify(settings),
  })
}

function savedSettings(userRules: string) {
  const channel = channelWithSettings({
    blacklist_user_ids: [669],
    tool_loss_policy: 'safe',
  })
  const payload = transformFormDataToUpdatePayload(
    { ...transformChannelToFormDefaults(channel), user_rules: userRules },
    channel.id
  )
  assert(typeof payload.settings === 'string')
  return JSON.parse(payload.settings)
}

describe('channel user rules', () => {
  test('loads the blacklist and narrowed rules as one list with one rule per line', () => {
    const values = transformChannelToFormDefaults(
      channelWithSettings({
        blacklist_user_ids: [761],
        user_rules: [
          {
            user_id: 822,
            models: ['claude-haiku-5-5'],
            non_stream_only: true,
          },
          { user_id: 822, rpm: 30 },
        ],
      })
    )

    expect(values.user_rules).toBe(
      [
        '[',
        '  {"user_id":761},',
        '  {"user_id":822,"models":["claude-haiku-5-5"],"non_stream_only":true},',
        '  {"user_id":822,"rpm":30}',
        ']',
      ].join('\n')
    )
  })

  test('saves user-only rules to the blacklist and narrowed rules to user_rules', () => {
    const settings = savedSettings(
      JSON.stringify([
        { user_id: 761 },
        { user_id: 822, non_stream_only: true, models: ['claude-haiku-5-5'] },
        { user_id: 822, rpm: 30 },
        { user_id: 761, rpm: 0 },
      ])
    )

    expect(settings.blacklist_user_ids).toEqual([761])
    expect(settings.user_rules).toEqual([
      { user_id: 822, models: ['claude-haiku-5-5'], non_stream_only: true },
      { user_id: 822, rpm: 30 },
    ])
    expect(settings.tool_loss_policy).toBe('safe')
  })

  test('removes both lists when the rules are cleared', () => {
    const settings = savedSettings('')

    expect(settings).not.toHaveProperty('blacklist_user_ids')
    expect(settings).not.toHaveProperty('user_rules')
    expect(settings.tool_loss_policy).toBe('safe')
  })

  test.each([
    ['{"user_id": 822}', 'User rules must be a JSON array of rule objects'],
    ['[{"user_id": 822', 'User rules must be a JSON array of rule objects'],
    [
      '[{"user_id": 822, "model": "claude-haiku-5-5"}]',
      'User rules only support user_id, models, non_stream_only and rpm',
    ],
    ['[{"rpm": 30}]', 'Each user rule needs a positive user_id'],
    ['[{"user_id": "822"}]', 'Each user rule needs a positive user_id'],
    [
      '[{"user_id": 822, "models": "claude-haiku-5-5"}]',
      'models must be a list of non-empty model names',
    ],
    [
      '[{"user_id": 822, "non_stream_only": "yes"}]',
      'non_stream_only must be true or false',
    ],
    [
      '[{"user_id": 822, "rpm": 1.5}]',
      'rpm must be a whole number from 0 to 1000000',
    ],
  ])('rejects %s', (value, error) => {
    expect(parseChannelUserRules(value).error).toBe(error)
  })

  test('reports an invalid rule on the user rules field', () => {
    const values = {
      ...transformChannelToFormDefaults(channelWithSettings({})),
      key: 'sk-test',
      user_rules: '[{"user_id": 0}]',
    }

    const result = channelFormSchema.safeParse(values)

    assert(!result.success)
    expect(
      result.error.issues.find((issue) => issue.path[0] === 'user_rules')
        ?.message
    ).toBe('Each user rule needs a positive user_id')
  })
})

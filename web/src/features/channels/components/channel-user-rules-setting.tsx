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
import { Code2, Eye } from 'lucide-react'
import { useMemo, useState } from 'react'
import { useFormContext, useWatch } from 'react-hook-form'
import { useTranslation } from 'react-i18next'

import { JsonCodeEditor } from '@/components/json-code-editor'
import { Button } from '@/components/ui/button'
import {
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'

import { parseModels, type ChannelFormValues } from '../lib/channel-form'
import { ChannelUserRulesEditor } from './channel-user-rules-editor'

const CHANNEL_USER_RULES_PLACEHOLDER = `[
  {"user_id": 101},
  {"user_id": 102, "non_stream_only": true, "models": ["claude-haiku-5-5"]},
  {"user_id": 102, "rpm": 30}
]`

/**
 * The channel user rules field, edited as a table of rules or as JSON text.
 */
export function ChannelUserRulesSetting(props: { disabled?: boolean }) {
  const { t } = useTranslation()
  const form = useFormContext<ChannelFormValues>()
  const models = useWatch({ control: form.control, name: 'models' })
  const modelOptions = useMemo(() => parseModels(models ?? ''), [models])
  const [editMode, setEditMode] = useState<'visual' | 'json'>('visual')

  return (
    <FormField
      control={form.control}
      name='user_rules'
      render={({ field, fieldState }) => (
        <FormItem>
          <div className='flex items-center justify-between gap-2'>
            <FormLabel>{t('Channel user rules')}</FormLabel>
            <Button
              type='button'
              variant='outline'
              size='sm'
              onClick={() =>
                setEditMode(editMode === 'visual' ? 'json' : 'visual')
              }
            >
              {editMode === 'visual' ? (
                <>
                  <Code2 data-icon='inline-start' />
                  {t('Switch to JSON')}
                </>
              ) : (
                <>
                  <Eye data-icon='inline-start' />
                  {t('Switch to Visual')}
                </>
              )}
            </Button>
          </div>
          <FormControl>
            {editMode === 'visual' ? (
              <ChannelUserRulesEditor
                value={field.value ?? ''}
                onChange={field.onChange}
                modelOptions={modelOptions}
                disabled={props.disabled}
              />
            ) : (
              <JsonCodeEditor
                value={field.value || ''}
                onChange={field.onChange}
                name={field.name}
                onBlur={field.onBlur}
                textareaRef={field.ref}
                disabled={props.disabled}
                placeholder={CHANNEL_USER_RULES_PLACEHOLDER}
                aria-invalid={fieldState.invalid}
              />
            )}
          </FormControl>
          <FormDescription>
            {editMode === 'visual'
              ? t(
                  'Requests that match a rule skip this channel and try the next priority. Requests over the per-minute allowance get 429 when no other channel can take them.'
                )
              : t(
                  'Requests that match a rule skip this channel and try the next priority. A rule with only user_id blocks the user. Add models to match only those models, non_stream_only: true to match only non-streaming requests, and rpm to let that many matching requests through each minute; the rest skip this channel and get 429 when no other channel can take them.'
                )}
          </FormDescription>
          <FormMessage />
        </FormItem>
      )}
    />
  )
}

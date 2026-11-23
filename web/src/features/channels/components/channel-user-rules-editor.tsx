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
import { Plus } from 'lucide-react'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { StaticDataTable, StaticRowActions } from '@/components/data-table'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { toIntlLocale } from '@/i18n/languages'
import { formatNumber } from '@/lib/format'

import {
  parseChannelUserRules,
  stringifyChannelUserRules,
} from '../lib/channel-user-rules'
import { ChannelUserRuleDialog } from './channel-user-rule-dialog'

type ChannelUserRulesEditorProps = {
  /** The user rules field text, as the JSON mode shows it. */
  value: string
  onChange: (value: string) => void
  modelOptions: string[]
  disabled?: boolean
  id?: string
  'aria-describedby'?: string
}

/**
 * Edits the channel user rules as a table, one row per rule, and writes every
 * change back to the field text.
 */
export function ChannelUserRulesEditor(props: ChannelUserRulesEditorProps) {
  const { t, i18n } = useTranslation()
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)
  const [dialogOpen, setDialogOpen] = useState(false)
  const [editingIndex, setEditingIndex] = useState<number | null>(null)
  const parsed = useMemo(
    () => parseChannelUserRules(props.value),
    [props.value]
  )

  if (parsed.error !== undefined) {
    return (
      <Alert
        variant='destructive'
        id={props.id}
        aria-describedby={props['aria-describedby']}
      >
        <AlertTitle>{t(parsed.error)}</AlertTitle>
        <AlertDescription>
          {t('Fix these rules in JSON mode to edit them here.')}
        </AlertDescription>
      </Alert>
    )
  }

  const rules = parsed.rules
  return (
    <div
      id={props.id}
      role='group'
      aria-label={t('Channel user rules')}
      aria-describedby={props['aria-describedby']}
      className='space-y-3'
    >
      <StaticDataTable
        data={rules}
        getRowKey={(rule, index) => `${index}-${rule.user_id}`}
        emptyContent={t('No user rules yet.')}
        columns={[
          {
            id: 'user',
            header: t('User ID'),
            cellClassName: 'font-mono',
            cell: (rule) => rule.user_id,
          },
          {
            id: 'models',
            header: t('Models'),
            cell: (rule) =>
              rule.models ? (
                <span className='font-mono text-xs break-all'>
                  {rule.models.join(', ')}
                </span>
              ) : (
                <span className='text-muted-foreground'>{t('All models')}</span>
              ),
          },
          {
            id: 'requests',
            header: t('Request type'),
            cell: (rule) =>
              rule.non_stream_only ? (
                t('Non-streaming only')
              ) : (
                <span className='text-muted-foreground'>
                  {t('All requests')}
                </span>
              ),
          },
          {
            id: 'limit',
            header: t('Limit'),
            cell: (rule) =>
              rule.rpm
                ? t('{{value}} per minute', {
                    value: formatNumber(rule.rpm, locale),
                  })
                : t('Always block'),
          },
          {
            id: 'actions',
            header: t('Actions'),
            className: 'text-right',
            cellClassName: 'text-right',
            cell: (_, index) => (
              <StaticRowActions
                editLabel={t('Edit')}
                deleteLabel={t('Delete')}
                menuLabel={t('Open menu')}
                editDisabled={props.disabled}
                deleteDisabled={props.disabled}
                onEdit={() => {
                  setEditingIndex(index)
                  setDialogOpen(true)
                }}
                onDelete={() =>
                  props.onChange(
                    stringifyChannelUserRules(
                      rules.filter((_, ruleIndex) => ruleIndex !== index)
                    )
                  )
                }
              />
            ),
          },
        ]}
      />
      <Button
        type='button'
        variant='outline'
        size='sm'
        disabled={props.disabled}
        onClick={() => {
          setEditingIndex(null)
          setDialogOpen(true)
        }}
      >
        <Plus data-icon='inline-start' />
        {t('Add rule')}
      </Button>
      <ChannelUserRuleDialog
        open={dialogOpen}
        onOpenChange={setDialogOpen}
        rule={editingIndex === null ? null : (rules[editingIndex] ?? null)}
        modelOptions={props.modelOptions}
        onSave={(rule) =>
          props.onChange(
            stringifyChannelUserRules(
              editingIndex === null
                ? [...rules, rule]
                : rules.map((current, index) =>
                    index === editingIndex ? rule : current
                  )
            )
          )
        }
      />
    </div>
  )
}

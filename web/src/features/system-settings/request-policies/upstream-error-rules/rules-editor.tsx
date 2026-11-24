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
import { Braces, ListFilter, Plus } from 'lucide-react'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { JsonCodeEditor } from '@/components/json-code-editor'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { Separator } from '@/components/ui/separator'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'

import { ErrorRuleEditorDialog } from './rule-editor-dialog'
import {
  createBlankErrorRule,
  errorRuleDisplayName,
  formatErrorRulesJson,
  parseErrorRulesJson,
} from './rules'
import { ErrorRulesTable } from './rules-table'
import type { UpstreamErrorRule } from './types'

type ErrorRulesEditorProps = {
  // The rules as JSON text; the JSON tab edits it directly.
  value: string
  onChange: (value: string) => void
}

type EditingRule = { index: number | null; rule: UpstreamErrorRule }

export function ErrorRulesEditor(props: ErrorRulesEditorProps) {
  const { t } = useTranslation()
  const [mode, setMode] = useState<'visual' | 'json'>('visual')
  const [editing, setEditing] = useState<EditingRule | null>(null)
  const [deleting, setDeleting] = useState<number | null>(null)
  const parsed = useMemo(() => parseErrorRulesJson(props.value), [props.value])
  const rules = parsed.ok ? parsed.rules : []

  const updateRules = (next: UpstreamErrorRule[]) =>
    props.onChange(formatErrorRulesJson(next))

  const deletingRule = deleting === null ? undefined : rules[deleting]

  return (
    <div className='bg-card min-w-0 overflow-hidden rounded-xl border'>
      <div className='flex flex-wrap items-center gap-2 px-4 pt-4 sm:px-5'>
        <h5 className='text-base font-semibold'>{t('Error rules')}</h5>
        <Badge variant='secondary' className='tabular-nums'>
          {rules.length}
        </Badge>
      </div>
      <Tabs
        value={mode}
        onValueChange={(value) => {
          if (value === 'json') {
            setMode('json')
            return
          }
          if (!parsed.ok) {
            toast.error(t(parsed.error))
            return
          }
          setMode('visual')
        }}
        className='gap-0'
      >
        <div className='flex flex-wrap items-center justify-between gap-3 px-4 py-4 sm:px-5'>
          <TabsList aria-label={t('Rule editor mode')}>
            <TabsTrigger value='visual' className='px-3'>
              <ListFilter aria-hidden='true' />
              {t('Visual')}
            </TabsTrigger>
            <TabsTrigger value='json' className='px-3'>
              <Braces aria-hidden='true' />
              JSON
            </TabsTrigger>
          </TabsList>
          <Button
            type='button'
            disabled={mode === 'json' || !parsed.ok}
            onClick={() =>
              setEditing({ index: null, rule: createBlankErrorRule() })
            }
          >
            <Plus aria-hidden='true' data-icon='inline-start' />
            {t('Add Rule')}
          </Button>
        </div>
        <Separator />
        <TabsContent value='visual' className='min-w-0'>
          <ErrorRulesTable
            rules={rules}
            onToggle={(index, enabled) =>
              updateRules(
                rules.map((rule, i) =>
                  i === index ? { ...rule, enabled } : rule
                )
              )
            }
            onEdit={(index) => setEditing({ index, rule: rules[index] })}
            onMove={(index, offset) => {
              const next = [...rules]
              const [moved] = next.splice(index, 1)
              next.splice(index + offset, 0, moved)
              updateRules(next)
            }}
            onDelete={setDeleting}
          />
        </TabsContent>
        <TabsContent value='json' className='min-w-0'>
          <div className='grid gap-2 p-4 sm:p-5'>
            <Label htmlFor='upstream-error-rules-json'>{t('Rules JSON')}</Label>
            <p className='text-muted-foreground text-xs'>
              {t('Switch to visual mode to add rules.')}
            </p>
            <JsonCodeEditor
              id='upstream-error-rules-json'
              value={props.value}
              onChange={props.onChange}
              aria-invalid={!parsed.ok}
              heightClassName='h-[360px] min-h-[360px] max-h-[360px]'
            />
            {parsed.ok ? null : (
              <p className='text-destructive text-xs'>{t(parsed.error)}</p>
            )}
          </div>
        </TabsContent>
      </Tabs>

      <ErrorRuleEditorDialog
        open={editing !== null}
        onOpenChange={(open) => {
          if (!open) setEditing(null)
        }}
        rule={editing?.rule ?? null}
        isNew={editing?.index === null}
        onSave={(rule) => {
          const index = editing?.index ?? null
          updateRules(
            index === null
              ? [...rules, rule]
              : rules.map((item, i) => (i === index ? rule : item))
          )
          setEditing(null)
        }}
      />
      <ConfirmDialog
        open={deletingRule !== undefined}
        onOpenChange={(open) => {
          if (!open) setDeleting(null)
        }}
        title={t('Delete Rule')}
        desc={t('Delete {{name}}? The change takes effect after saving.', {
          name:
            deletingRule && deleting !== null
              ? errorRuleDisplayName(t, deletingRule, deleting)
              : '',
        })}
        destructive
        handleConfirm={() => {
          if (deleting !== null) {
            updateRules(rules.filter((_, i) => i !== deleting))
          }
          setDeleting(null)
        }}
      />
    </div>
  )
}

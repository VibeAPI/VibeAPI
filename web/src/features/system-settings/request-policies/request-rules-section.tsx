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
import { useQuery } from '@tanstack/react-query'
import { Braces, FlaskConical, ListFilter, Plus } from 'lucide-react'
import { useEffect, useMemo, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { JsonCodeEditor } from '@/components/json-code-editor'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Label } from '@/components/ui/label'
import { Separator } from '@/components/ui/separator'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { handleServerError } from '@/lib/handle-server-error'

import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { getPolicyConfig } from './api'
import type { RequestRuleSettings } from './defaults'
import { RuleEditorDialog } from './request-rules/rule-editor-dialog'
import {
  compactRulesJson,
  createBlankRule,
  createRuleId,
  formatRulesJson,
  parseRulesJson,
  ruleDisplayName,
  ruleTemplates,
} from './request-rules/rules'
import { RulesTable } from './request-rules/rules-table'
import { TestRequestDialog } from './request-rules/test-request-dialog'
import type { RequestRule } from './request-rules/types'
import { useSavePolicy } from './use-save-policy'

const OPTION_KEY = 'request_rule_setting.rules'

type RequestRulesSectionProps = {
  defaultValues: RequestRuleSettings
}

type EditingRule = { index: number | null; rule: RequestRule }

// The JSON tab edits the draft text directly, so keep it readable.
function formatStoredRules(stored: string): string {
  const parsed = parseRulesJson(stored)
  return parsed.ok ? formatRulesJson(parsed.rules) : stored
}

export function RequestRulesSection(props: RequestRulesSectionProps) {
  const { t } = useTranslation()
  const updateOption = useSavePolicy()
  const policy = useQuery({
    queryKey: ['request-policy'],
    queryFn: getPolicyConfig,
    staleTime: Infinity,
  })
  const presets = useMemo(
    () =>
      new Map(
        (policy.data?.request_rule_presets ?? []).map((rule) => [rule.id, rule])
      ),
    [policy.data]
  )
  const stored = props.defaultValues[OPTION_KEY]
  const [draft, setDraft] = useState(() => formatStoredRules(stored))
  const baselineRef = useRef(compactRulesJson(stored))
  const [mode, setMode] = useState<'visual' | 'json'>('visual')
  const [editing, setEditing] = useState<EditingRule | null>(null)
  const [deleting, setDeleting] = useState<number | null>(null)
  const [testOpen, setTestOpen] = useState(false)
  const parsed = useMemo(() => parseRulesJson(draft), [draft])
  const rules = parsed.ok ? parsed.rules : []
  const templates = useMemo(() => ruleTemplates(t), [t])

  // Refreshed settings replace the draft only while it has no unsaved edits.
  useEffect(() => {
    const incoming = compactRulesJson(stored)
    const previous = baselineRef.current
    if (incoming === previous) return
    baselineRef.current = incoming
    setDraft((current) =>
      compactRulesJson(current) === previous
        ? formatStoredRules(stored)
        : current
    )
  }, [stored])

  const updateRules = (next: RequestRule[]) => setDraft(formatRulesJson(next))

  const save = async () => {
    if (!parsed.ok) {
      toast.error(t(parsed.error))
      return
    }
    const next = JSON.stringify(parsed.rules)
    if (next === baselineRef.current) {
      toast.info(t('No changes to save'))
      return
    }
    try {
      await updateOption.mutateAsync({ [OPTION_KEY]: next })
      baselineRef.current = next
    } catch (error) {
      handleServerError(error)
    }
  }

  const deletingRule = deleting === null ? undefined : rules[deleting]

  return (
    <SettingsSection title={t('Invalid requests')}>
      <div className='text-muted-foreground space-y-1 text-sm'>
        <p>{t('Source: global settings. Changes take effect after saving.')}</p>
        <p>
          {t(
            'Channels with Block invalid requests turned on check Claude, OpenAI Chat, Responses and Gemini requests against these rules in order before relaying. The first matching rule answers the request with its own error: the request is not sent upstream, retried on another channel or counted against channel health, and its log names the rule.'
          )}
        </p>
      </div>
      <SettingsPageFormActions
        onSave={() => void save()}
        isSaving={updateOption.isPending}
      />
      <div className='bg-card min-w-0 overflow-hidden rounded-xl border'>
        <div className='flex flex-wrap items-center gap-2 px-4 pt-4 sm:px-5'>
          <h3 className='text-base font-semibold'>{t('Request rules')}</h3>
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
            <div className='flex flex-wrap items-center gap-2'>
              <Button
                variant='outline'
                disabled={!parsed.ok}
                onClick={() => setTestOpen(true)}
              >
                <FlaskConical aria-hidden='true' data-icon='inline-start' />
                {t('Test request')}
              </Button>
              <DropdownMenu>
                <DropdownMenuTrigger
                  render={<Button disabled={mode === 'json' || !parsed.ok} />}
                >
                  <Plus aria-hidden='true' data-icon='inline-start' />
                  {t('Add Rule')}
                </DropdownMenuTrigger>
                <DropdownMenuContent align='end' className='w-80'>
                  <DropdownMenuGroup>
                    <DropdownMenuItem
                      onClick={() =>
                        setEditing({
                          index: null,
                          rule: createBlankRule(rules),
                        })
                      }
                    >
                      {t('Blank Rule')}
                    </DropdownMenuItem>
                  </DropdownMenuGroup>
                  <DropdownMenuSeparator />
                  <DropdownMenuGroup>
                    <DropdownMenuLabel>
                      {t('From recent upstream 400 errors')}
                    </DropdownMenuLabel>
                    {templates.map((template) => (
                      <DropdownMenuItem
                        key={template.key}
                        onClick={() =>
                          setEditing({
                            index: null,
                            rule: { ...template.rule, id: createRuleId(rules) },
                          })
                        }
                      >
                        {template.rule.name}
                      </DropdownMenuItem>
                    ))}
                  </DropdownMenuGroup>
                </DropdownMenuContent>
              </DropdownMenu>
            </div>
          </div>
          <Separator />
          <TabsContent value='visual' className='min-w-0'>
            <RulesTable
              rules={rules}
              presets={presets}
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
              onReset={(index) => {
                const preset = presets.get(rules[index].id)
                if (!preset) return
                updateRules(
                  rules.map((rule, i) => (i === index ? { ...preset } : rule))
                )
              }}
              onDelete={setDeleting}
            />
          </TabsContent>
          <TabsContent value='json' className='min-w-0'>
            <div className='grid gap-2 p-4 sm:p-5'>
              <Label htmlFor='request-rules-json'>{t('Rules JSON')}</Label>
              <p className='text-muted-foreground text-xs'>
                {t('Switch to visual mode to add rules or apply templates.')}
              </p>
              <JsonCodeEditor
                id='request-rules-json'
                value={draft}
                onChange={setDraft}
                aria-invalid={!parsed.ok}
                heightClassName='h-[420px] min-h-[420px] max-h-[420px]'
              />
              {parsed.ok ? null : (
                <p className='text-destructive text-xs'>{t(parsed.error)}</p>
              )}
            </div>
          </TabsContent>
        </Tabs>
      </div>

      <RuleEditorDialog
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
      <TestRequestDialog
        open={testOpen}
        onOpenChange={setTestOpen}
        rules={rules}
        presets={presets}
      />
      <ConfirmDialog
        open={deletingRule !== undefined}
        onOpenChange={(open) => {
          if (!open) setDeleting(null)
        }}
        title={t('Delete Rule')}
        desc={t('Delete {{name}}? The change takes effect after saving.', {
          name: deletingRule ? ruleDisplayName(t, deletingRule, presets) : '',
        })}
        destructive
        handleConfirm={() => {
          if (deleting !== null) {
            updateRules(rules.filter((_, i) => i !== deleting))
          }
          setDeleting(null)
        }}
      />
    </SettingsSection>
  )
}

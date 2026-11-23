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
  ArrowDown,
  ArrowUp,
  Edit,
  ListFilter,
  RotateCcw,
  Trash2,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'

import {
  DataTableRowActionMenu,
  StaticDataTable,
} from '@/components/data-table'
import { EmptyState } from '@/components/empty-state'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuSeparator,
} from '@/components/ui/dropdown-menu'
import { Switch } from '@/components/ui/switch'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'

import { presetText, ruleDisplayName } from './rules'
import {
  RULE_ENDPOINTS,
  type RequestRule,
  type RequestRuleCondition,
} from './types'

type RulesTableProps = {
  rules: RequestRule[]
  presets: Map<string, RequestRule>
  onToggle: (index: number, enabled: boolean) => void
  onEdit: (index: number) => void
  onMove: (index: number, offset: -1 | 1) => void
  onReset: (index: number) => void
  onDelete: (index: number) => void
}

function describeCondition(t: TFunction, condition: RequestRuleCondition) {
  let target = condition.path
  if (condition.source === 'body') {
    target = t('Whole request body')
  }
  if (condition.source === 'header') {
    target = t('Header {{name}}', { name: condition.path })
  }
  switch (condition.op) {
    case 'exists':
      return { target, op: t('exists'), value: '' }
    case 'not_exists':
      return { target, op: t('does not exist'), value: '' }
    case 'not_regex':
      return { target, op: t('does not match'), value: condition.value }
    default:
      return { target, op: t('matches'), value: condition.value }
  }
}

export function RulesTable(props: RulesTableProps) {
  const { t } = useTranslation()

  if (props.rules.length === 0) {
    return (
      <EmptyState
        icon={ListFilter}
        title={t('No rules yet')}
        description={t('Add a blank rule or start from a template.')}
        className='min-h-56'
      />
    )
  }

  return (
    <StaticDataTable
      className='focus-visible:outline-ring overflow-x-auto rounded-none border-0 focus-visible:-outline-offset-2'
      containerProps={{
        role: 'region',
        'aria-label': t('Request rules table'),
        tabIndex: 0,
      }}
      tableProps={{ withContainer: false }}
      tableClassName='min-w-[1040px] table-fixed [&_th]:px-4 [&_th]:text-muted-foreground [&_td]:px-4 [&_td]:py-4 [&_td]:align-top [&_code]:font-mono!'
      headerRowClassName='bg-muted/35 hover:bg-muted/35'
      data={props.rules}
      getRowKey={(rule) => rule.id}
      columns={[
        {
          id: 'enabled',
          header: t('Enabled'),
          className: 'w-24',
          cell: (rule, index) => (
            <div className='flex items-center gap-3'>
              <span className='text-muted-foreground w-5 text-right text-xs tabular-nums'>
                {index + 1}
              </span>
              <Switch
                checked={rule.enabled}
                aria-label={t('Enable {{name}}', {
                  name: ruleDisplayName(t, rule, props.presets),
                })}
                onCheckedChange={(checked) => props.onToggle(index, checked)}
              />
            </div>
          ),
        },
        {
          id: 'rule',
          header: t('Rule'),
          className: 'w-[24%]',
          cell: (rule) => {
            const preset = props.presets.get(rule.id)
            return (
              <div className='flex min-w-0 flex-col gap-1.5'>
                <div className='flex flex-wrap items-center gap-1.5'>
                  <span className='font-medium break-words'>
                    {presetText(t, rule.name, preset?.name)}
                  </span>
                  {rule.preset ? (
                    <Badge variant='secondary'>{t('Preset')}</Badge>
                  ) : null}
                  {rule.only_pass_through ? (
                    <Badge variant='warning'>{t('Pass-through only')}</Badge>
                  ) : null}
                </div>
                {rule.description ? (
                  <p className='text-muted-foreground text-xs break-words'>
                    {presetText(t, rule.description, preset?.description)}
                  </p>
                ) : null}
              </div>
            )
          },
        },
        {
          id: 'scope',
          header: t('Scope'),
          className: 'w-[16%]',
          cell: (rule) => (
            <div className='flex min-w-0 flex-col items-start gap-1.5'>
              <div className='flex flex-wrap gap-1'>
                {RULE_ENDPOINTS.filter((e) =>
                  rule.endpoints.includes(e.value)
                ).map((endpoint) => (
                  <Badge key={endpoint.value} variant='outline'>
                    {endpoint.label}
                  </Badge>
                ))}
              </div>
              {rule.model_regex ? (
                <code className='text-xs break-all'>{rule.model_regex}</code>
              ) : (
                <span className='text-muted-foreground text-xs'>
                  {t('All models')}
                </span>
              )}
            </div>
          ),
        },
        {
          id: 'conditions',
          header: t('Conditions'),
          className: 'w-[26%]',
          cell: (rule) => (
            <div className='flex min-w-0 flex-col gap-1 text-xs'>
              {rule.conditions.length > 1 ? (
                <span className='text-muted-foreground'>
                  {rule.match === 'any'
                    ? t('Any condition matches')
                    : t('All conditions match')}
                </span>
              ) : null}
              {rule.conditions.map((condition) => {
                const described = describeCondition(t, condition)
                return (
                  <span
                    key={`${condition.source}:${condition.path}:${condition.op}:${condition.value}`}
                    className='break-all'
                  >
                    <code>{described.target}</code>{' '}
                    <span className='text-muted-foreground'>
                      {described.op}
                    </span>{' '}
                    {described.value ? <code>{described.value}</code> : null}
                  </span>
                )
              })}
            </div>
          ),
        },
        {
          id: 'response',
          header: t('Error returned'),
          cell: (rule) => (
            <div className='flex min-w-0 flex-col gap-1 text-xs'>
              <span className='tabular-nums'>
                <span className='font-medium'>{rule.response.status}</span>
                {' · '}
                <code>{rule.response.code}</code>
              </span>
              <span className='text-muted-foreground line-clamp-3 break-words'>
                {rule.response.message}
              </span>
            </div>
          ),
        },
        {
          id: 'actions',
          header: t('Actions'),
          className: 'w-24 text-right',
          cell: (rule, index) => (
            <div
              role='group'
              aria-label={t('Actions for {{name}}', {
                name: ruleDisplayName(t, rule, props.presets),
              })}
              className='flex items-center justify-end gap-0.5'
            >
              <Tooltip>
                <TooltipTrigger
                  render={
                    <Button
                      variant='ghost'
                      size='icon'
                      aria-label={t('Edit Rule')}
                      onClick={() => props.onEdit(index)}
                    >
                      <Edit aria-hidden='true' />
                    </Button>
                  }
                />
                <TooltipContent>{t('Edit Rule')}</TooltipContent>
              </Tooltip>
              <DataTableRowActionMenu ariaLabel={t('More actions')}>
                <DropdownMenuGroup>
                  <DropdownMenuItem
                    disabled={index === 0}
                    onClick={() => props.onMove(index, -1)}
                  >
                    <ArrowUp aria-hidden='true' />
                    {t('Move up')}
                  </DropdownMenuItem>
                  <DropdownMenuItem
                    disabled={index === props.rules.length - 1}
                    onClick={() => props.onMove(index, 1)}
                  >
                    <ArrowDown aria-hidden='true' />
                    {t('Move down')}
                  </DropdownMenuItem>
                  <DropdownMenuSeparator />
                  {props.presets.has(rule.id) ? (
                    <DropdownMenuItem onClick={() => props.onReset(index)}>
                      <RotateCcw aria-hidden='true' />
                      {t('Reset to default')}
                    </DropdownMenuItem>
                  ) : (
                    <DropdownMenuItem
                      variant='destructive'
                      onClick={() => props.onDelete(index)}
                    >
                      <Trash2 aria-hidden='true' />
                      {t('Delete Rule')}
                    </DropdownMenuItem>
                  )}
                </DropdownMenuGroup>
              </DataTableRowActionMenu>
            </div>
          ),
        },
      ]}
    />
  )
}

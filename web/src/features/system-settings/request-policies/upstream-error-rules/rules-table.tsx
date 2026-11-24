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
import { ArrowDown, ArrowUp, Edit, ShieldAlert, Trash2 } from 'lucide-react'
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

import { errorRuleDisplayName, errorRuleResponse } from './rules'
import type { UpstreamErrorRule } from './types'

// The table lists a few keywords and counts the rest; the editor shows all.
const VISIBLE_KEYWORDS = 3

type ErrorRulesTableProps = {
  rules: UpstreamErrorRule[]
  onToggle: (index: number, enabled: boolean) => void
  onEdit: (index: number) => void
  onMove: (index: number, offset: -1 | 1) => void
  onDelete: (index: number) => void
}

export function ErrorRulesTable(props: ErrorRulesTableProps) {
  const { t } = useTranslation()

  if (props.rules.length === 0) {
    return (
      <EmptyState
        icon={ShieldAlert}
        title={t('No rules yet')}
        description={t(
          'Upstream errors reach callers unchanged. Add a rule to intercept or rewrite them.'
        )}
        className='min-h-48'
      />
    )
  }

  return (
    <StaticDataTable
      className='focus-visible:outline-ring overflow-x-auto rounded-none border-0 focus-visible:-outline-offset-2'
      containerProps={{
        role: 'region',
        'aria-label': t('Error rules table'),
        tabIndex: 0,
      }}
      tableProps={{ withContainer: false }}
      tableClassName='min-w-[880px] table-fixed [&_th]:px-4 [&_th]:text-muted-foreground [&_td]:px-4 [&_td]:py-4 [&_td]:align-top [&_code]:font-mono!'
      headerRowClassName='bg-muted/35 hover:bg-muted/35'
      data={props.rules}
      getRowKey={(_rule, index) => index}
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
                  name: errorRuleDisplayName(t, rule, index),
                })}
                onCheckedChange={(checked) => props.onToggle(index, checked)}
              />
            </div>
          ),
        },
        {
          id: 'match',
          header: t('Upstream error to match'),
          className: 'w-[38%]',
          cell: (rule, index) => {
            // Repeated keywords match the same errors, so list each once.
            const keywords = [...new Set(rule.keywords)]
            const hidden = keywords.length - VISIBLE_KEYWORDS
            return (
              <div className='flex min-w-0 flex-col gap-1.5 text-xs whitespace-normal'>
                <span className='text-sm font-medium break-words'>
                  {errorRuleDisplayName(t, rule, index)}
                </span>
                {rule.status_codes ? (
                  <span className='break-all'>
                    <span className='text-muted-foreground'>
                      {t('Status codes')}
                    </span>{' '}
                    <code>{rule.status_codes}</code>
                  </span>
                ) : null}
                {keywords.length > 0 ? (
                  <span className='break-all'>
                    <span className='text-muted-foreground'>
                      {t('Contains')}
                    </span>{' '}
                    {keywords.slice(0, VISIBLE_KEYWORDS).map((keyword) => (
                      <code key={keyword} className='mr-1.5'>
                        {keyword}
                      </code>
                    ))}
                    {hidden > 0 ? (
                      <span className='text-muted-foreground'>
                        {t('+{{count}} more', { count: hidden })}
                      </span>
                    ) : null}
                  </span>
                ) : null}
              </div>
            )
          },
        },
        {
          id: 'action',
          header: t('Handling'),
          className: 'w-28',
          cell: (rule) => (
            <div className='flex flex-col items-start gap-1.5 text-xs whitespace-normal'>
              {rule.action === 'rewrite' ? (
                <Badge variant='warning'>{t('Rewrite')}</Badge>
              ) : (
                <Badge variant='secondary'>{t('Intercept')}</Badge>
              )}
              {rule.skip_retry ? (
                <span className='text-muted-foreground'>{t('No retry')}</span>
              ) : null}
            </div>
          ),
        },
        {
          id: 'response',
          header: t('Error returned'),
          cell: (rule) => {
            const response = errorRuleResponse(rule)
            return (
              <div className='flex min-w-0 flex-col gap-1 text-xs whitespace-normal'>
                <span className='tabular-nums'>
                  <span className='font-medium'>{response.status}</span>
                  {' · '}
                  <code>{response.code}</code>
                </span>
                <span className='text-muted-foreground line-clamp-3 break-words'>
                  {response.message}
                </span>
              </div>
            )
          },
        },
        {
          id: 'actions',
          header: t('Actions'),
          className: 'w-24 text-right',
          cell: (rule, index) => (
            <div
              role='group'
              aria-label={t('Actions for {{name}}', {
                name: errorRuleDisplayName(t, rule, index),
              })}
              className='flex items-center justify-end gap-0.5'
            >
              <Tooltip>
                <TooltipTrigger
                  render={
                    <Button
                      type='button'
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
                  <DropdownMenuItem
                    variant='destructive'
                    onClick={() => props.onDelete(index)}
                  >
                    <Trash2 aria-hidden='true' />
                    {t('Delete Rule')}
                  </DropdownMenuItem>
                </DropdownMenuGroup>
              </DataTableRowActionMenu>
            </div>
          ),
        },
      ]}
    />
  )
}

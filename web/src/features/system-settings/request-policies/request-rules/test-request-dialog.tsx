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
import { useMutation } from '@tanstack/react-query'
import type { TFunction } from 'i18next'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Dialog } from '@/components/dialog'
import { JsonCodeEditor } from '@/components/json-code-editor'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import { handleServerError } from '@/lib/handle-server-error'

import { testRequestRules } from '../api'
import { ruleDisplayName } from './rules'
import {
  RULE_ENDPOINTS,
  type RequestRule,
  type RuleEndpoint,
  type RuleTraceResult,
} from './types'

type TestRequestDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  rules: RequestRule[]
  presets: Map<string, RequestRule>
}

const SAMPLES: Record<RuleEndpoint, { model: string; body: string }> = {
  claude_messages: {
    model: 'claude-sonnet-4-6',
    body: JSON.stringify(
      {
        model: 'claude-sonnet-4-6',
        max_tokens: 1024,
        messages: [
          { role: 'user', content: 'Translate this sentence into English.' },
          { role: 'assistant', content: 'Translation:' },
        ],
      },
      null,
      2
    ),
  },
  openai_chat: {
    model: 'gpt-6-astra',
    body: JSON.stringify(
      {
        model: 'gpt-6-astra',
        reasoning_effort: 'none',
        messages: [{ role: 'user', content: 'hi' }],
      },
      null,
      2
    ),
  },
  openai_responses: {
    model: 'gpt-6.1-sol',
    body: JSON.stringify(
      { model: 'gpt-6.1-sol', reasoning: { effort: 'none' }, input: 'hi' },
      null,
      2
    ),
  },
  gemini: {
    model: 'gemini-4-flash',
    body: JSON.stringify(
      { contents: [{ role: 'user', parts: [{ text: 'hi' }] }] },
      null,
      2
    ),
  },
}

function traceLabel(t: TFunction, value: RuleTraceResult): string {
  switch (value) {
    case 'disabled':
      return t('Disabled')
    case 'endpoint':
      return t('Endpoint not selected')
    case 'model':
      return t('Model does not match')
    case 'pass_through':
      return t('Only checked when the body is passed through')
    case 'conditions':
      return t('Conditions not met')
    case 'matched':
      return t('Matched')
    default:
      return t('Not checked after the match')
  }
}

export function TestRequestDialog(props: TestRequestDialogProps) {
  const { t } = useTranslation()
  const [endpoint, setEndpoint] = useState<RuleEndpoint>('claude_messages')
  const [model, setModel] = useState(SAMPLES.claude_messages.model)
  const [passThrough, setPassThrough] = useState(true)
  const [headers, setHeaders] = useState('')
  const [body, setBody] = useState(SAMPLES.claude_messages.body)
  const test = useMutation({
    mutationFn: testRequestRules,
    onError: (error) => handleServerError(error),
    meta: { errorToast: false },
  })
  const result = test.data

  const run = () => {
    const headerMap: Record<string, string> = {}
    for (const line of headers.split('\n')) {
      const separator = line.indexOf(':')
      if (separator > 0) {
        headerMap[line.slice(0, separator).trim()] = line
          .slice(separator + 1)
          .trim()
      }
    }
    test.mutate({
      rules: JSON.stringify(props.rules),
      endpoint,
      model,
      pass_through: passThrough,
      body,
      headers: headerMap,
    })
  }

  return (
    <Dialog
      open={props.open}
      onOpenChange={props.onOpenChange}
      title={t('Test request')}
      description={t(
        'Checks a sample request against the rules on this page, including unsaved changes. Nothing is sent upstream, billed or logged.'
      )}
      contentClassName='sm:max-w-5xl'
      contentHeight='auto'
      footer={
        <>
          <Button
            type='button'
            variant='outline'
            onClick={() => props.onOpenChange(false)}
          >
            {t('Close')}
          </Button>
          <Button type='button' onClick={run} disabled={test.isPending}>
            {test.isPending ? t('Checking...') : t('Run check')}
          </Button>
        </>
      }
    >
      <div className='grid gap-6 lg:grid-cols-2'>
        <div className='grid content-start gap-4'>
          <div className='grid gap-4 sm:grid-cols-2'>
            <div className='grid gap-2'>
              <Label htmlFor='rule-test-endpoint'>{t('Endpoint')}</Label>
              <NativeSelect
                id='rule-test-endpoint'
                value={endpoint}
                onChange={(event) => {
                  const next = event.target.value as RuleEndpoint
                  setEndpoint(next)
                  setModel(SAMPLES[next].model)
                  setBody(SAMPLES[next].body)
                  test.reset()
                }}
              >
                {RULE_ENDPOINTS.map((item) => (
                  <NativeSelectOption key={item.value} value={item.value}>
                    {item.label}
                  </NativeSelectOption>
                ))}
              </NativeSelect>
            </div>
            <div className='grid gap-2'>
              <Label htmlFor='rule-test-model'>{t('Model')}</Label>
              <Input
                id='rule-test-model'
                className='font-mono'
                value={model}
                onChange={(event) => setModel(event.target.value)}
              />
            </div>
          </div>
          <div className='flex items-center gap-3'>
            <Switch
              id='rule-test-pass-through'
              checked={passThrough}
              onCheckedChange={setPassThrough}
            />
            <Label htmlFor='rule-test-pass-through' className='font-normal'>
              {t('Check as a channel that passes the body through')}
            </Label>
          </div>
          <div className='grid gap-2'>
            <Label htmlFor='rule-test-body'>{t('Request body')}</Label>
            <JsonCodeEditor
              id='rule-test-body'
              value={body}
              onChange={setBody}
              heightClassName='h-[260px] min-h-[260px] max-h-[260px]'
            />
          </div>
          <div className='grid gap-2'>
            <Label htmlFor='rule-test-headers'>
              {t('Request headers (optional)')}
            </Label>
            <Textarea
              id='rule-test-headers'
              rows={2}
              className='font-mono text-xs'
              placeholder='anthropic-beta: context-management-2025-06-27'
              value={headers}
              onChange={(event) => setHeaders(event.target.value)}
            />
            <p className='text-muted-foreground text-xs'>
              {t('One header per line, written as Name: value.')}
            </p>
          </div>
        </div>

        <div
          className='bg-muted/20 grid content-start gap-4 rounded-lg border p-4'
          role='status'
          aria-live='polite'
        >
          {result ? null : (
            <p className='text-muted-foreground text-sm'>
              {t('Run the check to see whether this request would be blocked.')}
            </p>
          )}
          {result?.matched && result.rule ? (
            <>
              <div className='flex flex-wrap items-center gap-2'>
                <Badge variant='destructive'>{t('Blocked')}</Badge>
                <span className='font-medium'>
                  {t('Rule {{position}}: {{name}}', {
                    position: result.rule.position,
                    name: ruleDisplayName(t, result.rule, props.presets),
                  })}
                </span>
              </div>
              <dl className='grid grid-cols-[auto_minmax(0,1fr)] gap-x-4 gap-y-1 text-sm'>
                <dt className='text-muted-foreground'>{t('Matched field')}</dt>
                <dd className='font-mono break-all'>{result.field || '—'}</dd>
                <dt className='text-muted-foreground'>{t('Matched value')}</dt>
                <dd className='font-mono break-all'>{result.value || '—'}</dd>
              </dl>
              <div className='grid gap-2'>
                <span className='text-sm font-medium'>
                  {t('The client receives HTTP {{status}}', {
                    status: result.status,
                  })}
                </span>
                <pre className='bg-background overflow-x-auto rounded-lg border p-3 font-mono text-xs'>
                  {JSON.stringify(result.response, null, 2)}
                </pre>
              </div>
            </>
          ) : null}
          {result && !result.matched ? (
            <div className='flex flex-wrap items-center gap-2'>
              <Badge variant='secondary'>{t('Not blocked')}</Badge>
              <span className='text-sm'>
                {t('The request would be relayed to the upstream.')}
              </span>
            </div>
          ) : null}
          {result && result.trace.length > 0 ? (
            <div className='grid gap-2'>
              <span className='text-sm font-medium'>{t('Each rule')}</span>
              <ol className='grid gap-1 text-xs'>
                {result.trace.map((item) => (
                  <li
                    key={item.position}
                    className='grid grid-cols-[2rem_minmax(0,1fr)_auto] gap-2'
                  >
                    <span className='text-muted-foreground tabular-nums'>
                      {item.position}
                    </span>
                    <span className='break-words'>
                      {ruleDisplayName(t, item, props.presets)}
                    </span>
                    <span
                      className={
                        item.result === 'matched'
                          ? 'text-destructive font-medium'
                          : 'text-muted-foreground'
                      }
                    >
                      {traceLabel(t, item.result)}
                    </span>
                  </li>
                ))}
              </ol>
            </div>
          ) : null}
        </div>
      </div>
    </Dialog>
  )
}

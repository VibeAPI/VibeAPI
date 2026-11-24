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
import { useTranslation } from 'react-i18next'

import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'

import type { ErrorRuleResponse } from './types'

type ErrorRuleResponsePreviewProps = {
  response: ErrorRuleResponse
}

// Shows the response body a caller receives, in the Claude format and in the
// OpenAI format that Chat, Responses and Gemini callers get.
export function ErrorRuleResponsePreview(props: ErrorRuleResponsePreviewProps) {
  const { t } = useTranslation()
  const message = `${props.response.message} (request id: …)`
  const status = Number.isFinite(props.response.status)
    ? props.response.status
    : '—'

  return (
    <div className='grid min-w-0 gap-2'>
      <span className='text-sm font-medium'>
        {t('Response preview')}{' '}
        <span className='text-muted-foreground font-mono font-normal'>
          HTTP {status}
        </span>
      </span>
      <Tabs defaultValue='claude' className='gap-2'>
        <TabsList aria-label={t('Response format')}>
          <TabsTrigger value='claude' className='px-3'>
            Claude
          </TabsTrigger>
          <TabsTrigger value='openai' className='px-3'>
            {t('OpenAI (Chat, Responses, Gemini)')}
          </TabsTrigger>
        </TabsList>
        <TabsContent value='claude'>
          <pre className='bg-muted/40 rounded-lg border p-3 font-mono text-xs break-words whitespace-pre-wrap'>
            {JSON.stringify(
              {
                type: 'error',
                error: { type: 'new_api_error', message },
              },
              null,
              2
            )}
          </pre>
        </TabsContent>
        <TabsContent value='openai'>
          <pre className='bg-muted/40 rounded-lg border p-3 font-mono text-xs break-words whitespace-pre-wrap'>
            {JSON.stringify(
              {
                error: {
                  message,
                  type: 'new_api_error',
                  param: '',
                  code: props.response.code,
                },
              },
              null,
              2
            )}
          </pre>
        </TabsContent>
      </Tabs>
    </div>
  )
}

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
import { zodResolver } from '@hookform/resolvers/zod'
import { Plus, Trash2 } from 'lucide-react'
import { useEffect, useMemo } from 'react'
import { useFieldArray, useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'

import { Dialog } from '@/components/dialog'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import { Switch } from '@/components/ui/switch'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Textarea } from '@/components/ui/textarea'

import {
  createRuleFormSchema,
  formValuesToRule,
  isRegexOp,
  ruleToFormValues,
  type RuleFormValues,
} from './rule-form'
import {
  MAX_RULE_CONDITIONS,
  RULE_ENDPOINT_VALUES,
  RULE_ENDPOINTS,
  type RequestRule,
} from './types'

type RuleEditorDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  rule: RequestRule | null
  isNew: boolean
  onSave: (rule: RequestRule) => void
}

const MESSAGE_VARIABLES = [
  '{{rule}}',
  '{{model}}',
  '{{path}}',
  '{{field}}',
  '{{value}}',
] as const

export function RuleEditorDialog(props: RuleEditorDialogProps) {
  const { t } = useTranslation()
  const schema = useMemo(() => createRuleFormSchema(t), [t])
  const form = useForm<RuleFormValues>({
    resolver: zodResolver(schema),
    defaultValues: props.rule ? ruleToFormValues(props.rule) : undefined,
  })
  const conditions = useFieldArray({
    control: form.control,
    name: 'conditions',
  })

  useEffect(() => {
    if (props.open && props.rule) form.reset(ruleToFormValues(props.rule))
  }, [props.open, props.rule, form])

  const endpoints = form.watch('endpoints') ?? []
  const message = form.watch('message') ?? ''
  const code = form.watch('code') ?? ''
  const watchedConditions = form.watch('conditions') ?? []
  const responseMessage = `${message} (request id: …)`
  const showClaudePreview = endpoints.includes('claude_messages')
  const showOpenAIPreview = endpoints.some((e) => e !== 'claude_messages')
  const conditionsError =
    form.formState.errors.conditions?.message ??
    form.formState.errors.conditions?.root?.message

  const handleSubmit = form.handleSubmit((values) => {
    if (!props.rule) return
    props.onSave(formValuesToRule(props.rule, values))
  })

  return (
    <Dialog
      open={props.open}
      onOpenChange={props.onOpenChange}
      title={props.isNew ? t('Add Rule') : t('Edit Rule')}
      contentClassName='sm:max-w-4xl'
      contentHeight='auto'
      bodyClassName='pr-2'
      footer={
        <>
          <Button
            type='button'
            variant='outline'
            onClick={() => props.onOpenChange(false)}
          >
            {t('Cancel')}
          </Button>
          <Button type='button' onClick={() => void handleSubmit()}>
            {t('Save')}
          </Button>
        </>
      }
    >
      <Form {...form}>
        <form
          className='grid grid-cols-1 gap-6'
          onSubmit={(event) => {
            event.preventDefault()
            void handleSubmit()
          }}
        >
          <section className='grid gap-4' aria-labelledby='rule-basics-title'>
            <h3 id='rule-basics-title' className='text-sm font-semibold'>
              {t('Basics')}
            </h3>
            <div className='grid gap-4 sm:grid-cols-[minmax(0,1fr)_auto] sm:items-start'>
              <FormField
                control={form.control}
                name='name'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Name')}</FormLabel>
                    <FormControl>
                      <Input {...field} />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='enabled'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Enabled')}</FormLabel>
                    <FormControl>
                      <Switch
                        checked={field.value}
                        onCheckedChange={field.onChange}
                      />
                    </FormControl>
                  </FormItem>
                )}
              />
            </div>
            <FormField
              control={form.control}
              name='description'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('What this rule blocks')}</FormLabel>
                  <FormControl>
                    <Textarea rows={2} {...field} />
                  </FormControl>
                  <FormDescription>
                    {t(
                      'Shown to administrators in the rule list. Clients never see it.'
                    )}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
          </section>

          <section className='grid gap-4' aria-labelledby='rule-scope-title'>
            <h3 id='rule-scope-title' className='text-sm font-semibold'>
              {t('Scope')}
            </h3>
            <FormField
              control={form.control}
              name='endpoints'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Endpoints')}</FormLabel>
                  <div
                    role='group'
                    aria-label={t('Endpoints')}
                    className='grid gap-2 sm:grid-cols-2'
                  >
                    {RULE_ENDPOINTS.map((endpoint) => {
                      const id = `rule-endpoint-${endpoint.value}`
                      const checked = field.value.includes(endpoint.value)
                      return (
                        <Label
                          key={endpoint.value}
                          htmlFor={id}
                          className='hover:bg-muted/30 flex cursor-pointer items-center gap-3 rounded-lg border px-3 py-2.5 font-normal'
                        >
                          <Checkbox
                            id={id}
                            checked={checked}
                            onCheckedChange={(next) =>
                              field.onChange(
                                RULE_ENDPOINT_VALUES.filter((value) =>
                                  value === endpoint.value
                                    ? next === true
                                    : field.value.includes(value)
                                )
                              )
                            }
                          />
                          <span className='flex min-w-0 flex-col'>
                            <span>{endpoint.label}</span>
                            <code className='text-muted-foreground text-xs'>
                              {endpoint.path}
                            </code>
                          </span>
                        </Label>
                      )
                    })}
                  </div>
                  <FormDescription>
                    {t(
                      'Rules check the request as the client sent it, so write field paths in that endpoint’s format.'
                    )}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='model_regex'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Model regex')}</FormLabel>
                  <FormControl>
                    <Input
                      className='font-mono'
                      placeholder='^claude-opus-5-5'
                      {...field}
                    />
                  </FormControl>
                  <FormDescription>
                    {t(
                      'Matches the model the client asked for. Leave empty for all models.'
                    )}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='only_pass_through'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('When to check')}</FormLabel>
                  <RadioGroup
                    aria-label={t('When to check')}
                    value={field.value ? 'pass_through' : 'always'}
                    onValueChange={(value) =>
                      field.onChange(value === 'pass_through')
                    }
                    className='gap-2 sm:grid-cols-2'
                  >
                    <Label
                      htmlFor='rule-check-always'
                      className='hover:bg-muted/30 flex cursor-pointer items-center gap-3 rounded-lg border px-3 py-2.5 font-normal'
                    >
                      <RadioGroupItem id='rule-check-always' value='always' />
                      {t('Always')}
                    </Label>
                    <Label
                      htmlFor='rule-check-pass-through'
                      className='hover:bg-muted/30 flex cursor-pointer items-center gap-3 rounded-lg border px-3 py-2.5 font-normal'
                    >
                      <RadioGroupItem
                        id='rule-check-pass-through'
                        value='pass_through'
                      />
                      {t('Only when the channel passes the body through')}
                    </Label>
                  </RadioGroup>
                  <FormDescription>
                    {t(
                      'Use pass-through only for fields the gateway removes or fills in when it rebuilds the body, such as stop or max_tokens.'
                    )}
                  </FormDescription>
                </FormItem>
              )}
            />
          </section>

          <section
            className='grid gap-3'
            aria-labelledby='rule-conditions-title'
          >
            <div className='flex flex-wrap items-center justify-between gap-3'>
              <h3 id='rule-conditions-title' className='text-sm font-semibold'>
                {t('Conditions')}
              </h3>
              <NativeSelect
                id='rule-match-mode'
                aria-label={t('Match mode')}
                {...form.register('match')}
              >
                <NativeSelectOption value='all'>
                  {t('Block when all conditions match')}
                </NativeSelectOption>
                <NativeSelectOption value='any'>
                  {t('Block when any condition matches')}
                </NativeSelectOption>
              </NativeSelect>
            </div>
            <ul className='grid gap-3' aria-label={t('Conditions')}>
              {conditions.fields.map((item, index) => {
                const current = watchedConditions[index]
                const isBody = current?.source === 'body'
                const needsValue =
                  current !== undefined && isRegexOp(current.op)
                const errors = form.formState.errors.conditions?.[index]
                return (
                  <li
                    key={item.id}
                    className='bg-muted/20 grid gap-2 rounded-lg border p-3'
                  >
                    <div className='grid gap-2 sm:grid-cols-[9rem_minmax(0,1.2fr)_10rem_minmax(0,1fr)_auto] sm:items-center'>
                      <NativeSelect
                        aria-label={t('Where to look')}
                        {...form.register(`conditions.${index}.source`)}
                      >
                        <NativeSelectOption value='json'>
                          {t('JSON field')}
                        </NativeSelectOption>
                        <NativeSelectOption value='header'>
                          {t('Request header')}
                        </NativeSelectOption>
                        <NativeSelectOption value='body'>
                          {t('Whole request body')}
                        </NativeSelectOption>
                      </NativeSelect>
                      <Input
                        aria-label={t('Field path or header name')}
                        className='font-mono'
                        disabled={isBody}
                        placeholder={
                          current?.source === 'header'
                            ? 'anthropic-beta'
                            : 'messages.-1.role'
                        }
                        aria-invalid={Boolean(errors?.path)}
                        {...form.register(`conditions.${index}.path`)}
                      />
                      <NativeSelect
                        aria-label={t('Operator')}
                        aria-invalid={Boolean(errors?.op)}
                        {...form.register(`conditions.${index}.op`)}
                      >
                        <NativeSelectOption value='regex'>
                          {t('Matches regex')}
                        </NativeSelectOption>
                        <NativeSelectOption value='not_regex'>
                          {t('Does not match regex')}
                        </NativeSelectOption>
                        <NativeSelectOption value='exists' disabled={isBody}>
                          {t('Exists')}
                        </NativeSelectOption>
                        <NativeSelectOption
                          value='not_exists'
                          disabled={isBody}
                        >
                          {t('Does not exist')}
                        </NativeSelectOption>
                      </NativeSelect>
                      <Input
                        aria-label={t('Regex')}
                        className='font-mono'
                        disabled={!needsValue}
                        placeholder='^assistant$'
                        aria-invalid={Boolean(errors?.value)}
                        {...form.register(`conditions.${index}.value`)}
                      />
                      <Button
                        type='button'
                        variant='ghost'
                        size='icon'
                        aria-label={t('Remove condition')}
                        disabled={conditions.fields.length === 1}
                        onClick={() => conditions.remove(index)}
                      >
                        <Trash2 aria-hidden='true' />
                      </Button>
                    </div>
                    {[errors?.path, errors?.op, errors?.value].map((error) =>
                      error?.message ? (
                        <p
                          key={error.message}
                          className='text-destructive text-xs'
                        >
                          {error.message}
                        </p>
                      ) : null
                    )}
                  </li>
                )
              })}
            </ul>
            {conditionsError ? (
              <p className='text-destructive text-xs'>{conditionsError}</p>
            ) : null}
            <div>
              <Button
                type='button'
                variant='outline'
                size='sm'
                disabled={conditions.fields.length >= MAX_RULE_CONDITIONS}
                onClick={() =>
                  conditions.append({
                    source: 'json',
                    path: '',
                    op: 'regex',
                    value: '',
                  })
                }
              >
                <Plus aria-hidden='true' data-icon='inline-start' />
                {t('Add condition')}
              </Button>
            </div>
            <div className='text-muted-foreground bg-muted/30 grid gap-2 rounded-lg border p-3 text-xs'>
              <p>
                {t(
                  'Paths: tool_choice.type reads a field, messages.#.role reads every message, messages.-1.role reads the last message, and @keys at the end lists field names.'
                )}
              </p>
              <p>
                {t(
                  'Regexes use Go syntax, which has no lookahead, lookbehind or backreferences. To require that something is absent, use Does not match regex. Start with (?i) to ignore case.'
                )}
              </p>
              <p>
                {t(
                  'When a path reaches several values, Matches regex needs one of them to match and Does not match regex needs none to match.'
                )}
              </p>
            </div>
          </section>

          <section className='grid gap-4' aria-labelledby='rule-response-title'>
            <h3 id='rule-response-title' className='text-sm font-semibold'>
              {t('Error returned to the client')}
            </h3>
            <div className='grid gap-4 sm:grid-cols-[10rem_minmax(0,1fr)]'>
              <FormField
                control={form.control}
                name='status'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Status code')}</FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        inputMode='numeric'
                        min={400}
                        max={499}
                        value={Number.isNaN(field.value) ? '' : field.value}
                        onChange={(event) =>
                          field.onChange(event.target.valueAsNumber)
                        }
                        onBlur={field.onBlur}
                        name={field.name}
                        ref={field.ref}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='code'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Error code')}</FormLabel>
                    <FormControl>
                      <Input className='font-mono' {...field} />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </div>
            <FormField
              control={form.control}
              name='message'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Error message')}</FormLabel>
                  <FormControl>
                    <Textarea rows={3} {...field} />
                  </FormControl>
                  <FormDescription>
                    {t('Variables')}:{' '}
                    {MESSAGE_VARIABLES.map((variable) => (
                      <code key={variable} className='mr-2'>
                        {variable}
                      </code>
                    ))}
                    {t(
                      'They are replaced with the rule name, model, request path, matched field and matched value. The request ID is appended automatically.'
                    )}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
            {endpoints.length > 0 ? (
              <div className='grid min-w-0 gap-2'>
                <span className='text-sm font-medium'>
                  {t('Response preview')}
                </span>
                <Tabs
                  defaultValue={showClaudePreview ? 'claude' : 'openai'}
                  className='gap-2'
                >
                  <TabsList aria-label={t('Response format')}>
                    {showClaudePreview ? (
                      <TabsTrigger value='claude' className='px-3'>
                        Claude
                      </TabsTrigger>
                    ) : null}
                    {showOpenAIPreview ? (
                      <TabsTrigger value='openai' className='px-3'>
                        {t('OpenAI (Chat, Responses, Gemini)')}
                      </TabsTrigger>
                    ) : null}
                  </TabsList>
                  <TabsContent value='claude'>
                    <pre className='bg-muted/40 rounded-lg border p-3 font-mono text-xs break-words whitespace-pre-wrap'>
                      {JSON.stringify(
                        {
                          type: 'error',
                          error: {
                            type: 'new_api_error',
                            message: responseMessage,
                          },
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
                            message: responseMessage,
                            type: 'new_api_error',
                            param: '',
                            code,
                          },
                        },
                        null,
                        2
                      )}
                    </pre>
                  </TabsContent>
                </Tabs>
              </div>
            ) : null}
          </section>
        </form>
      </Form>
    </Dialog>
  )
}

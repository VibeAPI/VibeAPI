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
import { useEffect, useMemo } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'

import { Dialog } from '@/components/dialog'
import { Button } from '@/components/ui/button'
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
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'

import {
  SettingsSwitchContent,
  SettingsSwitchItem,
} from '../../components/settings-form-layout'
import { ErrorRuleResponsePreview } from './response-preview'
import {
  createErrorRuleFormSchema,
  errorRuleToFormValues,
  formValuesToErrorRule,
  type ErrorRuleFormValues,
} from './rule-form'
import {
  INTERCEPT_RESPONSE,
  type ErrorRuleAction,
  type UpstreamErrorRule,
} from './types'

type ErrorRuleEditorDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  rule: UpstreamErrorRule | null
  isNew: boolean
  onSave: (rule: UpstreamErrorRule) => void
}

const MESSAGE_VARIABLES = ['{{model}}', '{{group}}'] as const

export function ErrorRuleEditorDialog(props: ErrorRuleEditorDialogProps) {
  const { t } = useTranslation()
  const schema = useMemo(() => createErrorRuleFormSchema(t), [t])
  const form = useForm<ErrorRuleFormValues>({
    resolver: zodResolver(schema),
    defaultValues: props.rule ? errorRuleToFormValues(props.rule) : undefined,
  })

  useEffect(() => {
    if (props.open && props.rule) form.reset(errorRuleToFormValues(props.rule))
  }, [props.open, props.rule, form])

  const action = form.watch('action')
  const status = form.watch('status') ?? ''
  const code = form.watch('code') ?? ''
  const message = form.watch('message') ?? ''
  const response =
    action === 'rewrite'
      ? { status: Number(status), code, message }
      : INTERCEPT_RESPONSE

  const handleSubmit = form.handleSubmit((values) => {
    props.onSave(formValuesToErrorRule(values))
  })

  const actionOptions: Array<{
    value: ErrorRuleAction
    label: string
    description: string
  }> = [
    {
      value: 'intercept',
      label: t('Intercept'),
      description: t("Return this site's no-available-channel error (503)."),
    },
    {
      value: 'rewrite',
      label: t('Rewrite'),
      description: t('Return the status code and message set below.'),
    },
  ]

  return (
    <Dialog
      open={props.open}
      onOpenChange={props.onOpenChange}
      title={props.isNew ? t('Add Rule') : t('Edit Rule')}
      contentClassName='sm:max-w-3xl'
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
            // The editor sits inside the channel health form; a submit that
            // bubbled through the portal would save that whole form.
            event.preventDefault()
            event.stopPropagation()
            void handleSubmit()
          }}
        >
          <section
            className='grid gap-4'
            aria-labelledby='error-rule-basics-title'
          >
            <h3 id='error-rule-basics-title' className='text-sm font-semibold'>
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
                      <Input placeholder={t('Optional')} {...field} />
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
          </section>

          <section
            className='grid gap-4'
            aria-labelledby='error-rule-match-title'
          >
            <h3 id='error-rule-match-title' className='text-sm font-semibold'>
              {t('Upstream error to match')}
            </h3>
            <FormField
              control={form.control}
              name='status_codes'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Upstream status codes')}</FormLabel>
                  <FormControl>
                    <Input
                      className='font-mono'
                      placeholder={t('e.g. 500 or 401-403')}
                      {...field}
                    />
                  </FormControl>
                  <FormDescription>
                    {t(
                      'Accepts comma-separated status codes and inclusive ranges.'
                    )}{' '}
                    {t('Leave empty for any status code.')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='keywords'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Error keywords')}</FormLabel>
                  <FormControl>
                    <Textarea
                      rows={5}
                      className='font-mono'
                      placeholder={t('one keyword per line')}
                      {...field}
                    />
                  </FormControl>
                  <FormDescription>
                    {t(
                      'Matches when the upstream error code or message contains any of these keywords, ignoring case. Leave empty for any message. With both status codes and keywords, both must match.'
                    )}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
          </section>

          <section
            className='grid gap-4'
            aria-labelledby='error-rule-action-title'
          >
            <h3 id='error-rule-action-title' className='text-sm font-semibold'>
              {t('Error returned to the client')}
            </h3>
            <FormField
              control={form.control}
              name='action'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Handling')}</FormLabel>
                  <RadioGroup
                    aria-label={t('Handling')}
                    value={field.value}
                    onValueChange={field.onChange}
                    className='gap-2 sm:grid-cols-2'
                  >
                    {actionOptions.map((option) => (
                      <Label
                        key={option.value}
                        htmlFor={`error-rule-action-${option.value}`}
                        className='hover:bg-muted/30 flex cursor-pointer items-start gap-3 rounded-lg border px-3 py-2.5 font-normal'
                      >
                        <RadioGroupItem
                          id={`error-rule-action-${option.value}`}
                          value={option.value}
                          className='mt-0.5'
                        />
                        <span className='flex min-w-0 flex-col gap-0.5'>
                          <span className='font-medium'>{option.label}</span>
                          <span className='text-muted-foreground text-xs'>
                            {option.description}
                          </span>
                        </span>
                      </Label>
                    ))}
                  </RadioGroup>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='skip_retry'
              render={({ field }) => (
                <SettingsSwitchItem>
                  <SettingsSwitchContent>
                    <FormLabel>{t('Return at once, no retry')}</FormLabel>
                    <FormDescription>
                      {t(
                        'The request ends with the error below as soon as the upstream returns a matching error: it is not retried on this or another channel, and the error does not auto-disable the channel, like a blocked invalid request. Turn this off to leave retries and auto-disable to the global settings.'
                      )}
                    </FormDescription>
                  </SettingsSwitchContent>
                  <FormControl>
                    <Switch
                      checked={field.value}
                      onCheckedChange={field.onChange}
                    />
                  </FormControl>
                </SettingsSwitchItem>
              )}
            />
            {action === 'rewrite' ? (
              <>
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
                            max={599}
                            {...field}
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
                          'They are replaced with the requested model and group. The request ID is appended automatically.'
                        )}
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              </>
            ) : (
              <p className='text-muted-foreground text-sm'>
                {t(
                  "The message follows the caller's language setting; API callers usually get English."
                )}
              </p>
            )}
            <ErrorRuleResponsePreview response={response} />
          </section>
        </form>
      </Form>
    </Dialog>
  )
}

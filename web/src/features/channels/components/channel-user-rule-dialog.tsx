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
import { MultiSelect } from '@/components/multi-select'
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
import { Switch } from '@/components/ui/switch'

import {
  channelUserRuleFormSchema,
  MAX_CHANNEL_USER_RULE_RPM,
  type ChannelUserRule,
  type ChannelUserRuleFormValues,
} from '../lib/channel-user-rules'

const CHANNEL_USER_RULE_FORM_ID = 'channel-user-rule-form'

const EMPTY_RULE: ChannelUserRuleFormValues = {
  user_id: 0,
  models: [],
  non_stream_only: false,
  rpm: 0,
}

type ChannelUserRuleDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  /** The rule being edited, or null to add a new one. */
  rule: ChannelUserRule | null
  /** Models the channel serves, offered for the rule's model list. */
  modelOptions: string[]
  onSave: (rule: ChannelUserRule) => void
}

export function ChannelUserRuleDialog(props: ChannelUserRuleDialogProps) {
  const { t } = useTranslation()
  const form = useForm<ChannelUserRuleFormValues>({
    resolver: zodResolver(channelUserRuleFormSchema),
    defaultValues: EMPTY_RULE,
  })
  const modelOptions = useMemo(
    () => props.modelOptions.map((model) => ({ label: model, value: model })),
    [props.modelOptions]
  )

  useEffect(() => {
    if (!props.open) return
    form.reset(
      props.rule
        ? {
            user_id: props.rule.user_id,
            models: props.rule.models ?? [],
            non_stream_only: props.rule.non_stream_only === true,
            rpm: props.rule.rpm ?? 0,
          }
        : EMPTY_RULE
    )
  }, [props.open, props.rule, form])

  const saveRule = (values: ChannelUserRuleFormValues) => {
    props.onSave({
      user_id: values.user_id,
      ...(values.models.length > 0 ? { models: values.models } : {}),
      ...(values.non_stream_only ? { non_stream_only: true } : {}),
      ...(values.rpm > 0 ? { rpm: values.rpm } : {}),
    })
    props.onOpenChange(false)
  }

  return (
    <Dialog
      open={props.open}
      onOpenChange={props.onOpenChange}
      title={props.rule ? t('Edit rule') : t('Add rule')}
      description={t(
        'Requests from this user that match the rule skip this channel and try the next priority.'
      )}
      contentClassName='sm:max-w-[520px]'
      contentHeight='auto'
      bodyClassName='space-y-4'
      footer={
        <>
          <Button
            type='button'
            variant='outline'
            onClick={() => props.onOpenChange(false)}
          >
            {t('Cancel')}
          </Button>
          <Button type='submit' form={CHANNEL_USER_RULE_FORM_ID}>
            {props.rule ? t('Update') : t('Add')}
          </Button>
        </>
      }
    >
      <Form {...form}>
        <form
          id={CHANNEL_USER_RULE_FORM_ID}
          className='space-y-4'
          onSubmit={(event) => {
            // The dialog is opened from inside the channel form, and React
            // passes a submit up to it through the portal, which would save
            // the whole channel.
            event.stopPropagation()
            void form.handleSubmit(saveRule)(event)
          }}
        >
          <FormField
            control={form.control}
            name='user_id'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('User ID')}</FormLabel>
                <FormControl>
                  <Input
                    type='number'
                    min={1}
                    step={1}
                    name={field.name}
                    ref={field.ref}
                    onBlur={field.onBlur}
                    value={field.value || ''}
                    onChange={(e) =>
                      field.onChange(Number.parseInt(e.target.value) || 0)
                    }
                  />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='models'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Models')}</FormLabel>
                <FormControl>
                  <MultiSelect
                    options={modelOptions}
                    selected={field.value}
                    onChange={field.onChange}
                    placeholder={t('All models')}
                    allowCreate={false}
                  />
                </FormControl>
                <FormDescription>
                  {t('Leave empty to match every model of this channel.')}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='non_stream_only'
            render={({ field }) => (
              <FormItem className='flex items-center justify-between gap-4'>
                <div className='space-y-0.5'>
                  <FormLabel>{t('Non-streaming requests only')}</FormLabel>
                  <FormDescription>
                    {t('Streaming requests are not affected.')}
                  </FormDescription>
                </div>
                <FormControl>
                  <Switch
                    checked={field.value}
                    onCheckedChange={field.onChange}
                    onBlur={field.onBlur}
                    name={field.name}
                    ref={field.ref}
                  />
                </FormControl>
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='rpm'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Allowed per minute')}</FormLabel>
                <div className='flex items-center gap-2'>
                  <FormControl>
                    <Input
                      type='number'
                      min={0}
                      max={MAX_CHANNEL_USER_RULE_RPM}
                      step={1}
                      placeholder='0'
                      name={field.name}
                      ref={field.ref}
                      onBlur={field.onBlur}
                      value={field.value || ''}
                      onChange={(e) =>
                        field.onChange(Number.parseInt(e.target.value) || 0)
                      }
                    />
                  </FormControl>
                  <span className='text-muted-foreground text-sm'>
                    {t('times')}
                  </span>
                </div>
                <FormDescription>
                  {t(
                    '0 means every matching request skips this channel. A number lets that many matching requests a minute still use this channel; the rest skip it.'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />
        </form>
      </Form>
    </Dialog>
  )
}

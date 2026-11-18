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
import { useEffect, useMemo, useRef } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import * as z from 'zod'

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
import { Textarea } from '@/components/ui/textarea'
import { handleServerError } from '@/lib/handle-server-error'
import { parseHttpStatusCodeRules } from '@/lib/http-status-code-rules'

import {
  SettingsControlChildren,
  SettingsControlGroup,
  SettingsForm,
  SettingsSwitchContent,
  SettingsSwitchItem,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useResetForm } from '../hooks/use-reset-form'
import type { UpstreamErrorSettings } from './defaults'
import { useSavePolicy } from './use-save-policy'

const createUpstreamErrorSchema = (
  t: (key: string, options?: Record<string, unknown>) => string
) =>
  z.object({
    upstream_error_setting: z.object({
      mask_enabled: z.boolean(),
      mask_status_codes: z.string().superRefine((value, ctx) => {
        const parsed = parseHttpStatusCodeRules(value)
        if (!parsed.ok) {
          ctx.addIssue({
            code: 'custom',
            message: t('Invalid status code rules: {{tokens}}', {
              tokens: parsed.invalidTokens.join(', '),
            }),
          })
        }
      }),
      mask_keywords: z.string(),
    }),
  })

type UpstreamErrorFormValues = z.infer<
  ReturnType<typeof createUpstreamErrorSchema>
>

type UpstreamErrorOptionValues = Record<keyof UpstreamErrorSettings, string>

type UpstreamErrorsSectionProps = {
  defaultValues: UpstreamErrorSettings
}

function toFormValues(
  settings: UpstreamErrorSettings
): UpstreamErrorFormValues {
  return {
    upstream_error_setting: {
      mask_enabled: settings['upstream_error_setting.mask_enabled'],
      mask_status_codes: settings['upstream_error_setting.mask_status_codes'],
      mask_keywords: settings['upstream_error_setting.mask_keywords'],
    },
  }
}

// Option values are compared in the form the backend stores, so reformatting
// status codes or line endings alone does not count as a change.
function toOptionValues(
  values: UpstreamErrorFormValues
): UpstreamErrorOptionValues {
  const setting = values.upstream_error_setting
  return {
    'upstream_error_setting.mask_enabled': String(setting.mask_enabled),
    'upstream_error_setting.mask_status_codes': parseHttpStatusCodeRules(
      setting.mask_status_codes
    ).normalized,
    'upstream_error_setting.mask_keywords': setting.mask_keywords.replaceAll(
      '\r\n',
      '\n'
    ),
  }
}

export function UpstreamErrorsSection(props: UpstreamErrorsSectionProps) {
  const { t } = useTranslation()
  const updateOption = useSavePolicy()
  const formDefaults = useMemo(
    () => toFormValues(props.defaultValues),
    [props.defaultValues]
  )
  const baselineRef = useRef(toOptionValues(formDefaults))
  const form = useForm<UpstreamErrorFormValues>({
    resolver: zodResolver(createUpstreamErrorSchema(t)),
    defaultValues: formDefaults,
  })

  useResetForm(form, formDefaults)
  useEffect(() => {
    baselineRef.current = toOptionValues(formDefaults)
  }, [formDefaults])

  const statusCodes = form.watch('upstream_error_setting.mask_status_codes')
  const statusCodesParsed = useMemo(
    () => parseHttpStatusCodeRules(statusCodes),
    [statusCodes]
  )

  const onSubmit = async (values: UpstreamErrorFormValues) => {
    const next = toOptionValues(values)
    const updates = Object.entries(next).filter(
      ([key, value]) =>
        value !== baselineRef.current[key as keyof UpstreamErrorOptionValues]
    )
    if (updates.length === 0) {
      toast.info(t('No changes to save'))
      return
    }
    try {
      await updateOption.mutateAsync(Object.fromEntries(updates))
      baselineRef.current = next
    } catch (error) {
      handleServerError(error)
    }
  }

  return (
    <SettingsSection title={t('Upstream errors')}>
      <div className='text-muted-foreground space-y-1 text-sm'>
        <p>{t('Source: global settings. Changes take effect after saving.')}</p>
        <p>
          {t(
            "Hides upstream account problems, such as exhausted balance, a rejected key or a missing model. Callers get this site's own no-available-channel error in API responses and usage logs, while administrators still see the original error in usage logs."
          )}
        </p>
      </div>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)}>
          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            isSaving={form.formState.isSubmitting}
          />
          <SettingsControlGroup>
            <FormField
              control={form.control}
              name='upstream_error_setting.mask_enabled'
              render={({ field }) => (
                <SettingsSwitchItem>
                  <SettingsSwitchContent>
                    <FormLabel>{t('Hide upstream account errors')}</FormLabel>
                    <FormDescription>
                      {t(
                        "Applies only to errors returned by upstream channels. Errors from this site, such as a caller's own insufficient balance, stay unchanged."
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

            <SettingsControlChildren
              role='group'
              aria-label={t('Hiding rules')}
              className='space-y-4 pb-1'
            >
              <FormField
                control={form.control}
                name='upstream_error_setting.mask_status_codes'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Always-hidden status codes')}</FormLabel>
                    <FormControl>
                      <Input
                        placeholder={t('e.g. 401, 403, 429, 500-599')}
                        {...field}
                      />
                    </FormControl>
                    <FormDescription>
                      {t(
                        'Upstream errors with these status codes are always hidden.'
                      )}{' '}
                      {t(
                        'Accepts comma-separated status codes and inclusive ranges.'
                      )}{' '}
                      {statusCodesParsed.ok &&
                        statusCodesParsed.normalized &&
                        statusCodesParsed.normalized !== field.value.trim() && (
                          <span className='text-muted-foreground'>
                            {t('Normalized:')} {statusCodesParsed.normalized}
                          </span>
                        )}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='upstream_error_setting.mask_keywords'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Hidden error keywords')}</FormLabel>
                    <FormControl>
                      <Textarea
                        rows={10}
                        placeholder={t('one keyword per line')}
                        {...field}
                      />
                    </FormControl>
                    <FormDescription>
                      {t(
                        'Upstream errors whose error code or message contains any of these keywords are hidden. Matching ignores case.'
                      )}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </SettingsControlChildren>
          </SettingsControlGroup>
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}

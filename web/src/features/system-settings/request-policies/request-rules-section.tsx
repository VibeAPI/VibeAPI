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
import { useEffect, useMemo, useRef } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Form, FormField } from '@/components/ui/form'
import { handleServerError } from '@/lib/handle-server-error'

import {
  SettingsControlGroup,
  SettingsForm,
  SettingsSwitchField,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useResetForm } from '../hooks/use-reset-form'
import type { RequestRuleSettings } from './defaults'
import { useSavePolicy } from './use-save-policy'

// Option keys contain dots, so the form nests them the way react-hook-form
// reads dotted field names.
type RequestRuleFormValues = {
  request_rule_setting: {
    openai_content_blocks: boolean
    openai_tools: boolean
    openai_tool_role: boolean
    openai_fields: boolean
    missing_max_tokens: boolean
  }
}

type RequestRuleOptionValues = Record<keyof RequestRuleSettings, string>

type RequestRulesSectionProps = {
  defaultValues: RequestRuleSettings
}

function toFormValues(settings: RequestRuleSettings): RequestRuleFormValues {
  return {
    request_rule_setting: {
      openai_content_blocks:
        settings['request_rule_setting.openai_content_blocks'],
      openai_tools: settings['request_rule_setting.openai_tools'],
      openai_tool_role: settings['request_rule_setting.openai_tool_role'],
      openai_fields: settings['request_rule_setting.openai_fields'],
      missing_max_tokens: settings['request_rule_setting.missing_max_tokens'],
    },
  }
}

function toOptionValues(
  values: RequestRuleFormValues
): RequestRuleOptionValues {
  const rules = values.request_rule_setting
  return {
    'request_rule_setting.openai_content_blocks': String(
      rules.openai_content_blocks
    ),
    'request_rule_setting.openai_tools': String(rules.openai_tools),
    'request_rule_setting.openai_tool_role': String(rules.openai_tool_role),
    'request_rule_setting.openai_fields': String(rules.openai_fields),
    'request_rule_setting.missing_max_tokens': String(rules.missing_max_tokens),
  }
}

export function RequestRulesSection(props: RequestRulesSectionProps) {
  const { t } = useTranslation()
  const updateOption = useSavePolicy()
  const formDefaults = useMemo(
    () => toFormValues(props.defaultValues),
    [props.defaultValues]
  )
  const baselineRef = useRef(toOptionValues(formDefaults))
  const form = useForm<RequestRuleFormValues>({ defaultValues: formDefaults })

  useResetForm(form, formDefaults)
  useEffect(() => {
    baselineRef.current = toOptionValues(formDefaults)
  }, [formDefaults])

  const rules = useMemo(
    () =>
      [
        {
          name: 'request_rule_setting.openai_content_blocks',
          label: t('OpenAI image and file blocks'),
          description: t(
            'Content blocks of type image_url, input_image, file or input_file, including those inside tool results. Claude expects image and document blocks.'
          ),
        },
        {
          name: 'request_rule_setting.openai_tools',
          label: t('OpenAI-style tools'),
          description: t(
            'Tools written as type function with a nested function object. Claude expects name, description and input_schema on each tool.'
          ),
        },
        {
          name: 'request_rule_setting.openai_tool_role',
          label: t('Messages with role tool'),
          description: t(
            'Claude returns tool results as tool_result blocks inside a user message.'
          ),
        },
        {
          name: 'request_rule_setting.openai_fields',
          label: t('OpenAI-only parameters'),
          description: t(
            'Parameters such as stop, stream_options, n, seed and response_format, and message fields such as name and tool_calls. Checked only when the channel passes the body through, because a rebuilt body drops them.'
          ),
        },
        {
          name: 'request_rule_setting.missing_max_tokens',
          label: t('Missing max_tokens'),
          description: t(
            'Claude requires max_tokens. Checked only when the channel passes the body through, because a rebuilt body gets a default value.'
          ),
        },
      ] as const,
    [t]
  )

  const onSubmit = async (values: RequestRuleFormValues) => {
    const next = toOptionValues(values)
    const updates = Object.entries(next).filter(
      ([key, value]) =>
        value !== baselineRef.current[key as keyof RequestRuleOptionValues]
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
    <SettingsSection title={t('Invalid requests')}>
      <div className='text-muted-foreground space-y-1 text-sm'>
        <p>{t('Source: global settings. Changes take effect after saving.')}</p>
        <p>
          {t(
            'Channels with Block invalid requests turned on check /v1/messages requests against the enabled rules before relaying. A match gets a 400 error that names the rule. It is not sent upstream, retried on another channel or counted against channel health, and usage logs record it with the error code request_rule_blocked.'
          )}
        </p>
        <p>
          {t(
            'Each rule matches an OpenAI-format request that Claude upstreams reject. Limits of a specific model, such as an unsupported tool_choice or temperature, are left to the upstream.'
          )}
        </p>
      </div>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)}>
          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            isSaving={form.formState.isSubmitting}
          />
          <SettingsControlGroup role='group' aria-label={t('Request rules')}>
            {rules.map((rule) => (
              <FormField
                key={rule.name}
                control={form.control}
                name={rule.name}
                render={({ field }) => (
                  <SettingsSwitchField
                    controlId={rule.name}
                    checked={field.value}
                    onCheckedChange={field.onChange}
                    label={rule.label}
                    description={rule.description}
                  />
                )}
              />
            ))}
          </SettingsControlGroup>
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}

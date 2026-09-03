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
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Plus, Trash2 } from 'lucide-react'
import { nanoid } from 'nanoid'
import { useEffect, useMemo } from 'react'
import { useFieldArray, useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import * as z from 'zod'

import { PasswordInput } from '@/components/password-input'
import { Button } from '@/components/ui/button'
import { Empty, EmptyDescription, EmptyHeader } from '@/components/ui/empty'
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
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'

import {
  getUpstreamBalanceSettings,
  updateUpstreamBalanceSettings,
} from '../api'
import {
  SettingsForm,
  SettingsSwitchContent,
  SettingsSwitchItem,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { safeNumberFieldProps } from '../utils/numeric-field'

const createUpstreamBalanceSchema = (t: (key: string) => string) =>
  z
    .object({
      enabled: z.boolean(),
      visibility: z.enum(['all', 'admin']),
      refresh_interval_seconds: z.number().int().min(5).max(300),
      accounts: z
        .array(
          z
            .object({
              id: z.string().min(1).max(128),
              name: z.string().min(1, t('Account name is required')).max(128),
              base_url: z
                .string()
                .max(2048)
                .url(t('Must be a valid URL'))
                .refine((value) => value.startsWith('https://'), {
                  error: t('URL must use HTTPS'),
                }),
              token: z.string().max(128),
              has_token: z.boolean(),
            })
        )
        .max(20),
    })
    .superRefine((settings, context) => {
      if (settings.enabled && settings.accounts.length === 0) {
        context.addIssue({
          code: 'custom',
          path: ['accounts'],
          message: t('At least one upstream account is required when enabled'),
        })
      }
      if (settings.enabled) {
        settings.accounts.forEach((account, index) => {
          if (!account.has_token && account.token.trim() === '') {
            context.addIssue({
              code: 'custom',
              path: ['accounts', index, 'token'],
              message: t('A balance query token is required'),
            })
          }
        })
      }
    })

type UpstreamBalanceFormValues = z.infer<
  ReturnType<typeof createUpstreamBalanceSchema>
>

const defaultValues: UpstreamBalanceFormValues = {
  enabled: false,
  visibility: 'admin',
  refresh_interval_seconds: 10,
  accounts: [],
}

function createAccountId() {
  return nanoid()
}

export function UpstreamBalanceSection() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const upstreamBalanceSchema = useMemo(
    () => createUpstreamBalanceSchema(t),
    [t]
  )
  const form = useForm<UpstreamBalanceFormValues>({
    resolver: zodResolver(upstreamBalanceSchema),
    defaultValues,
  })
  const accounts = useFieldArray({
    control: form.control,
    name: 'accounts',
    keyName: 'fieldId',
  })
  const settingsQuery = useQuery({
    queryKey: ['upstream-balance-settings'],
    queryFn: getUpstreamBalanceSettings,
    staleTime: 60 * 1000,
  })

  useEffect(() => {
    const response = settingsQuery.data
    if (response?.success && response.data) {
      form.reset(response.data)
    }
  }, [form, settingsQuery.data])

  useEffect(() => {
    if (settingsQuery.error) {
      toast.error(settingsQuery.error.message || t('Failed to load settings'))
    }
  }, [settingsQuery.error, t])

  const onSubmit = async (values: UpstreamBalanceFormValues) => {
    try {
      const response = await updateUpstreamBalanceSettings(values)
      if (!response.success) {
        toast.error(response.message || t('Failed to update setting'))
        return
      }
      toast.success(t('Setting updated successfully'))
      form.reset({
        ...values,
        accounts: values.accounts.map((account) => ({
          ...account,
          token: '',
          has_token: account.has_token || account.token.trim() !== '',
        })),
      })
      try {
        window.localStorage.removeItem('status')
      } catch {
        /* empty */
      }
      await queryClient.invalidateQueries({ queryKey: ['status'] })
      await queryClient.invalidateQueries({
        queryKey: ['upstream-balance-settings'],
      })
      await queryClient.invalidateQueries({
        queryKey: ['dashboard', 'overview', 'upstream-balances'],
      })
    } catch (error) {
      toast.error(
        error instanceof Error ? error.message : t('Failed to update setting')
      )
    }
  }

  const enabled = form.watch('enabled')

  return (
    <SettingsSection title={t('Upstream account balances')}>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)} autoComplete='off'>
          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            isSaving={form.formState.isSubmitting}
            isSaveDisabled={
              !form.formState.isDirty ||
              settingsQuery.isLoading ||
              settingsQuery.isError
            }
          />
          <FormField
            control={form.control}
            name='enabled'
            render={({ field }) => (
              <SettingsSwitchItem>
                <SettingsSwitchContent>
                  <FormLabel>{t('Enable upstream balance module')}</FormLabel>
                  <FormDescription>
                    {t(
                      'Show configured upstream account balances on the dashboard'
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

          <FormField
            control={form.control}
            name='visibility'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Module visibility')}</FormLabel>
                <Select
                  items={[
                    { value: 'all', label: t('All signed-in users') },
                    { value: 'admin', label: t('Administrators only') },
                  ]}
                  value={field.value}
                  onValueChange={field.onChange}
                  disabled={!enabled}
                >
                  <FormControl>
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                  </FormControl>
                  <SelectContent>
                    <SelectGroup>
                      <SelectItem value='all'>
                        {t('All signed-in users')}
                      </SelectItem>
                      <SelectItem value='admin'>
                        {t('Administrators only')}
                      </SelectItem>
                    </SelectGroup>
                  </SelectContent>
                </Select>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='refresh_interval_seconds'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Refresh interval (seconds)')}</FormLabel>
                <FormControl>
                  <Input
                    type='number'
                    min={5}
                    max={300}
                    step={1}
                    disabled={!enabled}
                    {...safeNumberFieldProps(field)}
                  />
                </FormControl>
                <FormDescription>
                  {t(
                    'The server shares cached results to protect upstream sites from polling spikes'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <div className='flex flex-col gap-3 lg:col-span-2'>
            <div className='flex items-center justify-between gap-3'>
              <div>
                <FormLabel>{t('Upstream accounts')}</FormLabel>
                <p className='text-muted-foreground text-xs'>
                  {t(
                    'Tokens are stored on the server and never returned to the browser'
                  )}
                </p>
              </div>
              <Button
                type='button'
                size='sm'
                variant='outline'
                disabled={accounts.fields.length >= 20}
                onClick={() =>
                  accounts.append({
                    id: createAccountId(),
                    name: '',
                    base_url: '',
                    token: '',
                    has_token: false,
                  })
                }
              >
                <Plus data-icon='inline-start' />
                {t('Add account')}
              </Button>
            </div>
            {form.formState.errors.accounts?.root?.message && (
              <p className='text-destructive text-sm'>
                {t(form.formState.errors.accounts.root.message)}
              </p>
            )}

            {accounts.fields.length === 0 ? (
              <Empty className='border'>
                <EmptyHeader>
                  <EmptyDescription>
                    {t('No upstream balance accounts configured')}
                  </EmptyDescription>
                </EmptyHeader>
              </Empty>
            ) : (
              <div className='flex flex-col gap-3'>
                {accounts.fields.map((account, index) => (
                  <div
                    key={account.fieldId}
                    className='grid gap-4 rounded-xl border p-4 lg:grid-cols-2'
                  >
                    <FormField
                      control={form.control}
                      name={`accounts.${index}.name`}
                      render={({ field }) => (
                        <FormItem>
                          <FormLabel>{t('Account name')}</FormLabel>
                          <FormControl>
                            <Input {...field} />
                          </FormControl>
                          <FormMessage />
                        </FormItem>
                      )}
                    />
                    <FormField
                      control={form.control}
                      name={`accounts.${index}.base_url`}
                      render={({ field }) => (
                        <FormItem>
                          <FormLabel>{t('Upstream site URL')}</FormLabel>
                          <FormControl>
                            <Input
                              {...field}
                              placeholder='https://api.example.com'
                            />
                          </FormControl>
                          <FormMessage />
                        </FormItem>
                      )}
                    />
                    <FormField
                      control={form.control}
                      name={`accounts.${index}.token`}
                      render={({ field }) => (
                        <FormItem className='lg:col-span-2'>
                          <FormLabel>{t('Balance query token')}</FormLabel>
                          <FormControl>
                            <PasswordInput
                              {...field}
                              autoComplete='new-password'
                              placeholder={t(
                                'Leave blank to keep the saved token'
                              )}
                            />
                          </FormControl>
                          <FormDescription>
                            {t(
                              'Use a dedicated token with account balance query permission'
                            )}
                          </FormDescription>
                          <FormMessage />
                        </FormItem>
                      )}
                    />
                    <div className='flex justify-end lg:col-span-2'>
                      <Button
                        type='button'
                        size='sm'
                        variant='destructive'
                        onClick={() => accounts.remove(index)}
                      >
                        <Trash2 data-icon='inline-start' />
                        {t('Remove account')}
                      </Button>
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}

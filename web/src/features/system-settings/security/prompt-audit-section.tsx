import { zodResolver } from '@hookform/resolvers/zod'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useEffect, useMemo, useState } from 'react'
import { useForm, type FieldPath, type UseFormReturn } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import * as z from 'zod'

import { MultiSelect } from '@/components/multi-select'
import { PasswordInput } from '@/components/password-input'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardAction,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyTitle,
} from '@/components/ui/empty'
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
import { Textarea } from '@/components/ui/textarea'
import { searchChannels } from '@/features/channels/api'
import { searchUsers } from '@/features/users/api'
import { useDebounce } from '@/hooks'

import {
  getPromptAuditSettings,
  removePromptAuditBlacklistUser,
  setPromptAuditEnabled,
  testPromptAuditSettings,
  updatePromptAuditSettings,
} from '../api'
import {
  SettingsForm,
  SettingsFormGrid,
  SettingsSwitchContent,
  SettingsSwitchItem,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'

const endpointSchema = z.object({
  url: z.string(),
  api_key: z.string(),
  has_api_key: z.boolean(),
  model: z.string(),
  system_prompt: z.string().max(20000),
  timeout_seconds: z.number().int().min(1).max(60),
})

const promptAuditSchema = z
  .object({
    enabled: z.boolean(),
    mode: z.enum(['downgrade', 'reject']),
    protected_channel_ids: z.array(z.number().int().positive()).min(1),
    audience_user_ids: z.array(z.number().int().positive()),
    content_scope: z.enum(['latest', 'latest_tools', 'all']),
    max_characters: z.number().int().min(1).max(200000),
    main_threshold: z.number().min(0).max(1),
    allow_private_endpoints: z.boolean(),
    reject_message: z.string().max(500),
    version: z.number(),
    tested_version: z.number(),
    main_tested_version: z.number(),
    main: endpointSchema,
  })
  .superRefine((values, context) => {
    if (!values.main.url.startsWith('https://')) {
      context.addIssue({
        code: 'custom',
        path: ['main', 'url'],
        message: 'A complete HTTPS URL is required',
      })
    }
    if (!values.main.has_api_key && values.main.api_key.trim() === '') {
      context.addIssue({
        code: 'custom',
        path: ['main', 'api_key'],
        message: 'An API key is required',
      })
    }
    if (
      values.main.model.trim() === '' ||
      values.main.system_prompt.trim() === ''
    ) {
      context.addIssue({
        code: 'custom',
        path: ['main', 'model'],
        message: 'A model and system prompt are required',
      })
    }
  })

type PromptAuditFormValues = z.infer<typeof promptAuditSchema>

const defaultValues: PromptAuditFormValues = {
  enabled: false,
  mode: 'downgrade',
  audience_user_ids: [],
  protected_channel_ids: [],
  content_scope: 'latest_tools',
  max_characters: 40000,
  main_threshold: 0.7,
  allow_private_endpoints: false,
  reject_message: 'Your request was blocked by the content policy.',
  version: 1,
  tested_version: 0,
  main_tested_version: 0,
  main: {
    url: '',
    api_key: '',
    has_api_key: false,
    model: '',
    system_prompt: '',
    timeout_seconds: 8,
  },
}

export function PromptAuditSection() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [channelSearch, setChannelSearch] = useState('')
  const [userSearch, setUserSearch] = useState('')
  const [testContent, setTestContent] = useState('')
  const debouncedChannelSearch = useDebounce(channelSearch, 300)
  const debouncedUserSearch = useDebounce(userSearch, 300)
  const settingsQuery = useQuery({
    queryKey: ['prompt-audit-settings'],
    queryFn: getPromptAuditSettings,
  })
  const channelsQuery = useQuery({
    queryKey: ['prompt-audit-channels', debouncedChannelSearch],
    queryFn: () =>
      searchChannels({
        keyword: debouncedChannelSearch,
        p: 1,
        page_size: 100,
      }),
    staleTime: 30_000,
    placeholderData: (previousData) => previousData,
  })
  const usersQuery = useQuery({
    queryKey: ['prompt-audit-users', debouncedUserSearch],
    queryFn: () =>
      searchUsers({
        keyword: debouncedUserSearch,
        p: 1,
        page_size: 50,
      }),
    staleTime: 30_000,
    placeholderData: (previousData) => previousData,
  })
  const form = useForm<PromptAuditFormValues>({
    resolver: zodResolver(promptAuditSchema),
    defaultValues,
  })

  useEffect(() => {
    if (settingsQuery.data) form.reset(settingsQuery.data)
  }, [form, settingsQuery.data])

  const channelOptions = useMemo(
    () => {
      const options = new Map<string, string>()
      for (const channel of channelsQuery.data?.data?.items ?? []) {
        options.set(String(channel.id), `#${channel.id} · ${channel.name}`)
      }
      for (const channelId of settingsQuery.data?.protected_channel_ids ?? []) {
        const value = String(channelId)
        if (!options.has(value)) options.set(value, `#${channelId}`)
      }
      return [...options].map(([value, label]) => ({ value, label }))
    },
    [channelsQuery.data, settingsQuery.data?.protected_channel_ids]
  )
  const userOptions = useMemo(() => {
    const options = new Map<string, string>()
    for (const user of usersQuery.data?.data?.items ?? []) {
      options.set(
        String(user.id),
        `#${user.id} · ${user.username}${user.remark ? ` · ${user.remark}` : ''}`
      )
    }
    for (const userId of settingsQuery.data?.audience_user_ids ?? []) {
      const value = String(userId)
      if (!options.has(value)) options.set(value, `#${userId}`)
    }
    return [...options].map(([value, label]) => ({ value, label }))
  }, [settingsQuery.data?.audience_user_ids, usersQuery.data])

  const onSubmit = async (values: PromptAuditFormValues) => {
    const requestedEnabled = values.enabled
    const response = await updatePromptAuditSettings({
      ...values,
      enabled: false,
    })
    if (!response.success) {
      toast.error(response.message || t('Failed to update setting'))
      return
    }
    toast.success(
      requestedEnabled
        ? t('Settings saved. Run the connection test before enabling.')
        : t('Setting updated successfully')
    )
    await queryClient.invalidateQueries({
      queryKey: ['prompt-audit-settings'],
    })
  }

  const runTest = async () => {
    if (form.formState.isDirty) {
      toast.error(t('Save the settings before running a connection test.'))
      return
    }
    const response = await testPromptAuditSettings(testContent)
    if (!response.success) {
      toast.error(response.message || t('Connection test failed'))
      return
    }
    toast.success(t('Connection test succeeded'))
    await queryClient.invalidateQueries({ queryKey: ['prompt-audit-settings'] })
  }

  const toggleEnabled = async (enabled: boolean) => {
    const response = await setPromptAuditEnabled(enabled)
    if (!response.success) {
      toast.error(response.message || t('Failed to update setting'))
      return
    }
    toast.success(t('Setting updated successfully'))
    await queryClient.invalidateQueries({ queryKey: ['prompt-audit-settings'] })
  }

  const removeBlacklistedUser = async (userId: number) => {
    const response = await removePromptAuditBlacklistUser(userId)
    if (!response.success) {
      toast.error(response.message || t('Operation failed'))
      return
    }
    toast.success(t('User removed from protected channels'))
    await queryClient.invalidateQueries({ queryKey: ['prompt-audit-settings'] })
  }

  if (settingsQuery.isLoading) {
    return <div>{t('Loading...')}</div>
  }
  if (settingsQuery.isError) {
    return <Alert variant='destructive'>{t('Failed to load settings')}</Alert>
  }

  const enabled = settingsQuery.data?.enabled ?? false

  return (
    <SettingsSection title={t('Prompt Audit')}>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)} autoComplete='off'>
          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            isSaving={form.formState.isSubmitting}
            isSaveDisabled={!form.formState.isDirty}
          />

          <Alert>
            <AlertTitle>{t('Protected-channel policy')}</AlertTitle>
            <AlertDescription>
              {t(
                'Only selected users are audited on protected channels. A flagged user is added to every protected channel blacklist until you remove them.'
              )}
            </AlertDescription>
          </Alert>

          <SettingsSwitchItem>
            <SettingsSwitchContent>
              <FormLabel>{t('Enable prompt audit')}</FormLabel>
              <FormDescription>
                {enabled
                  ? t('Prompt audit is currently enabled')
                  : t('Save and test the audit endpoint before enabling')}
              </FormDescription>
            </SettingsSwitchContent>
            <Switch checked={enabled} onCheckedChange={toggleEnabled} />
          </SettingsSwitchItem>

          <AuditScopeCard
            form={form}
            channelOptions={channelOptions}
            userOptions={userOptions}
            onChannelSearchChange={setChannelSearch}
            onUserSearchChange={setUserSearch}
          />
          <AuditModelCard
            form={form}
            testContent={testContent}
            onTestContentChange={setTestContent}
            onTest={runTest}
          />
          <DecisionCard form={form} />

          <Card>
            <CardHeader>
              <CardTitle>{t('Protected-channel blacklist')}</CardTitle>
              <CardDescription>
                {t(
                  'Users listed by the selected protected channels. Removing a user releases them from all selected channels.'
                )}
              </CardDescription>
            </CardHeader>
            <CardContent>
              <ProtectedChannelBlacklist
                users={settingsQuery.data?.restricted_users ?? []}
                channelCount={settingsQuery.data?.protected_channel_count ?? 0}
                onRemove={removeBlacklistedUser}
              />
            </CardContent>
          </Card>
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}

type AuditScopeCardProps = {
  form: UseFormReturn<PromptAuditFormValues>
  channelOptions: Array<{ value: string; label: string }>
  userOptions: Array<{ value: string; label: string }>
  onChannelSearchChange: (value: string) => void
  onUserSearchChange: (value: string) => void
}

function AuditScopeCard(props: AuditScopeCardProps) {
  const { t } = useTranslation()
  return (
    <Card>
      <CardHeader>
        <CardTitle>{t('Audit scope')}</CardTitle>
        <CardDescription>
          {t('Choose the channels, users, and content sent for audit.')}
        </CardDescription>
      </CardHeader>
      <CardContent>
        <SettingsFormGrid>
          <FormField
            control={props.form.control}
            name='protected_channel_ids'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Protected channels')}</FormLabel>
                <FormControl>
                  <MultiSelect
                    options={props.channelOptions}
                    selected={field.value.map(String)}
                    onChange={(values) => field.onChange(values.map(Number))}
                    onSearchChange={props.onChannelSearchChange}
                    placeholder={t('Search protected channels...')}
                    emptyText={t('No matching channels')}
                    maxVisibleChips={3}
                  />
                </FormControl>
                <FormDescription>
                  {t('Search by channel ID, name, key, or base URL.')}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />
          <FormField
            control={props.form.control}
            name='audience_user_ids'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Audited users')}</FormLabel>
                <FormControl>
                  <MultiSelect
                    options={props.userOptions}
                    selected={field.value.map(String)}
                    onChange={(values) => field.onChange(values.map(Number))}
                    onSearchChange={props.onUserSearchChange}
                    placeholder={t(
                      'Search by ID, username, email, display name, or remark'
                    )}
                    emptyText={t('No matching users')}
                    maxVisibleChips={3}
                  />
                </FormControl>
                <FormDescription>
                  {t('Only selected users are sent to the audit model.')}
                </FormDescription>
              </FormItem>
            )}
          />
          <FormField
            control={props.form.control}
            name='content_scope'
            render={({ field }) => {
              const contentOptions = [
                { value: 'latest', label: t('Latest user input') },
                {
                  value: 'latest_tools',
                  label: t('Latest input and tool results'),
                },
                { value: 'all', label: t('All request text') },
              ]
              return (
                <FormItem>
                  <FormLabel>{t('Audited content')}</FormLabel>
                  <Select
                    items={contentOptions}
                    value={field.value}
                    onValueChange={field.onChange}
                  >
                    <FormControl>
                      <SelectTrigger>
                        <SelectValue />
                      </SelectTrigger>
                    </FormControl>
                    <SelectContent>
                      <SelectGroup>
                        {contentOptions.map((option) => (
                          <SelectItem key={option.value} value={option.value}>
                            {option.label}
                          </SelectItem>
                        ))}
                      </SelectGroup>
                    </SelectContent>
                  </Select>
                </FormItem>
              )
            }}
          />
          <FormField
            control={props.form.control}
            name='mode'
            render={({ field }) => (
              <SettingsSwitchItem>
                <SettingsSwitchContent>
                  <FormLabel>{t('Downgrade flagged requests')}</FormLabel>
                  <FormDescription>
                    {field.value === 'downgrade'
                      ? t(
                          'On: route the triggering request to an unprotected channel.'
                        )
                      : t('Off: reject the triggering request with an error.')}
                  </FormDescription>
                </SettingsSwitchContent>
                <Switch
                  checked={field.value === 'downgrade'}
                  onCheckedChange={(checked) =>
                    field.onChange(checked ? 'downgrade' : 'reject')
                  }
                />
              </SettingsSwitchItem>
            )}
          />
        </SettingsFormGrid>
      </CardContent>
    </Card>
  )
}

type AuditModelCardProps = {
  form: UseFormReturn<PromptAuditFormValues>
  testContent: string
  onTestContentChange: (value: string) => void
  onTest: () => void
}

function AuditModelCard(props: AuditModelCardProps) {
  const { t } = useTranslation()
  return (
    <Card>
      <CardHeader>
        <CardTitle>{t('Audit model')}</CardTitle>
        <CardDescription>
          {t('Each protected request is evaluated once by this model.')}
        </CardDescription>
        <CardAction>
          <Button
            type='button'
            variant='outline'
            size='sm'
            onClick={props.onTest}
          >
            {t('Connection test')}
          </Button>
        </CardAction>
      </CardHeader>
      <CardContent className='flex flex-col gap-5'>
        <FormItem>
          <FormLabel>{t('Test content')}</FormLabel>
          <FormControl>
            <Textarea
              value={props.testContent}
              onChange={(event) =>
                props.onTestContentChange(event.target.value)
              }
              placeholder={t('Optional custom content for the connection test')}
              className='min-h-20'
            />
          </FormControl>
          <FormDescription>
            {t('Connection tests use the most recently saved settings.')}
          </FormDescription>
        </FormItem>
        <SettingsFormGrid className='xl:grid-cols-4'>
          <FormField
            control={props.form.control}
            name='main.url'
            render={({ field }) => (
              <FormItem className='xl:col-span-2'>
                <FormLabel>{t('Chat Completions URL')}</FormLabel>
                <FormControl>
                  <Input
                    {...field}
                    placeholder='https://example.com/v1/chat/completions'
                  />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />
          <FormField
            control={props.form.control}
            name='main.model'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Model')}</FormLabel>
                <FormControl>
                  <Input {...field} />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />
          <NumberField
            form={props.form}
            name='main.timeout_seconds'
            label={t('Timeout (seconds)')}
            min={1}
            max={60}
          />
          <FormField
            control={props.form.control}
            name='main.api_key'
            render={({ field }) => (
              <FormItem className='xl:col-span-2'>
                <FormLabel>{t('API Key')}</FormLabel>
                <FormControl>
                  <PasswordInput
                    {...field}
                    autoComplete='new-password'
                    placeholder={t('Leave blank to keep the saved key')}
                  />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />
          <FormField
            control={props.form.control}
            name='allow_private_endpoints'
            render={({ field }) => (
              <SettingsSwitchItem className='xl:col-span-2'>
                <SettingsSwitchContent>
                  <FormLabel>{t('Allow private network endpoints')}</FormLabel>
                  <FormDescription>
                    {t('Cloud metadata addresses remain blocked.')}
                  </FormDescription>
                </SettingsSwitchContent>
                <Switch
                  checked={field.value}
                  onCheckedChange={field.onChange}
                />
              </SettingsSwitchItem>
            )}
          />
          <FormField
            control={props.form.control}
            name='main.system_prompt'
            render={({ field }) => (
              <FormItem className='xl:col-span-4'>
                <FormLabel>{t('System prompt')}</FormLabel>
                <FormControl>
                  <Textarea
                    {...field}
                    className='min-h-36 font-mono text-xs'
                    maxLength={20000}
                  />
                </FormControl>
                <FormDescription>
                  {t(
                    'Up to 20,000 characters. The input boundary and strict JSON response contract are enforced by the server.'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />
        </SettingsFormGrid>
      </CardContent>
    </Card>
  )
}

type DecisionCardProps = {
  form: UseFormReturn<PromptAuditFormValues>
}

function DecisionCard(props: DecisionCardProps) {
  const { t } = useTranslation()
  const rejectMode = props.form.watch('mode') === 'reject'
  return (
    <Card>
      <CardHeader>
        <CardTitle>{t('Decision')}</CardTitle>
        <CardDescription>
          {t('Configure the single audit decision threshold and response.')}
        </CardDescription>
      </CardHeader>
      <CardContent>
        <SettingsFormGrid className='xl:grid-cols-3'>
          <NumberField
            form={props.form}
            name='main_threshold'
            label={t('Confidence threshold')}
            min={0}
            max={1}
            step='0.05'
            description={t(
              'Flagged results at or above this value add the user to protected channel blacklists.'
            )}
          />
          <NumberField
            form={props.form}
            name='max_characters'
            label={t('Maximum audited characters')}
            min={1}
            max={200000}
            description={t(
              'Characters retained from request context, from 1 to 200,000.'
            )}
          />
          {rejectMode ? (
            <FormField
              control={props.form.control}
              name='reject_message'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Rejection message')}</FormLabel>
                  <FormControl>
                    <Input {...field} maxLength={500} />
                  </FormControl>
                  <FormDescription>
                    {t('Returned when downgrade is off (up to 500 characters).')}
                  </FormDescription>
                </FormItem>
              )}
            />
          ) : null}
        </SettingsFormGrid>
      </CardContent>
    </Card>
  )
}

type ProtectedChannelBlacklistProps = {
  users: Array<{
    user_id: number
    username: string
    display_name: string
    email: string
    remark: string
    channel_count: number
  }>
  channelCount: number
  onRemove: (userId: number) => void
}

function ProtectedChannelBlacklist(props: ProtectedChannelBlacklistProps) {
  const { t } = useTranslation()
  if (props.users.length === 0) {
    return (
      <Empty>
        <EmptyHeader>
          <EmptyTitle>{t('No users in protected channel blacklists')}</EmptyTitle>
          <EmptyDescription>
            {t('Users appear here after an audit limit is triggered.')}
          </EmptyDescription>
        </EmptyHeader>
      </Empty>
    )
  }
  return (
    <div className='divide-y'>
      {props.users.map((user) => {
        const name = user.username || user.display_name || `#${user.user_id}`
        return (
          <div
            key={user.user_id}
            className='flex min-w-0 items-center justify-between gap-4 py-3 first:pt-0 last:pb-0'
          >
            <div className='min-w-0'>
              <div className='flex flex-wrap items-center gap-2'>
                <span className='truncate font-medium'>{name}</span>
                <Badge variant='secondary'>#{user.user_id}</Badge>
                <Badge variant='outline'>
                  {t('{{count}}/{{total}} channels', {
                    count: user.channel_count,
                    total: props.channelCount,
                  })}
                </Badge>
              </div>
              {user.email || user.remark ? (
                <p className='text-muted-foreground mt-1 truncate text-xs'>
                  {[user.email, user.remark].filter(Boolean).join(' · ')}
                </p>
              ) : null}
            </div>
            <Button
              type='button'
              size='sm'
              variant='outline'
              onClick={() => props.onRemove(user.user_id)}
            >
              {t('Remove')}
            </Button>
          </div>
        )
      })}
    </div>
  )
}

type NumberFieldProps = {
  form: UseFormReturn<PromptAuditFormValues>
  name: FieldPath<PromptAuditFormValues>
  label: string
  step?: string
  description?: string
  min?: number
  max?: number
}

function NumberField(props: NumberFieldProps) {
  return (
    <FormField
      control={props.form.control}
      name={props.name}
      render={({ field }) => (
        <FormItem>
          <FormLabel>{props.label}</FormLabel>
          <FormControl>
            <Input
              type='number'
              step={props.step}
              min={props.min}
              max={props.max}
              name={field.name}
              ref={field.ref}
              onBlur={field.onBlur}
              value={String(field.value ?? '')}
              onChange={(event) => field.onChange(Number(event.target.value))}
            />
          </FormControl>
          {props.description ? (
            <FormDescription>{props.description}</FormDescription>
          ) : null}
          <FormMessage />
        </FormItem>
      )}
    />
  )
}

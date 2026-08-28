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
import { Separator } from '@/components/ui/separator'
import { Switch } from '@/components/ui/switch'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { Textarea } from '@/components/ui/textarea'
import { getChannels } from '@/features/channels/api'
import { searchUsers } from '@/features/users/api'

import {
  clearPromptAuditRestriction,
  getPromptAuditEvents,
  getPromptAuditSettings,
  resendPromptAuditEmail,
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
    audience_mode: z.enum(['all', 'whitelist', 'blacklist']),
    audience_user_ids: z.array(z.number().int().positive()),
    content_scope: z.enum(['latest', 'latest_tools', 'all']),
    max_characters: z.number().int().min(1).max(200000),
    main_threshold: z.number().min(0).max(1),
    review_threshold: z.number().min(0).max(1),
    review_enabled: z.boolean(),
    required_valid_votes: z.number().int().min(1).max(5),
    required_flagged_votes: z.number().int().min(1).max(5),
    review_total_timeout_seconds: z.number().int().min(1).max(120),
    allow_private_endpoints: z.boolean(),
    first_restriction_hours: z.number().int().min(1).max(8760),
    second_restriction_hours: z.number().int().min(1).max(87600),
    violation_reset_days: z.number().int().min(1).max(3650),
    dedupe_minutes: z.number().int().min(1).max(1440),
    retention_days: z.number().int().min(7).max(365),
    reject_message: z.string().max(500),
    appeal_contact: z.string().max(500),
    version: z.number(),
    tested_version: z.number(),
    main_tested_version: z.number(),
    review_tested_version: z.number(),
    main: endpointSchema,
    review: endpointSchema,
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
    if (
      values.review_enabled &&
      (values.review.model.trim() === '' ||
        values.review.system_prompt.trim() === '')
    ) {
      context.addIssue({
        code: 'custom',
        path: ['review', 'model'],
        message: 'A model and system prompt are required',
      })
    }
    if (values.second_restriction_hours < values.first_restriction_hours) {
      context.addIssue({
        code: 'custom',
        path: ['second_restriction_hours'],
        message: 'The second restriction cannot be shorter than the first',
      })
    }
    if (
      values.review_enabled &&
      values.required_flagged_votes > values.required_valid_votes
    ) {
      context.addIssue({
        code: 'custom',
        path: ['required_flagged_votes'],
        message: 'Flagged votes cannot exceed valid votes',
      })
    }
  })

type PromptAuditFormValues = z.infer<typeof promptAuditSchema>

export function PromptAuditSection() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [userKeyword, setUserKeyword] = useState('')
  const settingsQuery = useQuery({
    queryKey: ['prompt-audit-settings'],
    queryFn: getPromptAuditSettings,
  })
  const channelsQuery = useQuery({
    queryKey: ['prompt-audit-channels'],
    queryFn: () => getChannels({ p: 1, page_size: 100 }),
  })
  const usersQuery = useQuery({
    queryKey: ['prompt-audit-users', userKeyword],
    queryFn: () => searchUsers({ keyword: userKeyword, p: 1, page_size: 50 }),
    staleTime: 30_000,
  })
  const form = useForm<PromptAuditFormValues>({
    resolver: zodResolver(promptAuditSchema),
    defaultValues: {
      enabled: false,
      mode: 'downgrade',
      audience_mode: 'all',
      audience_user_ids: [],
      protected_channel_ids: [],
      content_scope: 'latest_tools',
      max_characters: 40000,
      main_threshold: 0.7,
      review_threshold: 0.7,
      review_enabled: true,
      required_valid_votes: 3,
      required_flagged_votes: 3,
      review_total_timeout_seconds: 15,
      allow_private_endpoints: false,
      first_restriction_hours: 24,
      second_restriction_hours: 168,
      violation_reset_days: 90,
      dedupe_minutes: 10,
      retention_days: 90,
      reject_message: 'Your request was blocked by the content policy.',
      appeal_contact: '',
      version: 1,
      tested_version: 0,
      main_tested_version: 0,
      review_tested_version: 0,
      main: {
        url: '',
        api_key: '',
        has_api_key: false,
        model: '',
        system_prompt: '',
        timeout_seconds: 8,
      },
      review: {
        url: '',
        api_key: '',
        has_api_key: false,
        model: '',
        system_prompt: '',
        timeout_seconds: 8,
      },
    },
  })

  useEffect(() => {
    if (settingsQuery.data) form.reset(settingsQuery.data)
  }, [form, settingsQuery.data])

  const channelOptions = useMemo(
    () =>
      (channelsQuery.data?.data?.items ?? []).map((channel) => ({
        value: String(channel.id),
        label: `#${channel.id} · ${channel.name}`,
      })),
    [channelsQuery.data]
  )
  const userOptions = useMemo(
    () =>
      (usersQuery.data?.data?.items ?? []).map((user) => ({
        value: String(user.id),
        label: `#${user.id} · ${user.username}${user.remark ? ` · ${user.remark}` : ''}${user.email ? ` · ${user.email}` : ''}`,
      })),
    [usersQuery.data]
  )

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
        ? t('Settings saved. Test every enabled audit stage before enabling.')
        : t('Setting updated successfully')
    )
    await queryClient.invalidateQueries({
      queryKey: ['prompt-audit-settings'],
    })
  }

  const [testContent, setTestContent] = useState('')

  const runTest = async (stage: 'main' | 'review') => {
    if (form.formState.isDirty) {
      toast.error(t('Save the settings before running a connection test.'))
      return
    }
    const response = await testPromptAuditSettings(stage, testContent)
    if (!response.success) {
      toast.error(response.message || t('Connection test failed'))
      return
    }
    toast.success(t('Connection test succeeded'))
    await queryClient.invalidateQueries({
      queryKey: ['prompt-audit-settings'],
    })
  }

  const toggleEnabled = async (enabled: boolean) => {
    const response = await setPromptAuditEnabled(enabled)
    if (!response.success) {
      toast.error(response.message || t('Failed to update setting'))
      return
    }
    toast.success(t('Setting updated successfully'))
    await queryClient.invalidateQueries({
      queryKey: ['prompt-audit-settings'],
    })
  }

  if (settingsQuery.isLoading) {
    return <div>{t('Loading...')}</div>
  }
  if (settingsQuery.isError) {
    return <Alert variant='destructive'>{t('Failed to load settings')}</Alert>
  }

  const enabled = settingsQuery.data?.enabled ?? false
  const audienceMode = form.watch('audience_mode')
  const reviewEnabled = form.watch('review_enabled')

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
                'Only requests selected for a protected channel are audited. Flagged or unavailable audits never return to protected channels.'
              )}
            </AlertDescription>
          </Alert>

          <SettingsSwitchItem>
            <SettingsSwitchContent>
              <FormLabel>{t('Enable prompt audit')}</FormLabel>
              <FormDescription>
                {enabled
                  ? t('Prompt audit is currently enabled')
                  : t(
                      'Save and test every enabled audit stage before enabling'
                    )}
              </FormDescription>
            </SettingsSwitchContent>
            <Switch checked={enabled} onCheckedChange={toggleEnabled} />
          </SettingsSwitchItem>

          <div className='flex min-w-0 flex-col gap-4'>
            <h4 className='text-sm font-medium'>{t('Audit scope')}</h4>
            <div className='grid min-w-0 gap-5 xl:grid-cols-3'>
              <FormField
                control={form.control}
                name='mode'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Enforcement mode')}</FormLabel>
                    <Select
                      items={[
                        {
                          value: 'downgrade',
                          label: t('Downgrade to an unprotected channel'),
                        },
                        { value: 'reject', label: t('Verify and reject') },
                      ]}
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
                          <SelectItem value='downgrade'>
                            {t('Downgrade to an unprotected channel')}
                          </SelectItem>
                          <SelectItem value='reject'>
                            {t('Verify and reject')}
                          </SelectItem>
                        </SelectGroup>
                      </SelectContent>
                    </Select>
                    <FormDescription>
                      {t(
                        'Choose whether a flagged request is routed to a lower-priority unprotected channel or rejected.'
                      )}
                    </FormDescription>
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='content_scope'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Audited content')}</FormLabel>
                    <Select
                      items={[
                        { value: 'latest', label: t('Latest user input') },
                        {
                          value: 'latest_tools',
                          label: t('Latest input and tool results'),
                        },
                        { value: 'all', label: t('All request text') },
                      ]}
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
                          <SelectItem value='latest'>
                            {t('Latest user input')}
                          </SelectItem>
                          <SelectItem value='latest_tools'>
                            {t('Latest input and tool results')}
                          </SelectItem>
                          <SelectItem value='all'>
                            {t('All request text')}
                          </SelectItem>
                        </SelectGroup>
                      </SelectContent>
                    </Select>
                    <FormDescription>
                      {t(
                        'Controls how much request context is sent to the audit model.'
                      )}
                    </FormDescription>
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='protected_channel_ids'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Protected channels')}</FormLabel>
                    <FormControl>
                      <MultiSelect
                        options={channelOptions}
                        selected={(field.value ?? []).map(String)}
                        onChange={(values) =>
                          field.onChange(values.map(Number))
                        }
                        placeholder={t('Select protected channels...')}
                        maxVisibleChips={2}
                      />
                    </FormControl>
                    <FormDescription>
                      {t(
                        'Only requests routed to these channel IDs are audited.'
                      )}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </div>
            <div className='grid min-w-0 gap-5 xl:grid-cols-3'>
              <FormField
                control={form.control}
                name='audience_mode'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Audit audience')}</FormLabel>
                    <Select
                      items={[
                        { value: 'all', label: t('Audit all users') },
                        {
                          value: 'whitelist',
                          label: t('Exclude whitelisted users'),
                        },
                        {
                          value: 'blacklist',
                          label: t('Audit only blacklisted users'),
                        },
                      ]}
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
                          <SelectItem value='all'>
                            {t('Audit all users')}
                          </SelectItem>
                          <SelectItem value='whitelist'>
                            {t('Exclude whitelisted users')}
                          </SelectItem>
                          <SelectItem value='blacklist'>
                            {t('Audit only blacklisted users')}
                          </SelectItem>
                        </SelectGroup>
                      </SelectContent>
                    </Select>
                    <FormDescription>
                      {t(
                        'Whitelist excludes selected users; blacklist audits only selected users.'
                      )}
                    </FormDescription>
                  </FormItem>
                )}
              />
              {audienceMode !== 'all' ? (
                <FormField
                  control={form.control}
                  name='audience_user_ids'
                  render={({ field }) => (
                    <FormItem className='xl:col-span-2'>
                      <FormLabel>{t('Users')}</FormLabel>
                      <Input
                        value={userKeyword}
                        onChange={(event) => setUserKeyword(event.target.value)}
                        placeholder={t(
                          'Search by ID, username, email, display name, or remark'
                        )}
                      />
                      <FormControl>
                        <MultiSelect
                          options={userOptions}
                          selected={(field.value ?? []).map(String)}
                          onChange={(values) =>
                            field.onChange(values.map(Number))
                          }
                          maxVisibleChips={3}
                          emptyText={t('No matching users')}
                        />
                      </FormControl>
                      <FormDescription>
                        {t('Search results include administrator remarks.')}
                      </FormDescription>
                    </FormItem>
                  )}
                />
              ) : null}
            </div>
          </div>

          <Separator />
          <div className='flex min-w-0 flex-col gap-4'>
            <h4 className='text-sm font-medium'>{t('Audit models')}</h4>
            <div className='grid min-w-0 gap-5 xl:grid-cols-3'>
              <FormItem className='xl:col-span-2'>
                <FormLabel>{t('Test content')}</FormLabel>
                <Textarea
                  value={testContent}
                  onChange={(event) => setTestContent(event.target.value)}
                  placeholder={t(
                    'Optional custom content for the connection test'
                  )}
                  className='min-h-20'
                />
                <FormDescription>
                  {t('Connection tests use the last saved settings.')}
                </FormDescription>
              </FormItem>
              <FormField
                control={form.control}
                name='allow_private_endpoints'
                render={({ field }) => (
                  <SettingsSwitchItem className='self-start'>
                    <SettingsSwitchContent>
                      <FormLabel>
                        {t('Allow private audit endpoints')}
                      </FormLabel>
                      <FormDescription>
                        {t(
                          'Allows audit endpoints on private networks; metadata addresses remain blocked.'
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
            </div>
          </div>
          <EndpointFields
            form={form}
            stage='main'
            title={t('Primary audit model')}
            onTest={() => runTest('main')}
            description={t(
              'Runs once for every request selected for a protected channel.'
            )}
          />
          <Separator />
          <FormField
            control={form.control}
            name='review_enabled'
            render={({ field }) => (
              <SettingsSwitchItem>
                <SettingsSwitchContent>
                  <FormLabel>{t('Enable five-vote review')}</FormLabel>
                  <FormDescription>
                    {t(
                      'When disabled, a primary violation is enforced immediately without five-model verification.'
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
          {reviewEnabled ? (
            <EndpointFields
              form={form}
              stage='review'
              title={t('Five-vote review model')}
              onTest={() => runTest('review')}
              description={t(
                'Five parallel votes independently verify a primary violation before applying restrictions.'
              )}
            />
          ) : null}
          <Separator />

          <div className='flex min-w-0 flex-col gap-4'>
            <h4 className='text-sm font-medium'>
              {t('Decision and retention')}
            </h4>
            <SettingsFormGrid className='xl:grid-cols-4'>
              <NumberField
                form={form}
                name='max_characters'
                label={t('Maximum audited characters')}
                min={1}
                max={200000}
                description={t(
                  'Characters retained from the request context, from 1 to 200,000.'
                )}
              />
              <NumberField
                form={form}
                name='main_threshold'
                label={t('Primary confidence threshold')}
                min={0}
                max={1}
                step='0.05'
                description={t(
                  'Values at or above this threshold are considered flagged (0–1).'
                )}
              />
              {reviewEnabled ? (
                <NumberField
                  form={form}
                  name='review_threshold'
                  label={t('Review confidence threshold')}
                  min={0}
                  max={1}
                  step='0.05'
                  description={t(
                    'Each review vote must meet this confidence threshold (0–1).'
                  )}
                />
              ) : null}
              {reviewEnabled ? (
                <NumberField
                  form={form}
                  name='review_total_timeout_seconds'
                  label={t('Review total timeout (seconds)')}
                  min={1}
                  max={120}
                  description={t(
                    'Maximum time allowed for all five review votes (1–120 seconds).'
                  )}
                />
              ) : null}
              {reviewEnabled ? (
                <NumberField
                  form={form}
                  name='required_valid_votes'
                  label={t('Required valid votes')}
                  min={1}
                  max={5}
                  description={t(
                    'Minimum successful responses required from five review calls (1–5).'
                  )}
                />
              ) : null}
              {reviewEnabled ? (
                <NumberField
                  form={form}
                  name='required_flagged_votes'
                  label={t('Required flagged votes')}
                  min={1}
                  max={5}
                  description={t(
                    'Minimum flagged votes required to confirm a violation; cannot exceed valid votes.'
                  )}
                />
              ) : null}
              <NumberField
                form={form}
                name='dedupe_minutes'
                label={t('Deduplication window (minutes)')}
                min={1}
                max={1440}
                description={t(
                  'Reuses a recent decision for identical user content (1–1,440 minutes).'
                )}
              />
              <NumberField
                form={form}
                name='retention_days'
                label={t('Event retention (days)')}
                min={7}
                max={365}
                description={t(
                  'Keeps audit event metadata for 7–365 days; raw prompts are never stored.'
                )}
              />
            </SettingsFormGrid>
          </div>
          <Separator />
          <div className='flex min-w-0 flex-col gap-4'>
            <h4 className='text-sm font-medium'>
              {t('Restrictions and notification')}
            </h4>
            <SettingsFormGrid className='xl:grid-cols-3'>
              <NumberField
                form={form}
                name='first_restriction_hours'
                label={t('First restriction (hours)')}
                min={1}
                max={8760}
                description={t(
                  'Protected-channel restriction after the first confirmed violation (1–8,760 hours).'
                )}
              />
              <NumberField
                form={form}
                name='second_restriction_hours'
                label={t('Second restriction (hours)')}
                min={1}
                max={87600}
                description={t(
                  'Restriction after the second violation; must be at least the first duration (up to 87,600 hours).'
                )}
              />
              <NumberField
                form={form}
                name='violation_reset_days'
                label={t('Violation reset window (days)')}
                min={1}
                max={3650}
                description={t(
                  'Resets the violation sequence after this many days without another violation (1–3,650).'
                )}
              />
              <FormField
                control={form.control}
                name='reject_message'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Rejection message')}</FormLabel>
                    <FormControl>
                      <Input {...field} maxLength={500} />
                    </FormControl>
                    <FormDescription>
                      {t(
                        'Returned to the user when reject mode blocks a request (up to 500 characters).'
                      )}
                    </FormDescription>
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='appeal_contact'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Appeal contact')}</FormLabel>
                    <FormControl>
                      <Input {...field} maxLength={500} />
                    </FormControl>
                    <FormDescription>
                      {t(
                        'Optional contact information included in violation emails (up to 500 characters).'
                      )}
                    </FormDescription>
                  </FormItem>
                )}
              />
            </SettingsFormGrid>
          </div>
          <Separator />
          <PromptAuditEvents />
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}

function PromptAuditEvents() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const eventsQuery = useQuery({
    queryKey: ['prompt-audit-events'],
    queryFn: () => getPromptAuditEvents(),
    refetchInterval: 30_000,
  })

  const clearRestriction = async (userId: number, resetCount: boolean) => {
    const response = await clearPromptAuditRestriction(userId, resetCount)
    if (!response.success) {
      toast.error(response.message || t('Operation failed'))
      return
    }
    toast.success(t('Operation completed successfully'))
    await queryClient.invalidateQueries({ queryKey: ['prompt-audit-events'] })
  }

  const resendEmail = async (eventId: string) => {
    const response = await resendPromptAuditEmail(eventId)
    if (!response.success) {
      toast.error(response.message || t('Operation failed'))
      return
    }
    toast.success(t('Email resend queued'))
    await queryClient.invalidateQueries({ queryKey: ['prompt-audit-events'] })
  }

  const events = eventsQuery.data?.data?.items ?? []
  return (
    <div className='flex flex-col gap-3'>
      <div>
        <h3 className='font-medium'>{t('Recent audit events')}</h3>
        <p className='text-muted-foreground text-sm'>
          {t('Raw prompts and model reasons are never stored.')}
        </p>
      </div>
      <div className='overflow-x-auto rounded-md border'>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{t('Event')}</TableHead>
              <TableHead>{t('User')}</TableHead>
              <TableHead>{t('Status')}</TableHead>
              <TableHead>{t('Votes')}</TableHead>
              <TableHead>{t('Email')}</TableHead>
              <TableHead>{t('Actions')}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {events.map((event) => (
              <TableRow key={event.event_id}>
                <TableCell className='font-mono text-xs'>
                  {event.event_id}
                </TableCell>
                <TableCell>#{event.user_id}</TableCell>
                <TableCell>
                  <Badge variant='secondary'>{event.status}</Badge>
                </TableCell>
                <TableCell>
                  {event.flagged_votes}/{event.valid_votes}
                </TableCell>
                <TableCell>{event.email_status}</TableCell>
                <TableCell>
                  <div className='flex flex-wrap gap-2'>
                    <Button
                      type='button'
                      size='sm'
                      variant='outline'
                      onClick={() => clearRestriction(event.user_id, false)}
                    >
                      {t('Release')}
                    </Button>
                    <Button
                      type='button'
                      size='sm'
                      variant='outline'
                      onClick={() => clearRestriction(event.user_id, true)}
                    >
                      {t('Reset count')}
                    </Button>
                    {event.status === 'violation' &&
                      event.email_status !== 'sent' && (
                        <Button
                          type='button'
                          size='sm'
                          variant='outline'
                          onClick={() => resendEmail(event.event_id)}
                        >
                          {t('Resend email')}
                        </Button>
                      )}
                  </div>
                </TableCell>
              </TableRow>
            ))}
            {!eventsQuery.isLoading && events.length === 0 && (
              <TableRow>
                <TableCell
                  colSpan={6}
                  className='text-muted-foreground text-center'
                >
                  {t('No audit events')}
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
      </div>
    </div>
  )
}

type EndpointFieldsProps = {
  form: UseFormReturn<PromptAuditFormValues>
  stage: 'main' | 'review'
  title: string
  description: string
  onTest: () => void
}

function EndpointFields(props: EndpointFieldsProps) {
  const { t } = useTranslation()
  return (
    <div className='flex flex-col gap-4'>
      <div className='flex items-center justify-between gap-3'>
        <div className='min-w-0'>
          <h3 className='font-medium'>{props.title}</h3>
          <p className='text-muted-foreground text-xs'>{props.description}</p>
        </div>
        <Button
          type='button'
          variant='outline'
          size='sm'
          onClick={props.onTest}
        >
          {t('Connection test')}
        </Button>
      </div>
      <SettingsFormGrid className='xl:grid-cols-4'>
        <FormField
          control={props.form.control}
          name={`${props.stage}.url`}
          render={({ field }) => (
            <FormItem className='xl:col-span-2'>
              <FormLabel>{t('Chat Completions URL')}</FormLabel>
              <FormControl>
                <Input
                  {...field}
                  placeholder={
                    props.stage === 'review'
                      ? t('Leave blank to inherit the primary URL')
                      : 'https://example.com/v1/chat/completions'
                  }
                />
              </FormControl>
              <FormDescription>
                {props.stage === 'review'
                  ? t('Leave blank to reuse the primary endpoint and API key.')
                  : t(
                      'A complete HTTPS Chat Completions endpoint is required.'
                    )}
              </FormDescription>
              <FormMessage />
            </FormItem>
          )}
        />
        <FormField
          control={props.form.control}
          name={`${props.stage}.model`}
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
          name={`${props.stage}.timeout_seconds`}
          label={t('Timeout (seconds)')}
          min={1}
          max={60}
        />
        <FormField
          control={props.form.control}
          name={`${props.stage}.api_key`}
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
              <FormDescription>
                {props.stage === 'review'
                  ? t('Leave blank to reuse the primary API key.')
                  : t(
                      'Stored encrypted; leave blank later to keep the saved key.'
                    )}
              </FormDescription>
              <FormMessage />
            </FormItem>
          )}
        />
        <FormField
          control={props.form.control}
          name={`${props.stage}.system_prompt`}
          render={({ field }) => (
            <FormItem className='xl:col-span-4'>
              <FormLabel>{t('System prompt')}</FormLabel>
              <FormControl>
                <Textarea
                  {...field}
                  className='min-h-40 font-mono text-xs'
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

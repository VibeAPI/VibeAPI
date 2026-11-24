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
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import {
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
  RouterProvider,
} from '@tanstack/react-router'
import {
  act,
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { api } from '@/lib/api'
import { Route as ModelsRoute } from '@/routes/_authenticated/system-settings/models/$section'
import { Route as OperationsRoute } from '@/routes/_authenticated/system-settings/operations/$section'
import { Route as PoliciesRoute } from '@/routes/_authenticated/system-settings/request-policies/$section'
import { Route as PolicyIndexRoute } from '@/routes/_authenticated/system-settings/request-policies/index'
import { Route as SecurityRoute } from '@/routes/_authenticated/system-settings/security/$section'

import { RequestPolicies } from '..'
import {
  defaultRequestPolicySettings,
  type RequestPolicySettings,
} from '../defaults'

type PolicyBeforeLoad = (context: { params: { section: string } }) => void

let queryClient: QueryClient
let settings: RequestPolicySettings

const RULE_PRESETS = [
  {
    id: 'openai_fields',
    name: 'OpenAI-only parameters',
    description: 'Parameters such as stop and stream_options.',
    preset: true,
    enabled: true,
    endpoints: ['claude_messages'],
    model_regex: '',
    only_pass_through: true,
    match: 'any',
    conditions: [
      { source: 'json', path: '@keys', op: 'regex', value: '^(stop|n)$' },
    ],
    response: {
      status: 400,
      code: 'request_rule_blocked',
      message: 'request rule openai_fields: "{{value}}" is OpenAI-only',
    },
  },
  {
    id: 'missing_max_tokens',
    name: 'Missing max_tokens',
    description: 'Claude requires max_tokens.',
    preset: true,
    enabled: true,
    endpoints: ['claude_messages'],
    model_regex: '',
    only_pass_through: true,
    match: 'all',
    conditions: [
      { source: 'json', path: 'max_tokens', op: 'not_exists', value: '' },
    ],
    response: {
      status: 400,
      code: 'request_rule_blocked',
      message: 'request rule missing_max_tokens: max_tokens is required',
    },
  },
]

function savedRules() {
  const [, payload] = vi.mocked(api.patch).mock.calls[0]
  const options = (payload as { options: Record<string, string> }).options
  expect(Object.keys(options)).toEqual(['request_rule_setting.rules'])
  return JSON.parse(options['request_rule_setting.rules'])
}

// Upstream error rules as the options API returns them for an install that
// still uses the legacy hiding settings, plus one named rule.
const ERROR_RULES = [
  {
    name: '',
    enabled: true,
    status_codes: '401-403',
    keywords: [],
    action: 'intercept',
  },
  {
    name: 'Balance',
    enabled: true,
    status_codes: '',
    keywords: ['insufficient balance', '余额不足'],
    action: 'intercept',
  },
]

function savedErrorRules() {
  const [, payload] = vi.mocked(api.patch).mock.calls[0]
  const options = (payload as { options: Record<string, string> }).options
  expect(Object.keys(options)).toEqual(['upstream_error_setting.rules'])
  return JSON.parse(options['upstream_error_setting.rules'])
}

function optionsResponse() {
  return {
    success: true,
    data: Object.entries(settings).map(([key, value]) => ({
      key,
      value: String(value),
    })),
  }
}

async function renderPolicies(path: string) {
  const root = createRootRoute()
  const authenticated = createRoute({
    getParentRoute: () => root,
    id: '_authenticated',
  })
  const policyRoute = createRoute({
    getParentRoute: () => authenticated,
    path: '/system-settings/request-policies/$section',
    beforeLoad: PoliciesRoute.options.beforeLoad as PolicyBeforeLoad,
    component: RequestPolicies,
  })
  const routes = [
    policyRoute,
    createRoute({
      getParentRoute: () => authenticated,
      path: '/system-settings/request-policies/',
      beforeLoad: PolicyIndexRoute.options.beforeLoad as () => void,
    }),
    createRoute({
      getParentRoute: () => authenticated,
      path: '/system-settings/models/$section',
      beforeLoad: ModelsRoute.options.beforeLoad as PolicyBeforeLoad,
    }),
    createRoute({
      getParentRoute: () => authenticated,
      path: '/system-settings/security/$section',
      beforeLoad: SecurityRoute.options.beforeLoad as PolicyBeforeLoad,
    }),
    createRoute({
      getParentRoute: () => authenticated,
      path: '/system-settings/operations/$section',
      beforeLoad: OperationsRoute.options.beforeLoad as PolicyBeforeLoad,
    }),
  ]
  const router = createRouter({
    routeTree: root.addChildren([authenticated.addChildren(routes)]),
    history: createMemoryHistory({ initialEntries: [path] }),
  })
  render(
    <QueryClientProvider client={queryClient}>
      <RouterProvider router={router} />
    </QueryClientProvider>
  )
  await act(() => router.load())
  return router
}

beforeEach(() => {
  vi.spyOn(window, 'scrollTo').mockImplementation(() => undefined)
  queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })
  settings = {
    ...defaultRequestPolicySettings,
    RetryTimes: 2,
    AutomaticRetryStatusCodes: '429,500-503',
    AutomaticDisableChannelEnabled: true,
    AutomaticEnableChannelEnabled: true,
    'monitor_setting.auto_test_channel_enabled': true,
    CheckSensitiveEnabled: true,
    CheckSensitiveOnPromptEnabled: true,
    SensitiveWords: 'blocked',
    'request_rule_setting.rules': JSON.stringify(RULE_PRESETS),
    'upstream_error_setting.rules': JSON.stringify(ERROR_RULES),
    'channel_affinity_setting.rules': JSON.stringify([
      {
        name: 'Existing session rule',
        model_regex: ['gpt-.*'],
        path_regex: ['/v1/chat/completions'],
        key_sources: [{ type: 'request_header', key: 'X-Session-ID' }],
        ttl_seconds: 600,
        skip_retry_on_failure: true,
        include_using_group: true,
        include_model_name: false,
        include_rule_name: true,
        param_override_template: { temperature: 0 },
      },
    ]),
  }
  vi.spyOn(api, 'patch').mockResolvedValue({
    data: {
      success: true,
      data: { options: {} },
    },
  })
  vi.spyOn(api, 'get').mockImplementation(async (url) => {
    if (url === '/api/option/') return { data: optionsResponse() }
    if (url === '/api/option/request_policy') {
      return {
        data: {
          success: true,
          data: {
            options: Object.fromEntries(
              Object.entries(settings).map(([key, value]) => [
                key,
                String(value),
              ])
            ),
            request_rule_presets: RULE_PRESETS,
          },
        },
      }
    }
    return {
      data: {
        success: true,
        data: {
          enabled: false,
          total: 0,
          unknown: 0,
          by_rule_name: {},
          cache_capacity: 100000,
          cache_algo: 'lru',
        },
      },
    }
  })
  vi.spyOn(api, 'put').mockResolvedValue({ data: { success: true } })
})

afterEach(() => {
  cleanup()
  queryClient.clear()
})

describe('request policy settings', () => {
  it.each([
    ['retry', 'Save Changes'],
    ['health', 'Save Changes'],
    ['filtering', 'Save sensitive words'],
    ['affinity', 'Save Changes'],
  ])(
    'opening %s and saving unchanged values does not write options',
    async (section, saveLabel) => {
      await renderPolicies(`/system-settings/request-policies/${section}`)
      const save = await screen.findByRole('button', { name: saveLabel })
      expect(api.put).not.toHaveBeenCalled()
      await userEvent.click(save)
      await waitFor(() => expect(save).toBeEnabled())
      expect(api.put).not.toHaveBeenCalled()
    }
  )

  it.each([
    [
      'health',
      'Channel test concurrency',
      '4',
      'monitor_setting.channel_test_concurrency',
      4,
    ],
  ])(
    'saving a changed %s field writes only that original option key',
    async (section, label, input, key, value) => {
      await renderPolicies(`/system-settings/request-policies/${section}`)
      fireEvent.change(await screen.findByRole('spinbutton', { name: label }), {
        target: { value: input },
      })
      await userEvent.click(
        screen.getByRole('button', { name: 'Save Changes' })
      )
      await waitFor(() =>
        expect(api.patch).toHaveBeenCalledExactlyOnceWith(
          '/api/option/request_policy',
          { options: { [key]: String(value) } }
        )
      )
    }
  )

  it('turning filtering off preserves the prompt switch and keyword list', async () => {
    await renderPolicies('/system-settings/request-policies/filtering')
    await userEvent.click(
      await screen.findByRole('switch', { name: 'Enable filtering' })
    )
    const prompt = screen.getByRole('switch', { name: 'Inspect user prompts' })
    expect(prompt).toHaveAttribute('aria-disabled', 'true')
    expect(prompt).toBeChecked()
    expect(
      screen.getByRole('textbox', { name: 'Blocked keywords' })
    ).toHaveValue('blocked')
    await userEvent.click(
      screen.getByRole('button', { name: 'Save sensitive words' })
    )
    await waitFor(() =>
      expect(api.patch).toHaveBeenCalledExactlyOnceWith(
        '/api/option/request_policy',
        { options: { CheckSensitiveEnabled: 'false' } }
      )
    )
  })

  it('refreshing another policy keeps unsaved filter text', async () => {
    await renderPolicies('/system-settings/request-policies/filtering')
    const keywords = await screen.findByRole('textbox', {
      name: 'Blocked keywords',
    })
    fireEvent.change(keywords, { target: { value: 'unsaved' } })
    settings.RetryTimes = 5
    await act(async () => {
      await queryClient.invalidateQueries({ queryKey: ['system-options'] })
    })
    expect(keywords).toHaveValue('unsaved')
  })

  it('invalid retry status ranges show validation and do not write options', async () => {
    await renderPolicies('/system-settings/request-policies/retry')
    const codes = await screen.findByRole('textbox', {
      name: 'Auto-retry status codes',
    })
    expect(codes).toBeVisible()
    expect(
      screen.queryByRole('button', { name: 'Advanced settings' })
    ).not.toBeInTheDocument()
    fireEvent.change(codes, { target: { value: '599-500' } })
    await userEvent.click(screen.getByRole('button', { name: 'Save Changes' }))
    expect(codes).toHaveAttribute('aria-invalid', 'true')
    expect(api.put).not.toHaveBeenCalled()
  })

  it('the health page lists the error rules in force and saving them unchanged writes nothing', async () => {
    await renderPolicies('/system-settings/request-policies/health')
    const table = await screen.findByRole('region', {
      name: 'Error rules table',
    })
    expect(within(table).getByText('401-403')).toBeVisible()
    expect(within(table).getByText('Balance')).toBeVisible()
    expect(within(table).getAllByText('Intercept')).toHaveLength(2)
    const save = screen.getByRole('button', { name: 'Save Changes' })
    await userEvent.click(save)
    await waitFor(() => expect(save).toBeEnabled())
    expect(api.patch).not.toHaveBeenCalled()
  })

  it('adding a rewrite rule previews its response and saves it after the existing rules', async () => {
    await renderPolicies('/system-settings/request-policies/health')
    await userEvent.click(
      await screen.findByRole('button', { name: 'Add Rule' })
    )
    const dialog = screen.getByRole('dialog')
    expect(within(dialog).getByText('HTTP 503')).toBeVisible()
    fireEvent.change(
      within(dialog).getByRole('textbox', { name: 'Upstream status codes' }),
      { target: { value: '500' } }
    )
    fireEvent.change(
      within(dialog).getByRole('textbox', { name: 'Error keywords' }),
      { target: { value: 'no upstream account token' } }
    )
    await userEvent.click(
      within(dialog).getByRole('radio', { name: /^Rewrite/ })
    )
    expect(within(dialog).getByText('HTTP 429')).toBeVisible()
    await userEvent.click(within(dialog).getByRole('button', { name: 'Save' }))
    await waitFor(() =>
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    )
    expect(api.patch).not.toHaveBeenCalled()
    expect(
      within(
        screen.getByRole('region', { name: 'Error rules table' })
      ).getByText('No retry')
    ).toBeVisible()
    await userEvent.click(screen.getByRole('button', { name: 'Save Changes' }))
    await waitFor(() => expect(api.patch).toHaveBeenCalledOnce())
    expect(savedErrorRules()).toEqual([
      ...ERROR_RULES,
      {
        name: '',
        enabled: true,
        status_codes: '500',
        keywords: ['no upstream account token'],
        action: 'rewrite',
        response: {
          status: 429,
          code: 'rate_limit_exceeded',
          message: 'Too many requests, please retry later',
        },
        skip_retry: true,
      },
    ])
  })

  it('turning off return at once saves a rule that leaves retries to the settings', async () => {
    await renderPolicies('/system-settings/request-policies/health')
    await userEvent.click(
      await screen.findByRole('button', { name: 'Add Rule' })
    )
    const dialog = screen.getByRole('dialog')
    fireEvent.change(
      within(dialog).getByRole('textbox', { name: 'Error keywords' }),
      { target: { value: 'upstream maintenance' } }
    )
    const returnAtOnce = within(dialog).getByRole('switch', {
      name: 'Return at once, no retry',
    })
    expect(returnAtOnce).toBeChecked()
    await userEvent.click(returnAtOnce)
    expect(returnAtOnce).not.toBeChecked()
    await userEvent.click(within(dialog).getByRole('button', { name: 'Save' }))
    await waitFor(() =>
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    )
    expect(
      within(
        screen.getByRole('region', { name: 'Error rules table' })
      ).queryByText('No retry')
    ).not.toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Save Changes' }))
    await waitFor(() => expect(api.patch).toHaveBeenCalledOnce())
    expect(savedErrorRules()[2]).toEqual({
      name: '',
      enabled: true,
      status_codes: '',
      keywords: ['upstream maintenance'],
      action: 'intercept',
    })
  })

  it('a rule without status codes or keywords stays in the dialog with an error', async () => {
    await renderPolicies('/system-settings/request-policies/health')
    await userEvent.click(
      await screen.findByRole('button', { name: 'Add Rule' })
    )
    const dialog = screen.getByRole('dialog')
    await userEvent.click(within(dialog).getByRole('button', { name: 'Save' }))
    expect(
      await within(dialog).findByText('Enter status codes, keywords or both')
    ).toBeVisible()
    expect(screen.getByRole('dialog')).toBeInTheDocument()
    expect(api.patch).not.toHaveBeenCalled()
  })

  it('turning an error rule off with the keyboard saves only the rules option', async () => {
    await renderPolicies('/system-settings/request-policies/health')
    const toggle = await screen.findByRole('switch', {
      name: 'Enable Balance',
    })
    toggle.focus()
    await userEvent.keyboard(' ')
    expect(toggle).not.toBeChecked()
    await userEvent.click(screen.getByRole('button', { name: 'Save Changes' }))
    await waitFor(() => expect(api.patch).toHaveBeenCalledOnce())
    expect(savedErrorRules()).toEqual([
      ERROR_RULES[0],
      { ...ERROR_RULES[1], enabled: false },
    ])
  })

  it('invalid error rules JSON stays in the JSON view and is not saved', async () => {
    await renderPolicies('/system-settings/request-policies/health')
    await userEvent.click(await screen.findByRole('tab', { name: 'JSON' }))
    const editor = await screen.findByRole('textbox', { name: 'Rules JSON' })
    fireEvent.input(editor, { target: { value: '[' } })
    expect(screen.getByText('Rules JSON is not valid JSON')).toBeVisible()
    await userEvent.click(screen.getByRole('tab', { name: 'Visual' }))
    expect(screen.getByRole('tab', { name: 'JSON' })).toHaveAttribute(
      'aria-selected',
      'true'
    )
    await userEvent.click(screen.getByRole('button', { name: 'Save Changes' }))
    expect(api.patch).not.toHaveBeenCalled()
  })

  it('the error rule dialog wraps long response previews instead of widening the dialog', async () => {
    await renderPolicies('/system-settings/request-policies/health')
    await userEvent.click(
      await screen.findByRole('button', { name: 'Add Rule' })
    )
    const dialog = screen.getByRole('dialog')
    expect(dialog.querySelector('form')).toHaveClass('grid-cols-1')
    const previews = dialog.querySelectorAll('pre')
    expect(previews.length).toBeGreaterThan(0)
    for (const preview of previews) {
      expect(preview).toHaveClass('whitespace-pre-wrap')
    }
  })

  it('the request rule dialog wraps long response previews instead of widening the dialog', async () => {
    await renderPolicies('/system-settings/request-policies/request-rules')
    await userEvent.click(
      await screen.findByRole('button', { name: 'Add Rule' })
    )
    await userEvent.click(screen.getByRole('menuitem', { name: 'Blank Rule' }))
    const dialog = screen.getByRole('dialog')
    expect(dialog.querySelector('form')).toHaveClass('grid-cols-1')
    const previews = dialog.querySelectorAll('pre')
    expect(previews.length).toBeGreaterThan(0)
    for (const preview of previews) {
      expect(preview).toHaveClass('whitespace-pre-wrap')
    }
  })

  it('request rules list the stored rules and saving them unchanged writes nothing', async () => {
    await renderPolicies('/system-settings/request-policies/request-rules')
    const table = await screen.findByRole('region', {
      name: 'Request rules table',
    })
    expect(within(table).getByText('OpenAI-only parameters')).toBeVisible()
    expect(within(table).getByText('Missing max_tokens')).toBeVisible()
    const save = screen.getByRole('button', { name: 'Save Changes' })
    await userEvent.click(save)
    await waitFor(() => expect(save).toBeEnabled())
    expect(api.patch).not.toHaveBeenCalled()
  })

  it('turning a rule off with the keyboard saves the rules with only that rule disabled', async () => {
    await renderPolicies('/system-settings/request-policies/request-rules')
    const toggle = await screen.findByRole('switch', {
      name: 'Enable OpenAI-only parameters',
    })
    toggle.focus()
    await userEvent.keyboard(' ')
    expect(toggle).not.toBeChecked()
    await userEvent.click(screen.getByRole('button', { name: 'Save Changes' }))
    await waitFor(() => expect(api.patch).toHaveBeenCalledOnce())
    expect(savedRules()).toEqual([
      { ...RULE_PRESETS[0], enabled: false },
      RULE_PRESETS[1],
    ])
  })

  it('adding a template opens it in the editor and saving appends it to the rules', async () => {
    await renderPolicies('/system-settings/request-policies/request-rules')
    await userEvent.click(
      await screen.findByRole('button', { name: 'Add Rule' })
    )
    await userEvent.click(
      screen.getByRole('menuitem', {
        name: 'Assistant prefill on claude-sonnet-4-6',
      })
    )
    const dialog = screen.getByRole('dialog')
    expect(within(dialog).getByRole('textbox', { name: 'Name' })).toHaveValue(
      'Assistant prefill on claude-sonnet-4-6'
    )
    expect(
      within(dialog).getByRole('textbox', { name: 'Field path or header name' })
    ).toHaveValue('messages.-1.role')
    await userEvent.click(within(dialog).getByRole('button', { name: 'Save' }))
    await waitFor(() =>
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    )
    expect(
      within(
        screen.getByRole('region', { name: 'Request rules table' })
      ).getByText('Assistant prefill on claude-sonnet-4-6')
    ).toBeVisible()
    await userEvent.click(screen.getByRole('button', { name: 'Save Changes' }))
    await waitFor(() => expect(api.patch).toHaveBeenCalledOnce())
    const rules = savedRules()
    expect(rules).toHaveLength(3)
    expect(rules[2]).toMatchObject({
      enabled: true,
      endpoints: ['claude_messages'],
      model_regex: '^claude-sonnet-4-6',
      conditions: [
        {
          source: 'json',
          path: 'messages.-1.role',
          op: 'regex',
          value: '^assistant$',
        },
      ],
      response: { status: 400, code: 'request_rule_blocked' },
    })
  })

  it('a regex condition without a regex keeps the rule editor open with an error', async () => {
    await renderPolicies('/system-settings/request-policies/request-rules')
    await userEvent.click(
      await screen.findByRole('button', { name: 'Add Rule' })
    )
    await userEvent.click(screen.getByRole('menuitem', { name: 'Blank Rule' }))
    const dialog = screen.getByRole('dialog')
    fireEvent.change(within(dialog).getByRole('textbox', { name: 'Name' }), {
      target: { value: 'No prefill' },
    })
    fireEvent.change(
      within(dialog).getByRole('textbox', {
        name: 'Field path or header name',
      }),
      { target: { value: 'messages.-1.role' } }
    )
    fireEvent.change(
      within(dialog).getByRole('textbox', { name: 'Error message' }),
      { target: { value: 'Prefill is not supported.' } }
    )
    await userEvent.click(within(dialog).getByRole('button', { name: 'Save' }))
    expect(await within(dialog).findByText('Regex is required')).toBeVisible()
    expect(screen.getByRole('dialog')).toBeInTheDocument()
    expect(api.patch).not.toHaveBeenCalled()
  })

  it('invalid rules JSON stays in the JSON view and is not saved', async () => {
    await renderPolicies('/system-settings/request-policies/request-rules')
    await userEvent.click(await screen.findByRole('tab', { name: 'JSON' }))
    const editor = await screen.findByRole('textbox', { name: 'Rules JSON' })
    fireEvent.input(editor, { target: { value: '[' } })
    expect(screen.getByText('Rules JSON is not valid JSON')).toBeVisible()
    await userEvent.click(screen.getByRole('tab', { name: 'Visual' }))
    expect(screen.getByRole('tab', { name: 'JSON' })).toHaveAttribute(
      'aria-selected',
      'true'
    )
    await userEvent.click(screen.getByRole('button', { name: 'Save Changes' }))
    expect(api.patch).not.toHaveBeenCalled()
  })

  it('testing a request sends the draft rules and shows the rule that blocks it', async () => {
    vi.spyOn(api, 'post').mockResolvedValue({
      data: {
        success: true,
        data: {
          matched: true,
          rule: {
            position: 2,
            id: 'missing_max_tokens',
            name: 'Missing max_tokens',
          },
          field: 'max_tokens',
          value: '',
          status: 400,
          response: {
            type: 'error',
            error: { type: 'new_api_error', message: 'max_tokens is required' },
          },
          trace: [
            {
              position: 1,
              id: 'openai_fields',
              name: 'OpenAI-only parameters',
              result: 'conditions',
            },
            {
              position: 2,
              id: 'missing_max_tokens',
              name: 'Missing max_tokens',
              result: 'matched',
            },
          ],
        },
      },
    })
    await renderPolicies('/system-settings/request-policies/request-rules')
    await userEvent.click(
      await screen.findByRole('button', { name: 'Test request' })
    )
    const dialog = screen.getByRole('dialog')
    await userEvent.click(
      within(dialog).getByRole('button', { name: 'Run check' })
    )
    expect(
      await within(dialog).findByText('Rule 2: Missing max_tokens')
    ).toBeVisible()
    expect(
      within(dialog).getByText('The client receives HTTP 400')
    ).toBeVisible()
    expect(api.post).toHaveBeenCalledOnce()
    const [url, request] = vi.mocked(api.post).mock.calls[0]
    expect(url).toBe('/api/option/request_policy/rules/test')
    expect(request).toMatchObject({
      endpoint: 'claude_messages',
      model: 'claude-sonnet-4-6',
      pass_through: true,
    })
    expect(JSON.parse((request as { rules: string }).rules)).toEqual(
      RULE_PRESETS
    )
  })

  it('a rejected save keeps the edited retry value and allows another save', async () => {
    vi.mocked(api.patch).mockResolvedValue({
      data: { success: false, message: 'Save rejected' },
    })
    await renderPolicies('/system-settings/request-policies/retry')
    const retries = await screen.findByRole('spinbutton', {
      name: 'Maximum retries',
    })
    fireEvent.change(retries, { target: { value: '4' } })
    await userEvent.click(screen.getByRole('button', { name: 'Save Changes' }))
    await waitFor(() =>
      expect(screen.getByRole('button', { name: 'Save Changes' })).toBeEnabled()
    )
    expect(retries).toHaveValue(4)
    await userEvent.click(screen.getByRole('button', { name: 'Save Changes' }))
    await waitFor(() => expect(api.patch).toHaveBeenCalledTimes(2))
  })

  it('failed initial loading shows a retry action instead of editable fallback values', async () => {
    vi.mocked(api.get).mockRejectedValueOnce(new Error('Unavailable'))
    await renderPolicies('/system-settings/request-policies/retry')
    expect(await screen.findByText('Failed to load settings')).toBeVisible()
    expect(screen.queryByRole('spinbutton')).not.toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Retry' }))
    expect(
      await screen.findByRole('spinbutton', {
        name: 'Maximum retries',
      })
    ).toHaveValue(2)
    expect(api.put).not.toHaveBeenCalled()
  })

  it('turning scheduled checks off explains recovery without changing the recovery setting', async () => {
    await renderPolicies('/system-settings/request-policies/health')
    await userEvent.click(
      await screen.findByRole('switch', { name: 'Scheduled channel tests' })
    )
    expect(
      screen.getByText(
        'Scheduled recovery is off. Bulk channel tests can still re-enable automatically disabled channels.'
      )
    ).toBeVisible()
    expect(
      screen.getByRole('switch', { name: 'Re-enable on success' })
    ).toBeChecked()
    expect(api.put).not.toHaveBeenCalled()
  })

  it('the affinity cache section opens with the keyboard and keeps the existing values', async () => {
    await renderPolicies('/system-settings/request-policies/affinity')
    const toggle = await screen.findByRole('button', {
      name: 'Affinity cache settings',
    })
    expect(toggle).toHaveAttribute('aria-expanded', 'false')
    toggle.focus()
    await userEvent.keyboard('{Enter}')
    expect(toggle).toHaveAttribute('aria-expanded', 'true')
    expect(
      screen.getByRole('spinbutton', { name: 'Maximum cached sessions' })
    ).toHaveValue(100000)
    expect(
      screen.getByRole('spinbutton', {
        name: 'Default session lifetime (seconds)',
      })
    ).toHaveValue(3600)
    expect(api.put).not.toHaveBeenCalled()
  })

  it.each([
    ['/system-settings/models/channel-affinity', 'routing'],
    ['/system-settings/models/routing-reliability', 'routing'],
    ['/system-settings/security/sensitive-words', 'filtering'],
    ['/system-settings/operations/monitoring', 'health'],
    ['/system-settings/request-policies/', 'routing'],
    ['/system-settings/request-policies/unknown', 'routing'],
  ])('%s opens the corresponding policy page', async (path, section) => {
    const router = await renderPolicies(path)
    await waitFor(() =>
      expect(router.state.location.pathname).toBe(
        `/system-settings/request-policies/${section}`
      )
    )
    expect(api.put).not.toHaveBeenCalled()
  })
})

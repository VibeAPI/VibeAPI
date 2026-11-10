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
import { act, cleanup, render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'

import { STATUS_QUERY_KEY, type StatusData } from '@/lib/status-query'

import { ApiKeysPrimaryButtons } from '../api-keys-primary-buttons'
import { ApiKeysProvider } from '../api-keys-provider'

let client: QueryClient

beforeEach(() => {
  localStorage.clear()
  client = new QueryClient({
    defaultOptions: { queries: { enabled: false, retry: false } },
  })
})

afterEach(() => {
  cleanup()
  client.clear()
  localStorage.clear()
})

function renderAddresses(status: StatusData) {
  client.setQueryData(STATUS_QUERY_KEY, status)
  return render(
    <QueryClientProvider client={client}>
      <ApiKeysProvider>
        <ApiKeysPrimaryButtons />
      </ApiKeysProvider>
    </QueryClientProvider>
  )
}

const twoAddresses: StatusData = {
  server_address: 'https://console.example.com',
  api_info_enabled: true,
  api_info: [
    {
      route: 'Global',
      description: 'Worldwide access',
      url: 'https://api.example.com/v1',
      color: 'blue',
    },
    {
      route: 'Asia',
      description: 'Regional access',
      url: 'https://asia.example.com/gateway/v1/',
      color: 'green',
    },
  ],
}

it('shows the first configured address inline and copies it without opening a panel', async () => {
  const user = userEvent.setup()
  const writeText = vi.spyOn(navigator.clipboard, 'writeText')
  renderAddresses(twoAddresses)

  const inline = screen.getByRole('group', { name: 'API site URL' })
  expect(within(inline).getByText('Global')).toBeVisible()
  expect(within(inline).getByText('https://api.example.com/v1')).toBeVisible()
  expect(screen.queryByRole('dialog')).not.toBeInTheDocument()

  await user.click(
    within(inline).getByRole('button', {
      name: 'Copy API URL: https://api.example.com/v1',
    })
  )
  expect(writeText).toHaveBeenCalledWith('https://api.example.com/v1')
})

it('lists every configured address in the panel and copies only the chosen URL', async () => {
  const user = userEvent.setup()
  const writeText = vi.spyOn(navigator.clipboard, 'writeText')
  renderAddresses(twoAddresses)

  await user.click(screen.getByRole('button', { name: 'API Addresses' }))
  const dialog = await screen.findByRole('dialog', { name: 'API Addresses' })
  const rows = within(dialog).getAllByRole('listitem')
  expect(rows).toHaveLength(2)
  expect(rows[0]).toHaveTextContent('Global')
  expect(rows[0]).toHaveTextContent('Worldwide access')
  expect(rows[1]).toHaveTextContent('Asia')
  expect(rows[1]).toHaveTextContent('Regional access')

  await user.click(
    within(rows[1]).getByRole('button', {
      name: 'Copy API URL: https://asia.example.com/gateway/v1/',
    })
  )
  expect(writeText).toHaveBeenCalledWith('https://asia.example.com/gateway/v1/')
  expect(
    await within(rows[1]).findByRole('button', { name: 'Copied' })
  ).toBeVisible()
})

it.each([
  {
    status: {
      api_info: [],
      server_address: 'https://gateway.example.com/proxy/',
    },
    url: 'https://gateway.example.com/proxy/',
  },
  {
    status: { api_info: [] },
    url: window.location.origin,
  },
  {
    status: {
      api_info_enabled: false,
      api_info: [
        {
          route: 'Hidden',
          description: 'Disabled address',
          url: 'https://hidden.example.com',
          color: 'blue',
        },
      ],
      server_address: 'https://gateway.example.com',
    },
    url: 'https://gateway.example.com',
  },
])(
  'shows the fallback $url inline without an address panel when nothing is configured',
  async ({ status, url }) => {
    const user = userEvent.setup()
    const writeText = vi.spyOn(navigator.clipboard, 'writeText')
    renderAddresses(status)

    const inline = screen.getByRole('group', { name: 'API site URL' })
    expect(within(inline).getByText('Default')).toBeVisible()
    expect(within(inline).getByText(url)).toBeVisible()
    expect(
      screen.queryByRole('button', { name: 'API Addresses' })
    ).not.toBeInTheDocument()

    await user.tab()
    const copy = within(inline).getByRole('button', {
      name: `Copy API URL: ${url}`,
    })
    expect(copy).toHaveFocus()
    await user.keyboard('{Enter}')
    expect(writeText).toHaveBeenCalledWith(url)
  }
)

it('truncates a long inline address, keeps its full value as a title, and updates when configuration changes', async () => {
  const url =
    'https://regional-api-gateway.example.com/organization/production/openai-compatible/v1/'
  renderAddresses({
    api_info: [
      {
        route: 'Regional gateway',
        description: 'Regional production gateway',
        url,
        color: 'blue',
      },
    ],
  })
  const inline = screen.getByRole('group', { name: 'API site URL' })
  const code = within(inline).getByText(url)
  expect(code).toHaveClass('truncate', 'min-w-0')
  expect(code).toHaveAttribute('title', url)

  act(() => {
    client.setQueryData(STATUS_QUERY_KEY, {
      api_info: [
        {
          route: 'Replacement',
          description: 'Updated gateway',
          url: 'https://new.example.com',
          color: 'blue',
        },
      ],
    })
  })
  expect(
    await within(inline).findByText('https://new.example.com')
  ).toBeVisible()
  expect(within(inline).queryByText(url)).not.toBeInTheDocument()
})

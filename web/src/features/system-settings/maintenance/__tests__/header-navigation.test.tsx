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
  createRouter,
  RouterProvider,
} from '@tanstack/react-router'
import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, test, vi } from 'vitest'

import { TopNav } from '@/components/layout/components/top-nav'
import { DirectionProvider } from '@/context/direction-provider'
import { useTopNavLinks } from '@/hooks/use-top-nav-links'
import { api } from '@/lib/api'
import { parseHeaderNavModules as parsePublicNav } from '@/lib/nav-modules'

import { SettingsPageProvider } from '../../components/settings-page-context'
import { parseHeaderNavModules, serializeHeaderNavModules } from '../config'
import { HeaderNavigationSection } from '../header-navigation-section'

const clients: QueryClient[] = []
const hosts: HTMLElement[] = []
afterEach(() => {
  clients.splice(0).forEach((client) => client.clear())
  hosts.splice(0).forEach((host) => host.remove())
  localStorage.clear()
})

function TestNavigation() {
  const links = useTopNavLinks()
  return <TopNav links={links} />
}

function renderSettings(raw = '') {
  const config = parseHeaderNavModules(raw)
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })
  clients.push(client)
  client.setQueryData(['status'], { HeaderNavModules: raw })
  const container = document.createElement('div')
  const actions = document.createElement('div')
  hosts.push(actions)
  document.body.append(container, actions)
  const router = createRouter({
    history: createMemoryHistory({ initialEntries: ['/'] }),
    routeTree: createRootRoute({
      component: () => (
        <DirectionProvider>
          <QueryClientProvider client={client}>
            <SettingsPageProvider actionsContainer={actions}>
              <HeaderNavigationSection
                config={config}
                initialSerialized={serializeHeaderNavModules(config)}
              />
            </SettingsPageProvider>
            <TestNavigation />
          </QueryClientProvider>
        </DirectionProvider>
      ),
    }),
  })
  render(<RouterProvider router={router} />, { container })
  return client
}

describe('header sponsor settings', () => {
  test('old settings expose three empty disabled slots and preserve navigation on save', async () => {
    const user = userEvent.setup()
    const raw = JSON.stringify({
      home: false,
      pricing: { enabled: true, requireAuth: true },
    })
    renderSettings(raw)
    await screen.findByRole('group', { name: 'Navigation link 1' })
    expect(screen.getAllByRole('textbox', { name: 'Link title' })).toHaveLength(
      3
    )
    const slot = within(
      await screen.findByRole('group', { name: 'Navigation link 1' })
    )
    expect(slot.getByRole('switch', { name: 'Show' })).not.toBeChecked()
    fireEvent.change(slot.getByRole('textbox', { name: 'Link title' }), {
      target: { value: 'Partner' },
    })
    fireEvent.change(slot.getByRole('textbox', { name: 'Site URL' }), {
      target: { value: 'https://docs.example.com' },
    })
    expect(
      screen.queryByRole('textbox', { name: 'Icon URL' })
    ).not.toBeInTheDocument()
    await user.click(slot.getByRole('switch', { name: 'Show' }))
    const savedConfig = {
      ...parseHeaderNavModules(raw),
      sponsors: [
        {
          enabled: true,
          name: 'Partner',
          url: 'https://docs.example.com',
        },
        { enabled: false, name: '', url: '' },
        { enabled: false, name: '', url: '' },
      ],
    }
    const serialized = JSON.stringify(savedConfig)
    const put = vi
      .spyOn(api, 'put')
      .mockResolvedValue({ data: { success: true } })
    vi.spyOn(api, 'get').mockResolvedValue({
      data: { data: { HeaderNavModules: serialized } },
    })
    await user.click(screen.getByRole('button', { name: 'Save navigation' }))
    await waitFor(() =>
      expect(put).toHaveBeenCalledWith('/api/option/', {
        key: 'HeaderNavModules',
        value: serialized,
      })
    )
    expect(
      await screen.findByRole('link', { name: 'Partner' })
    ).toHaveAttribute('href', 'https://docs.example.com')
    expect(parsePublicNav(serialized).sponsors).toEqual(savedConfig.sponsors)
    expect(parseHeaderNavModules(serialized).sponsors).toEqual(
      savedConfig.sponsors
    )
  })

  test('enabling a slot without required fields blocks saving and identifies invalid controls', async () => {
    const user = userEvent.setup()
    renderSettings()
    const put = vi.spyOn(api, 'put')
    const slot = within(
      await screen.findByRole('group', { name: 'Navigation link 1' })
    )
    await user.click(slot.getByRole('switch', { name: 'Show' }))
    await user.click(screen.getByRole('button', { name: 'Save navigation' }))
    expect(
      await slot.findByText('Link title is required when enabled')
    ).toBeInTheDocument()
    expect(slot.getByRole('textbox', { name: 'Link title' })).toHaveAttribute(
      'aria-invalid',
      'true'
    )
    expect(slot.getByRole('textbox', { name: 'Site URL' })).toHaveAttribute(
      'aria-invalid',
      'true'
    )
    expect(put).not.toHaveBeenCalled()
  })

  test('unsafe URLs are rejected even on a disabled slot', async () => {
    const user = userEvent.setup()
    renderSettings()
    const put = vi.spyOn(api, 'put')
    const slot = within(
      await screen.findByRole('group', { name: 'Navigation link 2' })
    )
    fireEvent.change(slot.getByRole('textbox', { name: 'Site URL' }), {
      target: { value: 'javascript:alert(1)' },
    })
    await user.click(screen.getByRole('button', { name: 'Save navigation' }))
    await waitFor(() =>
      expect(slot.getAllByText('Enter a valid HTTP or HTTPS URL')).toHaveLength(
        1
      )
    )
    expect(put).not.toHaveBeenCalled()
  })

  test('turning off a site keeps its configuration and reset clears all three slots', async () => {
    const user = userEvent.setup()
    renderSettings(
      JSON.stringify({
        sponsors: [
          {
            enabled: true,
            name: 'Partner',
            url: 'https://docs.example.com',
          },
        ],
      })
    )
    const slot = within(
      await screen.findByRole('group', { name: 'Navigation link 1' })
    )
    await user.click(slot.getByRole('switch', { name: 'Show' }))
    expect(slot.getByRole('textbox', { name: 'Link title' })).toHaveValue(
      'Partner'
    )
    const put = vi
      .spyOn(api, 'put')
      .mockResolvedValue({ data: { success: true } })
    vi.spyOn(api, 'get').mockResolvedValue({
      data: { data: { HeaderNavModules: '{}' } },
    })
    await user.click(screen.getByRole('button', { name: 'Save navigation' }))
    await waitFor(() => expect(put).toHaveBeenCalled())
    const payload = put.mock.calls[0][1] as { value: string }
    expect(parseHeaderNavModules(payload.value).sponsors[0]).toEqual({
      enabled: false,
      name: 'Partner',
      url: 'https://docs.example.com',
    })
    await waitFor(() =>
      expect(
        screen.queryByRole('link', { name: 'Partner' })
      ).not.toBeInTheDocument()
    )
    await user.click(screen.getByRole('button', { name: 'Reset to default' }))
    for (const input of screen.getAllByRole('textbox')) {
      expect(input).toHaveValue('')
    }
    for (const toggle of screen.getAllByRole('switch', { name: 'Show' })) {
      expect(toggle).not.toBeChecked()
    }
  })
})

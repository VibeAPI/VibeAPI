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
import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import i18next from 'i18next'
import { afterEach, describe, expect, test, vi } from 'vitest'

import { SidebarProvider } from '@/components/ui/sidebar'
import { DirectionProvider } from '@/context/direction-provider'
import { useSystemConfigStore } from '@/stores/system-config-store'

import { AppHeader } from '../app-header'
import { PublicHeader } from '../public-header'

// The unrelated provider icon package imports JSON unsupported by Vitest's ESM loader.
vi.mock('@lobehub/icons', () => ({ toc: [], CherryStudio: () => null }))

const clients: QueryClient[] = []
const initialSystemConfig = useSystemConfigStore.getState()
afterEach(async () => {
  clients.splice(0).forEach((client) => client.clear())
  useSystemConfigStore.setState(initialSystemConfig)
  localStorage.clear()
  await i18next.changeLanguage('en')
  i18next.removeResourceBundle('zhCN', 'translation')
})

function renderHeader(layout: string, sponsors?: unknown) {
  const client = new QueryClient()
  clients.push(client)
  client.setQueryData(['status'], {
    system_name: 'VibeAPI',
    HeaderNavModules: JSON.stringify({ sponsors }),
  })
  client.setQueryData(['notice'], { success: true, data: '' })
  useSystemConfigStore.getState().setConfig({ systemName: 'VibeAPI' })
  useSystemConfigStore.getState().setLoading(false)
  const router = createRouter({
    history: createMemoryHistory({ initialEntries: ['/'] }),
    routeTree: createRootRoute({
      component: () => (
        <QueryClientProvider client={client}>
          <DirectionProvider>
            {layout === 'public' ? (
              <PublicHeader
                showNotifications={false}
                showThemeSwitch={false}
                showAuthButtons={false}
                showLanguageSwitcher={false}
              />
            ) : (
              <SidebarProvider>
                <AppHeader
                  showSearch={false}
                  showNotifications={false}
                  showConfigDrawer={false}
                  showProfileDropdown={false}
                />
              </SidebarProvider>
            )}
          </DirectionProvider>
        </QueryClientProvider>
      ),
    }),
  })
  render(<RouterProvider router={router} />)
}

describe('custom header navigation', () => {
  test.each(['public', 'console'])(
    '%s header appends three text links after About in the same navigation',
    async (layout) => {
      const longTitle =
        'A long custom navigation title that should be truncated'
      renderHeader(layout, [
        {
          enabled: true,
          name: 'Partner',
          url: 'https://partner.example.com',
          icon: 'https://example.com/old-icon.png',
        },
        { enabled: true, name: longTitle, url: 'https://tools.example.com' },
        {
          enabled: true,
          name: 'Community',
          url: 'https://community.example.com',
        },
        { enabled: true, name: 'Fourth', url: 'https://fourth.example.com' },
      ])
      const about = (await screen.findAllByRole('link', { name: 'About' }))[0]
      const region = about.parentElement
      expect(region).not.toContainElement(screen.getByText('VibeAPI'))
      expect(region).toHaveClass('overflow-x-auto')
      if (!region) throw new Error('Navigation region is missing')
      const links = within(region).getAllByRole('link')
      expect(links.slice(-4).map((link) => link.textContent)).toEqual([
        'About',
        'Partner',
        longTitle,
        'Community',
      ])
      for (const link of links.slice(-3)) {
        expect(link).toHaveAttribute('target', '_blank')
        expect(link).toHaveAttribute('rel', 'noopener noreferrer')
        expect(link.querySelector('img, svg')).toBeNull()
      }
      expect(within(region).getByRole('link', { name: longTitle })).toHaveClass(
        'truncate',
        'max-w-40'
      )
      expect(
        screen.queryByRole('link', { name: 'Fourth' })
      ).not.toBeInTheDocument()
    }
  )

  test('custom titles remain literal when they match a translation key', async () => {
    i18next.addResourceBundle('zhCN', 'translation', {
      About: '关于',
      Home: '主页',
    })
    await i18next.changeLanguage('zhCN')
    renderHeader('public', [
      { enabled: true, name: 'Home', url: 'https://partner.example.com' },
    ])
    expect(
      (await screen.findAllByRole('link', { name: 'Home' }))[0]
    ).toHaveAttribute('href', 'https://partner.example.com')
    expect(screen.getAllByRole('link', { name: '关于' })).toHaveLength(2)
  })

  test('mobile navigation exposes custom links with the same external destination', async () => {
    const user = userEvent.setup()
    renderHeader('public', [
      { enabled: true, name: 'Partner', url: 'https://partner.example.com' },
    ])
    await user.click(
      await screen.findByRole('button', { name: 'Toggle navigation menu' })
    )
    const links = screen.getAllByRole('link', { name: 'Partner' })
    expect(links).toHaveLength(2)
    expect(links[1]).toHaveAttribute('href', 'https://partner.example.com')
    expect(links[1]).toHaveAttribute('target', '_blank')
    links[1].focus()
    expect(links[1]).toHaveFocus()
  })

  test('disabled, incomplete and unsafe custom links are omitted', async () => {
    renderHeader('public', [
      { enabled: false, name: 'Hidden', url: 'https://hidden.example.com' },
      { enabled: true, name: 'Unsafe', url: 'javascript:alert(1)' },
      { enabled: true, name: '  ', url: 'https://example.com' },
    ])
    await screen.findAllByRole('link', { name: 'About' })
    expect(
      screen.queryByRole('link', { name: 'Hidden' })
    ).not.toBeInTheDocument()
    expect(
      screen.queryByRole('link', { name: 'Unsafe' })
    ).not.toBeInTheDocument()
    expect(
      screen
        .getAllByRole('link')
        .filter((link) => link.getAttribute('target') === '_blank')
    ).toHaveLength(0)
  })
})

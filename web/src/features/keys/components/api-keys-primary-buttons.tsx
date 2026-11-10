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
import { ChevronDown, Globe, Plus } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { CopyButton } from '@/components/copy-button'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Item,
  ItemActions,
  ItemContent,
  ItemDescription,
  ItemGroup,
  ItemTitle,
} from '@/components/ui/item'
import {
  Popover,
  PopoverContent,
  PopoverTitle,
  PopoverTrigger,
} from '@/components/ui/popover'
import { Skeleton } from '@/components/ui/skeleton'
import { useApiInfo } from '@/features/dashboard/hooks/use-status-data'
import { useStatus } from '@/hooks/use-status'

import { useApiKeys } from './api-keys-provider'

export function ApiKeysPrimaryButtons() {
  const { t } = useTranslation()
  const { setOpen } = useApiKeys()
  const { status, loading } = useStatus()
  const { items } = useApiInfo()
  const serverAddress =
    (typeof status?.server_address === 'string' &&
      status.server_address.trim()) ||
    ''
  const addresses = items.length
    ? items
    : [
        {
          url: serverAddress || window.location.origin,
          route: t('Default'),
          description: serverAddress
            ? t('Default API address')
            : t('Current domain'),
        },
      ]
  const primary = addresses[0]

  return (
    <div className='flex max-w-full min-w-0 flex-wrap items-center gap-2'>
      {loading ? (
        <Skeleton className='h-8 w-64 max-w-full' />
      ) : (
        <div
          role='group'
          aria-label={t('API site URL')}
          className='bg-muted/40 flex h-8 max-w-full min-w-0 items-center gap-2 rounded-md border pr-0.5 pl-2.5'
        >
          <span className='text-muted-foreground shrink-0 text-xs font-medium max-sm:hidden'>
            {t('API site URL')}
          </span>
          <Badge variant='secondary' className='shrink-0'>
            {primary.route}
          </Badge>
          <code
            className='max-w-40 min-w-0 truncate text-xs select-text sm:max-w-80'
            title={primary.url}
          >
            {primary.url}
          </code>
          <CopyButton
            value={primary.url}
            size='icon'
            className='size-7'
            tooltip={t('Copy API URL')}
            aria-label={`${t('Copy API URL')}: ${primary.url}`}
          />
        </div>
      )}
      {!loading && addresses.length > 1 && (
        <Popover>
          <PopoverTrigger
            render={
              <Button
                variant='outline'
                size='sm'
                aria-label={t('API Addresses')}
              />
            }
          >
            <Globe aria-hidden='true' />
            {t('All addresses ({{count}})', { count: addresses.length })}
            <ChevronDown aria-hidden='true' />
          </PopoverTrigger>
          <PopoverContent
            align='end'
            className='max-h-[min(28rem,var(--available-height))] w-96 max-w-[calc(100vw-2rem)] overflow-y-auto'
          >
            <PopoverTitle>{t('API Addresses')}</PopoverTitle>
            <ItemGroup>
              {addresses.map((address) => (
                <Item
                  key={address.url}
                  role='listitem'
                  variant='muted'
                  size='xs'
                  className='flex-nowrap items-start'
                >
                  <ItemContent className='min-w-0 gap-1'>
                    <ItemTitle className='line-clamp-none break-all'>
                      {address.route}
                    </ItemTitle>
                    <code className='text-xs break-all select-text'>
                      {address.url}
                    </code>
                    {address.description && (
                      <ItemDescription className='line-clamp-none break-words'>
                        {address.description}
                      </ItemDescription>
                    )}
                  </ItemContent>
                  <ItemActions>
                    <CopyButton
                      value={address.url}
                      size='sm'
                      tooltip={t('Copy API URL')}
                      aria-label={`${t('Copy API URL')}: ${address.url}`}
                    />
                  </ItemActions>
                </Item>
              ))}
            </ItemGroup>
          </PopoverContent>
        </Popover>
      )}
      <Button size='sm' onClick={() => setOpen('create')}>
        <Plus className='h-4 w-4' />
        {t('Create API Key')}
      </Button>
    </div>
  )
}

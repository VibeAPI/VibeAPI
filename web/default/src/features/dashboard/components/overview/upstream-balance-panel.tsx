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
import { useQuery } from '@tanstack/react-query'
import { CircleDollarSign, RotateCw } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { IconBadge } from '@/components/ui/icon-badge'
import { getUpstreamBalances } from '@/features/dashboard/api'
import { formatCurrencyFromUSD } from '@/lib/currency'
import { cn } from '@/lib/utils'

import { PanelWrapper } from '../ui/panel-wrapper'

export function UpstreamBalancePanel(props: {
  refreshIntervalSeconds: number
}) {
  const { t } = useTranslation()
  const balancesQuery = useQuery({
    queryKey: ['dashboard', 'overview', 'upstream-balances'],
    queryFn: getUpstreamBalances,
    refetchInterval: props.refreshIntervalSeconds * 1000,
    refetchIntervalInBackground: false,
    retry: false,
  })
  // React Query retains the last successful data while an error is exposed.
  // Do not keep rendering balances after a permission or upstream failure.
  const accounts = balancesQuery.isError ? [] : balancesQuery.data?.data ?? []

  return (
    <PanelWrapper
      title={
        <span className='flex items-center gap-2'>
          <IconBadge tone='success' size='sm'>
            <CircleDollarSign />
          </IconBadge>
          {t('Upstream account balances')}
        </span>
      }
      description={t(
        'Balances are fetched by the server without exposing upstream tokens'
      )}
      loading={balancesQuery.isLoading}
      empty={!accounts.length}
      emptyMessage={
        balancesQuery.isError
          ? t('Failed to load upstream balances')
          : t('No upstream balance accounts configured')
      }
      height='h-48'
      contentClassName='p-0'
      headerActions={
        <Button
          variant='ghost'
          size='sm'
          className='size-7 p-0'
          onClick={() => void balancesQuery.refetch()}
          disabled={balancesQuery.isFetching}
          aria-label={t('Refresh')}
        >
          <RotateCw
            className={cn(
              'size-3.5',
              balancesQuery.isFetching && 'animate-spin'
            )}
          />
        </Button>
      }
    >
      <div className='divide-y'>
        {accounts.map((account) => {
          let balanceUSD: number | null = null
          if (account.success && typeof account.balance === 'number') {
            balanceUSD = account.balance
          } else if (account.success && account.quota_per_unit) {
            balanceUSD = Number(account.quota ?? 0) / account.quota_per_unit
          }
          return (
            <div
              key={account.id}
              className='hover:bg-muted/30 flex items-center justify-between gap-4 px-4 py-3 transition-colors sm:px-5'
            >
              <div className='min-w-0'>
                <div className='truncate text-sm font-medium'>
                  {account.name}
                </div>
                <div className='text-muted-foreground truncate text-xs'>
                  {account.success ? t('Available') : t('Unavailable')}
                </div>
              </div>
              <div className='shrink-0 text-right'>
                <div
                  className={cn(
                    'font-mono text-base font-semibold tabular-nums',
                    !account.success && 'text-destructive'
                  )}
                >
                  {account.success
                    ? formatCurrencyFromUSD(balanceUSD, { abbreviate: true })
                    : t('Unavailable')}
                </div>
                <div className='text-muted-foreground text-[11px]'>
                  {account.success ? t('Live balance') : t('Request failed')}
                </div>
              </div>
            </div>
          )
        })}
      </div>
    </PanelWrapper>
  )
}

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

import { StaggerItem } from '@/components/page-transition'
import { Button } from '@/components/ui/button'
import { getUpstreamBalances } from '@/features/dashboard/api'
import { formatCurrencyFromUSD } from '@/lib/currency'
import { cn } from '@/lib/utils'

import { StatCard } from '../ui/stat-card'

const upstreamCardClassName =
  'bg-background/60 rounded-lg border px-2 py-1.5 sm:rounded-xl sm:p-3'

export function UpstreamBalanceCards(props: {
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
  const accounts = balancesQuery.isError ? [] : (balancesQuery.data?.data ?? [])

  const refreshAction = (
    <Button
      variant='ghost'
      size='sm'
      className='size-7 p-0'
      onClick={() => void balancesQuery.refetch()}
      disabled={balancesQuery.isFetching}
      aria-label={t('Refresh')}
    >
      <RotateCw
        className={cn('size-3.5', balancesQuery.isFetching && 'animate-spin')}
      />
    </Button>
  )

  if (balancesQuery.isLoading) {
    return (
      <StaggerItem className={upstreamCardClassName}>
        <StatCard
          title={t('System total available quota')}
          value=''
          description={t(
            'Balances are fetched by the server without exposing upstream tokens'
          )}
          icon={CircleDollarSign}
          tone='accent-2'
          iconTone='success'
          loading
          compactMobile
        />
      </StaggerItem>
    )
  }

  if (!accounts.length) {
    return (
      <StaggerItem className={upstreamCardClassName}>
        <StatCard
          title={t('System total available quota')}
          value='--'
          description={
            balancesQuery.isError
              ? t('Failed to load upstream balances')
              : t('No upstream balance accounts configured')
          }
          icon={CircleDollarSign}
          tone='accent-2'
          iconTone='success'
          error
          action={refreshAction}
          compactMobile
        />
      </StaggerItem>
    )
  }

  return (
    <>
      {accounts.map((account, index) => {
        let balanceUSD: number | null = null
        let usedBalanceUSD: number | null = null
        if (account.success && typeof account.balance === 'number') {
          balanceUSD = account.balance
        } else if (account.success && account.quota_per_unit) {
          balanceUSD = Number(account.quota ?? 0) / account.quota_per_unit
        }
        if (
          account.success &&
          typeof account.used_quota === 'number' &&
          account.quota_per_unit
        ) {
          usedBalanceUSD = account.used_quota / account.quota_per_unit
        }

        return (
          <StaggerItem key={account.id} className={upstreamCardClassName}>
            <StatCard
              title={t('System total available quota')}
              value={
                account.success
                  ? formatCurrencyFromUSD(balanceUSD, { abbreviate: true })
                  : '--'
              }
              description={
                usedBalanceUSD !== null
                  ? `${account.name} · ${t('Used:')} ${formatCurrencyFromUSD(usedBalanceUSD, { abbreviate: true })}`
                  : `${account.name} · ${
                      account.success ? t('Live balance') : t('Request failed')
                    }`
              }
              icon={CircleDollarSign}
              tone='accent-2'
              iconTone='success'
              error={!account.success}
              action={index === 0 ? refreshAction : undefined}
              sparkline={account.usage?.map((point) => point.quota)}
              sparklineVariant='line'
              compactMobile
            />
          </StaggerItem>
        )
      })}
    </>
  )
}

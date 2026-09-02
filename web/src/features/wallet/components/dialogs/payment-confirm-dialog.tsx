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
import { Loader2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { CopyButton } from '@/components/copy-button'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Skeleton } from '@/components/ui/skeleton'
import { formatLocalCurrencyAmount } from '@/lib/currency'
import { formatQuota } from '@/lib/format'

import { DEFAULT_DISCOUNT_RATE } from '../../constants'
import { formatCurrency, getPaymentIcon } from '../../lib'
import type { PaymentMethod } from '../../types'

interface PaymentConfirmDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  onConfirm: () => void
  topupAmount: number
  paymentAmount: number
  paymentMethod: PaymentMethod | undefined
  calculating: boolean
  processing: boolean
  discountRate?: number
  usdExchangeRate?: number
  currentBalance?: number
}

export function PaymentConfirmDialog({
  open,
  onOpenChange,
  onConfirm,
  topupAmount,
  paymentAmount,
  paymentMethod,
  calculating,
  processing,
  discountRate = DEFAULT_DISCOUNT_RATE,
  usdExchangeRate = 1,
  currentBalance = 0,
}: PaymentConfirmDialogProps) {
  const { t } = useTranslation()
  const hasDiscount = discountRate > 0 && discountRate < 1 && paymentAmount > 0
  const originalAmount = hasDiscount ? paymentAmount / discountRate : 0
  const discountAmount = hasDiscount ? originalAmount - paymentAmount : 0
  const hasRecipientInfo =
    !!paymentMethod?.recipient_name ||
    !!paymentMethod?.recipient_bank ||
    !!paymentMethod?.recipient_account ||
    !!paymentMethod?.contact_phone ||
    !!paymentMethod?.contact_wechat
  const isCorporate = paymentMethod?.type === 'corporate'
  const paymentMethodName = paymentMethod?.name ? t(paymentMethod.name) : ''
  const paymentInstructions = paymentMethod?.payment_instructions
    ? t(paymentMethod.payment_instructions)
    : ''

  return (
    <AlertDialog open={open} onOpenChange={onOpenChange}>
      <AlertDialogContent className='max-sm:w-[calc(100vw-1.5rem)] sm:max-w-md'>
        <AlertDialogHeader>
          <AlertDialogTitle className='text-xl font-semibold'>
            {isCorporate ? t('Corporate Payment') : t('Confirm Payment')}
          </AlertDialogTitle>
          <AlertDialogDescription>
            {isCorporate
              ? t('Review the bank transfer details before uploading proof')
              : t('Review your payment details')}
          </AlertDialogDescription>
        </AlertDialogHeader>

        <div className='space-y-3 py-3 sm:space-y-4 sm:py-4'>
          {isCorporate && (
            <div className='bg-muted/50 grid grid-cols-[minmax(0,1fr)_auto] items-center gap-x-4 rounded-lg px-3 py-3'>
              <span className='text-muted-foreground text-sm'>
                {t('Current Balance')}
              </span>
              <span className='justify-self-end text-right font-semibold'>
                {formatQuota(currentBalance)}
              </span>
            </div>
          )}
          <div className='grid grid-cols-[minmax(0,1fr)_auto] items-center gap-x-4 px-3'>
            <span className='text-muted-foreground text-sm'>
              {t('Topup Amount')}
            </span>
            <span className='justify-self-end text-right text-lg font-semibold'>
              {formatLocalCurrencyAmount(topupAmount * usdExchangeRate, {
                digitsLarge: 2,
                digitsSmall: 2,
                abbreviate: false,
              })}
            </span>
          </div>

          <div className='grid grid-cols-[minmax(0,1fr)_auto] items-center gap-x-4 px-3'>
            <span className='text-muted-foreground text-sm'>
              {t('You Pay')}
            </span>
            {calculating ? (
              <Skeleton className='h-6 w-24' />
            ) : (
              <div className='flex items-baseline gap-2 justify-self-end text-right'>
                <span className='text-2xl font-semibold'>
                  {formatCurrency(paymentAmount)}
                </span>
                {hasDiscount && (
                  <span className='text-muted-foreground text-sm line-through'>
                    {formatCurrency(originalAmount)}
                  </span>
                )}
              </div>
            )}
          </div>

          {hasDiscount && !calculating && (
            <div className='bg-muted/50 rounded-lg p-3'>
              <div className='flex items-center justify-between text-sm'>
                <span className='text-muted-foreground'>{t('You save')}</span>
                <span className='font-semibold text-green-600'>
                  {formatCurrency(discountAmount)}
                </span>
              </div>
            </div>
          )}

          <div className='border-t pt-4'>
            <div className='grid grid-cols-[minmax(0,1fr)_auto] items-center gap-x-4 px-3'>
              <span className='text-muted-foreground text-sm'>
                {t('Payment Method')}
              </span>
              <div className='flex items-center gap-2 justify-self-end text-right'>
                {getPaymentIcon(
                  paymentMethod?.type,
                  'h-4 w-4',
                  paymentMethod?.icon,
                  paymentMethodName
                )}
                <span className='font-medium'>{paymentMethodName}</span>
              </div>
            </div>
          </div>

          {hasRecipientInfo && (
            <div className='bg-muted/50 space-y-2 rounded-lg px-3 py-3 text-sm'>
              <div className='font-medium'>{t('Recipient information')}</div>
              {paymentMethod?.recipient_name && (
                <div className='grid grid-cols-[auto_minmax(0,1fr)] gap-x-4'>
                  <span className='text-muted-foreground'>
                    {t('Recipient name')}
                  </span>
                  <div className='flex min-w-0 items-center justify-self-end'>
                    <span className='text-right break-words'>
                      {paymentMethod.recipient_name}
                    </span>
                    <CopyButton
                      value={paymentMethod.recipient_name}
                      className='size-7'
                      iconClassName='size-3.5'
                    />
                  </div>
                </div>
              )}
              {paymentMethod?.recipient_bank && (
                <div className='grid grid-cols-[auto_minmax(0,1fr)] gap-x-4'>
                  <span className='text-muted-foreground'>
                    {t('Recipient bank')}
                  </span>
                  <div className='flex min-w-0 items-center justify-self-end'>
                    <span className='text-right break-words'>
                      {paymentMethod.recipient_bank}
                    </span>
                    <CopyButton
                      value={paymentMethod.recipient_bank}
                      className='size-7'
                      iconClassName='size-3.5'
                    />
                  </div>
                </div>
              )}
              {paymentMethod?.recipient_account && (
                <div className='grid grid-cols-[auto_minmax(0,1fr)] gap-x-4'>
                  <span className='text-muted-foreground'>
                    {t('Recipient account')}
                  </span>
                  <div className='flex min-w-0 items-center justify-self-end'>
                    <span className='text-right font-mono break-all'>
                      {paymentMethod.recipient_account}
                    </span>
                    <CopyButton
                      value={paymentMethod.recipient_account}
                      className='size-7'
                      iconClassName='size-3.5'
                    />
                  </div>
                </div>
              )}
              {paymentMethod?.contact_phone && (
                <div className='grid grid-cols-[auto_minmax(0,1fr)] gap-x-4'>
                  <span className='text-muted-foreground'>
                    {t('Contact phone')}
                  </span>
                  <div className='flex min-w-0 items-center justify-self-end'>
                    <span className='text-right break-all'>
                      {paymentMethod.contact_phone}
                    </span>
                    <CopyButton
                      value={paymentMethod.contact_phone}
                      className='size-7'
                      iconClassName='size-3.5'
                    />
                  </div>
                </div>
              )}
              {paymentMethod?.contact_wechat && (
                <div className='grid grid-cols-[auto_minmax(0,1fr)] gap-x-4'>
                  <span className='text-muted-foreground'>
                    {t('Contact WeChat')}
                  </span>
                  <div className='flex min-w-0 items-center justify-self-end'>
                    <span className='text-right break-all'>
                      {paymentMethod.contact_wechat}
                    </span>
                    <CopyButton
                      value={paymentMethod.contact_wechat}
                      className='size-7'
                      iconClassName='size-3.5'
                    />
                  </div>
                </div>
              )}
            </div>
          )}
          {isCorporate && paymentInstructions && (
            <div className='border-primary/20 bg-primary/5 rounded-lg border p-3'>
              <div className='mb-1 text-sm font-medium'>
                {t('Payment instructions')}
              </div>
              <p className='text-muted-foreground text-sm leading-6 whitespace-pre-wrap'>
                {paymentInstructions}
              </p>
            </div>
          )}
        </div>

        <AlertDialogFooter className='grid grid-cols-2 gap-2 sm:flex'>
          <AlertDialogCancel disabled={processing}>
            {t('Cancel')}
          </AlertDialogCancel>
          <AlertDialogAction onClick={onConfirm} disabled={processing}>
            {processing && <Loader2 className='mr-2 h-4 w-4 animate-spin' />}
            {isCorporate ? t('Upload Proof') : t('Confirm Payment')}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  )
}

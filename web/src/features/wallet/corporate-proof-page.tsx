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
import { zodResolver } from '@hookform/resolvers/zod'
import { useNavigate } from '@tanstack/react-router'
import { Building2, CheckCircle2, UploadCloud } from 'lucide-react'
import { useState } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { z } from 'zod'

import { CopyButton } from '@/components/copy-button'
import { SectionPageLayout } from '@/components/layout'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import {
  Form,
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'

import { submitCorporateTopup } from './api'
import { useTopupInfo } from './hooks'

const MAX_PROOF_SIZE = 5 * 1024 * 1024
const ACCEPTED_PROOF_TYPES = new Set([
  'image/jpeg',
  'image/png',
  'application/pdf',
])

function createCorporateProofSchema(t: (key: string) => string) {
  return z.object({
    bankName: z.string().trim().min(1, t('Bank name is required')).max(255),
    bankAccount: z
      .string()
      .trim()
      .min(1, t('Bank account is required'))
      .max(255),
    payerName: z.string().trim().min(1, t('Payer name is required')).max(255),
    transferDate: z.string().min(1, t('Transfer date is required')),
    phone: z.string().trim().min(1, t('Phone number is required')).max(50),
    proof: z
      .instanceof(File, { message: t('Transfer proof is required') })
      .refine(
        (file) => file.size <= MAX_PROOF_SIZE,
        t('File size must not exceed 5MB')
      )
      .refine(
        (file) => ACCEPTED_PROOF_TYPES.has(file.type),
        t('Only JPG, PNG, or PDF files are supported')
      ),
  })
}

type CorporateProofForm = z.infer<ReturnType<typeof createCorporateProofSchema>>

type CorporateProofPageProps = {
  amount: number
}

export function CorporateProofPage(props: CorporateProofPageProps) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { topupInfo, loading } = useTopupInfo()
  const [submittedTradeNo, setSubmittedTradeNo] = useState('')
  const corporateMethod = topupInfo?.pay_methods.find(
    (method) => method.type === 'corporate'
  )
  let accountDetails = null
  if (loading) {
    accountDetails = (
      <p className='text-muted-foreground text-sm'>{t('Loading...')}</p>
    )
  } else if (!corporateMethod) {
    accountDetails = (
      <Alert variant='destructive'>
        <AlertTitle>{t('Corporate payment is unavailable')}</AlertTitle>
      </Alert>
    )
  }
  const schema = createCorporateProofSchema(t)
  const form = useForm<CorporateProofForm>({
    resolver: zodResolver(schema),
    defaultValues: {
      bankName: '',
      bankAccount: '',
      payerName: '',
      transferDate: new Date().toISOString().slice(0, 10),
      phone: '',
    },
  })

  const onSubmit = async (values: CorporateProofForm) => {
    const response = await submitCorporateTopup({
      amount: props.amount,
      bankName: values.bankName,
      bankAccount: values.bankAccount,
      payerName: values.payerName,
      transferDate: values.transferDate,
      phone: values.phone,
      proof: values.proof,
    })
    if (!response.success || !response.data?.trade_no) {
      toast.error(response.message || t('Failed to submit transfer proof'))
      return
    }
    setSubmittedTradeNo(response.data.trade_no)
    toast.success(t('Transfer proof submitted for review'))
  }

  if (submittedTradeNo) {
    return (
      <SectionPageLayout>
        <SectionPageLayout.Title>
          {t('Corporate Payment')}
        </SectionPageLayout.Title>
        <SectionPageLayout.Content>
          <Card className='mx-auto max-w-xl'>
            <CardContent className='flex flex-col items-center py-10 text-center'>
              <CheckCircle2 className='mb-4 size-12 text-emerald-600' />
              <h2 className='text-xl font-semibold'>{t('Proof submitted')}</h2>
              <p className='text-muted-foreground mt-2 text-sm'>
                {t('Your transfer will be reviewed by an administrator.')}
              </p>
              <code className='bg-muted mt-5 rounded-md px-3 py-2 text-sm'>
                {submittedTradeNo}
              </code>
              <Button
                className='mt-6'
                onClick={() => navigate({ to: '/wallet' })}
              >
                {t('Back to Wallet')}
              </Button>
            </CardContent>
          </Card>
        </SectionPageLayout.Content>
      </SectionPageLayout>
    )
  }

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>
        {t('Upload Transfer Proof')}
      </SectionPageLayout.Title>
      <SectionPageLayout.Content>
        <div className='mx-auto grid max-w-5xl gap-5 lg:grid-cols-[0.85fr_1.15fr]'>
          <Card>
            <CardHeader>
              <CardTitle className='flex items-center gap-2'>
                <Building2 className='size-5' />
                {t('Corporate account')}
              </CardTitle>
              <CardDescription>
                {t('Transfer the payment to this account')}
              </CardDescription>
            </CardHeader>
            <CardContent className='space-y-4'>
              {accountDetails ||
                (corporateMethod && (
                  <>
                    <dl className='space-y-3 text-sm'>
                      <div>
                        <dt className='text-muted-foreground'>
                          {t('Recipient name')}
                        </dt>
                        <dd className='mt-1 flex items-center gap-1 font-medium'>
                          <span className='min-w-0 break-words'>
                            {corporateMethod.recipient_name}
                          </span>
                          <CopyButton
                            value={corporateMethod.recipient_name || ''}
                            className='size-7'
                            iconClassName='size-3.5'
                          />
                        </dd>
                      </div>
                      <div>
                        <dt className='text-muted-foreground'>
                          {t('Recipient bank')}
                        </dt>
                        <dd className='mt-1 flex items-center gap-1 font-medium'>
                          <span className='min-w-0 break-words'>
                            {corporateMethod.recipient_bank}
                          </span>
                          <CopyButton
                            value={corporateMethod.recipient_bank || ''}
                            className='size-7'
                            iconClassName='size-3.5'
                          />
                        </dd>
                      </div>
                      <div>
                        <dt className='text-muted-foreground'>
                          {t('Recipient account')}
                        </dt>
                        <dd className='mt-1 flex items-center gap-1 font-mono font-medium'>
                          <span className='min-w-0 break-all'>
                            {corporateMethod.recipient_account}
                          </span>
                          <CopyButton
                            value={corporateMethod.recipient_account || ''}
                            className='size-7'
                            iconClassName='size-3.5'
                          />
                        </dd>
                      </div>
                      <div>
                        <dt className='text-muted-foreground'>
                          {t('Recharge Amount')}
                        </dt>
                        <dd className='mt-1 text-lg font-semibold'>
                          {props.amount}
                        </dd>
                      </div>
                      {corporateMethod.contact_phone && (
                        <div>
                          <dt className='text-muted-foreground'>
                            {t('Contact phone')}
                          </dt>
                          <dd className='mt-1 flex items-center gap-1 font-medium'>
                            <span className='min-w-0 break-all'>
                              {corporateMethod.contact_phone}
                            </span>
                            <CopyButton
                              value={corporateMethod.contact_phone}
                              className='size-7'
                              iconClassName='size-3.5'
                            />
                          </dd>
                        </div>
                      )}
                      {corporateMethod.contact_wechat && (
                        <div>
                          <dt className='text-muted-foreground'>
                            {t('Contact WeChat')}
                          </dt>
                          <dd className='mt-1 flex items-center gap-1 font-medium'>
                            <span className='min-w-0 break-all'>
                              {corporateMethod.contact_wechat}
                            </span>
                            <CopyButton
                              value={corporateMethod.contact_wechat}
                              className='size-7'
                              iconClassName='size-3.5'
                            />
                          </dd>
                        </div>
                      )}
                    </dl>
                    {corporateMethod.payment_instructions && (
                      <Alert>
                        <AlertTitle>{t('Payment instructions')}</AlertTitle>
                        <AlertDescription className='whitespace-pre-wrap'>
                          {corporateMethod.payment_instructions}
                        </AlertDescription>
                      </Alert>
                    )}
                  </>
                ))}
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle>{t('Transfer details')}</CardTitle>
              <CardDescription>
                {t(
                  'Complete the transfer first, then submit the information below.'
                )}
              </CardDescription>
            </CardHeader>
            <CardContent>
              <Form {...form}>
                <form
                  onSubmit={form.handleSubmit(onSubmit)}
                  className='space-y-4'
                >
                  <div className='grid gap-4 sm:grid-cols-2'>
                    <FormField
                      control={form.control}
                      name='bankName'
                      render={({ field }) => (
                        <FormItem>
                          <FormLabel>{t('Remitting bank')}</FormLabel>
                          <FormControl>
                            <Input {...field} />
                          </FormControl>
                          <FormMessage />
                        </FormItem>
                      )}
                    />
                    <FormField
                      control={form.control}
                      name='bankAccount'
                      render={({ field }) => (
                        <FormItem>
                          <FormLabel>{t('Remitting bank account')}</FormLabel>
                          <FormControl>
                            <Input {...field} />
                          </FormControl>
                          <FormMessage />
                        </FormItem>
                      )}
                    />
                    <FormField
                      control={form.control}
                      name='payerName'
                      render={({ field }) => (
                        <FormItem>
                          <FormLabel>{t('Payer name')}</FormLabel>
                          <FormControl>
                            <Input {...field} />
                          </FormControl>
                          <FormMessage />
                        </FormItem>
                      )}
                    />
                    <FormField
                      control={form.control}
                      name='transferDate'
                      render={({ field }) => (
                        <FormItem>
                          <FormLabel>{t('Transfer date')}</FormLabel>
                          <FormControl>
                            <Input type='date' {...field} />
                          </FormControl>
                          <FormMessage />
                        </FormItem>
                      )}
                    />
                    <FormField
                      control={form.control}
                      name='phone'
                      render={({ field }) => (
                        <FormItem className='sm:col-span-2'>
                          <FormLabel>{t('Contact phone')}</FormLabel>
                          <FormControl>
                            <Input type='tel' {...field} />
                          </FormControl>
                          <FormMessage />
                        </FormItem>
                      )}
                    />
                  </div>
                  <FormField
                    control={form.control}
                    name='proof'
                    render={({ field: _field }) => (
                      <FormItem>
                        <FormLabel>{t('Transfer proof')}</FormLabel>
                        <FormControl>
                          <label className='hover:bg-muted/40 flex cursor-pointer flex-col items-center rounded-xl border border-dashed p-8 text-center transition-colors'>
                            <UploadCloud className='text-muted-foreground mb-3 size-9' />
                            <span className='text-sm font-medium'>
                              {form.watch('proof')?.name ||
                                t('Choose a receipt file')}
                            </span>
                            <span className='text-muted-foreground mt-1 text-xs'>
                              {t('JPG, PNG, or PDF, up to 5MB')}
                            </span>
                            <input
                              className='sr-only'
                              type='file'
                              accept='image/jpeg,image/png,application/pdf'
                              onChange={(event) =>
                                form.setValue(
                                  'proof',
                                  event.target.files?.[0] as File,
                                  { shouldValidate: true }
                                )
                              }
                            />
                          </label>
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                  <div className='flex justify-end gap-2 pt-2'>
                    <Button
                      type='button'
                      variant='outline'
                      onClick={() => navigate({ to: '/wallet' })}
                    >
                      {t('Cancel')}
                    </Button>
                    <Button
                      type='submit'
                      disabled={!corporateMethod || form.formState.isSubmitting}
                    >
                      {form.formState.isSubmitting
                        ? t('Submitting...')
                        : t('Submit for Review')}
                    </Button>
                  </div>
                </form>
              </Form>
            </CardContent>
          </Card>
        </div>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}

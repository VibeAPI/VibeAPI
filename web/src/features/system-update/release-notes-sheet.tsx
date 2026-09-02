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
import { ExternalLink, RefreshCw } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import {
  sideDrawerContentClassName,
  sideDrawerFooterClassName,
  sideDrawerHeaderClassName,
} from '@/components/drawer-layout'
import { ErrorState } from '@/components/error-state'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Markdown } from '@/components/ui/markdown'
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import { Skeleton } from '@/components/ui/skeleton'
import { formatTimestampToDate } from '@/lib/format'

import { useSystemUpdateCatalog } from './queries'

type ReleaseNotesSheetProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  selectedReleaseId?: string | null
  onManageVersions?: () => void
}

export function ReleaseNotesSheet(props: ReleaseNotesSheetProps) {
  const { t } = useTranslation()
  const { releasesQuery } = useSystemUpdateCatalog(props.open)
  const releases = releasesQuery.data?.releases ?? []
  const releasesToShow = props.selectedReleaseId
    ? releases.filter((release) => release.id === props.selectedReleaseId)
    : releases
  let content: React.ReactNode
  if (releasesQuery.isLoading) {
    content = (
      <div className='space-y-6'>
        {['first', 'second', 'third'].map((key) => (
          <div key={key} className='space-y-3'>
            <Skeleton className='h-6 w-40' />
            <Skeleton className='h-20 w-full' />
          </div>
        ))}
      </div>
    )
  } else if (releasesQuery.isError) {
    content = (
      <ErrorState
        title={t('We could not load version history.')}
        description={
          releasesQuery.error instanceof Error
            ? releasesQuery.error.message
            : undefined
        }
        onRetry={() => void releasesQuery.refetch()}
        className='min-h-[320px]'
      />
    )
  } else if (releasesToShow.length === 0) {
    content = (
      <div className='text-muted-foreground flex min-h-[320px] items-center justify-center text-center text-sm'>
        {t('No version history is available yet.')}
      </div>
    )
  } else {
    content = (
      <div className='divide-border divide-y'>
        {releasesToShow.map((release) => (
          <article key={release.id} className='py-5 first:pt-0 last:pb-0'>
            <div className='mb-3 flex flex-wrap items-center gap-2'>
              <h3 className='font-mono text-base font-semibold'>
                {release.version}
              </h3>
              {release.current && (
                <Badge
                  variant='secondary'
                  className='bg-emerald-50 text-emerald-700 dark:bg-emerald-500/15 dark:text-emerald-300'
                >
                  {t('Current')}
                </Badge>
              )}
              {!release.available && (
                <Badge variant='destructive'>{t('Unavailable')}</Badge>
              )}
              {release.published_at && (
                <span className='text-muted-foreground text-xs'>
                  {formatTimestampToDate(
                    new Date(release.published_at).getTime(),
                    'milliseconds'
                  )}
                </span>
              )}
            </div>
            {release.release_notes ? (
              <Markdown className='text-sm'>{release.release_notes}</Markdown>
            ) : (
              <p className='text-muted-foreground text-sm'>
                {t('No release notes provided.')}
              </p>
            )}
            {release.release_url && (
              <Button
                variant='link'
                size='sm'
                className='mt-2 px-0'
                render={
                  <a
                    href={release.release_url}
                    target='_blank'
                    rel='noopener noreferrer'
                  />
                }
              >
                <ExternalLink data-icon='inline-start' aria-hidden='true' />
                {t('Open release')}
              </Button>
            )}
          </article>
        ))}
      </div>
    )
  }

  return (
    <Sheet open={props.open} onOpenChange={props.onOpenChange}>
      <SheetContent
        className={sideDrawerContentClassName('w-full sm:max-w-2xl')}
        aria-busy={releasesQuery.isFetching}
      >
        <SheetHeader className={sideDrawerHeaderClassName()}>
          <SheetTitle>{t('Version history')}</SheetTitle>
          <SheetDescription>
            {t('Review release notes for available Docker versions.')}
          </SheetDescription>
        </SheetHeader>

        <div className='min-h-0 flex-1 overflow-y-auto overscroll-contain px-4 py-4 sm:px-6'>
          {content}
        </div>

        <SheetFooter className={sideDrawerFooterClassName()}>
          <Button
            type='button'
            variant='outline'
            onClick={() => void releasesQuery.refetch()}
            disabled={releasesQuery.isFetching}
          >
            <RefreshCw
              data-icon='inline-start'
              className={releasesQuery.isFetching ? 'animate-spin' : undefined}
              aria-hidden='true'
            />
            {releasesQuery.isFetching ? t('Refreshing...') : t('Refresh')}
          </Button>
          {props.onManageVersions && (
            <Button type='button' onClick={props.onManageVersions}>
              {t('Manage versions')}
            </Button>
          )}
        </SheetFooter>
      </SheetContent>
    </Sheet>
  )
}

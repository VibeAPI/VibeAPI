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
import { useMutation, useQueryClient } from '@tanstack/react-query'
import type { TFunction } from 'i18next'
import {
  AlertCircle,
  CheckCircle2,
  ChevronRight,
  History,
  LoaderCircle,
  RefreshCw,
} from 'lucide-react'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { ErrorState } from '@/components/error-state'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from '@/components/ui/empty'
import { Label } from '@/components/ui/label'
import { Markdown } from '@/components/ui/markdown'
import { Progress } from '@/components/ui/progress'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'
import { useStatus } from '@/hooks/use-status'
import { formatTimestampToDate } from '@/lib/format'
import { ROLE } from '@/lib/roles'
import { cn } from '@/lib/utils'
import { useAuthStore } from '@/stores/auth-store'

import { preflightSystemUpdate, startSystemUpdate } from './api'
import {
  systemUpdateQueryKeys,
  useSystemUpdateCatalog,
  useSystemUpdateOperation,
} from './queries'
import { ReleaseNotesSheet } from './release-notes-sheet'
import {
  isActiveSystemUpdateOperation,
  type SystemUpdateAction,
  type SystemUpdateOperation,
  type SystemUpdatePreflight,
} from './types'
import { compareVersions, normalizeVersion } from './version'

const PHASE_PROGRESS: Record<string, number> = {
  validating: 10,
  pulling: 35,
  switching: 60,
  restarting: 72,
  health_check: 90,
  completed: 100,
}

function resolveAction(
  currentVersion: string,
  targetVersion: string
): SystemUpdateAction {
  const comparison = compareVersions(targetVersion, currentVersion)
  if (comparison > 0) return 'upgrade'
  if (comparison < 0) return 'rollback'
  return 'reinstall'
}

function progressForOperation(operation: SystemUpdateOperation): number {
  if (operation.status === 'succeeded') return 100
  if (operation.status === 'failed' || operation.status === 'rolled_back') {
    return 100
  }
  if (typeof operation.progress === 'number') {
    return Math.min(100, Math.max(0, operation.progress))
  }
  return PHASE_PROGRESS[operation.phase ?? ''] ?? 5
}

function buildIdempotencyKey(releaseId: string): string {
  if (typeof crypto !== 'undefined' && 'randomUUID' in crypto) {
    return crypto.randomUUID()
  }
  return `${releaseId}-${Date.now()}-${Math.random().toString(16).slice(2)}`
}

function getOperationStatusLabel(
  t: TFunction,
  status: SystemUpdateOperation['status']
): string {
  switch (status) {
    case 'pending':
      return t('Pending')
    case 'running':
      return t('Running')
    case 'succeeded':
      return t('Succeeded')
    case 'failed':
      return t('Failed')
    case 'rolled_back':
      return t('Rolled back')
  }
}

function getOperationPhaseLabel(t: TFunction, phase?: string): string {
  switch (phase) {
    case 'validating':
      return t('Validating')
    case 'pulling':
      return t('Pulling image')
    case 'switching':
      return t('Switching container')
    case 'restarting':
      return t('Restarting service')
    case 'health_check':
      return t('Checking service health')
    case 'completed':
      return t('Completed')
    default:
      return phase || t('Waiting')
  }
}

type VersionManagerProps = {
  currentVersion?: string | null
  startTime?: number | null
}

export function VersionManager(props: VersionManagerProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const user = useAuthStore((state) => state.auth.user)
  const isRoot = user?.role === ROLE.SUPER_ADMIN
  const { status, error: statusError } = useStatus()
  const { capabilitiesQuery, releasesQuery, operationsQuery, activeOperation } =
    useSystemUpdateCatalog(true)
  const [selectedReleaseId, setSelectedReleaseId] = useState<string | null>(
    null
  )
  const [notesOpen, setNotesOpen] = useState(false)
  const [confirmOpen, setConfirmOpen] = useState(false)
  const [preflight, setPreflight] = useState<SystemUpdatePreflight | null>(null)
  const [idempotencyKey, setIdempotencyKey] = useState<string | null>(null)
  const [operationId, setOperationId] = useState<string | null>(
    activeOperation?.id ?? null
  )
  const operationQuery = useSystemUpdateOperation(operationId)
  const operation = operationQuery.data ?? activeOperation
  const releases = releasesQuery.data?.releases ?? []
  const capability = capabilitiesQuery.data
  const currentVersion =
    String(
      status?.version ||
        capability?.current_version ||
        props.currentVersion ||
        ''
    ) || t('Unknown')
  const currentRelease = releases.find((release) => release.current)
  const defaultReleaseId =
    releases.find((release) => release.available && !release.current)?.id ??
    currentRelease?.id ??
    releases[0]?.id
  const selectedRelease = releases.find(
    (release) => release.id === selectedReleaseId
  )
  const action = selectedRelease
    ? resolveAction(currentVersion, selectedRelease.version)
    : null
  let actionLabel = t('Reinstall {{version}}', {
    version: selectedRelease?.version,
  })
  if (action === 'upgrade') {
    actionLabel = t('Upgrade to {{version}}', {
      version: selectedRelease?.version,
    })
  } else if (action === 'rollback') {
    actionLabel = t('Roll back to {{version}}', {
      version: selectedRelease?.version,
    })
  }
  const updateAvailable = releases.some(
    (release) =>
      release.available && compareVersions(release.version, currentVersion) > 0
  )
  const updaterReady = Boolean(
    capability?.updater_configured && capability.updater_reachable
  )
  const active = isActiveSystemUpdateOperation(operation)
  const operationProgress = operation ? progressForOperation(operation) : 0
  const catalogLoading = capabilitiesQuery.isLoading || releasesQuery.isLoading
  const catalogError = capabilitiesQuery.isError || releasesQuery.isError

  useEffect(() => {
    if (!selectedReleaseId && defaultReleaseId) {
      setSelectedReleaseId(defaultReleaseId)
    }
  }, [defaultReleaseId, selectedReleaseId])

  useEffect(() => {
    if (!operationId && activeOperation?.id) setOperationId(activeOperation.id)
  }, [activeOperation?.id, operationId])

  useEffect(() => {
    if (!operation || isActiveSystemUpdateOperation(operation)) return
    if (operation.status === 'succeeded') {
      void queryClient.invalidateQueries({ queryKey: ['status'] })
      void queryClient.invalidateQueries({
        queryKey: systemUpdateQueryKeys.capabilities,
      })
      void queryClient.invalidateQueries({
        queryKey: systemUpdateQueryKeys.releases,
      })
    }
  }, [operation, queryClient])

  const preflightMutation = useMutation({
    mutationFn: preflightSystemUpdate,
    onSuccess: (result) => {
      setPreflight(result)
      setConfirmOpen(true)
    },
    onError: (error) => {
      toast.error(
        error instanceof Error ? error.message : t('Version preflight failed.')
      )
    },
  })

  const startMutation = useMutation({
    mutationFn: startSystemUpdate,
    onSuccess: (result) => {
      setOperationId(result.id)
      setIdempotencyKey(null)
      queryClient.setQueryData(
        systemUpdateQueryKeys.operation(result.id),
        result
      )
      setConfirmOpen(false)
      toast.success(t('Version switch started.'))
      void queryClient.invalidateQueries({
        queryKey: systemUpdateQueryKeys.operations,
      })
    },
    onError: async (error) => {
      await operationsQuery.refetch()
      toast.error(
        error instanceof Error
          ? error.message
          : t('Failed to start version switch.')
      )
    },
  })

  const canSwitch = Boolean(
    isRoot &&
    updaterReady &&
    selectedRelease?.available &&
    selectedRelease.digest &&
    !selectedRelease.current &&
    (action !== 'rollback' || capability?.rollback_available) &&
    !active &&
    !preflightMutation.isPending &&
    !startMutation.isPending
  )
  let actionButtonLabel = actionLabel
  if (selectedRelease?.current) actionButtonLabel = t('Current version')
  else if (preflightMutation.isPending) {
    actionButtonLabel = t('Checking compatibility...')
  }

  const handleRequestSwitch = () => {
    if (!selectedRelease?.digest) return
    setPreflight(null)
    setIdempotencyKey(buildIdempotencyKey(selectedRelease.id))
    preflightMutation.mutate({
      release_id: selectedRelease.id,
      expected_digest: selectedRelease.digest,
    })
  }

  const handleConfirmSwitch = () => {
    if (!preflight?.ready || !selectedRelease?.digest || !idempotencyKey) return
    startMutation.mutate({
      release_id: selectedRelease.id,
      expected_digest: selectedRelease.digest,
      idempotency_key: idempotencyKey,
    })
  }

  const refreshCatalog = () => {
    void Promise.all([
      capabilitiesQuery.refetch(),
      releasesQuery.refetch(),
      operationsQuery.refetch(),
    ])
  }

  if (catalogLoading) {
    return (
      <div className='space-y-4' aria-busy='true'>
        <Skeleton className='h-28 w-full' />
        <Skeleton className='h-44 w-full' />
        <Skeleton className='h-32 w-full' />
      </div>
    )
  }

  if (catalogError && !capability && releases.length === 0) {
    const error = capabilitiesQuery.error ?? releasesQuery.error
    return (
      <ErrorState
        title={t('We could not load version management.')}
        description={error instanceof Error ? error.message : undefined}
        onRetry={refreshCatalog}
      />
    )
  }

  return (
    <div className='space-y-5'>
      <section className='rounded-xl border p-4 sm:p-5'>
        <div className='flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between'>
          <div className='min-w-0 space-y-3'>
            <div>
              <p className='text-muted-foreground text-xs font-medium tracking-wide uppercase'>
                {t('Current version')}
              </p>
              <div className='mt-1 flex flex-wrap items-center gap-2'>
                <span className='font-mono text-xl font-semibold'>
                  {currentVersion}
                </span>
                {updateAvailable && (
                  <Badge
                    variant='secondary'
                    className='bg-amber-50 text-amber-700 dark:bg-amber-500/15 dark:text-amber-300'
                  >
                    {t('Update available')}
                  </Badge>
                )}
              </div>
            </div>
            <dl className='grid gap-2 text-xs sm:grid-cols-2'>
              <div className='min-w-0'>
                <dt className='text-muted-foreground'>{t('Image')}</dt>
                <dd className='mt-0.5 font-mono break-all'>
                  {capability?.current_image_ref ||
                    currentRelease?.image_ref ||
                    capability?.image_repository ||
                    '-'}
                </dd>
              </div>
              <div className='min-w-0'>
                <dt className='text-muted-foreground'>{t('Digest')}</dt>
                <dd className='mt-0.5 font-mono break-all'>
                  {capability?.current_digest || currentRelease?.digest || '-'}
                </dd>
              </div>
              <div>
                <dt className='text-muted-foreground'>{t('Uptime since')}</dt>
                <dd className='mt-0.5'>
                  {props.startTime
                    ? formatTimestampToDate(props.startTime)
                    : t('Unknown')}
                </dd>
              </div>
              <div>
                <dt className='text-muted-foreground'>{t('Updater status')}</dt>
                <dd className='mt-0.5'>
                  {updaterReady ? t('Ready') : t('Unavailable')}
                </dd>
              </div>
            </dl>
          </div>
          <Button
            variant='outline'
            size='sm'
            onClick={refreshCatalog}
            disabled={
              capabilitiesQuery.isFetching ||
              releasesQuery.isFetching ||
              operationsQuery.isFetching
            }
          >
            <RefreshCw data-icon='inline-start' aria-hidden='true' />
            {t('Refresh')}
          </Button>
        </div>
      </section>

      {!updaterReady && (
        <Alert>
          <AlertCircle aria-hidden='true' />
          <AlertTitle>
            {t('Panel version switching is unavailable.')}
          </AlertTitle>
          <AlertDescription>
            {capability?.reason ||
              t(
                'You can browse release history, but this deployment is not paired with a reachable updater.'
              )}
          </AlertDescription>
        </Alert>
      )}

      <section className='space-y-4 rounded-xl border p-4 sm:p-5'>
        <div>
          <h3 className='text-sm font-semibold'>
            {t('Choose Docker version')}
          </h3>
          <p className='text-muted-foreground mt-1 text-xs'>
            {t(
              'Only verified images from the server release catalog are shown.'
            )}
          </p>
        </div>

        {releases.length === 0 ? (
          <Empty className='min-h-48 rounded-lg border border-dashed'>
            <EmptyHeader>
              <EmptyMedia variant='icon'>
                <History aria-hidden='true' />
              </EmptyMedia>
              <EmptyTitle>
                {t('No version history is available yet.')}
              </EmptyTitle>
              <EmptyDescription>
                {t('Refresh the release catalog to try again.')}
              </EmptyDescription>
            </EmptyHeader>
          </Empty>
        ) : (
          <div className='grid gap-4 lg:grid-cols-[minmax(0,20rem)_1fr]'>
            <div className='space-y-3'>
              <div className='space-y-1.5'>
                <Label htmlFor='system-version-select'>
                  {t('Target version')}
                </Label>
                <Select
                  items={releases.map((release) => ({
                    value: release.id,
                    label: release.version,
                  }))}
                  value={selectedReleaseId}
                  onValueChange={(value) => setSelectedReleaseId(value)}
                  disabled={active}
                >
                  <SelectTrigger id='system-version-select' className='w-full'>
                    <SelectValue placeholder={t('Select a version')} />
                  </SelectTrigger>
                  <SelectContent align='start'>
                    <SelectGroup>
                      {releases.map((release) => (
                        <SelectItem key={release.id} value={release.id}>
                          <span className='font-mono'>{release.version}</span>
                          {release.current && (
                            <span className='text-muted-foreground text-xs'>
                              {t('Current')}
                            </span>
                          )}
                          {!release.available && (
                            <span className='text-destructive text-xs'>
                              {t('Unavailable')}
                            </span>
                          )}
                        </SelectItem>
                      ))}
                    </SelectGroup>
                  </SelectContent>
                </Select>
              </div>

              {selectedRelease && (
                <dl className='text-muted-foreground space-y-2 text-xs'>
                  <div>
                    <dt>{t('Published')}</dt>
                    <dd className='text-foreground'>
                      {selectedRelease.published_at
                        ? formatTimestampToDate(
                            new Date(selectedRelease.published_at).getTime(),
                            'milliseconds'
                          )
                        : '-'}
                    </dd>
                  </div>
                  <div>
                    <dt>{t('Image')}</dt>
                    <dd className='text-foreground font-mono break-all'>
                      {selectedRelease.image_ref}
                    </dd>
                  </div>
                  <div>
                    <dt>{t('Digest')}</dt>
                    <dd className='text-foreground font-mono break-all'>
                      {selectedRelease.digest || '-'}
                    </dd>
                  </div>
                </dl>
              )}

              <div className='flex flex-col gap-2 sm:flex-row lg:flex-col'>
                <Button
                  type='button'
                  variant='outline'
                  onClick={() => setNotesOpen(true)}
                  disabled={!selectedRelease}
                >
                  <History data-icon='inline-start' aria-hidden='true' />
                  {t('View release notes')}
                </Button>
                <Button
                  type='button'
                  variant={action === 'rollback' ? 'destructive' : 'default'}
                  onClick={handleRequestSwitch}
                  disabled={!canSwitch}
                >
                  {preflightMutation.isPending ? (
                    <LoaderCircle
                      data-icon='inline-start'
                      className='animate-spin'
                      aria-hidden='true'
                    />
                  ) : (
                    <ChevronRight data-icon='inline-end' aria-hidden='true' />
                  )}
                  {actionButtonLabel}
                </Button>
              </div>
              {!isRoot && (
                <p className='text-muted-foreground text-xs'>
                  {t('Only the root administrator can switch versions.')}
                </p>
              )}
              {selectedRelease && !selectedRelease.available && (
                <p className='text-destructive text-xs'>
                  {selectedRelease.unavailable_reason ||
                    t('This image is unavailable.')}
                </p>
              )}
              {action === 'rollback' && !capability?.rollback_available && (
                <p className='text-destructive text-xs'>
                  {t('Panel version switching is unavailable.')}
                </p>
              )}
            </div>

            <div className='bg-muted/10 min-h-48 rounded-lg border p-4'>
              <h4 className='mb-3 font-mono text-sm font-semibold'>
                {selectedRelease?.version || t('Release notes')}
              </h4>
              {selectedRelease?.release_notes ? (
                <Markdown className='text-sm'>
                  {selectedRelease.release_notes}
                </Markdown>
              ) : (
                <p className='text-muted-foreground text-sm'>
                  {t('No release notes provided.')}
                </p>
              )}
            </div>
          </div>
        )}
      </section>

      {operation && (
        <section
          className='space-y-3 rounded-xl border p-4 sm:p-5'
          aria-busy={active}
          aria-live='polite'
        >
          <div className='flex flex-wrap items-center justify-between gap-3'>
            <div>
              <h3 className='text-sm font-semibold'>
                {t('Version switch status')}
              </h3>
              <p className='text-muted-foreground mt-1 text-xs'>
                {operation.version} ·{' '}
                {operation.phase
                  ? getOperationPhaseLabel(t, operation.phase)
                  : getOperationStatusLabel(t, operation.status)}
              </p>
            </div>
            <Badge
              variant={
                operation.status === 'failed' ? 'destructive' : 'secondary'
              }
              className={cn(
                operation.status === 'succeeded' &&
                  'bg-emerald-50 text-emerald-700 dark:bg-emerald-500/15 dark:text-emerald-300'
              )}
            >
              {active && (
                <LoaderCircle className='animate-spin' aria-hidden='true' />
              )}
              {getOperationStatusLabel(t, operation.status)}
            </Badge>
          </div>
          <Progress value={operationProgress} />
          <div className='text-muted-foreground flex items-center justify-between gap-3 text-xs'>
            <span>
              {operation.error ||
                operation.message ||
                t('Waiting for updater status...')}
            </span>
            <span className='tabular-nums'>{operationProgress}%</span>
          </div>
          {operationQuery.isError && active && (
            <Alert>
              <RefreshCw aria-hidden='true' />
              <AlertTitle>{t('The service is restarting.')}</AlertTitle>
              <AlertDescription>
                {t(
                  'The dashboard will keep reconnecting and verify the running version when the service returns.'
                )}
              </AlertDescription>
            </Alert>
          )}
          {operation.status === 'succeeded' && statusError && (
            <Alert>
              <AlertCircle aria-hidden='true' />
              <AlertTitle>
                {t(
                  'The update finished, but the running version is not verified yet.'
                )}
              </AlertTitle>
              <AlertDescription>
                {t('Refresh after the service becomes reachable.')}
              </AlertDescription>
            </Alert>
          )}
          {operation.status === 'succeeded' && !statusError && (
            <div className='inline-flex items-center gap-2 text-sm text-emerald-700 dark:text-emerald-300'>
              <CheckCircle2 aria-hidden='true' />
              {normalizeVersion(currentVersion) ===
              normalizeVersion(operation.version)
                ? t('Running version verified: {{version}}', {
                    version: currentVersion,
                  })
                : t(
                    'The updater completed, but the running version differs: {{version}}',
                    {
                      version: currentVersion,
                    }
                  )}
            </div>
          )}
          {operation.status === 'rolled_back' && (
            <Alert>
              <AlertCircle aria-hidden='true' />
              <AlertTitle>{t('The previous image was restored.')}</AlertTitle>
              <AlertDescription>
                {operation.rollback_succeeded
                  ? t('Automatic rollback completed successfully.')
                  : t('Check the updater logs to confirm the running image.')}
              </AlertDescription>
            </Alert>
          )}
        </section>
      )}

      <ReleaseNotesSheet
        open={notesOpen}
        onOpenChange={setNotesOpen}
        selectedReleaseId={selectedReleaseId}
      />

      <ConfirmDialog
        open={confirmOpen}
        onOpenChange={setConfirmOpen}
        title={
          action === 'rollback'
            ? t('Confirm version rollback')
            : t('Confirm version switch')
        }
        destructive={action === 'rollback'}
        isLoading={startMutation.isPending}
        disabled={!preflight?.ready}
        confirmText={actionLabel}
        handleConfirm={handleConfirmSwitch}
        desc={
          <div className='space-y-3 text-left'>
            <p>
              {t(
                'The service will restart and this dashboard may be temporarily unavailable.'
              )}
            </p>
            {action === 'rollback' && (
              <p className='text-destructive'>
                {t(
                  'Rolling back the image does not roll back database migrations. Continue only when this release is compatible with current data.'
                )}
              </p>
            )}
            {(preflight?.warnings?.length ?? 0) > 0 && (
              <ul className='list-disc space-y-1 pl-5'>
                {preflight?.warnings?.map((warning) => (
                  <li key={warning}>{warning}</li>
                ))}
              </ul>
            )}
            {(preflight?.blocking_reasons?.length ?? 0) > 0 && (
              <Alert variant='destructive'>
                <AlertCircle aria-hidden='true' />
                <AlertTitle>{t('This version cannot be selected.')}</AlertTitle>
                <AlertDescription>
                  <ul className='list-disc space-y-1 pl-5'>
                    {preflight?.blocking_reasons?.map((reason) => (
                      <li key={reason}>{reason}</li>
                    ))}
                  </ul>
                </AlertDescription>
              </Alert>
            )}
          </div>
        }
      />
    </div>
  )
}

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
import { api } from '@/lib/api'

import {
  selectLatestRelease,
  SYSTEM_RELEASE_REPOSITORY,
  type SystemRelease,
} from './releases'
import type {
  SystemReleaseList,
  SystemUpdateCapability,
  SystemUpdateOperation,
  SystemUpdatePreflight,
} from './types'

type ApiEnvelope<T> = {
  success: boolean
  message?: string
  data?: T
}

function unwrapResponse<T>(payload: ApiEnvelope<T> | T): T {
  if (payload && typeof payload === 'object' && 'success' in payload) {
    const envelope = payload as ApiEnvelope<T>
    if (!envelope.success || envelope.data === undefined) {
      throw new Error(envelope.message || 'Request failed')
    }
    return envelope.data
  }
  return payload as T
}

const quietRequest = {
  skipBusinessError: true,
  skipErrorHandler: true,
} as const

export async function getSystemUpdateCapabilities() {
  const response = await api.get<
    ApiEnvelope<SystemUpdateCapability> | SystemUpdateCapability
  >('/api/system-update/capabilities', quietRequest)
  return unwrapResponse(response.data)
}

export async function getSystemReleases() {
  const response = await api.get<
    ApiEnvelope<SystemReleaseList> | SystemReleaseList
  >('/api/system-update/releases', quietRequest)
  return unwrapResponse(response.data)
}

export async function getSystemUpdateOperations() {
  const response = await api.get<
    | ApiEnvelope<SystemUpdateOperation[] | SystemUpdateOperation | null>
    | SystemUpdateOperation[]
    | SystemUpdateOperation
    | null
  >('/api/system-update/operations', quietRequest)
  const result = unwrapResponse(response.data)
  if (Array.isArray(result)) return result
  return result ? [result] : []
}

export async function getSystemUpdateOperation(operationId: string) {
  const response = await api.get<
    ApiEnvelope<SystemUpdateOperation> | SystemUpdateOperation
  >(
    `/api/system-update/operations/${encodeURIComponent(operationId)}`,
    quietRequest
  )
  return unwrapResponse(response.data)
}

export async function preflightSystemUpdate(request: {
  release_id: string
  expected_digest: string
}) {
  const response = await api.post<
    ApiEnvelope<SystemUpdatePreflight> | SystemUpdatePreflight
  >('/api/system-update/preflight', request, quietRequest)
  return unwrapResponse(response.data)
}

export async function startSystemUpdate(request: {
  release_id: string
  expected_digest: string
  idempotency_key: string
}) {
  const response = await api.post<
    ApiEnvelope<SystemUpdateOperation> | SystemUpdateOperation
  >('/api/system-update/operations', request, quietRequest)
  return unwrapResponse(response.data)
}

export type UpdateCheckErrorCode =
  | 'network'
  | 'rate-limit'
  | 'timeout'
  | 'payload'

export class UpdateCheckError extends Error {
  constructor(public readonly code: UpdateCheckErrorCode) {
    super(code)
    this.name = 'UpdateCheckError'
  }
}

export async function fetchLatestSystemRelease(
  signal: AbortSignal
): Promise<SystemRelease | null> {
  const controller = new AbortController()
  const cancel = () => controller.abort()
  signal.addEventListener('abort', cancel, { once: true })
  if (signal.aborted) controller.abort()
  const timeout = setTimeout(cancel, 10_000)

  try {
    const response = await fetch(
      `https://api.github.com/repos/${SYSTEM_RELEASE_REPOSITORY}/releases?per_page=100`,
      {
        credentials: 'omit',
        headers: { Accept: 'application/vnd.github+json' },
        signal: controller.signal,
      }
    )
    if (response.status === 403 || response.status === 429) {
      throw new UpdateCheckError('rate-limit')
    }
    if (!response.ok) throw new UpdateCheckError('network')

    try {
      return selectLatestRelease(await response.json())
    } catch (error) {
      if (controller.signal.aborted) throw error
      throw new UpdateCheckError('payload')
    }
  } catch (error) {
    if (signal.aborted) throw error
    if (controller.signal.aborted) throw new UpdateCheckError('timeout')
    if (error instanceof UpdateCheckError) throw error
    throw new UpdateCheckError('network')
  } finally {
    clearTimeout(timeout)
    signal.removeEventListener('abort', cancel)
  }
}

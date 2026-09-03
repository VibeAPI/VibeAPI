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

import {
  getSystemReleases,
  getSystemUpdateCapabilities,
  getSystemUpdateOperation,
  getSystemUpdateOperations,
} from './api'
import { isActiveSystemUpdateOperation } from './types'

export const systemUpdateQueryKeys = {
  capabilities: ['system-update', 'capabilities'] as const,
  releases: ['system-update', 'releases'] as const,
  operations: ['system-update', 'operations'] as const,
  operation: (id: string) => ['system-update', 'operations', id] as const,
}

export function useSystemUpdateCatalog(enabled = true) {
  const capabilitiesQuery = useQuery({
    queryKey: systemUpdateQueryKeys.capabilities,
    queryFn: getSystemUpdateCapabilities,
    enabled,
    staleTime: 60 * 1000,
    retry: 1,
  })
  const releasesQuery = useQuery({
    queryKey: systemUpdateQueryKeys.releases,
    queryFn: getSystemReleases,
    enabled,
    staleTime: 5 * 60 * 1000,
    retry: 1,
  })
  const operationsQuery = useQuery({
    queryKey: systemUpdateQueryKeys.operations,
    queryFn: getSystemUpdateOperations,
    enabled,
    staleTime: 10 * 1000,
    retry: 1,
    refetchInterval: (query) =>
      query.state.data?.some(isActiveSystemUpdateOperation) ? 5000 : false,
  })

  const operations = operationsQuery.data ?? []
  const activeOperation =
    capabilitiesQuery.data?.active_operation ??
    operations.find(isActiveSystemUpdateOperation) ??
    operations[0] ??
    null

  return {
    capabilitiesQuery,
    releasesQuery,
    operationsQuery,
    activeOperation,
  }
}

export function useSystemUpdateOperation(operationId?: string | null) {
  return useQuery({
    queryKey: systemUpdateQueryKeys.operation(operationId ?? 'none'),
    queryFn: () => {
      if (!operationId) throw new Error('Missing operation ID')
      return getSystemUpdateOperation(operationId)
    },
    enabled: Boolean(operationId),
    retry: true,
    retryDelay: (attempt) => Math.min(1000 * 2 ** attempt, 10_000),
    refetchInterval: (query) =>
      isActiveSystemUpdateOperation(query.state.data) ? 2000 : false,
  })
}

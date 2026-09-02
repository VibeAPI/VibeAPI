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

export type SystemUpdateCapability = {
  release_catalog_available: boolean
  updater_configured: boolean
  updater_reachable: boolean
  current_version: string
  image_repository: string
  release_repository: string
  deployment_kind?: string
  owner?: string
  compose_project?: string
  compose_service?: string
  architecture?: string
  current_image_ref?: string
  current_digest?: string
  rollback_available: boolean
  active_operation?: SystemUpdateOperation
  reason?: string
}

export type SystemRelease = {
  id: string
  version: string
  name?: string
  published_at?: string
  release_notes?: string
  release_url?: string
  image_ref: string
  digest?: string
  architectures?: string[]
  current: boolean
  available: boolean
  unavailable_reason?: string
}

export type SystemReleaseList = {
  releases: SystemRelease[]
  cached: boolean
}

export type SystemUpdatePreflight = {
  release_id: string
  version: string
  image_ref: string
  digest: string
  current_version: string
  current_digest?: string
  action: string
  expected_disconnect: boolean
  automatic_rollback_available: boolean
  ready: boolean
  warnings?: string[]
  blocking_reasons?: string[]
}

export type SystemUpdateOperationStatus =
  | 'pending'
  | 'running'
  | 'succeeded'
  | 'failed'
  | 'rolled_back'

export type SystemUpdateOperation = {
  id: string
  idempotency_key: string
  release_id: string
  version: string
  image_ref: string
  digest: string
  current_version?: string
  current_image_ref?: string
  current_digest?: string
  previous_image_ref?: string
  previous_digest?: string
  status: SystemUpdateOperationStatus
  phase?: string
  message?: string
  error?: string
  rollback_available: boolean
  automatic_rollback: boolean
  rollback_attempted: boolean
  rollback_succeeded: boolean
  error_code?: string
  progress?: number
  created_at: number
  updated_at: number
}

export type SystemUpdateAction = 'upgrade' | 'rollback' | 'reinstall'

export function isActiveSystemUpdateOperation(
  operation?: SystemUpdateOperation | null
): boolean {
  return operation?.status === 'pending' || operation?.status === 'running'
}

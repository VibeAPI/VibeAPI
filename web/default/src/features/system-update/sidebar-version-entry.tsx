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
import { useNavigate } from '@tanstack/react-router'
import { History } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import {
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  useSidebar,
} from '@/components/ui/sidebar'
import { useStatus } from '@/hooks/use-status'
import { ROLE } from '@/lib/roles'
import { cn } from '@/lib/utils'
import { useAuthStore } from '@/stores/auth-store'

import { useSystemUpdateCatalog } from './queries'
import { ReleaseNotesSheet } from './release-notes-sheet'
import { isActiveSystemUpdateOperation } from './types'
import { compareVersions } from './version'

export function SidebarVersionEntry() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { status } = useStatus()
  const user = useAuthStore((state) => state.auth.user)
  const { setOpenMobile } = useSidebar()
  const [open, setOpen] = useState(false)
  const isRoot = user?.role === ROLE.SUPER_ADMIN
  const { releasesQuery, activeOperation } = useSystemUpdateCatalog(isRoot)
  const releases = releasesQuery.data?.releases ?? []
  const version = String(status?.version || t('Unknown version'))
  const hasUpdate = releases.some(
    (release) =>
      release.available && compareVersions(release.version, version) > 0
  )
  const active = isActiveSystemUpdateOperation(activeOperation)
  const failed = activeOperation?.status === 'failed'
  let statusDotClassName = 'bg-amber-500'
  let statusText = t('A newer version is available.')
  if (failed) {
    statusDotClassName = 'bg-destructive'
    statusText = t('The last version switch failed.')
  } else if (active) {
    statusDotClassName = 'animate-pulse bg-sky-500'
    statusText = t('A version switch is in progress.')
  } else if (!hasUpdate) {
    statusText = t('No newer version is available.')
  }

  if (!isRoot) return null

  const handleOpen = () => {
    setOpenMobile(false)
    window.setTimeout(() => setOpen(true), 0)
  }

  return (
    <>
      <SidebarMenu>
        <SidebarMenuItem>
          <SidebarMenuButton
            tooltip={t('Version history')}
            onClick={handleOpen}
            aria-label={t(
              'Open version history. Current version: {{version}}',
              {
                version,
              }
            )}
          >
            <History aria-hidden='true' />
            <span className='min-w-0 flex-1 truncate font-mono'>{version}</span>
            {(hasUpdate || active || failed) && (
              <span
                className={cn(
                  'size-2 shrink-0 rounded-full',
                  statusDotClassName
                )}
                aria-hidden='true'
              />
            )}
            <span className='sr-only'>{statusText}</span>
          </SidebarMenuButton>
        </SidebarMenuItem>
      </SidebarMenu>
      <ReleaseNotesSheet
        open={open}
        onOpenChange={setOpen}
        onManageVersions={() => {
          setOpen(false)
          void navigate({
            to: '/system-settings/operations/$section',
            params: { section: 'update-checker' },
          })
        }}
      />
    </>
  )
}

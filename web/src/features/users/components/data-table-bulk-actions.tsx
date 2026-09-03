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
import type { Table } from '@tanstack/react-table'
import { StickyNote } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Dialog } from '@/components/dialog'
import { DataTableBulkActions as BulkActionsToolbar } from '@/components/data-table'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'

import { updateUsersRemark } from '../api'
import type { User } from '../types'
import { useUsers } from './users-provider'

interface DataTableBulkActionsProps {
  table: Table<User>
}

export function DataTableBulkActions({ table }: DataTableBulkActionsProps) {
  const { t } = useTranslation()
  const { triggerRefresh } = useUsers()
  const [open, setOpen] = useState(false)
  const [remark, setRemark] = useState('')
  const [loading, setLoading] = useState(false)
  const selectedRows = table.getFilteredSelectedRowModel().rows

  const handleSave = async () => {
    setLoading(true)
    try {
      const result = await updateUsersRemark({
        user_ids: selectedRows.map((row) => row.original.id),
        remark: remark.trim(),
      })
      if (!result.success) {
        toast.error(result.message || t('Failed to update remarks'))
        return
      }
      toast.success(t('Remarks updated successfully'))
      setRemark('')
      setOpen(false)
      table.resetRowSelection()
      triggerRefresh()
    } catch (error: unknown) {
      toast.error(
        error instanceof Error ? error.message : t('Failed to update remarks')
      )
    } finally {
      setLoading(false)
    }
  }

  return (
    <>
      <BulkActionsToolbar table={table} entityName={t('user')}>
        <Button size='sm' variant='outline' onClick={() => setOpen(true)}>
          <StickyNote />
          {t('Set Remark')}
        </Button>
      </BulkActionsToolbar>
      <Dialog
        open={open}
        onOpenChange={setOpen}
        title={t('Set Remark for Selected Users')}
        description={t('The same admin remark will be applied to {{count}} users.', {
          count: selectedRows.length,
        })}
        contentHeight='auto'
        footer={
          <>
            <Button variant='outline' onClick={() => setOpen(false)}>
              {t('Cancel')}
            </Button>
            <Button onClick={handleSave} disabled={loading}>
              {loading ? t('Saving...') : t('Save')}
            </Button>
          </>
        }
      >
        <div className='space-y-2'>
          <Label htmlFor='bulk-user-remark'>{t('Remark')}</Label>
          <Textarea
            id='bulk-user-remark'
            value={remark}
            maxLength={255}
            rows={4}
            placeholder={t('Admin notes (only visible to admins)')}
            onChange={(event) => setRemark(event.target.value)}
          />
          <p className='text-muted-foreground text-xs'>{remark.length}/255</p>
        </div>
      </Dialog>
    </>
  )
}

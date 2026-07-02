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
import { useState, useEffect } from 'react'
import { useTranslation } from 'react-i18next'
import { useQuery } from '@tanstack/react-query'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { updateUser, getGroups } from '../../api'

interface UserGroupDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  userId: number
  username: string
  currentGroup: string
  onSuccess: () => void
}

export function UserGroupDialog(props: UserGroupDialogProps) {
  const { t } = useTranslation()
  const [selectedGroup, setSelectedGroup] = useState(props.currentGroup)
  const [loading, setLoading] = useState(false)

  const { data: groupsResponse } = useQuery({
    queryKey: ['groups', 'user'],
    queryFn: () => getGroups({ type: 'user' }),
  })

  const groups = groupsResponse?.data || []

  useEffect(() => {
    if (props.open) {
      setSelectedGroup(props.currentGroup)
    }
  }, [props.open, props.currentGroup])

  const handleConfirm = async () => {
    if (!selectedGroup) {
      toast.error(t('Please select a group'))
      return
    }

    if (selectedGroup === props.currentGroup) {
      props.onOpenChange(false)
      return
    }

    setLoading(true)
    try {
      const result = await updateUser({
        id: props.userId,
        username: props.username,
        display_name: '',
        group: selectedGroup,
      })
      if (result.success) {
        toast.success(t('Group updated successfully'))
        props.onOpenChange(false)
        props.onSuccess()
      } else {
        toast.error(result.message || t('Failed to update group'))
      }
    } catch (e: unknown) {
      toast.error(e instanceof Error ? e.message : t('Failed to update group'))
    } finally {
      setLoading(false)
    }
  }

  const handleCancel = () => {
    setSelectedGroup(props.currentGroup)
    props.onOpenChange(false)
  }

  return (
    <Dialog open={props.open} onOpenChange={props.onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t('Adjust Group')}</DialogTitle>
          <DialogDescription>
            {t('Change the group for user')} {props.username}
          </DialogDescription>
        </DialogHeader>
        <div className='space-y-4'>
          <div className='space-y-2'>
            <Label>{t('Group')}</Label>
            <Select value={selectedGroup} onValueChange={setSelectedGroup}>
              <SelectTrigger>
                <SelectValue placeholder={t('Select a group')} />
              </SelectTrigger>
              <SelectContent>
                <SelectGroup>
                  {groups.map((group) => (
                    <SelectItem key={group} value={group}>
                      {group}
                    </SelectItem>
                  ))}
                </SelectGroup>
              </SelectContent>
            </Select>
          </div>
        </div>
        <DialogFooter>
          <Button variant='outline' onClick={handleCancel}>
            {t('Cancel')}
          </Button>
          <Button onClick={handleConfirm} disabled={loading}>
            {loading ? t('Processing...') : t('Confirm')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

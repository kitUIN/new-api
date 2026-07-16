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
import { useState, useMemo, useEffect, useCallback, memo } from 'react'
import { useQuery } from '@tanstack/react-query'
import {
  Pencil,
  Plus,
  Trash2,
  GripVertical,
  ChevronDown,
  Link2,
  Unlink,
  ArrowRightLeft,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Checkbox } from '@/components/ui/checkbox'
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from '@/components/ui/collapsible'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { getGroupQuerySources } from '../api'
import type {
  GroupQuerySource,
  UpstreamGroupRatioBinding,
  UpstreamGroupRatioBindingSourceType,
} from '../types'
import { safeJsonParse } from '../utils/json-parser'

type GroupRatioVisualEditorProps = {
  groupRatio: string
  topupGroupRatio: string
  userUsableGroups: string
  groupGroupRatio: string
  groupTypes: string
  autoGroups: string
  upstreamGroupRatioBindings: string
  onChange: (field: string, value: string) => void
}

type SimpleGroup = {
  name: string
  value: string
}

type GroupPricingRow = {
  _id: string
  name: string
  ratio: number
  selectable: boolean
  description: string
  type: 'billing' | 'user'
}

type GroupOverride = {
  targetGroup: string
  ratio: number
}

type BindingSourceKey = `${'channel' | 'provider'}:${number}`

const sectionCardClassName =
  'relative shadow-sm ring-0 before:pointer-events-none before:absolute before:inset-0 before:rounded-xl before:border before:border-border/90'
const sectionHeaderClassName = 'border-b bg-muted/20'

let groupPricingIdCounter = 0
function createGroupPricingId() {
  groupPricingIdCounter += 1
  return `gpr_${groupPricingIdCounter}`
}

function normalizeRatio(value: unknown): number {
  const parsed = Number(value)
  return Number.isFinite(parsed) ? parsed : 1
}

function normalizeOffset(value: unknown): number {
  const parsed = Number(value)
  return Number.isFinite(parsed) ? parsed : 0
}

function getBindingOffsetExpression(
  binding: UpstreamGroupRatioBinding | undefined
): string {
  if (!binding) return '0'
  const expression = String(binding.offset_expr ?? '').trim()
  if (expression) return expression
  if (typeof binding.offset === 'string') {
    return binding.offset.trim() || '0'
  }
  return String(normalizeOffset(binding.offset))
}

function isNumericOffsetExpression(expression: string): boolean {
  if (expression.trim() === '') return true
  return Number.isFinite(Number(expression))
}

function evaluateOffsetExpression(expression: string, upstreamRatio: number) {
  const trimmed = expression.trim()
  if (trimmed === '') return upstreamRatio
  if (isNumericOffsetExpression(trimmed)) {
    return upstreamRatio + Number(trimmed)
  }

  if (!/^[0-9xX+\-*/().\s]+$/.test(trimmed)) return null
  try {
    const fn = new Function('x', `"use strict"; return (${trimmed});`)
    const result = Number(fn(upstreamRatio))
    return Number.isFinite(result) ? result : null
  } catch {
    return null
  }
}

function serializeOffsetExpression(expression: string): {
  offset?: number | string
  offset_expr?: string
} {
  const trimmed = expression.trim()
  if (trimmed === '' || trimmed === '0') {
    return {}
  }
  if (isNumericOffsetExpression(trimmed)) {
    return { offset: Number(trimmed) }
  }
  return { offset: trimmed }
}

function getSourceKey(sourceType: 'channel' | 'provider', sourceId: number) {
  return `${sourceType}:${sourceId}` as BindingSourceKey
}

function parseSourceKey(value: string): {
  sourceType: UpstreamGroupRatioBindingSourceType
  sourceID: number
} | null {
  const [sourceType, rawID] = value.split(':')
  const sourceID = Number(rawID)
  if (
    (sourceType !== 'channel' && sourceType !== 'provider') ||
    !Number.isFinite(sourceID)
  ) {
    return null
  }
  return { sourceType, sourceID }
}

function getSourceLabel(source: GroupQuerySource | undefined): string {
  if (!source) return ''
  const prefix = source.source_type === 'provider' ? 'Provider' : 'Channel'
  return `${prefix} #${source.id} ${source.name}`
}

function calculateFinalRatio(
  source: GroupQuerySource | undefined,
  binding: UpstreamGroupRatioBinding | undefined
) {
  if (!source || !binding) return null
  const upstream = source.last_result?.[binding.upstream_group]?.ratio
  if (!Number.isFinite(Number(upstream))) return null
  const value = evaluateOffsetExpression(
    getBindingOffsetExpression(binding),
    Number(upstream)
  )
  return value === null ? null : Math.max(0, value)
}

function buildGroupPricingRows(
  groupRatio: string,
  userUsableGroups: string,
  groupTypes: string
): GroupPricingRow[] {
  const ratioMap = safeJsonParse<Record<string, number>>(groupRatio, {
    fallback: {},
    context: 'group ratios',
  })
  const usableMap = safeJsonParse<Record<string, string>>(userUsableGroups, {
    fallback: {},
    context: 'user usable groups',
  })
  const typesMap = safeJsonParse<Record<string, string>>(groupTypes, {
    fallback: {},
    context: 'group types',
  })
  const disabledDescriptionPrefix = '__disabled_description__:'
  const names = new Set([
    ...Object.keys(ratioMap),
    ...Object.keys(usableMap)
      .filter((name) => !name.startsWith(disabledDescriptionPrefix))
      .map((name) => name.trim())
      .filter(Boolean),
  ])

  return Array.from(names).map((name) => ({
    _id: createGroupPricingId(),
    name,
    ratio: normalizeRatio(ratioMap[name]),
    selectable: Object.prototype.hasOwnProperty.call(usableMap, name),
    description: String(
      usableMap[name] ?? usableMap[`${disabledDescriptionPrefix}${name}`] ?? ''
    ),
    type: (typesMap[name] === 'user' ? 'user' : 'billing') as 'billing' | 'user',
  }))
}

function serializeGroupPricingRows(rows: GroupPricingRow[]) {
  const groupRatio: Record<string, number> = {}
  const userUsableGroups: Record<string, string> = {}
  const groupTypes: Record<string, string> = {}
  const disabledDescriptionPrefix = '__disabled_description__:'

  for (const row of rows) {
    const name = row.name.trim()
    if (!name) continue
    groupRatio[name] = normalizeRatio(row.ratio)
    if (row.selectable) {
      userUsableGroups[name] = row.description
    } else if (row.description.trim()) {
      userUsableGroups[`${disabledDescriptionPrefix}${name}`] = row.description
    }
    if (row.type === 'user') {
      groupTypes[name] = 'user'
    }
  }

  return {
    GroupRatio: JSON.stringify(groupRatio, null, 2),
    UserUsableGroups: JSON.stringify(userUsableGroups, null, 2),
    GroupTypes: JSON.stringify(groupTypes, null, 2),
  }
}

function groupPricingSignature(rows: GroupPricingRow[]): string {
  const serialized = serializeGroupPricingRows(rows)
  return JSON.stringify({
    groupRatio: safeJsonParse(serialized.GroupRatio, {
      fallback: {},
      silent: true,
    }),
    userUsableGroups: safeJsonParse(serialized.UserUsableGroups, {
      fallback: {},
      silent: true,
    }),
    groupTypes: safeJsonParse(serialized.GroupTypes, {
      fallback: {},
      silent: true,
    }),
  })
}

function sourceGroupPricingSignature(
  groupRatio: string,
  userUsableGroups: string,
  groupTypes: string
): string {
  return JSON.stringify({
    groupRatio: safeJsonParse(groupRatio, { fallback: {}, silent: true }),
    userUsableGroups: safeJsonParse(userUsableGroups, {
      fallback: {},
      silent: true,
    }),
    groupTypes: safeJsonParse(groupTypes, { fallback: {}, silent: true }),
  })
}

export const GroupRatioVisualEditor = memo(function GroupRatioVisualEditor({
  groupRatio,
  topupGroupRatio,
  userUsableGroups,
  groupGroupRatio,
  groupTypes,
  autoGroups,
  upstreamGroupRatioBindings,
  onChange,
}: GroupRatioVisualEditorProps) {
  const { t } = useTranslation()
  const [simpleDialogOpen, setSimpleDialogOpen] = useState(false)
  const [simpleDialogType, setSimpleDialogType] = useState<
    'groupRatio' | 'topupGroupRatio' | null
  >(null)
  const [simpleEditData, setSimpleEditData] = useState<SimpleGroup | null>(null)

  const [autoGroupDialogOpen, setAutoGroupDialogOpen] = useState(false)
  const [autoGroupInput, setAutoGroupInput] = useState('')

  const [groupOverrideDialogOpen, setGroupOverrideDialogOpen] = useState(false)
  const [groupOverrideUserGroup, setGroupOverrideUserGroup] = useState<
    string | null
  >(null)
  const [groupOverrideEditData, setGroupOverrideEditData] =
    useState<GroupOverride | null>(null)

  const [userGroupDialogOpen, setUserGroupDialogOpen] = useState(false)
  const [userGroupInput, setUserGroupInput] = useState('')

  // Parse topup group ratios
  const topupRatioList = useMemo(() => {
    const map = safeJsonParse<Record<string, number>>(topupGroupRatio, {
      fallback: {},
      context: 'topup group ratios',
    })
    return Object.entries(map).map(([name, value]) => ({
      name,
      value: String(value),
    }))
  }, [topupGroupRatio])

  // Parse auto groups
  const autoGroupsList = useMemo(() => {
    return safeJsonParse<string[]>(autoGroups, {
      fallback: [],
      context: 'auto groups',
    })
  }, [autoGroups])

  // Parse group-group ratios
  const groupGroupRatioList = useMemo(() => {
    const map = safeJsonParse<Record<string, Record<string, number>>>(
      groupGroupRatio,
      {
        fallback: {},
        context: 'group-group ratios',
      }
    )
    return Object.entries(map).map(([userGroup, overrides]) => ({
      userGroup,
      overrides: Object.entries(overrides).map(([targetGroup, ratio]) => ({
        targetGroup,
        ratio,
      })),
    }))
  }, [groupGroupRatio])

  // Simple group handlers (for groupRatio and topupGroupRatio)
  const handleSimpleAdd = (type: 'groupRatio' | 'topupGroupRatio') => {
    setSimpleDialogType(type)
    setSimpleEditData(null)
    setSimpleDialogOpen(true)
  }

  const handleSimpleEdit = (
    type: 'groupRatio' | 'topupGroupRatio',
    group: SimpleGroup
  ) => {
    setSimpleDialogType(type)
    setSimpleEditData(group)
    setSimpleDialogOpen(true)
  }

  const handleSimpleSave = (name: string, value: string) => {
    if (!simpleDialogType) return

    const fieldName =
      simpleDialogType === 'groupRatio' ? groupRatio : topupGroupRatio
    const map = safeJsonParse<Record<string, number>>(fieldName, {
      fallback: {},
      silent: true,
    })

    if (simpleEditData && simpleEditData.name !== name) {
      delete map[simpleEditData.name]
    }

    map[name] = parseFloat(value)

    const field =
      simpleDialogType === 'groupRatio' ? 'GroupRatio' : 'TopupGroupRatio'
    onChange(field, JSON.stringify(map, null, 2))
    setSimpleDialogOpen(false)
  }

  const handleSimpleDelete = (
    type: 'groupRatio' | 'topupGroupRatio',
    name: string
  ) => {
    const fieldName = type === 'groupRatio' ? groupRatio : topupGroupRatio
    const map = safeJsonParse<Record<string, number>>(fieldName, {
      fallback: {},
      silent: true,
    })
    delete map[name]

    const field = type === 'groupRatio' ? 'GroupRatio' : 'TopupGroupRatio'
    onChange(field, JSON.stringify(map, null, 2))
  }

  // Auto groups handlers
  const handleAutoGroupAdd = () => {
    setAutoGroupInput('')
    setAutoGroupDialogOpen(true)
  }

  const handleAutoGroupSave = () => {
    if (!autoGroupInput.trim()) return

    const list = [...autoGroupsList, autoGroupInput.trim()]
    onChange('AutoGroups', JSON.stringify(list, null, 2))
    setAutoGroupDialogOpen(false)
  }

  const handleAutoGroupDelete = (index: number) => {
    const list = autoGroupsList.filter((_, i) => i !== index)
    onChange('AutoGroups', JSON.stringify(list, null, 2))
  }

  const handleAutoGroupMove = (index: number, direction: 'up' | 'down') => {
    const list = [...autoGroupsList]
    const newIndex = direction === 'up' ? index - 1 : index + 1

    if (newIndex < 0 || newIndex >= list.length) return
    ;[list[index], list[newIndex]] = [list[newIndex], list[index]]
    onChange('AutoGroups', JSON.stringify(list, null, 2))
  }

  // Group-group ratio handlers
  const handleUserGroupAdd = () => {
    setUserGroupInput('')
    setUserGroupDialogOpen(true)
  }

  const handleUserGroupSave = () => {
    if (!userGroupInput.trim()) return

    const map = safeJsonParse<Record<string, Record<string, number>>>(
      groupGroupRatio,
      {
        fallback: {},
        silent: true,
      }
    )

    if (!map[userGroupInput.trim()]) {
      map[userGroupInput.trim()] = {}
    }

    onChange('GroupGroupRatio', JSON.stringify(map, null, 2))
    setUserGroupDialogOpen(false)
  }

  const handleUserGroupDelete = (userGroup: string) => {
    const map = safeJsonParse<Record<string, Record<string, number>>>(
      groupGroupRatio,
      {
        fallback: {},
        silent: true,
      }
    )
    delete map[userGroup]
    onChange('GroupGroupRatio', JSON.stringify(map, null, 2))
  }

  const handleOverrideAdd = (userGroup: string) => {
    setGroupOverrideUserGroup(userGroup)
    setGroupOverrideEditData(null)
    setGroupOverrideDialogOpen(true)
  }

  const handleOverrideEdit = (userGroup: string, override: GroupOverride) => {
    setGroupOverrideUserGroup(userGroup)
    setGroupOverrideEditData(override)
    setGroupOverrideDialogOpen(true)
  }

  const handleOverrideSave = (
    targetGroup: string,
    ratio: number,
    oldTargetGroup?: string
  ) => {
    if (!groupOverrideUserGroup) return

    const map = safeJsonParse<Record<string, Record<string, number>>>(
      groupGroupRatio,
      {
        fallback: {},
        silent: true,
      }
    )

    if (!map[groupOverrideUserGroup]) {
      map[groupOverrideUserGroup] = {}
    }

    if (oldTargetGroup && oldTargetGroup !== targetGroup) {
      delete map[groupOverrideUserGroup][oldTargetGroup]
    }

    map[groupOverrideUserGroup][targetGroup] = ratio

    onChange('GroupGroupRatio', JSON.stringify(map, null, 2))
    setGroupOverrideDialogOpen(false)
  }

  const handleOverrideDelete = (userGroup: string, targetGroup: string) => {
    const map = safeJsonParse<Record<string, Record<string, number>>>(
      groupGroupRatio,
      {
        fallback: {},
        silent: true,
      }
    )

    if (map[userGroup]) {
      delete map[userGroup][targetGroup]
      if (Object.keys(map[userGroup]).length === 0) {
        delete map[userGroup]
      }
    }

    onChange('GroupGroupRatio', JSON.stringify(map, null, 2))
  }

  return (
    <div className='space-y-4'>
      <GroupPricingTable
        groupRatio={groupRatio}
        userUsableGroups={userUsableGroups}
        groupTypes={groupTypes}
        upstreamGroupRatioBindings={upstreamGroupRatioBindings}
        onChange={onChange}
      />

      {/* Topup Group Ratios */}
      <Card className={sectionCardClassName}>
        <CardHeader className={sectionHeaderClassName}>
          <CardTitle>{t('Top-up group ratios')}</CardTitle>
          <CardDescription>
            {t('Multipliers for recharge pricing based on user groups.')}
          </CardDescription>
        </CardHeader>
        <CardContent>
          <div className='space-y-4'>
            <Button
              onClick={() => handleSimpleAdd('topupGroupRatio')}
              size='sm'
            >
              <Plus className='mr-2 h-4 w-4' />
              {t('Add group')}
            </Button>
            {topupRatioList.length > 0 && (
              <div className='rounded-md border'>
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>{t('Group name')}</TableHead>
                      <TableHead>{t('Multiplier')}</TableHead>
                      <TableHead className='text-right'>
                        {t('Actions')}
                      </TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {topupRatioList.map((group) => (
                      <TableRow key={group.name}>
                        <TableCell className='font-medium'>
                          {group.name}
                        </TableCell>
                        <TableCell>{group.value}</TableCell>
                        <TableCell className='text-right'>
                          <div className='flex justify-end gap-2'>
                            <Button
                              variant='ghost'
                              size='sm'
                              onClick={() =>
                                handleSimpleEdit('topupGroupRatio', group)
                              }
                            >
                              <Pencil className='h-4 w-4' />
                            </Button>
                            <Button
                              variant='ghost'
                              size='sm'
                              onClick={() =>
                                handleSimpleDelete(
                                  'topupGroupRatio',
                                  group.name
                                )
                              }
                            >
                              <Trash2 className='h-4 w-4' />
                            </Button>
                          </div>
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </div>
            )}
          </div>
        </CardContent>
      </Card>

      {/* Inter-group ratio overrides */}
      <Card className={sectionCardClassName}>
        <CardHeader className={sectionHeaderClassName}>
          <CardTitle>{t('Inter-group ratio overrides')}</CardTitle>
          <CardDescription>
            {t(
              'Custom multipliers when specific user groups use specific token groups. Example: VIP users get 0.9x rate when using "edit_this" group tokens.'
            )}
          </CardDescription>
        </CardHeader>
        <CardContent>
          <div className='space-y-4'>
            <Button onClick={handleUserGroupAdd} size='sm'>
              <Plus className='mr-2 h-4 w-4' />
              {t('Add user group')}
            </Button>
            {groupGroupRatioList.length > 0 && (
              <div className='space-y-3'>
                {groupGroupRatioList.map((userGroupData) => (
                  <Collapsible key={userGroupData.userGroup}>
                    <div className='rounded-lg border'>
                      <div className='flex items-center justify-between p-4'>
                        <div className='flex items-center gap-2'>
                          <CollapsibleTrigger
                            render={<Button variant='ghost' size='sm' />}
                          >
                            <ChevronDown className='h-4 w-4' />
                          </CollapsibleTrigger>
                          <span className='font-semibold'>
                            {userGroupData.userGroup}
                          </span>
                          <span className='text-muted-foreground text-sm'>
                            {t('{{count}} override', {
                              count: userGroupData.overrides.length,
                            })}
                          </span>
                        </div>
                        <div className='flex gap-2'>
                          <Button
                            variant='ghost'
                            size='sm'
                            onClick={() =>
                              handleOverrideAdd(userGroupData.userGroup)
                            }
                          >
                            <Plus className='h-4 w-4' />
                          </Button>
                          <Button
                            variant='ghost'
                            size='sm'
                            onClick={() =>
                              handleUserGroupDelete(userGroupData.userGroup)
                            }
                          >
                            <Trash2 className='h-4 w-4' />
                          </Button>
                        </div>
                      </div>
                      <CollapsibleContent>
                        {userGroupData.overrides.length > 0 && (
                          <div className='border-t'>
                            <Table>
                              <TableHeader>
                                <TableRow>
                                  <TableHead>{t('Target group')}</TableHead>
                                  <TableHead>{t('Ratio')}</TableHead>
                                  <TableHead className='text-right'>
                                    {t('Actions')}
                                  </TableHead>
                                </TableRow>
                              </TableHeader>
                              <TableBody>
                                {userGroupData.overrides.map((override) => (
                                  <TableRow key={override.targetGroup}>
                                    <TableCell className='font-medium'>
                                      {override.targetGroup}
                                    </TableCell>
                                    <TableCell>{override.ratio}</TableCell>
                                    <TableCell className='text-right'>
                                      <div className='flex justify-end gap-2'>
                                        <Button
                                          variant='ghost'
                                          size='sm'
                                          onClick={() =>
                                            handleOverrideEdit(
                                              userGroupData.userGroup,
                                              override
                                            )
                                          }
                                        >
                                          <Pencil className='h-4 w-4' />
                                        </Button>
                                        <Button
                                          variant='ghost'
                                          size='sm'
                                          onClick={() =>
                                            handleOverrideDelete(
                                              userGroupData.userGroup,
                                              override.targetGroup
                                            )
                                          }
                                        >
                                          <Trash2 className='h-4 w-4' />
                                        </Button>
                                      </div>
                                    </TableCell>
                                  </TableRow>
                                ))}
                              </TableBody>
                            </Table>
                          </div>
                        )}
                      </CollapsibleContent>
                    </div>
                  </Collapsible>
                ))}
              </div>
            )}
          </div>
        </CardContent>
      </Card>

      {/* Auto Groups */}
      <Card className={sectionCardClassName}>
        <CardHeader className={sectionHeaderClassName}>
          <CardTitle>{t('Auto assignment order')}</CardTitle>
          <CardDescription>
            {t(
              'Priority order for automatic group assignment. New tokens rotate through this list.'
            )}
          </CardDescription>
        </CardHeader>
        <CardContent>
          <div className='space-y-4'>
            <Button onClick={handleAutoGroupAdd} size='sm'>
              <Plus className='mr-2 h-4 w-4' />
              {t('Add group')}
            </Button>
            {autoGroupsList.length > 0 && (
              <div className='space-y-2'>
                {autoGroupsList.map((group, index) => (
                  <div
                    key={index}
                    className='flex items-center gap-2 rounded-md border p-3'
                  >
                    <GripVertical className='text-muted-foreground h-4 w-4' />
                    <span className='flex-1 font-medium'>{group}</span>
                    <div className='flex gap-1'>
                      <Button
                        variant='ghost'
                        size='sm'
                        disabled={index === 0}
                        onClick={() => handleAutoGroupMove(index, 'up')}
                      >
                        ↑
                      </Button>
                      <Button
                        variant='ghost'
                        size='sm'
                        disabled={index === autoGroupsList.length - 1}
                        onClick={() => handleAutoGroupMove(index, 'down')}
                      >
                        ↓
                      </Button>
                      <Button
                        variant='ghost'
                        size='sm'
                        onClick={() => handleAutoGroupDelete(index)}
                      >
                        <Trash2 className='h-4 w-4' />
                      </Button>
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>
        </CardContent>
      </Card>

      {/* Simple Group Dialog */}
      <SimpleGroupDialog
        open={simpleDialogOpen}
        onOpenChange={setSimpleDialogOpen}
        onSave={handleSimpleSave}
        editData={simpleEditData}
        type={simpleDialogType}
      />

      {/* Auto Group Dialog */}
      <Dialog open={autoGroupDialogOpen} onOpenChange={setAutoGroupDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('Add auto group')}</DialogTitle>
            <DialogDescription>
              {t('Add a group identifier to the auto assignment list.')}
            </DialogDescription>
          </DialogHeader>
          <div className='space-y-4 py-4'>
            <div className='space-y-2'>
              <Label>{t('Group identifier')}</Label>
              <Input
                value={autoGroupInput}
                onChange={(e) => setAutoGroupInput(e.target.value)}
                placeholder={t('default')}
              />
            </div>
          </div>
          <DialogFooter>
            <Button
              variant='outline'
              onClick={() => setAutoGroupDialogOpen(false)}
            >
              {t('Cancel')}
            </Button>
            <Button onClick={handleAutoGroupSave}>{t('Add')}</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* User Group Dialog */}
      <Dialog open={userGroupDialogOpen} onOpenChange={setUserGroupDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('Add user group')}</DialogTitle>
            <DialogDescription>
              {t('Create a new user group to configure ratio overrides for.')}
            </DialogDescription>
          </DialogHeader>
          <div className='space-y-4 py-4'>
            <div className='space-y-2'>
              <Label>{t('User group name')}</Label>
              <Input
                value={userGroupInput}
                onChange={(e) => setUserGroupInput(e.target.value)}
                placeholder={t('vip')}
              />
            </div>
          </div>
          <DialogFooter>
            <Button
              variant='outline'
              onClick={() => setUserGroupDialogOpen(false)}
            >
              {t('Cancel')}
            </Button>
            <Button onClick={handleUserGroupSave}>{t('Add')}</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Group Override Dialog */}
      <GroupOverrideDialog
        open={groupOverrideDialogOpen}
        onOpenChange={setGroupOverrideDialogOpen}
        onSave={handleOverrideSave}
        editData={groupOverrideEditData}
        userGroup={groupOverrideUserGroup}
      />
    </div>
  )
})

type GroupPricingTableProps = {
  groupRatio: string
  userUsableGroups: string
  groupTypes: string
  upstreamGroupRatioBindings: string
  onChange: (field: string, value: string) => void
}

function GroupPricingTable({
  groupRatio,
  userUsableGroups,
  groupTypes,
  upstreamGroupRatioBindings,
  onChange,
}: GroupPricingTableProps) {
  const { t } = useTranslation()
  const [activeTab, setActiveTab] = useState<'billing' | 'user'>('billing')
  const [bindingDialogRow, setBindingDialogRow] =
    useState<GroupPricingRow | null>(null)
  const [rows, setRows] = useState<GroupPricingRow[]>(() =>
    buildGroupPricingRows(groupRatio, userUsableGroups, groupTypes)
  )

  const bindings = useMemo(
    () =>
      safeJsonParse<Record<string, UpstreamGroupRatioBinding>>(
        upstreamGroupRatioBindings,
        {
          fallback: {},
          context: 'upstream group ratio bindings',
        }
      ),
    [upstreamGroupRatioBindings]
  )

  const { data: sourcesData, isLoading: sourcesLoading } = useQuery({
    queryKey: ['group-query-sources'],
    queryFn: getGroupQuerySources,
  })

  const sources = useMemo(() => sourcesData?.data ?? [], [sourcesData?.data])

  const sourceMap = useMemo(() => {
    const map = new Map<BindingSourceKey, GroupQuerySource>()
    for (const source of sources) {
      map.set(getSourceKey(source.source_type, source.id), source)
    }
    return map
  }, [sources])

  useEffect(() => {
    const incomingSignature = sourceGroupPricingSignature(
      groupRatio,
      userUsableGroups,
      groupTypes
    )
    setRows((currentRows) => {
      if (groupPricingSignature(currentRows) === incomingSignature) {
        return currentRows
      }
      return buildGroupPricingRows(groupRatio, userUsableGroups, groupTypes)
    })
  }, [groupRatio, userUsableGroups, groupTypes])

  const emitRows = useCallback(
    (nextRows: GroupPricingRow[]) => {
      setRows(nextRows)
      const serialized = serializeGroupPricingRows(nextRows)
      onChange('GroupRatio', serialized.GroupRatio)
      onChange('UserUsableGroups', serialized.UserUsableGroups)
      onChange('GroupTypes', serialized.GroupTypes)
    },
    [onChange]
  )

  const emitBindings = useCallback(
    (nextBindings: Record<string, UpstreamGroupRatioBinding>) => {
      onChange(
        'UpstreamGroupRatioBindings',
        JSON.stringify(nextBindings, null, 2)
      )
    },
    [onChange]
  )

  const saveBinding = useCallback(
    (groupName: string, binding: UpstreamGroupRatioBinding) => {
      const name = groupName.trim()
      if (!name) return
      emitBindings({
        ...bindings,
        [name]: binding,
      })
      setBindingDialogRow(null)
    },
    [bindings, emitBindings]
  )

  const removeBinding = useCallback(
    (groupName: string) => {
      const name = groupName.trim()
      if (!name || !bindings[name]) return
      const next = { ...bindings }
      delete next[name]
      emitBindings(next)
    },
    [bindings, emitBindings]
  )

  const updateRow = useCallback(
    (
      id: string,
      field: Exclude<keyof GroupPricingRow, '_id'>,
      value: string | number | boolean
    ) => {
      emitRows(
        rows.map((row) => (row._id === id ? { ...row, [field]: value } : row))
      )
    },
    [emitRows, rows]
  )

  const addRow = useCallback(() => {
    const existingNames = new Set(rows.map((row) => row.name))
    let index = 1
    let name = `group_${index}`
    while (existingNames.has(name)) {
      index += 1
      name = `group_${index}`
    }
    emitRows([
      ...rows,
      {
        _id: createGroupPricingId(),
        name,
        ratio: 1,
        selectable: true,
        description: '',
        type: activeTab,
      },
    ])
  }, [activeTab, emitRows, rows])

  const removeRow = useCallback(
    (id: string) => {
      emitRows(rows.filter((row) => row._id !== id))
    },
    [emitRows, rows]
  )

  const duplicateNames = useMemo(() => {
    const counts = new Map<string, number>()
    for (const row of rows) {
      const name = row.name.trim()
      if (!name) continue
      counts.set(name, (counts.get(name) ?? 0) + 1)
    }
    return Array.from(counts.entries())
      .filter(([, count]) => count > 1)
      .map(([name]) => name)
  }, [rows])

  const visibleRows = rows.filter((row) => row.type === activeTab)

  const handleMigrateGroup = useCallback(
    (id: string) => {
      const targetType: GroupPricingRow['type'] =
        activeTab === 'billing' ? 'user' : 'billing'
      const updatedRows = rows.map((row) =>
        row._id === id ? { ...row, type: targetType } : row
      )
      emitRows(updatedRows)
    },
    [activeTab, emitRows, rows]
  )

  return (
    <Card className={sectionCardClassName}>
      <CardHeader className={sectionHeaderClassName}>
        <div className='flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between'>
          <div>
            <CardTitle>{t('Pricing groups')}</CardTitle>
            <CardDescription>
              {t(
                'Edit billing ratios and user-selectable groups in one table.'
              )}
            </CardDescription>
          </div>
          <Button onClick={addRow} size='sm' className='sm:self-start'>
            <Plus className='mr-2 h-4 w-4' />
            {t('Add group')}
          </Button>
        </div>
        <Tabs
          value={activeTab}
          onValueChange={(v) => setActiveTab(v as 'billing' | 'user')}
        >
          <TabsList>
            <TabsTrigger value='billing'>{t('Billing groups')}</TabsTrigger>
            <TabsTrigger value='user'>{t('User groups')}</TabsTrigger>
          </TabsList>
        </Tabs>
      </CardHeader>
      <CardContent>
        <div className='space-y-3'>
          <div className='rounded-md border'>
            <Table className='min-w-[920px] table-fixed'>
              <TableHeader>
                <TableRow>
                  <TableHead className='w-44'>{t('Group name')}</TableHead>
                  <TableHead className='w-32'>{t('Ratio')}</TableHead>
                  <TableHead className='w-32 text-center'>
                    {t('User selectable')}
                  </TableHead>
                  <TableHead className='w-60'>{t('Description')}</TableHead>
                  <TableHead className='w-80'>
                    {t('Upstream binding')}
                  </TableHead>
                  <TableHead className='w-16 text-right'>
                    {t('Actions')}
                  </TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {visibleRows.length === 0 ? (
                  <TableRow>
                    <TableCell
                      colSpan={6}
                      className='text-muted-foreground h-20 text-center text-sm'
                    >
                      {t('No groups yet. Add a group to get started.')}
                    </TableCell>
                  </TableRow>
                ) : (
                  visibleRows.map((row) => {
                    const groupName = row.name.trim()
                    const binding = bindings[groupName]
                    const source = binding
                      ? sourceMap.get(
                          getSourceKey(binding.source_type, binding.source_id)
                        )
                      : undefined
                    const finalRatio = calculateFinalRatio(source, binding)

                    return (
                      <TableRow key={row._id}>
                        <TableCell className='w-44'>
                          <Input
                            value={row.name}
                            onChange={(event) =>
                              updateRow(row._id, 'name', event.target.value)
                            }
                            aria-invalid={duplicateNames.includes(
                              row.name.trim()
                            )}
                          />
                        </TableCell>
                        <TableCell className='w-32'>
                          <Input
                            className='min-w-28'
                            type='number'
                            min={0}
                            step={0.1}
                            value={String(row.ratio)}
                            disabled={!!binding}
                            onChange={(event) =>
                              updateRow(
                                row._id,
                                'ratio',
                                normalizeRatio(event.target.value)
                              )
                            }
                          />
                        </TableCell>
                        <TableCell className='w-32'>
                          <div className='flex justify-center'>
                            <Checkbox
                              checked={row.selectable}
                              onCheckedChange={(checked) =>
                                updateRow(
                                  row._id,
                                  'selectable',
                                  checked === true
                                )
                              }
                              aria-label={t('User selectable')}
                            />
                          </div>
                        </TableCell>
                        <TableCell className='w-60'>
                          <Input
                            value={row.description}
                            placeholder={t('Group description')}
                            onChange={(event) =>
                              updateRow(
                                row._id,
                                'description',
                                event.target.value
                              )
                            }
                          />
                        </TableCell>
                        <TableCell className='w-80'>
                          {binding ? (
                            <div className='flex items-start justify-between gap-2'>
                              <div className='min-w-0 space-y-1'>
                                <div className='flex flex-wrap items-center gap-1.5'>
                                  <Badge variant='secondary'>
                                    {t('Bound')}
                                  </Badge>
                                  <span className='text-muted-foreground text-xs'>
                                    {getSourceLabel(source) ||
                                      `${binding.source_type} #${binding.source_id}`}
                                  </span>
                                </div>
                                <p className='truncate text-xs'>
                                  {binding.upstream_group}
                                  <span className='text-muted-foreground'>
                                    {' '}
                                    {t('offset')}{' '}
                                    {getBindingOffsetExpression(binding)}
                                  </span>
                                  {finalRatio !== null && (
                                    <span className='text-muted-foreground'>
                                      {' '}
                                      {t('final')} {finalRatio}
                                    </span>
                                  )}
                                </p>
                              </div>
                              <div className='flex shrink-0 gap-1'>
                                <Button
                                  variant='ghost'
                                  size='sm'
                                  onClick={() => setBindingDialogRow(row)}
                                  aria-label={t('Edit binding')}
                                >
                                  <Link2 className='h-4 w-4' />
                                </Button>
                                <Button
                                  variant='ghost'
                                  size='sm'
                                  onClick={() => removeBinding(groupName)}
                                  aria-label={t('Unbind')}
                                >
                                  <Unlink className='h-4 w-4' />
                                </Button>
                              </div>
                            </div>
                          ) : (
                            <Button
                              variant='outline'
                              size='sm'
                              onClick={() => setBindingDialogRow(row)}
                              disabled={!groupName}
                            >
                              <Link2 className='mr-2 h-4 w-4' />
                              {t('Bind')}
                            </Button>
                          )}
                        </TableCell>
                        <TableCell className='text-right'>
                          <div className='flex items-center justify-end gap-1'>
                            <Button
                              variant='ghost'
                              size='sm'
                              onClick={() => handleMigrateGroup(row._id)}
                              aria-label={t('Migrate to {{target}}', {
                                target:
                                  activeTab === 'billing'
                                    ? t('User groups')
                                    : t('Billing groups'),
                              })}
                            >
                              <ArrowRightLeft className='h-4 w-4' />
                            </Button>
                            <Button
                              variant='ghost'
                              size='sm'
                              onClick={() => removeRow(row._id)}
                              aria-label={t('Delete')}
                            >
                              <Trash2 className='h-4 w-4' />
                            </Button>
                          </div>
                        </TableCell>
                      </TableRow>
                    )
                  })
                )}
              </TableBody>
            </Table>
          </div>

          {duplicateNames.length > 0 && (
            <p className='text-destructive text-sm'>
              {t('Duplicate group names: {{names}}', {
                names: duplicateNames.join(', '),
              })}
            </p>
          )}
        </div>
      </CardContent>

      <GroupBindingDialog
        open={bindingDialogRow !== null}
        onOpenChange={(open) => {
          if (!open) setBindingDialogRow(null)
        }}
        row={bindingDialogRow}
        binding={
          bindingDialogRow ? bindings[bindingDialogRow.name.trim()] : undefined
        }
        sources={sources}
        isLoading={sourcesLoading}
        onSave={saveBinding}
      />
    </Card>
  )
}

type GroupBindingDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  row: GroupPricingRow | null
  binding?: UpstreamGroupRatioBinding
  sources: GroupQuerySource[]
  isLoading: boolean
  onSave: (groupName: string, binding: UpstreamGroupRatioBinding) => void
}

function GroupBindingDialog({
  open,
  onOpenChange,
  row,
  binding,
  sources,
  isLoading,
  onSave,
}: GroupBindingDialogProps) {
  const { t } = useTranslation()
  const [sourceKey, setSourceKey] = useState('')
  const [upstreamGroup, setUpstreamGroup] = useState('')
  const [offset, setOffset] = useState('0')

  const sourcesWithGroups = useMemo(
    () =>
      sources.filter(
        (source) => Object.keys(source.last_result || {}).length > 0
      ),
    [sources]
  )

  const selectedSource = useMemo(
    () =>
      sourcesWithGroups.find(
        (source) => getSourceKey(source.source_type, source.id) === sourceKey
      ),
    [sourceKey, sourcesWithGroups]
  )

  const upstreamGroups = useMemo(
    () => Object.keys(selectedSource?.last_result || {}),
    [selectedSource]
  )

  useEffect(() => {
    if (!open) {
      setSourceKey('')
      setUpstreamGroup('')
      setOffset('0')
      return
    }

    if (binding) {
      setSourceKey(getSourceKey(binding.source_type, binding.source_id))
      setUpstreamGroup(binding.upstream_group)
      setOffset(getBindingOffsetExpression(binding))
      return
    }

    const firstSource = sourcesWithGroups[0]
    setSourceKey(
      firstSource ? getSourceKey(firstSource.source_type, firstSource.id) : ''
    )
    setUpstreamGroup(
      firstSource ? Object.keys(firstSource.last_result || {})[0] || '' : ''
    )
    setOffset('0')
  }, [binding, open, sourcesWithGroups])

  useEffect(() => {
    if (!open || !selectedSource || !sourceKey) return
    if (upstreamGroup && upstreamGroups.includes(upstreamGroup)) return
    setUpstreamGroup(upstreamGroups[0] || '')
  }, [open, selectedSource, sourceKey, upstreamGroup, upstreamGroups])

  const handleSave = () => {
    if (!row) return
    const parsedSource = parseSourceKey(sourceKey)
    if (!parsedSource || !upstreamGroup.trim()) return
    onSave(row.name, {
      source_type: parsedSource.sourceType,
      source_id: parsedSource.sourceID,
      upstream_group: upstreamGroup.trim(),
      ...serializeOffsetExpression(offset),
    })
  }

  const selectedItem = selectedSource?.last_result?.[upstreamGroup]
  const finalRatio =
    selectedItem && Number.isFinite(Number(selectedItem.ratio))
      ? (() => {
          const value = evaluateOffsetExpression(offset, Number(selectedItem.ratio))
          return value === null ? null : Math.max(0, value)
        })()
      : null

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t('Bind upstream group ratio')}</DialogTitle>
          <DialogDescription>
            {row
              ? t('Control "{{group}}" with an upstream group ratio.', {
                  group: row.name,
                })
              : t('Control this group with an upstream group ratio.')}
          </DialogDescription>
        </DialogHeader>
        <div className='space-y-4 py-4'>
          <div className='space-y-2'>
            <Label>{t('Source')}</Label>
            <Select
              items={sourcesWithGroups.map((source) => ({
                value: getSourceKey(source.source_type, source.id),
                label: getSourceLabel(source),
              }))}
              value={sourceKey}
              onValueChange={(value) => {
                if (value === null) return
                setSourceKey(value)
              }}
              disabled={isLoading || sourcesWithGroups.length === 0}
            >
              <SelectTrigger className='w-full'>
                <SelectValue placeholder={t('Select source')} />
              </SelectTrigger>
              <SelectContent alignItemWithTrigger={false}>
                <SelectGroup>
                  {sourcesWithGroups.map((source) => (
                    <SelectItem
                      key={getSourceKey(source.source_type, source.id)}
                      value={getSourceKey(source.source_type, source.id)}
                    >
                      {getSourceLabel(source)}
                    </SelectItem>
                  ))}
                </SelectGroup>
              </SelectContent>
            </Select>
          </div>

          <div className='space-y-2'>
            <Label>{t('Upstream group')}</Label>
            <Select
              items={upstreamGroups.map((group) => ({
                value: group,
                label: group,
              }))}
              value={upstreamGroup}
              onValueChange={(value) => {
                if (value !== null) setUpstreamGroup(value)
              }}
              disabled={!selectedSource || upstreamGroups.length === 0}
            >
              <SelectTrigger className='w-full'>
                <SelectValue placeholder={t('Select upstream group')} />
              </SelectTrigger>
              <SelectContent alignItemWithTrigger={false}>
                <SelectGroup>
                  {upstreamGroups.map((group) => (
                    <SelectItem key={group} value={group}>
                      {group}
                    </SelectItem>
                  ))}
                </SelectGroup>
              </SelectContent>
            </Select>
          </div>

          <div className='space-y-2'>
            <Label>{t('Offset')}</Label>
            <Input
              value={offset}
              onChange={(event) => setOffset(event.target.value)}
              placeholder='(x + 0.3) / 10 + 0.4'
            />
          </div>

          {selectedItem && (
            <div className='text-muted-foreground rounded-md border px-3 py-2 text-sm'>
              {t('Upstream ratio')}: {selectedItem.ratio}
              {finalRatio !== null && (
                <>
                  {' '}
                  {t('Final ratio')}: {finalRatio}
                </>
              )}
            </div>
          )}
        </div>
        <DialogFooter>
          <Button variant='outline' onClick={() => onOpenChange(false)}>
            {t('Cancel')}
          </Button>
          <Button
            onClick={handleSave}
            disabled={!parseSourceKey(sourceKey) || !upstreamGroup.trim()}
          >
            {t('Save')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

// Simple Group Dialog Component
type SimpleGroupDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  onSave: (name: string, value: string) => void
  editData: SimpleGroup | null
  type: 'groupRatio' | 'topupGroupRatio' | null
}

function SimpleGroupDialog({
  open,
  onOpenChange,
  onSave,
  editData,
  type,
}: SimpleGroupDialogProps) {
  const { t } = useTranslation()
  const [name, setName] = useState('')
  const [value, setValue] = useState('')

  const title = type === 'groupRatio' ? t('group ratio') : t('top-up ratio')

  useEffect(() => {
    if (!open) {
      setName('')
      setValue('')
      return
    }

    setName(editData?.name ?? '')
    setValue(editData?.value ?? '')
  }, [editData, open])

  const handleSave = () => {
    if (!name.trim() || !value.trim()) return
    onSave(name.trim(), value.trim())
    setName('')
    setValue('')
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>
            {editData
              ? t('Edit {{title}}', { title })
              : t('Add {{title}}', { title })}
          </DialogTitle>
          <DialogDescription>
            {t('Configure the ratio for this group.')}
          </DialogDescription>
        </DialogHeader>
        <div className='space-y-4 py-4'>
          <div className='space-y-2'>
            <Label>{t('Group name')}</Label>
            <Input
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder={t('default')}
              disabled={!!editData}
            />
          </div>
          <div className='space-y-2'>
            <Label>{t('Ratio')}</Label>
            <Input
              value={value}
              onChange={(e) => {
                const val = e.target.value
                if (val === '' || !isNaN(parseFloat(val))) {
                  setValue(val)
                }
              }}
              placeholder='1.0'
            />
          </div>
        </div>
        <DialogFooter>
          <Button variant='outline' onClick={() => onOpenChange(false)}>
            {t('Cancel')}
          </Button>
          <Button onClick={handleSave}>
            {editData ? t('Update') : t('Add')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

// Group Override Dialog Component
type GroupOverrideDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  onSave: (targetGroup: string, ratio: number, oldTargetGroup?: string) => void
  editData: GroupOverride | null
  userGroup: string | null
}

function GroupOverrideDialog({
  open,
  onOpenChange,
  onSave,
  editData,
  userGroup,
}: GroupOverrideDialogProps) {
  const { t } = useTranslation()
  const [targetGroup, setTargetGroup] = useState('')
  const [ratio, setRatio] = useState('')

  useEffect(() => {
    if (!open) {
      setTargetGroup('')
      setRatio('')
      return
    }

    setTargetGroup(editData?.targetGroup ?? '')
    setRatio(editData ? String(editData.ratio) : '')
  }, [editData, open])

  const handleSave = () => {
    if (!targetGroup.trim() || !ratio.trim()) return
    const parsedRatio = parseFloat(ratio)
    if (isNaN(parsedRatio)) return

    onSave(targetGroup.trim(), parsedRatio, editData?.targetGroup)
    setTargetGroup('')
    setRatio('')
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>
            {editData ? t('Edit ratio override') : t('Add ratio override')}
          </DialogTitle>
          <DialogDescription>
            {userGroup
              ? t(
                  'Configure a custom ratio for "{{userGroup}}" users when using a specific token group.',
                  { userGroup }
                )
              : t(
                  'Configure a custom ratio for when users use a specific token group.'
                )}
          </DialogDescription>
        </DialogHeader>
        <div className='space-y-4 py-4'>
          <div className='space-y-2'>
            <Label>{t('Target group')}</Label>
            <Input
              value={targetGroup}
              onChange={(e) => setTargetGroup(e.target.value)}
              placeholder={t('edit_this')}
              disabled={!!editData}
            />
            <p className='text-muted-foreground text-xs'>
              {t('The token group that will have a custom ratio')}
            </p>
          </div>
          <div className='space-y-2'>
            <Label>{t('Ratio')}</Label>
            <Input
              value={ratio}
              onChange={(e) => {
                const val = e.target.value
                if (val === '' || !isNaN(parseFloat(val))) {
                  setRatio(val)
                }
              }}
              placeholder='0.9'
            />
            <p className='text-muted-foreground text-xs'>
              {t('Multiplier applied when {{userGroup}} uses {{targetGroup}}', {
                userGroup: userGroup || t('this user group'),
                targetGroup: targetGroup || t('this token group'),
              })}
            </p>
          </div>
        </div>
        <DialogFooter>
          <Button variant='outline' onClick={() => onOpenChange(false)}>
            {t('Cancel')}
          </Button>
          <Button onClick={handleSave}>
            {editData ? t('Update') : t('Add')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

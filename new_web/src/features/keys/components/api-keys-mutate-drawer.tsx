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
import { useEffect, useState, type DragEvent } from 'react'
import { useForm, type SubmitErrorHandler } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useQuery } from '@tanstack/react-query'
import {
  ArrowDown,
  ArrowUp,
  ChevronDown,
  GripVertical,
  KeyRound,
  Plus,
  RotateCcw,
  Settings2,
  Trash2,
  WalletCards,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { getUserModels, getUserGroups } from '@/lib/api'
import { getCurrencyDisplay, getCurrencyLabel } from '@/lib/currency'
import {
  getRuleAutoGroupLabel,
  isRuleAutoGroupName,
} from '@/lib/rule-auto-groups'
import { cn } from '@/lib/utils'
import { useStatus } from '@/hooks/use-status'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from '@/components/ui/collapsible'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import {
  Sheet,
  SheetClose,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'
import { DateTimePicker } from '@/components/datetime-picker'
import {
  SideDrawerSection,
  SideDrawerSectionHeader,
  sideDrawerContentClassName,
  sideDrawerFooterClassName,
  sideDrawerFormClassName,
  sideDrawerHeaderClassName,
  sideDrawerSwitchItemClassName,
} from '@/components/drawer-layout'
import { MultiSelect } from '@/components/multi-select'
import { getPerfGroupHealth } from '@/features/performance-metrics/api'
import type { PerfGroupHealth } from '@/features/performance-metrics/types'
import {
  createApiKey,
  updateApiKey,
  getApiKey,
  resetApiKeyFailoverToP0,
} from '../api'
import { ERROR_MESSAGES, SUCCESS_MESSAGES } from '../constants'
import {
  getApiKeyFormSchema,
  type ApiKeyFormValues,
  getApiKeyFormDefaultValues,
  transformFormDataToPayload,
  transformApiKeyToFormDefaults,
} from '../lib'
import { type ApiKey } from '../types'
import {
  ApiKeyGroupCombobox,
  type ApiKeyGroupOption,
} from './api-key-group-combobox'
import { useApiKeys } from './api-keys-provider'

const GROUP_HEALTH_WINDOW_HOURS = 24
const GROUP_HEALTH_INTERVAL_MINUTES = 10

type FailoverDragPosition = 'before' | 'after'

type GroupSourceEntry = [
  string,
  {
    label?: string
    desc: string
    ratio: ApiKeyGroupOption['ratio']
    is_auto_group?: boolean
    auto_group_type?: string
  },
]

type ApiKeyMutateDrawerProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  currentRow?: ApiKey
}

function calculateRecentAvailability(group?: PerfGroupHealth) {
  const lastBucket = group?.buckets.at(-1)
  if (lastBucket?.request_count && lastBucket.request_count > 0) {
    return {
      recentAvailability: lastBucket.success_rate,
      recentWindowMinutes: 10 as const,
    }
  }

  return {
    recentAvailability: null,
    recentWindowMinutes: 10 as const,
  }
}

function calculateWindowAvailability(
  group: PerfGroupHealth | undefined,
  windowMinutes: number
) {
  const buckets = group?.buckets ?? []
  const latestEndTs = buckets.at(-1)?.end_ts
  if (!latestEndTs) return null

  const cutoffTs = latestEndTs - windowMinutes * 60
  const totals = buckets
    .filter((bucket) => bucket.end_ts > cutoffTs)
    .reduce(
      (acc, bucket) => {
        acc.requests += bucket.request_count
        acc.successes += bucket.success_count
        return acc
      },
      { requests: 0, successes: 0 }
    )

  if (totals.requests <= 0) return null
  return (totals.successes / totals.requests) * 100
}

function getSortableGroupRatio(ratio: ApiKeyGroupOption['ratio']) {
  if (typeof ratio === 'number' && Number.isFinite(ratio)) return ratio
  const parsedRatio = Number(ratio)
  return Number.isFinite(parsedRatio) ? parsedRatio : Number.POSITIVE_INFINITY
}

function compareGroupOptionsByRatio(
  a: ApiKeyGroupOption,
  b: ApiKeyGroupOption
) {
  const ratioDiff =
    getSortableGroupRatio(a.ratio) - getSortableGroupRatio(b.ratio)
  if (ratioDiff !== 0) return ratioDiff
  return a.label.localeCompare(b.label)
}

export function ApiKeysMutateDrawer({
  open,
  onOpenChange,
  currentRow,
}: ApiKeyMutateDrawerProps) {
  const { t } = useTranslation()
  const isUpdate = !!currentRow
  const { triggerRefresh } = useApiKeys()
  const { status } = useStatus()
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [isResettingFailover, setIsResettingFailover] = useState(false)
  const [editingApiKey, setEditingApiKey] = useState<ApiKey | undefined>(
    currentRow
  )
  const [draggedFailoverIndex, setDraggedFailoverIndex] = useState<
    number | null
  >(null)
  const [dragOverFailoverIndex, setDragOverFailoverIndex] = useState<
    number | null
  >(null)
  const [dragOverFailoverPosition, setDragOverFailoverPosition] =
    useState<FailoverDragPosition>('before')
  const [advancedOpen, setAdvancedOpen] = useState(false)
  const defaultUseAutoGroup = status?.default_use_auto_group === true

  // Fetch models
  const { data: modelsData } = useQuery({
    queryKey: ['user-models'],
    queryFn: getUserModels,
    staleTime: 5 * 60 * 1000, // Cache for 5 minutes
  })

  // Fetch groups
  const { data: groupsData } = useQuery({
    queryKey: ['user-groups'],
    queryFn: getUserGroups,
    staleTime: 5 * 60 * 1000,
  })

  const { data: groupHealthData } = useQuery({
    queryKey: [
      'perf-group-health',
      GROUP_HEALTH_WINDOW_HOURS,
      GROUP_HEALTH_INTERVAL_MINUTES,
    ],
    queryFn: () =>
      getPerfGroupHealth(
        GROUP_HEALTH_WINDOW_HOURS,
        GROUP_HEALTH_INTERVAL_MINUTES
      ),
    enabled: open,
    refetchOnMount: 'always',
    refetchOnReconnect: 'always',
    refetchOnWindowFocus: true,
    staleTime: 60 * 1000,
    retry: false,
  })

  const models = modelsData?.data || []
  const groupsRaw = groupsData?.data || {}
  const groupHealthMap = new Map(
    (groupHealthData?.data.groups ?? []).map((group) => [group.group, group])
  )
  const groupsSource: GroupSourceEntry[] = Array.isArray(groupsData?.groups)
    ? groupsData.groups.map((group) => [
        group.name,
        {
          label: group.label,
          desc: group.desc,
          ratio: group.ratio,
          is_auto_group: group.is_auto_group,
          auto_group_type: group.auto_group_type,
        },
      ])
    : Object.entries(groupsRaw).sort(([aKey, a], [bKey, b]) =>
        compareGroupOptionsByRatio(
          { value: aKey, label: a.label || aKey, ratio: a.ratio },
          { value: bKey, label: b.label || bKey, ratio: b.ratio }
        )
      )
  const groups: ApiKeyGroupOption[] = groupsSource.map(([key, info]) => {
    const health = groupHealthMap.get(key)
    return {
      value: key,
      label: info.is_auto_group
        ? getRuleAutoGroupLabel(key, info.label || key, t)
        : info.label || key,
      desc: info.desc || key,
      ratio: info.ratio,
      isAutoGroup: info.is_auto_group,
      autoGroupType: info.auto_group_type,
      health: {
        availability24h:
          health && health.request_count > 0 ? health.success_rate : null,
        availability2h: calculateWindowAvailability(health, 120),
        ...calculateRecentAvailability(health),
      },
    }
  })
  const backendHasAuto = groups.some((g) => g.value === 'auto')
  const concreteGroups = groups.filter((g) => !g.isAutoGroup)
  const schema = getApiKeyFormSchema(t)

  const form = useForm<ApiKeyFormValues>({
    resolver: zodResolver(schema),
    defaultValues: getApiKeyFormDefaultValues(defaultUseAutoGroup),
  })

  // Load existing data when updating
  useEffect(() => {
    if (open && isUpdate && currentRow) {
      setEditingApiKey(currentRow)
      getApiKey(currentRow.id).then((result) => {
        if (result.success && result.data) {
          setEditingApiKey(result.data)
          form.reset(transformApiKeyToFormDefaults(result.data))
        }
      })
    } else if (open && !isUpdate) {
      setEditingApiKey(undefined)
      form.reset(
        getApiKeyFormDefaultValues(defaultUseAutoGroup && backendHasAuto)
      )
    } else if (!open) {
      setEditingApiKey(undefined)
    }
  }, [open, isUpdate, currentRow, form, defaultUseAutoGroup, backendHasAuto])

  // Correct group after groups load: if the form value is not in available groups, fall back
  useEffect(() => {
    if (groups.length === 0) return
    const currentGroup = form.getValues('group')
    if (currentGroup && !groups.some((g) => g.value === currentGroup)) {
      const fallback =
        groups.find((g) => g.value === 'default')?.value ??
        groups[0]?.value ??
        ''
      form.setValue('group', fallback)
      if (currentGroup === 'auto' || isRuleAutoGroupName(currentGroup || '')) {
        form.setValue('cross_group_retry', false)
        form.setValue('auto_group_mode', 'low_ratio')
      }
    }
  }, [groups, form])

  const onSubmit = async (data: ApiKeyFormValues) => {
    setIsSubmitting(true)
    try {
      const basePayload = transformFormDataToPayload(data)

      if (isUpdate && currentRow) {
        const result = await updateApiKey({
          ...basePayload,
          id: currentRow.id,
        })
        if (result.success) {
          toast.success(t(SUCCESS_MESSAGES.API_KEY_UPDATED))
          onOpenChange(false)
          triggerRefresh()
        } else {
          toast.error(result.message || t(ERROR_MESSAGES.UPDATE_FAILED))
        }
      } else {
        // Create mode - handle batch creation
        const count = data.tokenCount || 1
        let successCount = 0

        for (let i = 0; i < count; i++) {
          const result = await createApiKey({
            ...basePayload,
            name:
              i === 0 && data.name
                ? data.name
                : `${data.name || 'default'}-${Math.random().toString(36).slice(2, 8)}`,
          })
          if (result.success) {
            successCount++
          } else {
            toast.error(result.message || t(ERROR_MESSAGES.CREATE_FAILED))
            break
          }
        }

        if (successCount > 0) {
          toast.success(
            t('Successfully created {{count}} API Key(s)', {
              count: successCount,
            })
          )
          onOpenChange(false)
          triggerRefresh()
        }
      }
    } catch (_error) {
      toast.error(t(ERROR_MESSAGES.UNEXPECTED))
    } finally {
      setIsSubmitting(false)
    }
  }

  const onInvalid: SubmitErrorHandler<ApiKeyFormValues> = () => {
    toast.error(t('Please fix the highlighted fields before saving'))
  }

  const handleSetExpiry = (months: number, days: number, hours: number) => {
    if (months === 0 && days === 0 && hours === 0) {
      form.setValue('expired_time', undefined)
      return
    }

    const now = new Date()
    now.setMonth(now.getMonth() + months)
    now.setDate(now.getDate() + days)
    now.setHours(now.getHours() + hours)

    form.setValue('expired_time', now)
  }

  const { meta: currencyMeta } = getCurrencyDisplay()
  const currencyLabel = getCurrencyLabel()
  const tokensOnly = currencyMeta.kind === 'tokens'
  const quotaLabel = t('Quota ({{currency}})', { currency: currencyLabel })
  const quotaPlaceholder = tokensOnly
    ? t('Enter quota in tokens')
    : t('Enter quota in {{currency}}', { currency: currencyLabel })
  const selectedGroup = form.watch('group')
  const selectedGroupOption = groups.find(
    (group) => group.value === selectedGroup
  )
  const selectedRuleAutoGroup =
    !!selectedGroupOption?.isAutoGroup && selectedGroup !== 'auto'
  const autoGroupMode = form.watch('auto_group_mode') || 'low_ratio'
  const unlimitedQuota = form.watch('unlimited_quota')
  const sessionFailoverEnabled = form.watch('session_group_failover_enabled')
  const sessionFailoverGroups = form.watch('session_failover_groups') || []

  useEffect(() => {
    if (!selectedRuleAutoGroup) return
    form.setValue('cross_group_retry', false)
    if (form.getValues('session_group_failover_enabled')) {
      form.setValue('session_group_failover_enabled', false)
      form.setValue('session_failover_groups', [])
    }
  }, [form, selectedRuleAutoGroup])
  const failoverRuntime =
    editingApiKey?.api_key_group_failover_runtime ??
    currentRow?.api_key_group_failover_runtime
  const hasFailoverRuntime = isUpdate && sessionFailoverEnabled
  const currentFailoverLevel =
    hasFailoverRuntime && failoverRuntime?.current_level !== undefined
      ? Math.max(0, failoverRuntime.current_level)
      : 0
  const currentFailoverFailureCount =
    hasFailoverRuntime && failoverRuntime?.failure_count !== undefined
      ? Math.max(0, failoverRuntime.failure_count)
      : 0

  const setFailoverGroups = (nextGroups: string[]) => {
    form.setValue('session_failover_groups', nextGroups, {
      shouldDirty: true,
      shouldValidate: true,
    })
    if (nextGroups.length > 0) {
      form.setValue('group', nextGroups[0], { shouldDirty: true })
    }
  }

  const getFailoverOptions = (index: number) => {
    const used = new Set(
      sessionFailoverGroups.filter((_, groupIndex) => groupIndex !== index)
    )
    return concreteGroups.filter((group) => !used.has(group.value))
  }

  const handleFailoverEnabledChange = (checked: boolean) => {
    form.setValue('session_group_failover_enabled', checked, {
      shouldDirty: true,
      shouldValidate: true,
    })
    if (!checked) {
      return
    }
    const currentGroups = form.getValues('session_failover_groups') || []
    if (currentGroups.length >= 2) {
      setFailoverGroups(currentGroups)
      return
    }
    const primary =
      selectedGroup && !selectedGroup.startsWith('auto')
        ? selectedGroup
        : concreteGroups[0]?.value
    const secondary = concreteGroups.find((group) => group.value !== primary)
    const nextGroups = [primary, secondary?.value].filter(
      (group): group is string => group !== undefined
    )
    setFailoverGroups(nextGroups)
    form.setValue('cross_group_retry', false, { shouldDirty: true })
  }

  const handleAddFailoverGroup = () => {
    const next = concreteGroups.find(
      (group) => !sessionFailoverGroups.includes(group.value)
    )
    if (!next) return
    setFailoverGroups([...sessionFailoverGroups, next.value])
  }

  const handleMoveFailoverGroup = (index: number, direction: -1 | 1) => {
    const next = [...sessionFailoverGroups]
    const target = index + direction
    if (target < 0 || target >= next.length) return
    ;[next[index], next[target]] = [next[target], next[index]]
    setFailoverGroups(next)
  }

  const resetFailoverDragState = () => {
    setDraggedFailoverIndex(null)
    setDragOverFailoverIndex(null)
    setDragOverFailoverPosition('before')
  }

  const handleFailoverDragStart = (
    event: DragEvent<HTMLButtonElement>,
    index: number
  ) => {
    setDraggedFailoverIndex(index)
    event.dataTransfer.effectAllowed = 'move'
    event.dataTransfer.setData('text/plain', String(index))
  }

  const handleFailoverDragOver = (
    event: DragEvent<HTMLDivElement>,
    index: number
  ) => {
    if (draggedFailoverIndex === null || draggedFailoverIndex === index) {
      return
    }
    event.preventDefault()
    event.dataTransfer.dropEffect = 'move'

    const rect = event.currentTarget.getBoundingClientRect()
    const position: FailoverDragPosition =
      event.clientY < rect.top + rect.height / 2 ? 'before' : 'after'
    setDragOverFailoverIndex(index)
    setDragOverFailoverPosition(position)
  }

  const handleFailoverDrop = (
    event: DragEvent<HTMLDivElement>,
    index: number
  ) => {
    event.preventDefault()
    if (draggedFailoverIndex === null || draggedFailoverIndex === index) {
      resetFailoverDragState()
      return
    }

    const next = [...sessionFailoverGroups]
    const [draggedGroup] = next.splice(draggedFailoverIndex, 1)
    if (!draggedGroup) {
      resetFailoverDragState()
      return
    }

    let targetIndex = dragOverFailoverPosition === 'after' ? index + 1 : index
    if (draggedFailoverIndex < targetIndex) {
      targetIndex -= 1
    }
    next.splice(
      Math.max(0, Math.min(targetIndex, next.length)),
      0,
      draggedGroup
    )
    setFailoverGroups(next)
    resetFailoverDragState()
  }

  const handleRemoveFailoverGroup = (index: number) => {
    setFailoverGroups(
      sessionFailoverGroups.filter((_, groupIndex) => groupIndex !== index)
    )
  }

  const handleResetFailoverToP0 = async () => {
    if (!currentRow) return

    setIsResettingFailover(true)
    try {
      const result = await resetApiKeyFailoverToP0(currentRow.id)
      if (result.success) {
        if (result.data) {
          setEditingApiKey(result.data)
        }
        toast.success(`${t('Reset')} P0`)
        triggerRefresh()
      } else {
        toast.error(result.message || t(ERROR_MESSAGES.UPDATE_FAILED))
      }
    } catch (_error) {
      toast.error(t(ERROR_MESSAGES.UNEXPECTED))
    } finally {
      setIsResettingFailover(false)
    }
  }

  useEffect(() => {
    if (!sessionFailoverEnabled || sessionFailoverGroups.length === 0) return
    if (sessionFailoverGroups[0] !== selectedGroup) {
      form.setValue('group', sessionFailoverGroups[0])
    }
  }, [form, selectedGroup, sessionFailoverEnabled, sessionFailoverGroups])

  return (
    <Sheet
      open={open}
      onOpenChange={(v) => {
        onOpenChange(v)
        if (!v) {
          form.reset()
        }
      }}
    >
      <SheetContent
        className={sideDrawerContentClassName('max-w-none sm:!max-w-[620px]')}
      >
        <SheetHeader className={sideDrawerHeaderClassName()}>
          <SheetTitle>
            {isUpdate ? t('Update API Key') : t('Create API Key')}
          </SheetTitle>
          <SheetDescription>
            {isUpdate
              ? t('Update the API key by providing necessary info.')
              : t('Add a new API key by providing necessary info.')}
          </SheetDescription>
        </SheetHeader>
        <Form {...form}>
          <form
            id='api-key-form'
            onSubmit={form.handleSubmit(onSubmit, onInvalid)}
            className={sideDrawerFormClassName('gap-5')}
          >
            <SideDrawerSection>
              <SideDrawerSectionHeader
                title={t('Basic Information')}
                description={t('Set API key basic information')}
                icon={<KeyRound className='size-4' />}
              />
              <FormField
                control={form.control}
                name='name'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Name')}</FormLabel>
                    <FormControl>
                      <Input {...field} placeholder={t('Enter a name')} />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='group'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Group')}</FormLabel>
                    <FormControl>
                      <ApiKeyGroupCombobox
                        options={groups}
                        value={field.value}
                        onValueChange={field.onChange}
                        placeholder={t('Select a group')}
                        disabled={sessionFailoverEnabled}
                      />
                    </FormControl>
                    {sessionFailoverEnabled && (
                      <FormDescription>
                        {t('P0 is controlled by the API key failover chain')}
                      </FormDescription>
                    )}
                    <FormMessage />
                  </FormItem>
                )}
              />

              {selectedGroup === 'auto' && (
                <FormField
                  control={form.control}
                  name='cross_group_retry'
                  render={({ field }) => (
                    <FormItem className={sideDrawerSwitchItemClassName()}>
                      <div className='flex flex-col gap-0.5'>
                        <FormLabel className='text-sm'>
                          {t('Cross-group retry')}
                        </FormLabel>
                        <FormDescription className='line-clamp-2 text-xs sm:line-clamp-none'>
                          {t(
                            'When enabled, if channels in the current group fail, it will try channels in the next group in order.'
                          )}
                        </FormDescription>
                      </div>
                      <FormControl>
                        <Switch
                          checked={!!field.value}
                          onCheckedChange={field.onChange}
                        />
                      </FormControl>
                    </FormItem>
                  )}
                />
              )}

              {selectedRuleAutoGroup && (
                <FormField
                  control={form.control}
                  name='auto_group_mode'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Auto group mode')}</FormLabel>
                      <FormControl>
                        <ToggleGroup
                          value={[autoGroupMode]}
                          variant='outline'
                          size='sm'
                          onValueChange={(values) => {
                            const next = values[0]
                            if (next === 'low_ratio' || next === 'balanced') {
                              field.onChange(next)
                            }
                          }}
                          className='w-full'
                          aria-label={t('Auto group mode')}
                        >
                          <ToggleGroupItem value='low_ratio' className='flex-1'>
                            {t('Low ratio first')}
                          </ToggleGroupItem>
                          <ToggleGroupItem value='balanced' className='flex-1'>
                            {t('Balanced mode')}
                          </ToggleGroupItem>
                        </ToggleGroup>
                      </FormControl>
                      <FormDescription>
                        {t(
                          'Balanced mode switches after two consecutive first-token delays over 10 seconds.'
                        )}
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              )}

              <FormField
                control={form.control}
                name='session_group_failover_enabled'
                render={({ field }) => (
                  <FormItem className={sideDrawerSwitchItemClassName()}>
                    <div className='flex flex-col gap-0.5'>
                      <FormLabel className='text-sm'>
                        {t('API key group failover')}
                      </FormLabel>
                      <FormDescription className='line-clamp-2 text-xs sm:line-clamp-none'>
                        {t(
                          'Keeps this API key on the current priority group and moves to the next group after consecutive final failures.'
                        )}
                      </FormDescription>
                    </div>
                    <FormControl>
                      <Switch
                        checked={!!field.value}
                        onCheckedChange={handleFailoverEnabledChange}
                        disabled={selectedRuleAutoGroup}
                      />
                    </FormControl>
                  </FormItem>
                )}
              />

              {sessionFailoverEnabled && (
                <>
                  <FormField
                    control={form.control}
                    name='session_failover_groups'
                    render={() => (
                      <FormItem>
                        <div className='flex items-center justify-between gap-3'>
                          <FormLabel>{t('Failover group chain')}</FormLabel>
                          <div className='flex shrink-0 items-center gap-2'>
                            {isUpdate && (
                              <Button
                                type='button'
                                variant='outline'
                                size='sm'
                                onClick={handleResetFailoverToP0}
                                disabled={
                                  isResettingFailover ||
                                  (currentFailoverLevel === 0 &&
                                    currentFailoverFailureCount === 0)
                                }
                              >
                                <RotateCcw
                                  className={cn(
                                    'size-4',
                                    isResettingFailover && 'animate-spin'
                                  )}
                                />
                                {t('Reset')} P0
                              </Button>
                            )}
                            <Button
                              type='button'
                              variant='outline'
                              size='sm'
                              onClick={handleAddFailoverGroup}
                              disabled={
                                sessionFailoverGroups.length >=
                                concreteGroups.length
                              }
                            >
                              <Plus className='size-4' />
                              {t('Add group')}
                            </Button>
                          </div>
                        </div>
                        <div className='flex flex-col gap-2'>
                          {sessionFailoverGroups.map((group, index) => {
                            const isCurrent =
                              index === currentFailoverLevel &&
                              hasFailoverRuntime
                            const isDragging = draggedFailoverIndex === index
                            const isDragOver =
                              dragOverFailoverIndex === index &&
                              draggedFailoverIndex !== null &&
                              draggedFailoverIndex !== index
                            return (
                              <div
                                key={index}
                                onDragOver={(event) =>
                                  handleFailoverDragOver(event, index)
                                }
                                onDrop={(event) =>
                                  handleFailoverDrop(event, index)
                                }
                                onDragLeave={(event) => {
                                  if (
                                    !event.currentTarget.contains(
                                      event.relatedTarget as Node | null
                                    )
                                  ) {
                                    setDragOverFailoverIndex(null)
                                  }
                                }}
                                className={cn(
                                  'border-border bg-muted/20 relative flex items-center gap-2 rounded-md border p-2 transition-[opacity,box-shadow,border-color,background-color]',
                                  isCurrent &&
                                    'border-success/70 bg-success/5 ring-success/25 ring-1',
                                  isDragging && 'opacity-50',
                                  isDragOver &&
                                    dragOverFailoverPosition === 'before' &&
                                    'before:bg-primary before:absolute before:-top-1 before:right-2 before:left-2 before:h-0.5 before:rounded-full',
                                  isDragOver &&
                                    dragOverFailoverPosition === 'after' &&
                                    'after:bg-primary after:absolute after:-right-2 after:-bottom-1 after:left-2 after:h-0.5 after:rounded-full'
                                )}
                              >
                                <Button
                                  type='button'
                                  variant='ghost'
                                  size='icon'
                                  draggable={sessionFailoverGroups.length > 1}
                                  onDragStart={(event) =>
                                    handleFailoverDragStart(event, index)
                                  }
                                  onDragEnd={resetFailoverDragState}
                                  disabled={sessionFailoverGroups.length <= 1}
                                  className='cursor-grab active:cursor-grabbing'
                                  aria-label={t('Drag to reorder')}
                                >
                                  <GripVertical className='size-4' />
                                </Button>
                                <div className='flex w-10 shrink-0 flex-col items-stretch gap-1'>
                                  <Badge
                                    variant='outline'
                                    className={cn(
                                      'w-full',
                                      isCurrent && 'border-success text-success'
                                    )}
                                  >
                                    P{index}
                                  </Badge>
                                  {isCurrent && (
                                    <Badge
                                      variant='outline'
                                      className='border-success bg-success/10 text-success w-full px-1 text-[10px]'
                                    >
                                      {t('Current')}
                                    </Badge>
                                  )}
                                </div>
                                <div className='min-w-0 flex-1'>
                                  <ApiKeyGroupCombobox
                                    options={getFailoverOptions(index)}
                                    value={group}
                                    onValueChange={(value) => {
                                      const next = [...sessionFailoverGroups]
                                      next[index] = value
                                      setFailoverGroups(next)
                                    }}
                                    placeholder={t('Select a group')}
                                  />
                                </div>
                                <div className='flex shrink-0 items-center gap-1'>
                                  <Button
                                    type='button'
                                    variant='ghost'
                                    size='icon'
                                    onClick={() =>
                                      handleMoveFailoverGroup(index, -1)
                                    }
                                    disabled={index === 0}
                                    aria-label={t('Move up')}
                                  >
                                    <ArrowUp className='size-4' />
                                  </Button>
                                  <Button
                                    type='button'
                                    variant='ghost'
                                    size='icon'
                                    onClick={() =>
                                      handleMoveFailoverGroup(index, 1)
                                    }
                                    disabled={
                                      index === sessionFailoverGroups.length - 1
                                    }
                                    aria-label={t('Move down')}
                                  >
                                    <ArrowDown className='size-4' />
                                  </Button>
                                  <Button
                                    type='button'
                                    variant='ghost'
                                    size='icon'
                                    onClick={() =>
                                      handleRemoveFailoverGroup(index)
                                    }
                                    aria-label={t('Remove')}
                                  >
                                    <Trash2 className='size-4' />
                                  </Button>
                                </div>
                              </div>
                            )
                          })}
                        </div>
                        <FormDescription>
                          {t(
                            'This API key starts at P0 and keeps the promoted priority globally until the Redis state is reset.'
                          )}
                        </FormDescription>
                        <FormMessage />
                      </FormItem>
                    )}
                  />

                  <FormField
                    control={form.control}
                    name='session_failover_threshold'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>
                          {t('Consecutive failure threshold')}
                        </FormLabel>
                        <FormControl>
                          <Input
                            {...field}
                            type='number'
                            min='1'
                            step='1'
                            onChange={(e) =>
                              field.onChange(parseInt(e.target.value, 10) || 1)
                            }
                          />
                        </FormControl>
                        <FormDescription>
                          {t(
                            'This API key moves to the next priority group after this many final failed requests.'
                          )}
                        </FormDescription>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                </>
              )}

              <FormField
                control={form.control}
                name='expired_time'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Expiration Time')}</FormLabel>
                    <div className='grid gap-2 sm:grid-cols-[minmax(0,1fr)_auto] sm:items-center'>
                      <FormControl>
                        <DateTimePicker
                          value={field.value}
                          onChange={field.onChange}
                          placeholder={t('Never expires')}
                          className='min-w-0 [&_input[type=time]]:w-24 sm:[&_input[type=time]]:w-32'
                        />
                      </FormControl>
                      <div className='grid grid-cols-4 gap-2 sm:flex'>
                        <Button
                          type='button'
                          variant='outline'
                          size='sm'
                          className='px-2 text-xs sm:px-3 sm:text-sm'
                          onClick={() => handleSetExpiry(0, 0, 0)}
                        >
                          {t('Never')}
                        </Button>
                        <Button
                          type='button'
                          variant='outline'
                          size='sm'
                          className='px-2 text-xs sm:px-3 sm:text-sm'
                          onClick={() => handleSetExpiry(1, 0, 0)}
                        >
                          {t('1 Month')}
                        </Button>
                        <Button
                          type='button'
                          variant='outline'
                          size='sm'
                          className='px-2 text-xs sm:px-3 sm:text-sm'
                          onClick={() => handleSetExpiry(0, 1, 0)}
                        >
                          {t('1 Day')}
                        </Button>
                        <Button
                          type='button'
                          variant='outline'
                          size='sm'
                          className='px-2 text-xs sm:px-3 sm:text-sm'
                          onClick={() => handleSetExpiry(0, 0, 1)}
                        >
                          {t('1 Hour')}
                        </Button>
                      </div>
                    </div>
                    <FormMessage />
                  </FormItem>
                )}
              />

              {!isUpdate && (
                <FormField
                  control={form.control}
                  name='tokenCount'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Quantity')}</FormLabel>
                      <FormControl>
                        <Input
                          {...field}
                          type='number'
                          min='1'
                          placeholder={t('Number of keys to create')}
                          onChange={(e) =>
                            field.onChange(parseInt(e.target.value, 10) || 1)
                          }
                        />
                      </FormControl>
                      <FormDescription>
                        {t(
                          'Create multiple API keys at once (random suffix will be added to names)'
                        )}
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              )}
            </SideDrawerSection>

            <SideDrawerSection>
              <SideDrawerSectionHeader
                title={t('Quota Settings')}
                description={t('Set quota amount and limits')}
                icon={<WalletCards className='size-4' />}
              />
              {!unlimitedQuota && (
                <FormField
                  control={form.control}
                  name='remain_quota_dollars'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{quotaLabel}</FormLabel>
                      <FormControl>
                        <Input
                          {...field}
                          type='number'
                          step={tokensOnly ? 1 : 0.01}
                          placeholder={quotaPlaceholder}
                          onChange={(e) =>
                            field.onChange(parseFloat(e.target.value) || 0)
                          }
                        />
                      </FormControl>
                      <FormDescription>
                        {tokensOnly
                          ? t('Enter the quota amount in tokens')
                          : t('Enter the quota amount in {{currency}}', {
                              currency: currencyLabel,
                            })}
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              )}

              <FormField
                control={form.control}
                name='unlimited_quota'
                render={({ field }) => (
                  <FormItem className={sideDrawerSwitchItemClassName()}>
                    <div className='flex flex-col gap-0.5'>
                      <FormLabel className='text-sm'>
                        {t('Unlimited Quota')}
                      </FormLabel>
                      <FormDescription className='text-xs'>
                        {t('Enable unlimited quota for this API key')}
                      </FormDescription>
                    </div>
                    <FormControl>
                      <Switch
                        checked={field.value}
                        onCheckedChange={field.onChange}
                      />
                    </FormControl>
                  </FormItem>
                )}
              />
            </SideDrawerSection>

            <Collapsible open={advancedOpen} onOpenChange={setAdvancedOpen}>
              <SideDrawerSection>
                <CollapsibleTrigger
                  render={
                    <button
                      type='button'
                      className='hover:bg-muted/40 flex w-full items-center gap-3 rounded-md py-1.5 text-left transition-colors'
                    />
                  }
                >
                  <SideDrawerSectionHeader
                    className='flex-1'
                    title={t('Advanced Settings')}
                    description={t('Set API key access restrictions')}
                    icon={<Settings2 className='size-4' />}
                  />
                  <ChevronDown
                    className={cn(
                      'text-muted-foreground size-4 shrink-0 transition-transform',
                      advancedOpen && 'rotate-180'
                    )}
                  />
                </CollapsibleTrigger>
                <CollapsibleContent>
                  <div className='flex flex-col gap-4 pt-2'>
                    <FormField
                      control={form.control}
                      name='model_limits'
                      render={({ field }) => (
                        <FormItem>
                          <FormLabel>{t('Model Limits')}</FormLabel>
                          <FormControl>
                            <MultiSelect
                              options={models.map((m) => ({
                                label: m,
                                value: m,
                              }))}
                              selected={field.value}
                              onChange={field.onChange}
                              placeholder={t(
                                'Select models (empty for allow all)'
                              )}
                            />
                          </FormControl>
                          <FormDescription>
                            {t('Limit which models can be used with this key')}
                          </FormDescription>
                          <FormMessage />
                        </FormItem>
                      )}
                    />

                    <FormField
                      control={form.control}
                      name='allow_ips'
                      render={({ field }) => (
                        <FormItem>
                          <FormLabel>
                            {t('IP Whitelist (supports CIDR)')}
                          </FormLabel>
                          <FormControl>
                            <Textarea
                              {...field}
                              className='min-h-20 resize-none'
                              placeholder={t(
                                'One IP per line (empty for no restriction)'
                              )}
                              rows={3}
                            />
                          </FormControl>
                          <FormDescription>
                            {t(
                              'Do not over-trust this feature. IP may be spoofed. Please use with nginx, CDN and other gateways.'
                            )}
                          </FormDescription>
                          <FormMessage />
                        </FormItem>
                      )}
                    />
                  </div>
                </CollapsibleContent>
              </SideDrawerSection>
            </Collapsible>
          </form>
        </Form>
        <SheetFooter className={sideDrawerFooterClassName()}>
          <SheetClose
            render={<Button variant='outline' className='w-full sm:w-auto' />}
          >
            {t('Close')}
          </SheetClose>
          <Button
            type='button'
            onClick={form.handleSubmit(onSubmit, onInvalid)}
            disabled={isSubmitting}
            className='w-full sm:w-auto'
          >
            {isSubmitting ? t('Saving...') : t('Save changes')}
          </Button>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  )
}

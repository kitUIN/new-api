import { useQuery } from '@tanstack/react-query'
import { zodResolver } from '@hookform/resolvers/zod'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { z } from 'zod'
import { api } from '@/lib/api'
import { Button } from '@/components/ui/button'
import { Form, FormControl, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'
import type { OperationsSettings } from '../types'

const schema = z.object({
  cache_mb: z.number().int().min(1).max(16384),
  cache_dir: z.string().trim().min(1),
  ttl_seconds: z.number().int().min(60).max(86400),
})
type Values = z.infer<typeof schema>
type Stats = { entries: number; payload_bytes: number; capacity_bytes: number }

export function RelayAssetSection(props: { settings: OperationsSettings }) {
  const { t } = useTranslation()
  const update = useUpdateOption()
  const form = useForm<Values>({
    resolver: zodResolver(schema),
    values: {
      cache_mb: props.settings['relay_asset_setting.cache_mb'],
      cache_dir: props.settings['relay_asset_setting.cache_dir'],
      ttl_seconds: props.settings['relay_asset_setting.ttl_seconds'],
    },
    resetOptions: { keepDirtyValues: true },
  })
  const stats = useQuery({
    queryKey: ['relay-asset-stats'],
    queryFn: async () => {
      const response = await api.get<{ success: boolean; data: Stats }>('/api/performance/relay_assets')
      if (!response.data.success) throw new Error(t('Failed to load cache statistics'))
      return response.data.data
    },
    refetchInterval: 30000,
  })
  const submit = async (values: Values) => {
    for (const key of ['cache_mb', 'cache_dir', 'ttl_seconds'] as const) {
      const result = await update.mutateAsync({ key: `relay_asset_setting.${key}`, value: String(values[key]) })
      if (!result.success) return
    }
    form.reset(values)
  }
  return (
    <SettingsSection title={t('Temporary URL cache')}>
      <p className='text-muted-foreground text-sm'>{t('Restart required. Environment variables override these settings. Each instance needs its own local directory and reachable URL; anyone holding an unexpired URL can read the asset.')}</p>
      <Form {...form}>
        <form onSubmit={form.handleSubmit(submit)} className='space-y-4'>
          <FormField control={form.control} name='cache_mb' render={({ field }) => (
            <FormItem><FormLabel>{t('Reserved capacity (MiB)')}</FormLabel><FormControl><Input type='number' min={1} max={16384} {...field} onChange={(event) => field.onChange(event.target.valueAsNumber)} /></FormControl><FormMessage /></FormItem>
          )} />
          <FormField control={form.control} name='cache_dir' render={({ field }) => (
            <FormItem><FormLabel>{t('Cache directory')}</FormLabel><FormControl><Input {...field} /></FormControl><FormMessage /></FormItem>
          )} />
          <FormField control={form.control} name='ttl_seconds' render={({ field }) => (
            <FormItem><FormLabel>{t('Signed URL lifetime (seconds)')}</FormLabel><FormControl><Input type='number' min={60} max={86400} {...field} onChange={(event) => field.onChange(event.target.valueAsNumber)} /></FormControl><FormMessage /></FormItem>
          )} />
          <Button type='submit' disabled={form.formState.isSubmitting}>{t('Save')}</Button>
        </form>
      </Form>
      {stats.isError && <p role='alert'>{t('Failed to load cache statistics')}</p>}
      {stats.data && <p className='text-sm'>{t('Cache entries: {{entries}}; payload: {{payload}} MiB; reserved: {{capacity}} MiB', {
        entries: stats.data.entries,
        payload: (stats.data.payload_bytes / 1048576).toFixed(2),
        capacity: (stats.data.capacity_bytes / 1048576).toFixed(0),
      })}</p>}
    </SettingsSection>
  )
}

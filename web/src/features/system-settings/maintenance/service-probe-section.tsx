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
import { Controller, useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { api } from '@/lib/api'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import {
  Field,
  FieldContent,
  FieldDescription,
  FieldError,
  FieldGroup,
  FieldLabel,
} from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { Skeleton } from '@/components/ui/skeleton'
import { Switch } from '@/components/ui/switch'
import { FormNavigationGuard } from '../components/form-navigation-guard'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import {
  serviceProbeSettingsSchema,
  type ServiceProbeSettingsValues,
} from './lib/service-probe-schema'

type ConfigView = { enabled: boolean; url: string; key_configured: boolean }
type Response = { success: boolean; message?: string; data: ConfigView }
const settingsPath = '/api/option/service-probes'
const settingsQueryKey = ['service-probe-settings']
const requestOptions = {
  headers: {},
  skipBusinessError: true,
  skipErrorHandler: true,
}

function unwrap(response: Response): ConfigView {
  if (!response.success)
    throw new Error(response.message || 'serviceProbe.settings.saveFailed')
  return response.data
}

export function ServiceProbeSection() {
  const { t } = useTranslation()
  const query = useQuery({
    queryKey: settingsQueryKey,
    queryFn: async () =>
      unwrap(
        (await api.get<Response>(settingsPath, { ...requestOptions })).data
      ),
    refetchOnWindowFocus: false,
    gcTime: 0,
  })
  if (query.isPending) return <Skeleton className='h-48 max-w-2xl' />
  if (query.isError)
    return (
      <Alert variant='destructive'>
        <AlertDescription>
          {t(query.error.message, {
            defaultValue: t('serviceProbe.loadFailed'),
          })}
          <Button variant='outline' onClick={() => void query.refetch()}>
            {t('Retry')}
          </Button>
        </AlertDescription>
      </Alert>
    )
  return <ServiceProbeSettingsForm config={query.data} />
}

function ServiceProbeSettingsForm(props: { config: ConfigView }) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const form = useForm<ServiceProbeSettingsValues>({
    resolver: zodResolver(
      serviceProbeSettingsSchema(t, props.config.key_configured)
    ),
    defaultValues: {
      enabled: props.config.enabled,
      url: props.config.url,
      read_api_key: '',
    },
  })
  const save = useMutation({
    mutationFn: async (values: ServiceProbeSettingsValues) =>
      unwrap(
        (await api.put<Response>(settingsPath, values, { ...requestOptions }))
          .data
      ),
    gcTime: 0,
    onSuccess: (config) => {
      form.reset({ enabled: config.enabled, url: config.url, read_api_key: '' })
      queryClient.setQueryData(settingsQueryKey, config)
      void queryClient.invalidateQueries({ queryKey: ['service-probes'] })
      toast.success(t('Setting updated successfully'))
    },
    onError: (error) =>
      toast.error(
        t(error.message, {
          defaultValue: t('serviceProbe.settings.saveFailed'),
        })
      ),
  })
  const submit = form.handleSubmit((values) => save.mutate(values))
  return (
    <SettingsSection title={t('serviceProbe.title')}>
      <FormNavigationGuard when={form.formState.isDirty && !save.isPending} />
      <SettingsPageFormActions
        onSave={() => void submit()}
        isSaving={save.isPending}
        isSaveDisabled={!form.formState.isDirty}
      />
      <form onSubmit={submit} className='max-w-2xl'>
        <FieldGroup>
          <Field orientation='horizontal'>
            <FieldContent>
              <FieldLabel htmlFor='hive-enabled'>
                {t('serviceProbe.settings.enabled')}
              </FieldLabel>
              <FieldDescription>
                {t('serviceProbe.settings.immediate')}
              </FieldDescription>
            </FieldContent>
            <Controller
              control={form.control}
              name='enabled'
              render={({ field }) => (
                <Switch
                  id='hive-enabled'
                  checked={field.value}
                  onCheckedChange={field.onChange}
                  disabled={save.isPending}
                />
              )}
            />
          </Field>
          <Field data-invalid={!!form.formState.errors.url}>
            <FieldLabel htmlFor='hive-url'>
              {t('serviceProbe.settings.url')}
            </FieldLabel>
            <Input
              id='hive-url'
              placeholder='https://hive.example.com:7443'
              autoComplete='off'
              disabled={save.isPending}
              aria-invalid={!!form.formState.errors.url}
              {...form.register('url')}
            />
            <FieldError errors={[form.formState.errors.url]} />
          </Field>
          <Field data-invalid={!!form.formState.errors.read_api_key}>
            <FieldLabel htmlFor='hive-key'>
              {t('serviceProbe.settings.key')}
            </FieldLabel>
            <Input
              id='hive-key'
              type='password'
              autoComplete='new-password'
              disabled={save.isPending}
              placeholder={
                props.config.key_configured
                  ? t('serviceProbe.settings.keySaved')
                  : t('serviceProbe.settings.keyEmpty')
              }
              aria-invalid={!!form.formState.errors.read_api_key}
              {...form.register('read_api_key')}
            />
            <FieldError errors={[form.formState.errors.read_api_key]} />
          </Field>
        </FieldGroup>
      </form>
    </SettingsSection>
  )
}

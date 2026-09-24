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
import { useState, useEffect, useMemo } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { useAuthStore } from '@/stores/auth-store'
import { getUserModels } from '@/lib/api'
import { useStatus } from '@/hooks/use-status'
import { Button } from '@/components/ui/button'
import { ComboboxInput } from '@/components/ui/combobox-input'
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Label } from '@/components/ui/label'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import { getAccessToken } from '@/features/profile/api'

const USAGE_SCRIPT =
  'KHsKICByZXF1ZXN0OiB7CiAgICB1cmw6ICJ7e2Jhc2VVcmx9fS9hcGkvdXNlci9zZWxmIiwKICAgIG1ldGhvZDogIkdFVCIsCiAgICBoZWFkZXJzOiB7CiAgICAgICJDb250ZW50LVR5cGUiOiAiYXBwbGljYXRpb24vanNvbiIsCiAgICAgICJBdXRob3JpemF0aW9uIjogIkJlYXJlciB7e2FjY2Vzc1Rva2VufX0iLAogICAgICAiTmV3LUFwaS1Vc2VyIjogInt7dXNlcklkfX0iCiAgICB9LAogIH0sCiAgZXh0cmFjdG9yOiBmdW5jdGlvbiAocmVzcG9uc2UpIHsKICAgIGlmIChyZXNwb25zZS5zdWNjZXNzICYmIHJlc3BvbnNlLmRhdGEpIHsKICAgICAgcmV0dXJuIHsKICAgICAgICBwbGFuTmFtZTogcmVzcG9uc2UuZGF0YS5ncm91cCB8fCAi6buY6K6k5aWX6aSQIiwKICAgICAgICByZW1haW5pbmc6IHJlc3BvbnNlLmRhdGEucXVvdGEgLyA1MDAwMDAsCiAgICAgICAgdXNlZDogcmVzcG9uc2UuZGF0YS51c2VkX3F1b3RhIC8gNTAwMDAwLAogICAgICAgIHRvdGFsOiAocmVzcG9uc2UuZGF0YS5xdW90YSArIHJlc3BvbnNlLmRhdGEudXNlZF9xdW90YSkgLyA1MDAwMDAsCiAgICAgICAgdW5pdDogIlVTRCIsCiAgICAgIH07CiAgICB9CiAgICByZXR1cm4gewogICAgICBpc1ZhbGlkOiBmYWxzZSwKICAgICAgaW52YWxpZE1lc3NhZ2U6IHJlc3BvbnNlLm1lc3NhZ2UgfHwgIuafpeivouWksei0pSIKICAgIH07CiAgfSwKfSk'
const USAGE_AUTO_INTERVAL_MINUTES = 30

const APP_CONFIGS = {
  claude: {
    label: 'Claude',
    modelFields: [
      { key: 'model', labelKey: 'Primary Model', required: true },
      { key: 'haikuModel', labelKey: 'Haiku Model', required: false },
      { key: 'sonnetModel', labelKey: 'Sonnet Model', required: false },
      { key: 'opusModel', labelKey: 'Opus Model', required: false },
    ],
  },
  codex: {
    label: 'Codex',
    modelFields: [{ key: 'model', labelKey: 'Primary Model', required: true }],
  },
  gemini: {
    label: 'Gemini',
    modelFields: [{ key: 'model', labelKey: 'Primary Model', required: true }],
  },
} as const

type AppType = keyof typeof APP_CONFIGS

function normalizeBaseUrl(url?: string): string {
  return (url || '').trim().replace(/\/+$/, '')
}

function getServerAddress(status?: Record<string, unknown> | null): string {
  const statusAddress =
    typeof status?.server_address === 'string'
      ? status.server_address
      : typeof (status?.data as Record<string, unknown> | undefined)
            ?.server_address === 'string'
        ? ((status?.data as Record<string, unknown>).server_address as string)
        : ''
  if (statusAddress) return normalizeBaseUrl(statusAddress)

  try {
    const raw = localStorage.getItem('status')
    if (raw) {
      const status = JSON.parse(raw)
      if (status.server_address) return normalizeBaseUrl(status.server_address)
    }
  } catch {
    /* empty */
  }
  return normalizeBaseUrl(window.location.origin)
}

function getApiInfoBaseUrls(status?: Record<string, unknown> | null) {
  const directApiInfo = status?.api_info
  const nestedApiInfo = (status?.data as Record<string, unknown> | undefined)
    ?.api_info
  const apiInfo = Array.isArray(directApiInfo)
    ? directApiInfo
    : Array.isArray(nestedApiInfo)
      ? nestedApiInfo
      : []

  return apiInfo
    .map((item) => {
      const apiInfoItem = item as Record<string, unknown>
      const url =
        typeof apiInfoItem.url === 'string'
          ? normalizeBaseUrl(apiInfoItem.url)
          : ''
      const route =
        typeof apiInfoItem.route === 'string' ? apiInfoItem.route.trim() : ''
      const description =
        typeof apiInfoItem.description === 'string'
          ? apiInfoItem.description.trim()
          : ''
      return {
        value: url,
        label: [route, description].filter(Boolean).join(' - ') || url,
        isDefault: Boolean(apiInfoItem.is_default),
      }
    })
    .filter((item) => item.value)
}

function getDefaultBaseUrl(status?: Record<string, unknown> | null): string {
  return (
    getApiInfoBaseUrls(status).find((item) => item.isDefault)?.value ||
    getServerAddress(status)
  )
}

function buildCCSwitchURL(
  app: string,
  name: string,
  models: Record<string, string>,
  apiKey: string,
  accessToken: string,
  userId: number,
  baseUrl: string
): string {
  const serverAddress = normalizeBaseUrl(baseUrl) || getServerAddress()
  const endpoint = app === 'codex' ? serverAddress + '/v1' : serverAddress
  const params = new URLSearchParams()
  params.set('resource', 'provider')
  params.set('app', app)
  params.set('name', name)
  params.set('endpoint', endpoint)
  params.set('apiKey', apiKey)
  for (const [k, v] of Object.entries(models)) {
    if (v) params.set(k, v)
  }
  params.set('homepage', serverAddress)
  params.set('enabled', 'true')
  params.set('usageEnabled', 'true')
  params.set('usageApiKey', apiKey)
  params.set('usageAccessToken', accessToken)
  params.set('usageBaseUrl', serverAddress)
  params.set('usageUserId', String(userId))
  params.set('usageScript', USAGE_SCRIPT)
  params.set('usageAutoInterval', String(USAGE_AUTO_INTERVAL_MINUTES))
  return `ccswitch://v1/import?${params.toString()}`
}

function buildDefaultName(apiKeyName?: string): string {
  const trimmedName = apiKeyName?.trim()
  return `大猫猫站-${trimmedName || 'API Key'}`
}

interface Props {
  open: boolean
  onOpenChange: (open: boolean) => void
  tokenKey: string
  tokenName?: string
}

export function CCSwitchDialog(props: Props) {
  const { t } = useTranslation()
  const userId = useAuthStore((state) => state.auth.user?.id)
  const { status } = useStatus()
  const [app, setApp] = useState<AppType>('claude')
  const [name, setName] = useState<string>(() =>
    buildDefaultName(props.tokenName)
  )
  const [models, setModels] = useState<Record<string, string>>({})
  const [baseUrl, setBaseUrl] = useState<string>(() => getDefaultBaseUrl())
  const [isSubmitting, setIsSubmitting] = useState(false)

  const { data: modelsData } = useQuery({
    queryKey: ['user-models-ccswitch'],
    queryFn: getUserModels,
    enabled: props.open,
    staleTime: 5 * 60 * 1000,
  })

  const modelOptions = useMemo(() => {
    const items = modelsData?.data ?? []
    return items.map((m) => ({ value: m, label: m }))
  }, [modelsData?.data])

  const baseUrlOptions = useMemo(() => {
    const serverAddress = getServerAddress(status)
    const options = [
      {
        value: serverAddress,
        label: `${t('Current domain')} (${serverAddress})`,
      },
      ...getApiInfoBaseUrls(status).map((item) => ({
        value: item.value,
        label: `${item.label} (${item.value})`,
      })),
    ]
    const seen = new Set<string>()
    return options.filter((item) => {
      if (!item.value || seen.has(item.value)) return false
      seen.add(item.value)
      return true
    })
  }, [status, t])

  useEffect(() => {
    if (props.open) {
      // eslint-disable-next-line react-hooks/set-state-in-effect
      setModels({})

      setApp('claude')

      setName(buildDefaultName(props.tokenName))

      setBaseUrl(getDefaultBaseUrl(status))
    }
  }, [props.open, props.tokenName, status])

  const currentConfig = APP_CONFIGS[app]

  const handleAppChange = (val: string) => {
    const appVal = val as AppType
    setApp(appVal)
    setModels({})
  }

  const handleSubmit = async () => {
    if (!models.model) {
      toast.warning(t('Please select a primary model'))
      return
    }
    const key = props.tokenKey.startsWith('sk-')
      ? props.tokenKey
      : `sk-${props.tokenKey}`
    if (!userId) {
      toast.error(t('Failed to load profile'))
      return
    }

    setIsSubmitting(true)
    try {
      const response = await getAccessToken()
      const accessToken = response.data
      if (!accessToken) {
        toast.error(response.message || t('No token found.'))
        return
      }

      const url = buildCCSwitchURL(
        app,
        name,
        models,
        key,
        accessToken,
        userId,
        baseUrl
      )
      window.open(url, '_blank')
      props.onOpenChange(false)
    } catch {
      toast.error(t('Failed to load profile'))
    } finally {
      setIsSubmitting(false)
    }
  }

  return (
    <Dialog open={props.open} onOpenChange={props.onOpenChange}>
      <DialogContent className='sm:max-w-md'>
        <DialogHeader>
          <DialogTitle>{t('CC Switch one-click import')}</DialogTitle>
        </DialogHeader>

        <div className='space-y-4'>
          <div className='space-y-2'>
            <Label>{t('Application')}</Label>
            <RadioGroup
              value={app}
              onValueChange={handleAppChange}
              className='flex gap-4'
            >
              {(
                Object.entries(APP_CONFIGS) as [
                  AppType,
                  (typeof APP_CONFIGS)[AppType],
                ][]
              ).map(([key, cfg]) => (
                <div key={key} className='flex items-center gap-2'>
                  <RadioGroupItem value={key} id={`app-${key}`} />
                  <Label htmlFor={`app-${key}`} className='cursor-pointer'>
                    {cfg.label}
                  </Label>
                </div>
              ))}
            </RadioGroup>
          </div>

          <div className='space-y-2'>
            <Label>{t('Base URL')}</Label>
            <ComboboxInput
              options={baseUrlOptions}
              value={baseUrl}
              onValueChange={setBaseUrl}
              placeholder={t('Base URL')}
              emptyText='No data'
            />
          </div>

          <div className='space-y-2'>
            <Label>{t('Name')}</Label>
            <ComboboxInput
              options={[]}
              value={name}
              onValueChange={setName}
              placeholder={buildDefaultName(props.tokenName)}
              emptyText=''
            />
          </div>

          {currentConfig.modelFields.map((field) => (
            <div key={field.key} className='space-y-2'>
              <Label>
                {t(field.labelKey)}
                {field.required && (
                  <span className='text-destructive ml-0.5'>*</span>
                )}
              </Label>
              <ComboboxInput
                options={modelOptions}
                value={models[field.key] || ''}
                onValueChange={(v) =>
                  setModels((prev) => ({ ...prev, [field.key]: v }))
                }
                placeholder={t('Select or enter model name')}
                emptyText={t('No models found')}
              />
            </div>
          ))}
        </div>

        <DialogFooter>
          <Button variant='outline' onClick={() => props.onOpenChange(false)}>
            {t('Cancel')}
          </Button>
          <Button onClick={handleSubmit} disabled={isSubmitting}>
            {isSubmitting ? t('Loading...') : t('CC Switch one-click import')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

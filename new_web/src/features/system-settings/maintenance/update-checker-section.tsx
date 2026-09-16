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
import { useCallback, useEffect, useState, type FormEvent } from 'react'
import {
  ExternalLinkIcon,
  Loader2Icon,
  RefreshCcwIcon,
  SaveIcon,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { formatTimestamp, formatTimestampToDate } from '@/lib/format'
import { Button } from '@/components/ui/button'
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
import { Markdown } from '@/components/ui/markdown'
import { ConfirmDialog } from '@/components/confirm-dialog'
import {
  SecureVerificationDialog,
  useSecureVerification,
} from '@/features/auth/secure-verification'
import { applySystemUpdate, checkSystemUpdate } from '../api'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'
import type { SystemUpdateInfo } from '../types'

const DEFAULT_GITHUB_REPOSITORY = 'Calcium-Ion/new-api'
const AUTO_CHECK_INTERVAL_MS = 15 * 60 * 1000
const RESTART_TIMEOUT_MS = 60 * 1000
const RESTART_POLL_INTERVAL_MS = 2000

type UpdateCheckerSectionProps = {
  currentVersion?: string | null
  startTime?: number | null
  githubRepository?: string | null
}

function normalizeGitHubRepository(value: string): string | null {
  const trimmed = value.trim().replace(/\/+$/, '')
  if (!trimmed) return null

  let path = trimmed
  if (/^https?:\/\//i.test(trimmed)) {
    try {
      const url = new URL(trimmed)
      if (url.hostname.toLowerCase() !== 'github.com') return null
      path = url.pathname.replace(/^\/+|\/+$/g, '')
    } catch {
      return null
    }
  }

  const parts = path.split('/')
  if (parts.length !== 2) return null

  const owner = parts[0]
  const repository = parts[1].replace(/\.git$/i, '')
  const validPart = /^[A-Za-z0-9_.-]+$/
  if (
    !owner ||
    !repository ||
    !validPart.test(owner) ||
    !validPart.test(repository)
  ) {
    return null
  }

  return `${owner}/${repository}`
}

function delay(milliseconds: number) {
  return new Promise((resolve) => window.setTimeout(resolve, milliseconds))
}

export function UpdateCheckerSection(props: UpdateCheckerSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const [checking, setChecking] = useState(false)
  const [updating, setUpdating] = useState(false)
  const [confirmOpen, setConfirmOpen] = useState(false)
  const [detailsOpen, setDetailsOpen] = useState(false)
  const [updateInfo, setUpdateInfo] = useState<SystemUpdateInfo | null>(null)
  const [checkError, setCheckError] = useState<string | null>(null)
  const [repositoryInput, setRepositoryInput] = useState(
    props.githubRepository || DEFAULT_GITHUB_REPOSITORY
  )
  const [activeRepository, setActiveRepository] = useState(
    props.githubRepository || DEFAULT_GITHUB_REPOSITORY
  )

  const {
    open: verificationOpen,
    methods: verificationMethods,
    state: verificationState,
    executeVerification,
    withVerification,
    cancel: cancelVerification,
    setCode: setVerificationCode,
    switchMethod: switchVerificationMethod,
  } = useSecureVerification()

  useEffect(() => {
    const repository = props.githubRepository || DEFAULT_GITHUB_REPOSITORY
    setRepositoryInput(repository)
    setActiveRepository(repository)
  }, [props.githubRepository])

  const uptime = props.startTime
    ? formatTimestamp(props.startTime)
    : t('Unknown')
  const currentVersion =
    updateInfo?.current_version || props.currentVersion || t('Unknown')

  const getNormalizedRepository = useCallback(
    (showError = true) => {
      const repository = normalizeGitHubRepository(repositoryInput)
      if (!repository && showError) {
        toast.error(
          t('Enter a valid GitHub repository, for example owner/repository.')
        )
      }
      return repository
    },
    [repositoryInput, t]
  )

  const checkForUpdates = useCallback(
    async (repository: string, showFeedback = false) => {
      setChecking(true)
      try {
        const response = await checkSystemUpdate(repository)
        if (!response.success || !response.data) {
          throw new Error(response.message || t('Failed to check for updates'))
        }

        setUpdateInfo(response.data)
        setCheckError(null)
        if (showFeedback) {
          if (response.data.update_available) {
            toast.success(
              t('New version available: {{version}}', {
                version: response.data.latest_version,
              })
            )
          } else {
            toast.success(
              t('You are running the latest version ({{version}}).', {
                version: response.data.current_version,
              })
            )
          }
        }
      } catch (error) {
        const message =
          error instanceof Error
            ? error.message
            : t('Failed to check for updates')
        setCheckError(message)
        if (showFeedback) toast.error(message)
      } finally {
        setChecking(false)
      }
    },
    [t]
  )

  useEffect(() => {
    const repository = normalizeGitHubRepository(activeRepository)
    if (!repository) return

    setUpdateInfo(null)
    setCheckError(null)
    void checkForUpdates(repository, false)
    const timer = window.setInterval(() => {
      void checkForUpdates(repository, false)
    }, AUTO_CHECK_INTERVAL_MS)
    return () => window.clearInterval(timer)
  }, [activeRepository, checkForUpdates])

  const handleSaveRepository = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    const repository = getNormalizedRepository()
    if (!repository) return

    const response = await updateOption.mutateAsync({
      key: 'GitHubUpdateRepository',
      value: repository,
    })
    if (!response.success) return

    setRepositoryInput(repository)
    if (normalizeGitHubRepository(activeRepository) === repository) {
      await checkForUpdates(repository, false)
    } else {
      setActiveRepository(repository)
    }
  }

  const waitForRestart = useCallback(
    async (expectedVersion: string) => {
      const deadline = Date.now() + RESTART_TIMEOUT_MS
      await delay(1000)

      while (Date.now() < deadline) {
        try {
          const response = await fetch(`/api/status?_=${Date.now()}`, {
            cache: 'no-store',
            credentials: 'include',
          })
          if (response.ok) {
            const payload = (await response.json()) as {
              data?: { version?: string }
            }
            if (payload.data?.version === expectedVersion) {
              window.location.reload()
              return
            }
          }
        } catch {
          // The service is expected to be temporarily unavailable while restarting.
        }
        await delay(RESTART_POLL_INTERVAL_MS)
      }

      setUpdating(false)
      toast.error(t('The service did not come back online in time.'))
    },
    [t]
  )

  const applyUpdate = useCallback(async () => {
    if (!updateInfo) return

    setUpdating(true)
    try {
      const response = await applySystemUpdate({
        repository: updateInfo.repository,
        version: updateInfo.latest_version,
      })
      if (!response.success) {
        throw new Error(response.message || t('Failed to update system'))
      }

      toast.info(t('Update installed. Waiting for the service to restart...'))
      void waitForRestart(updateInfo.latest_version)
      return response
    } catch (error) {
      setUpdating(false)
      throw error
    }
  }, [t, updateInfo, waitForRestart])

  const handleConfirmUpdate = async () => {
    setConfirmOpen(false)
    try {
      await withVerification(applyUpdate, {
        title: t('Confirm system update'),
        description: t('Confirm your identity before updating the system.'),
      })
    } catch (error) {
      const message =
        error instanceof Error ? error.message : t('Failed to update system')
      toast.error(message)
    }
  }

  const latestVersion = updateInfo?.latest_version || t('Not checked')
  const canInstallUpdate = Boolean(
    updateInfo?.update_available && updateInfo.can_update
  )

  return (
    <>
      <SettingsSection title={t('System maintenance')}>
        <div className='space-y-6'>
          <div className='grid gap-4 md:grid-cols-3'>
            <div className='rounded-lg border p-4'>
              <div className='text-muted-foreground text-sm'>
                {t('Current version')}
              </div>
              <div className='text-lg font-semibold break-all'>
                {currentVersion}
              </div>
            </div>
            <div className='rounded-lg border p-4'>
              <div className='text-muted-foreground text-sm'>
                {t('Latest version')}
              </div>
              <div className='text-lg font-semibold break-all'>
                {checking && !updateInfo
                  ? t('Checking updates...')
                  : latestVersion}
              </div>
            </div>
            <div className='rounded-lg border p-4'>
              <div className='text-muted-foreground text-sm'>
                {t('Uptime since')}
              </div>
              <div className='text-lg font-semibold'>{uptime}</div>
            </div>
          </div>

          <form className='space-y-2' onSubmit={handleSaveRepository}>
            <Label htmlFor='github-update-repository'>
              {t('GitHub release repository')}
            </Label>
            <div className='flex flex-col gap-2 sm:flex-row'>
              <Input
                id='github-update-repository'
                className='font-mono'
                value={repositoryInput}
                onChange={(event) => setRepositoryInput(event.target.value)}
                placeholder='owner/repository'
                autoComplete='off'
                spellCheck={false}
                disabled={updating}
              />
              <Button
                type='submit'
                variant='secondary'
                disabled={updateOption.isPending || updating}
                className='sm:shrink-0'
              >
                <SaveIcon className='me-2 h-4 w-4' />
                {t('Save')}
              </Button>
            </div>
          </form>

          {checkError && (
            <p className='text-destructive text-sm'>{checkError}</p>
          )}
          {updateInfo?.update_available && !updateInfo.can_update && (
            <p className='text-muted-foreground text-sm'>
              {t('No compatible binary is available for {{platform}}.', {
                platform: updateInfo.platform,
              })}
            </p>
          )}

          <div className='flex flex-wrap gap-2'>
            <Button
              type='button'
              variant='secondary'
              onClick={() => {
                const repository = getNormalizedRepository()
                if (repository) void checkForUpdates(repository, true)
              }}
              disabled={checking || updating}
            >
              {checking ? (
                <Loader2Icon className='me-2 h-4 w-4 animate-spin' />
              ) : (
                <RefreshCcwIcon className='me-2 h-4 w-4' />
              )}
              {checking ? t('Checking updates...') : t('Check for updates')}
            </Button>
            {updateInfo && (
              <Button
                type='button'
                variant='outline'
                onClick={() => setDetailsOpen(true)}
                disabled={updating}
              >
                <ExternalLinkIcon className='me-2 h-4 w-4' />
                {t('Release details')}
              </Button>
            )}
            {canInstallUpdate && (
              <Button
                type='button'
                onClick={() => setConfirmOpen(true)}
                disabled={updating}
              >
                {updating && (
                  <Loader2Icon className='me-2 h-4 w-4 animate-spin' />
                )}
                {updating ? t('Updating...') : t('Update and restart')}
              </Button>
            )}
          </div>
        </div>
      </SettingsSection>

      <Dialog open={detailsOpen} onOpenChange={setDetailsOpen}>
        <DialogContent className='max-h-[80vh] overflow-y-auto'>
          <DialogHeader>
            <DialogTitle>
              {updateInfo?.update_available
                ? t('New version available: {{version}}', {
                    version: updateInfo.latest_version,
                  })
                : t('Release details')}
            </DialogTitle>
            {updateInfo?.published_at && (
              <DialogDescription>
                {t('Published')}{' '}
                {formatTimestampToDate(
                  new Date(updateInfo.published_at).getTime(),
                  'milliseconds'
                )}
              </DialogDescription>
            )}
          </DialogHeader>

          {updateInfo?.release_notes ? (
            <Markdown>{updateInfo.release_notes}</Markdown>
          ) : (
            <p className='text-muted-foreground text-sm'>
              {t('No release notes provided.')}
            </p>
          )}

          <DialogFooter>
            <Button
              type='button'
              variant='secondary'
              onClick={() => setDetailsOpen(false)}
            >
              {t('Close')}
            </Button>
            {updateInfo?.release_url && (
              <Button
                type='button'
                onClick={() =>
                  window.open(
                    updateInfo.release_url,
                    '_blank',
                    'noopener,noreferrer'
                  )
                }
              >
                <ExternalLinkIcon className='me-2 h-4 w-4' />
                {t('Open release')}
              </Button>
            )}
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <ConfirmDialog
        open={confirmOpen}
        onOpenChange={setConfirmOpen}
        title={t('Confirm system update')}
        desc={t(
          'The binary will be verified, replaced, and the service will restart.'
        )}
        confirmText={t('Update and restart')}
        destructive
        handleConfirm={() => void handleConfirmUpdate()}
      />

      <SecureVerificationDialog
        open={verificationOpen}
        onOpenChange={(open) => {
          if (!open) cancelVerification()
        }}
        methods={verificationMethods}
        state={verificationState}
        onVerify={async (method, code) => {
          await executeVerification(method, code)
        }}
        onCancel={cancelVerification}
        onCodeChange={setVerificationCode}
        onMethodChange={switchVerificationMethod}
      />
    </>
  )
}

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
import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { useStatus } from '@/hooks/use-status'
import { AuthLayout } from '../auth-layout'
import { TermsFooter } from '../components/terms-footer'
import { SignUpForm } from './components/sign-up-form'

export function SignUp() {
  const { t } = useTranslation()
  const { status } = useStatus()
  const registerEnabled =
    status?.register_enabled ?? status?.data?.register_enabled ?? true

  if (!registerEnabled) {
    return (
      <AuthLayout>
        <div className='w-full space-y-6 text-center sm:text-left'>
          <div className='space-y-2'>
            <h2 className='text-2xl font-semibold tracking-tight'>
              {t('Registration is not open')}
            </h2>
            <p className='text-muted-foreground text-sm sm:text-base'>
              {t(
                'Please sign in with an existing account or contact the administrator.'
              )}
            </p>
          </div>

          <Link
            to='/sign-in'
            className='bg-primary text-primary-foreground hover:bg-primary/90 inline-flex h-9 items-center justify-center rounded-lg px-4 text-sm font-medium transition-colors'
          >
            {t('Sign in')}
          </Link>
        </div>
      </AuthLayout>
    )
  }

  return (
    <AuthLayout>
      <div className='w-full space-y-8'>
        <div className='space-y-2'>
          <h2 className='text-center text-2xl font-semibold tracking-tight sm:text-left'>
            {t('Create an account')}
          </h2>
          <p className='text-muted-foreground text-left text-sm sm:text-base'>
            {t('Already have an account?')}{' '}
            <Link
              to='/sign-in'
              className='hover:text-primary font-medium underline underline-offset-4'
            >
              {t('Sign in')}
            </Link>
            .
          </p>
        </div>

        <SignUpForm />

        <TermsFooter
          variant='sign-up'
          status={status}
          className='text-center'
        />
      </div>
    </AuthLayout>
  )
}

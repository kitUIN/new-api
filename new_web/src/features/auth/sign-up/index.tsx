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
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { AuthLayout } from '../auth-layout'

export function SignUp() {
  const { t } = useTranslation()

  return (
    <AuthLayout>
      <Card className='w-full text-center'>
        <CardHeader>
          <CardTitle className='text-2xl'>
            {t('Registration is not open')}
          </CardTitle>
          <CardDescription>
            {t(
              'Please sign in with an existing account or contact the administrator.'
            )}
          </CardDescription>
        </CardHeader>
        <CardContent>
          <Link
            to='/sign-in'
            className='bg-primary text-primary-foreground hover:bg-primary/90 inline-flex h-9 items-center justify-center rounded-lg px-4 text-sm font-medium transition-colors'
          >
            {t('Sign in')}
          </Link>
        </CardContent>
      </Card>
    </AuthLayout>
  )
}

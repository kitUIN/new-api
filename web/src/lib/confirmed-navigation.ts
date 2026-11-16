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
import { useNavigationConfirmationStore } from '@/stores/navigation-confirmation-store'

export function opensNewContext(target: string): boolean {
  return !['', '_self', '_parent', '_top'].includes(target.toLowerCase())
}

export function requiresNavigationConfirmation(
  url: string,
  target: string,
  currentUrl: string
): boolean {
  const destination = new URL(url, currentUrl)
  return (
    opensNewContext(target) ||
    destination.origin !== new URL(currentUrl).origin ||
    !['http:', 'https:', 'blob:'].includes(destination.protocol)
  )
}

export function confirmNavigation(
  url: string,
  target: string,
  action: () => void
): Promise<boolean> {
  const destination = new URL(url, document.baseURI)
  if (['javascript:', 'data:', 'vbscript:'].includes(destination.protocol)) {
    return Promise.resolve(false)
  }
  return useNavigationConfirmationStore
    .getState()
    .request(destination.href, target, action)
}

/** All application-initiated external navigation and new tabs go through here. */
export function openWithConfirmation(
  url: string,
  target = '_blank',
  features?: string
): Promise<boolean> {
  return confirmNavigation(url, target, () => {
    if (target.toLowerCase() === '_self') {
      window.location.assign(url)
    } else {
      // Preserve callers' features, including OAuth flows that need window.opener.
      window.open(url, target, features)
    }
  })
}

export function handleNavigationClick(event: MouseEvent): void {
  if (event.defaultPrevented || event.button > 1) return
  // composedPath also handles clicks on icons nested inside links.
  const link = event
    .composedPath()
    .find(
      (element): element is HTMLAnchorElement =>
        element instanceof HTMLAnchorElement
    )
  if (!link?.hasAttribute('href') || link.hasAttribute('download')) return

  let target = link.target || document.querySelector('base')?.target || '_self'
  if (event.ctrlKey || event.metaKey || event.shiftKey || event.button === 1) {
    target = '_blank'
  }
  if (
    !requiresNavigationConfirmation(link.href, target, window.location.href)
  ) {
    return
  }

  event.preventDefault()
  event.stopImmediatePropagation()
  const features = link.relList.contains('noreferrer')
    ? 'noopener,noreferrer'
    : 'noopener'
  void openWithConfirmation(link.href, target, features)
}

export function installLinkConfirmation(): () => void {
  document.addEventListener('click', handleNavigationClick, true)
  document.addEventListener('auxclick', handleNavigationClick, true)
  return () => {
    document.removeEventListener('click', handleNavigationClick, true)
    document.removeEventListener('auxclick', handleNavigationClick, true)
  }
}

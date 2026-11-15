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
import { useCallback, useState } from 'react'
import { Eraser } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { ParameterSettings } from './components/parameter-settings'
import { PlaygroundChat } from './components/playground-chat'
import { PlaygroundInput } from './components/playground-input'
import { usePlaygroundState, useChatHandler } from './hooks'
import { usePlaygroundOptions } from './hooks/use-playground-options'
import { createUserMessage, createLoadingAssistantMessage } from './lib'
import type { Message as MessageType } from './types'

export function Playground() {
  const { t } = useTranslation()
  const {
    config,
    parameterEnabled,
    messages,
    updateMessages,
    clearMessages,
    updateConfig,
    updateParameters,
  } = usePlaygroundState()
  const options = usePlaygroundOptions(config)

  const { sendChat, stopGeneration, isGenerating } = useChatHandler({
    config: { ...config, group: options.group, model: options.model },
    parameterEnabled,
    onMessageUpdate: updateMessages,
  })

  // Edit dialog state
  const [editingMessageKey, setEditingMessageKey] = useState<string | null>(
    null
  )

  const handleSendMessage = (text: string) => {
    if (!options.canSend || isGenerating) return
    const userMessage = createUserMessage(text)
    const assistantMessage = createLoadingAssistantMessage()

    const newMessages = [...messages, userMessage, assistantMessage]
    updateMessages(newMessages)

    // Send chat request
    sendChat(newMessages)
  }

  const handleCopyMessage = (message: MessageType) => {
    // Copy is handled in MessageActions component
    // eslint-disable-next-line no-console
    console.log('Message copied:', message.key)
  }

  const handleRegenerateMessage = (message: MessageType) => {
    if (!options.canSend || isGenerating) return
    // Find the message index and regenerate from there
    const messageIndex = messages.findIndex((m) => m.key === message.key)
    if (messageIndex === -1) return

    // Remove messages after this one and regenerate
    const messagesUpToHere = messages.slice(0, messageIndex)
    const loadingMessage = createLoadingAssistantMessage()
    const newMessages = [...messagesUpToHere, loadingMessage]

    updateMessages(newMessages)
    sendChat(newMessages)
  }

  const handleEditMessage = useCallback((message: MessageType) => {
    setEditingMessageKey(message.key)
  }, [])

  const handleEditOpenChange = useCallback((open: boolean) => {
    if (!open) setEditingMessageKey(null)
  }, [])

  // Apply edit and optionally re-submit from the edited user message
  const applyEdit = useCallback(
    (newContent: string, submit: boolean) => {
      if (!editingMessageKey) return
      if (submit && (!options.canSend || isGenerating)) return
      const index = messages.findIndex((m) => m.key === editingMessageKey)
      if (index === -1) return

      const updated = messages.map((m) =>
        m.key === editingMessageKey
          ? { ...m, versions: [{ ...m.versions[0], content: newContent }] }
          : m
      )

      setEditingMessageKey(null)

      if (!submit || updated[index].from !== 'user') {
        updateMessages(updated)
        return
      }

      const toSubmit = [
        ...updated.slice(0, index + 1),
        createLoadingAssistantMessage(),
      ]
      updateMessages(toSubmit)
      sendChat(toSubmit)
    },
    [
      editingMessageKey,
      messages,
      updateMessages,
      sendChat,
      options.canSend,
      isGenerating,
    ]
  )

  const handleDeleteMessage = (message: MessageType) => {
    const newMessages = messages.filter((m) => m.key !== message.key)
    updateMessages(newMessages)
  }

  const handleClearContext = () => {
    stopGeneration()
    setEditingMessageKey(null)
    clearMessages()
  }

  return (
    <div className='relative flex size-full flex-col overflow-hidden'>
      <div className='mx-auto flex w-full max-w-4xl justify-end gap-2 px-4 py-2'>
        <ParameterSettings
          config={config}
          enabled={parameterEnabled}
          disabled={isGenerating}
          onApply={updateParameters}
        />
        <Button
          type='button'
          variant='outline'
          size='sm'
          disabled={messages.length === 0}
          onClick={handleClearContext}
        >
          <Eraser aria-hidden='true' />
          {t('Clear context')}
        </Button>
      </div>
      {/* Full-width scroll container: scrolling works even over side whitespace */}
      <div className='flex flex-1 flex-col overflow-hidden'>
        <PlaygroundChat
          messages={messages}
          onCopyMessage={handleCopyMessage}
          onRegenerateMessage={handleRegenerateMessage}
          onEditMessage={handleEditMessage}
          onDeleteMessage={handleDeleteMessage}
          isGenerating={isGenerating}
          editingKey={editingMessageKey}
          onCancelEdit={handleEditOpenChange}
          onSaveEdit={(newContent) => applyEdit(newContent, false)}
          onSaveEditAndSubmit={(newContent) => applyEdit(newContent, true)}
        />
      </div>

      {/* Input area: center content and constrain to the same container width */}
      <div className='mx-auto w-full max-w-4xl'>
        {!options.isLoading && !options.canSend && (
          <Alert
            className='mb-2'
            variant={options.hasError ? 'destructive' : 'default'}
          >
            <AlertDescription className='flex items-center justify-between gap-2'>
              {options.hasError
                ? t('playground.selection.loadFailed')
                : t('playground.selection.noModels')}
              <Button
                type='button'
                variant='outline'
                size='sm'
                onClick={options.retry}
              >
                {t('Retry')}
              </Button>
            </AlertDescription>
          </Alert>
        )}
        <PlaygroundInput
          disabled={isGenerating}
          canSend={options.canSend}
          groups={options.groups}
          groupValue={options.group}
          isGenerating={isGenerating}
          isModelLoading={options.isLoading}
          modelValue={options.model}
          models={options.models}
          onGroupChange={(value) => updateConfig('group', value)}
          onModelChange={(value) => updateConfig('model', value)}
          onStop={stopGeneration}
          onSubmit={handleSendMessage}
        />
      </div>
    </div>
  )
}

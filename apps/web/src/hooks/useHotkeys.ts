import { useCallback, useEffect } from 'react'

import { useAppStore } from '@/store/useAppStore'
import { useWindowStore } from '@/store/useWindowStore'

export interface HotkeyMapEntry {
  keys: string
  action: string
}

export const HOTKEYS: HotkeyMapEntry[] = [
  { keys: 'Space', action: 'Start or pause the timer' },
  { keys: 'Cmd/Ctrl + T', action: 'Open a To Do window' },
  { keys: 'Cmd/Ctrl + P', action: 'Open a Pomodoro window' },
  { keys: 'Cmd/Ctrl + S', action: 'Open an Insights window' },
  { keys: 'Cmd/Ctrl + W', action: 'Close the focused window' },
  { keys: 'Cmd/Ctrl + `', action: 'Close every window' },
  { keys: 'Shift + Enter', action: 'Maximize the focused window' },
  { keys: 'Esc', action: 'Minimize the focused window' },
  { keys: 'N', action: 'Skip to the next phase' },
  { keys: 'R', action: 'Reset the current phase' },
  { keys: 'Alt + T / P / S', action: 'Close that window type' },
]

function isEditableTarget(target: EventTarget | null): boolean {
  if (!(target instanceof HTMLElement)) {
    return false
  }
  return (
    target.tagName === 'INPUT' ||
    target.tagName === 'TEXTAREA' ||
    target.isContentEditable
  )
}

export function useHotkeys(): void {
  const onKeyDown = useCallback((event: KeyboardEvent) => {
    if (isEditableTarget(event.target)) {
      return
    }

    const windowStore = useWindowStore.getState()
    const appStore = useAppStore.getState()
    const focusedId = windowStore.focusedId
    const meta = event.metaKey || event.ctrlKey
    const key = event.key.toLowerCase()

    if (meta) {
      switch (key) {
        case 't':
          event.preventDefault()
          windowStore.open('todo')
          return
        case 'p':
          event.preventDefault()
          windowStore.open('pomodoro')
          return
        case 's':
          event.preventDefault()
          windowStore.open('stats')
          return
        case 'w':
          if (focusedId) {
            event.preventDefault()
            windowStore.close(focusedId)
          }
          return
        case '`':
          event.preventDefault()
          windowStore.closeAll()
          return
        default:
          return
      }
    }

    if (event.altKey) {
      switch (key) {
        case 't':
          event.preventDefault()
          windowStore.closeKind('todo')
          return
        case 'p':
          event.preventDefault()
          windowStore.closeKind('pomodoro')
          return
        case 's':
          event.preventDefault()
          windowStore.closeKind('stats')
          return
        default:
          return
      }
    }

    switch (event.key) {
      case ' ':
        event.preventDefault()
        if (appStore.timer.status === 'running') {
          void appStore.pauseTimer()
        } else {
          void appStore.startTimer()
        }
        return
      case 'Escape':
        if (focusedId) {
          event.preventDefault()
          windowStore.toggleMinimize(focusedId)
        }
        return
      case 'Enter':
        if (event.shiftKey && focusedId) {
          event.preventDefault()
          windowStore.toggleMaximize(focusedId)
        }
        return
      case 'n':
      case 'N':
        event.preventDefault()
        void appStore.skipTimer()
        return
      case 'r':
      case 'R':
        event.preventDefault()
        void appStore.resetTimer()
        return
      default:
        return
    }
  }, [])

  useEffect(() => {
    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [onKeyDown])
}

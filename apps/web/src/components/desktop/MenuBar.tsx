import { useEffect, useState } from 'react'

import { Icon } from '@/components/common/Icon'
import { useAppStore } from '@/store/useAppStore'
import { useWindowStore } from '@/store/useWindowStore'

function useClock(): string {
  const [now, setNow] = useState(() => Date.now())

  useEffect(() => {
    const interval = window.setInterval(() => setNow(Date.now()), 1000)
    return () => window.clearInterval(interval)
  }, [])

  return useMemoLabel(now)
}

function useMemoLabel(now: number): string {
  const date = new Date(now)
  const weekday = date.toLocaleDateString([], { weekday: 'short' })
  const time = date.toLocaleTimeString([], { hour: 'numeric', minute: '2-digit' })
  return `${weekday} ${time}`
}

const CONNECTION_LABEL = {
  idle: 'Offline',
  connecting: 'Connecting',
  online: 'Live',
  offline: 'Reconnecting',
} as const

export function MenuBar() {
  const connection = useAppStore((state) => state.connection)
  const deviceCount = useAppStore((state) => state.deviceCount)
  const webCount = useAppStore((state) => state.webCount)
  const timer = useAppStore((state) => state.timer)
  const reconnect = useAppStore((state) => state.reconnect)

  const windows = useWindowStore((state) => state.windows)
  const focusedId = useWindowStore((state) => state.focusedId)
  const open = useWindowStore((state) => state.open)
  const closeKind = useWindowStore((state) => state.closeKind)
  const closeAll = useWindowStore((state) => state.closeAll)

  const clock = useClock()

  const focused = windows.find((win) => win.id === focusedId)
  const focusLabel =
    focused?.kind === 'todo'
      ? 'To Do'
      : focused?.kind === 'pomodoro'
        ? 'Pomodoro'
        : focused?.kind === 'stats'
          ? 'Insights'
          : 'Finder'

  const dotClass =
    connection === 'online'
      ? timer.status === 'running'
        ? 'menubar__dot--pulsing'
        : 'menubar__dot--online'
      : connection === 'connecting'
        ? 'menubar__dot--connecting'
        : 'menubar__dot--offline'

  const timerLabel =
    timer.status === 'running'
      ? `${Math.floor(timer.remainingSeconds / 60)}:${String(timer.remainingSeconds % 60).padStart(2, '0')}`
      : timer.status === 'paused'
        ? 'Paused'
        : 'Ready'

  const deviceLabel = deviceCount > 0 ? `ESP32 x${deviceCount}` : 'No display'

  return (
    <header className="menubar">
      <div className="menubar__group">
        <span className="menubar__item menubar__brand">
          <span className="menubar__logo">
            <Icon name="sparkle" size={9} strokeWidth={2.4} />
          </span>
          prodi
        </span>
        <button type="button" className="menubar__button menubar__item--strong">
          {focusLabel}
        </button>
        <button type="button" className="menubar__button" onClick={() => open('todo')}>
          File
        </button>
        <button
          type="button"
          className="menubar__button"
          onClick={() => closeKind('todo')}
          disabled={!windows.some((win) => win.kind === 'todo')}
        >
          Close To Do
        </button>
        <button
          type="button"
          className="menubar__button"
          onClick={() => closeAll()}
          disabled={windows.length === 0}
        >
          Close All
        </button>
      </div>

      <div className="menubar__group menubar__group--right">
        <span className="menubar__status">
          <Icon name="focus" size={12} strokeWidth={2} />
          <span className="sr-only">Timer</span>
          {timerLabel}
        </span>
        <span className="menubar__status" title={`${webCount} browser client(s)`}>
          <Icon name="wifi" size={12} strokeWidth={2} />
          {webCount}
        </span>
        <button
          type="button"
          className="menubar__status"
          title={
            deviceCount > 0
              ? `${deviceCount} device client(s) connected`
              : 'No ESP32 connected. Connect one over WebSocket.'
          }
          onClick={reconnect}
        >
          <span className={`menubar__dot ${dotClass}`} />
          {deviceLabel}
        </button>
        <button type="button" className="menubar__button" onClick={reconnect}>
          {CONNECTION_LABEL[connection]}
        </button>
        <span className="menubar__clock">{clock}</span>
      </div>
    </header>
  )
}

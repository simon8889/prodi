import { useEffect } from 'react'

import { Icon } from '@/components/common/Icon'
import { Launcher } from '@/components/desktop/Launcher'
import { MenuBar } from '@/components/desktop/MenuBar'
import { NotificationStack } from '@/components/desktop/NotificationStack'
import { WindowLayer } from '@/components/window/WindowLayer'
import { useHotkeys } from '@/hooks/useHotkeys'
import { useAppStore } from '@/store/useAppStore'
import { useWindowStore } from '@/store/useWindowStore'

function BootScreen({ error }: { error: string | null }) {
  const reconnect = useAppStore((state) => state.reconnect)
  const loading = useAppStore((state) => state.loading)

  return (
    <div className="boot">
      <div className="boot__logo">
        <Icon name="sparkle" size={28} strokeWidth={1.6} />
      </div>
      <div className="boot__name">prodi</div>
      {loading ? (
        <div className="boot__status">
          <span className="menubar__dot menubar__dot--connecting" />
          Connecting to the API
        </div>
      ) : (
        <>
          <div className="boot__status boot__status--error">{error ?? 'The API is unreachable'}</div>
          <button type="button" className="control control--primary control--large" onClick={reconnect}>
            <Icon name="reset" size={15} />
            Retry
          </button>
          <div className="boot__hint">
            Start it with <code>make dev-api</code> in the repository root.
          </div>
        </>
      )}
    </div>
  )
}

export default function App() {
  const bootstrap = useAppStore((state) => state.bootstrap)
  const teardown = useAppStore((state) => state.teardown)
  const reconnect = useAppStore((state) => state.reconnect)
  const connection = useAppStore((state) => state.connection)
  const error = useAppStore((state) => state.error)
  const tasks = useAppStore((state) => state.tasks)

  const windows = useWindowStore((state) => state.windows)
  const hydrate = useWindowStore((state) => state.hydrate)
  const hydrated = useWindowStore((state) => state.hydrated)
  const open = useWindowStore((state) => state.open)

  useHotkeys()

  useEffect(() => {
    hydrate()
    void bootstrap()
    return () => teardown()
  }, [bootstrap, hydrate, teardown])

  useEffect(() => {
    const onVisibility = () => {
      if (document.visibilityState === 'visible' && connection !== 'online') {
        reconnect()
      }
    }
    document.addEventListener('visibilitychange', onVisibility)
    window.addEventListener('online', reconnect)
    return () => {
      document.removeEventListener('visibilitychange', onVisibility)
      window.removeEventListener('online', reconnect)
    }
  }, [connection, reconnect])

  if (!hydrated) {
    return <div className="desktop" />
  }

  const offline = connection === 'offline'
  const showBoot = connection === 'offline' && tasks.length === 0 && error !== null

  return (
    <div className="desktop">
      <div className="desktop__wallpaper" />
      <div className="desktop__grain" />
      <div className="desktop__vignette" />

      <MenuBar />
      <div className="desktop__layer">
        <WindowLayer />
      </div>

      {windows.length === 0 && !offline ? (
        <div className="desktop__empty">
          <div className="desktop__empty-icon">
            <Icon name="sparkle" size={22} strokeWidth={1.5} />
          </div>
          <div className="desktop__empty-title">No windows open</div>
          <div className="desktop__empty-hint">
            Use the launcher below, or press{' '}
            <kbd>Cmd</kbd> <kbd>T</kbd> for a To Do list and <kbd>Cmd</kbd> <kbd>P</kbd> for the
            timer.
          </div>
          <button type="button" className="control" onClick={() => open('todo')}>
            <Icon name="list" size={14} />
            Open To Do
          </button>
        </div>
      ) : null}

      {showBoot ? <BootScreen error={error} /> : null}
      {offline && !showBoot ? (
        <div className="desktop__offline" role="status">
          <Icon name="wifi" size={13} strokeWidth={2} />
          Reconnecting to the API
        </div>
      ) : null}

      <div className="desktop__floor" />
      <Launcher />
      <NotificationStack />
    </div>
  )
}

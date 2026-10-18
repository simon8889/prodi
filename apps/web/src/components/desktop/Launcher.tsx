import { Icon, type IconName } from '@/components/common/Icon'
import { useAppStore } from '@/store/useAppStore'
import { useWindowStore, type WindowKind } from '@/store/useWindowStore'

interface DockEntry {
  kind: WindowKind
  label: string
  icon: IconName
  tone: 'todo' | 'pomodoro' | 'stats' | 'settings'
}

const ENTRIES: DockEntry[] = [
  { kind: 'todo', label: 'To Do', icon: 'list', tone: 'todo' },
  { kind: 'pomodoro', label: 'Pomodoro', icon: 'focus', tone: 'pomodoro' },
  { kind: 'stats', label: 'Insights', icon: 'chart', tone: 'stats' },
]

export function Launcher() {
  const windows = useWindowStore((state) => state.windows)
  const open = useWindowStore((state) => state.open)
  const tasks = useAppStore((state) => state.tasks)
  const running = useAppStore((state) => state.timer.status === 'running')

  const openCount = (kind: WindowKind) => windows.filter((win) => win.kind === kind).length
  const pendingTasks = tasks.filter((task) => task.status === 'pending').length

  const handleClick = (kind: WindowKind) => {
    const existing = windows.filter((win) => win.kind === kind)
    if (existing.length > 0) {
      existing.forEach((win) => {
        if (win.minimized) {
          useWindowStore.getState().focus(win.id)
        }
      })
    }
    open(kind)
  }

  return (
    <div className="dock" role="toolbar" aria-label="Window launcher">
      {ENTRIES.map((entry) => {
        const count = openCount(entry.kind)
        return (
          <button
            key={entry.kind}
            type="button"
            className="dock__item"
            onClick={() => handleClick(entry.kind)}
            aria-label={`Open ${entry.label}`}
          >
            <span className={`dock__icon dock__icon--${entry.tone}`}>
              <Icon name={entry.icon} size={22} strokeWidth={1.7} />
              {count > 0 ? <span className="dock__badge">{count}</span> : null}
              {entry.kind === 'todo' && pendingTasks > 0 && count === 0 ? (
                <span className="dock__badge">{pendingTasks}</span>
              ) : null}
            </span>
            <span className="dock__label">{entry.label}</span>
          </button>
        )
      })}

      <span className="dock__separator" />

      <div className="dock__item" aria-hidden="true">
        <span
          className={`dock__icon ${running ? 'dock__icon--pomodoro' : 'dock__icon--disabled'}`}
          style={running ? { animation: 'prodi-breathe 2.6s var(--ease-out) infinite' } : undefined}
        >
          <Icon name="clock" size={21} strokeWidth={1.7} />
        </span>
        <span className="dock__label">{running ? 'Focusing' : 'Idle'}</span>
      </div>
    </div>
  )
}

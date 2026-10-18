import { useCallback } from 'react'

import { PomodoroWindow } from '@/components/pomodoro/PomodoroWindow'
import { StatsWindow } from '@/components/stats/StatsWindow'
import { TodoWindow } from '@/components/todo/TodoWindow'
import { WindowFrame } from '@/components/window/WindowFrame'
import { useAppStore } from '@/store/useAppStore'
import { WINDOW_PRESETS, useWindowStore, type WindowState } from '@/store/useWindowStore'
import { PHASE_SHORT_LABELS, isBreakPhase } from '@/types'

function WindowLayer() {
  const windows = useWindowStore((state) => state.windows)
  const focusedId = useWindowStore((state) => state.focusedId)
  const close = useWindowStore((state) => state.close)
  const focus = useWindowStore((state) => state.focus)
  const move = useWindowStore((state) => state.move)
  const resize = useWindowStore((state) => state.resize)
  const toggleMaximize = useWindowStore((state) => state.toggleMaximize)
  const toggleCollapse = useWindowStore((state) => state.toggleCollapse)

  const tasks = useAppStore((state) => state.tasks)
  const timer = useAppStore((state) => state.timer)

  const ordered = [...windows].sort((a, b) => a.z - b.z)

  const handleClose = useCallback((id: string) => close(id), [close])
  const handleFocus = useCallback((id: string) => focus(id), [focus])
  const handleMove = useCallback((id: string, x: number, y: number) => move(id, x, y), [move])
  const handleResize = useCallback(
    (id: string, width: number, height: number) => resize(id, width, height),
    [resize],
  )
  const handleMaximize = useCallback(
    (id: string) => toggleMaximize(id),
    [toggleMaximize],
  )
  const handleCollapse = useCallback(
    (id: string) => toggleCollapse(id),
    [toggleCollapse],
  )

  return (
    <>
      {ordered.map((win) => (
        <WindowRenderer
          key={win.id}
          win={win}
          active={win.id === focusedId}
          pendingCount={tasks.filter((task) => task.status === 'pending').length}
          doneCount={tasks.filter((task) => task.status === 'done').length}
          timerLabel={isBreakPhase(timer.phase) ? PHASE_SHORT_LABELS[timer.phase] : 'Focus'}
          timerRunning={timer.status === 'running'}
          onClose={handleClose}
          onFocus={handleFocus}
          onMove={handleMove}
          onResize={handleResize}
          onToggleMaximize={handleMaximize}
          onToggleCollapse={handleCollapse}
        />
      ))}
    </>
  )
}

interface WindowRendererProps {
  win: WindowState
  active: boolean
  pendingCount: number
  doneCount: number
  timerLabel: string
  timerRunning: boolean
  onClose: (id: string) => void
  onFocus: (id: string) => void
  onMove: (id: string, x: number, y: number) => void
  onResize: (id: string, width: number, height: number) => void
  onToggleMaximize: (id: string) => void
  onToggleCollapse: (id: string) => void
}

function WindowRenderer({
  win,
  active,
  pendingCount,
  doneCount,
  timerLabel,
  timerRunning,
  onClose,
  onFocus,
  onMove,
  onResize,
  onToggleMaximize,
  onToggleCollapse,
}: WindowRendererProps) {
  const preset = WINDOW_PRESETS[win.kind]

  const subtitle =
    win.kind === 'todo'
      ? `${pendingCount} open${doneCount > 0 ? ` · ${doneCount} done` : ''}`
      : win.kind === 'pomodoro'
        ? `${timerLabel}${timerRunning ? ' · running' : ''}`
        : undefined

  return (
    <WindowFrame
      window={win}
      active={active}
      title={preset.title}
      subtitle={subtitle}
      collapsible={win.kind !== 'pomodoro'}
      onClose={() => onClose(win.id)}
      onFocus={() => onFocus(win.id)}
      onMove={(x, y) => onMove(win.id, x, y)}
      onResize={(width, height) => onResize(win.id, width, height)}
      onToggleMaximize={() => onToggleMaximize(win.id)}
      onToggleCollapse={() => onToggleCollapse(win.id)}
    >
      {win.kind === 'todo' ? <TodoWindow /> : null}
      {win.kind === 'pomodoro' ? <PomodoroWindow /> : null}
      {win.kind === 'stats' ? <StatsWindow /> : null}
    </WindowFrame>
  )
}

export { WindowLayer }

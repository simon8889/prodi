import { useMemo } from 'react'

import { Icon, type IconName } from '@/components/common/Icon'
import { useAppStore } from '@/store/useAppStore'
import type { Phase, Session } from '@/types'

import '@/styles/stats.css'

const PHASE_COLOR: Record<Phase, string> = {
  focus: 'var(--accent)',
  shortBreak: 'var(--green)',
  longBreak: 'var(--purple)',
}

function formatDuration(totalSeconds: number): string {
  const hours = Math.floor(totalSeconds / 3600)
  const minutes = Math.floor((totalSeconds % 3600) / 60)
  if (hours > 0) {
    return `${hours}h ${minutes}m`
  }
  if (minutes > 0) {
    return `${minutes}m`
  }
  return `${totalSeconds}s`
}

function formatTime(iso: string): string {
  const date = new Date(iso)
  return date.toLocaleTimeString([], { hour: 'numeric', minute: '2-digit' })
}

function sessionIcon(phase: Phase): IconName {
  if (phase === 'focus') {
    return 'focus'
  }
  return 'clock'
}

export function StatsWindow() {
  const stats = useAppStore((state) => state.stats)
  const sessions = useAppStore((state) => state.sessions)

  const breakdown = useMemo(() => {
    const counts: Record<Phase, number> = { focus: 0, shortBreak: 0, longBreak: 0 }
    for (const session of sessions) {
      if (session.completed) {
        counts[session.phase] += 1
      }
    }
    const total = counts.focus + counts.shortBreak + counts.longBreak
    return { counts, total }
  }, [sessions])

  const completionRate =
    stats.tasksTotal > 0 ? Math.round((stats.tasksCompleted / stats.tasksTotal) * 100) : 0

  return (
    <div className="stats">
      <div className="stats__body desktop-scrollbar">
        <div className="stats__grid">
          <div className="stats__card">
            <span className="stats__card-value">{formatDuration(stats.focusSeconds)}</span>
            <span className="stats__card-label">Total focus time</span>
          </div>
          <div className="stats__card">
            <span className="stats__card-value">{stats.focusSessions}</span>
            <span className="stats__card-label">Focus blocks</span>
          </div>
          <div className="stats__card">
            <span className="stats__card-value">{stats.currentStreakDays}</span>
            <span className="stats__card-label">Day streak</span>
          </div>
          <div className="stats__card">
            <span className="stats__card-value">{completionRate}%</span>
            <span className="stats__card-label">Tasks completed</span>
          </div>
        </div>

        {breakdown.total > 0 ? (
          <>
            <div className="stats__section-title">Recent split</div>
            <div className="stats__bar">
              {(['focus', 'shortBreak', 'longBreak'] as Phase[]).map((phase) => {
                const share = breakdown.counts[phase] / breakdown.total
                if (share === 0) {
                  return null
                }
                return (
                  <div
                    key={phase}
                    className="stats__bar-segment"
                    style={{ width: `${share * 100}%`, background: PHASE_COLOR[phase] }}
                  />
                )
              })}
            </div>
            <div className="stats__legend">
              {(['focus', 'shortBreak', 'longBreak'] as Phase[]).map((phase) => (
                <span key={phase} className="stats__legend-item">
                  <span
                    className="stats__legend-swatch"
                    style={{ background: PHASE_COLOR[phase] }}
                  />
                  {phase === 'focus' ? 'Focus' : phase === 'shortBreak' ? 'Short' : 'Long'}
                  <strong>{breakdown.counts[phase]}</strong>
                </span>
              ))}
            </div>
          </>
        ) : null}

        <div className="stats__section-title">Recent sessions</div>
        {sessions.length === 0 ? (
          <div className="stats__empty">
            Finish a focus block and it will show up here.
          </div>
        ) : (
          <ul className="stats__sessions">
            {sessions.map((session) => (
              <SessionRow key={session.id} session={session} />
            ))}
          </ul>
        )}
      </div>
    </div>
  )
}

function SessionRow({ session }: { session: Session }) {
  return (
    <li className="stats__session">
      <span className={`stats__session-badge stats__session-badge--${session.phase}`}>
        <Icon name={sessionIcon(session.phase)} size={13} />
      </span>
      <span className="stats__session-title truncate">
        {session.taskTitle || (session.phase === 'focus' ? 'Focus block' : 'Break')}
      </span>
      {session.abandoned ? <span className="stats__session-flag">Abandoned</span> : null}
      <span className="stats__session-time">{formatTime(session.endedAt)}</span>
    </li>
  )
}

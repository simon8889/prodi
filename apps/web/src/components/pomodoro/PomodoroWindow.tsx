import { useMemo } from 'react'

import { Icon } from '@/components/common/Icon'
import { Progress } from '@/components/common/Progress'
import { Stepper } from '@/components/common/Stepper'
import { Switch } from '@/components/common/Switch'
import { TimerRing } from '@/components/pomodoro/TimerRing'
import { useAppStore } from '@/store/useAppStore'
import { useServerClock } from '@/hooks/useNow'
import type { Settings } from '@/types'

import '@/styles/pomodoro.css'

function formatDuration(totalSeconds: number): string {
  const minutes = Math.floor(totalSeconds / 60)
  const seconds = totalSeconds % 60
  if (minutes === 0) {
    return `${seconds}s`
  }
  if (seconds === 0) {
    return `${minutes}m`
  }
  return `${minutes}m ${seconds}s`
}

export function PomodoroWindow() {
  const timer = useAppStore((state) => state.timer)
  const settings = useAppStore((state) => state.settings)
  const stats = useAppStore((state) => state.stats)
  const tasks = useAppStore((state) => state.tasks)
  const startTimer = useAppStore((state) => state.startTimer)
  const pauseTimer = useAppStore((state) => state.pauseTimer)
  const resetTimer = useAppStore((state) => state.resetTimer)
  const skipTimer = useAppStore((state) => state.skipTimer)
  const saveSettings = useAppStore((state) => state.saveSettings)

  const serverNow = useServerClock(timer.serverOffsetMs, true)

  const remainingSeconds = useMemo(() => {
    if (timer.status !== 'running') {
      return Math.max(0, timer.remainingSeconds)
    }
    const elapsed = Math.floor((serverNow - timer.receivedAtMs) / 1000)
    return Math.max(0, timer.remainingSeconds - elapsed)
  }, [serverNow, timer.receivedAtMs, timer.remainingSeconds, timer.status])

  const running = timer.status === 'running'
  const isFocus = timer.phase === 'focus'

  const activeTask = useMemo(
    () => tasks.find((task) => task.id === timer.activeTaskId),
    [tasks, timer.activeTaskId],
  )

  const activeTitle = timer.activeTaskTitle || activeTask?.title || ''
  const activeCompleted = activeTask?.completedPomodoros ?? timer.taskCompletedPomodoros
  const activeEstimated = activeTask?.estimatedPomodoros ?? timer.taskEstimatedPomodoros

  const update = (patch: Partial<Settings>) => {
    void saveSettings({ ...settings, ...patch })
  }

  const cycleTotal = settings.longBreakEveryCycles
  const cycleDone = ((timer.cycleCount - 1) % cycleTotal) + 1

  return (
    <div className="pomodoro">
      <div className="pomodoro__main desktop-scrollbar">
        <TimerRing timer={timer} remainingSeconds={remainingSeconds} />

        <div className="pomodoro__controls">
          <button
            type="button"
            className="pomodoro__side-button"
            aria-label="Reset the current phase"
            title="Reset (R)"
            onClick={() => void resetTimer()}
          >
            <Icon name="reset" size={16} />
          </button>

          <button
            type="button"
            className={`pomodoro__play${running ? ' pomodoro__play--pause' : ''}`}
            aria-label={running ? 'Pause the timer' : 'Start the timer'}
            title={running ? 'Pause (Space)' : 'Start (Space)'}
            onClick={() => void (running ? pauseTimer() : startTimer())}
          >
            <Icon name={running ? 'pause' : 'play'} size={running ? 20 : 22} strokeWidth={0} />
          </button>

          <button
            type="button"
            className="pomodoro__side-button"
            aria-label="Skip to the next phase"
            title="Skip (N)"
            onClick={() => void skipTimer()}
          >
            <Icon name="skip" size={16} />
          </button>
        </div>

        <div className="pomodoro__cycles" aria-label={`Cycle ${cycleDone} of ${cycleTotal}`}>
          {Array.from({ length: cycleTotal }, (_, index) => (
            <span
              key={index}
              className={[
                'pomodoro__cycle-dot',
                index < cycleDone ? 'pomodoro__cycle-dot--done' : '',
                index === cycleDone ? 'pomodoro__cycle-dot--current' : '',
              ]
                .filter(Boolean)
                .join(' ')}
            />
          ))}
          <span className="pomodoro__cycle-label">
            {isFocus ? 'Focus' : 'Break'} {cycleDone}/{cycleTotal}
          </span>
        </div>

        <div className="pomodoro__active-task">
          <span className="pomodoro__active-label">
            {isFocus ? 'Working on' : 'Break, take it easy'}
          </span>
          {activeTitle ? (
            <>
              <span className="pomodoro__active-title">{activeTitle}</span>
              {isFocus && activeEstimated > 0 ? (
                <>
                  <div className="pomodoro__active-progress">
                    <Progress
                      value={activeEstimated > 0 ? activeCompleted / activeEstimated : 0}
                      variant="accent"
                    />
                  </div>
                  <span className="pomodoro__active-counter">
                    {activeCompleted} of {activeEstimated} focus blocks
                  </span>
                </>
              ) : null}
            </>
          ) : (
            <span className="pomodoro__active-empty">
              {isFocus ? 'No active task. Open To Do and pick one.' : 'Enjoy the pause.'}
            </span>
          )}
        </div>

        <PomodoroSettings settings={settings} onChange={update} />
      </div>

      <div className="pomodoro__today">
        <div className="pomodoro__stat">
          <span className="pomodoro__stat-value">{stats.todaySessions}</span>
          <span className="pomodoro__stat-label">Today</span>
        </div>
        <div className="pomodoro__stat">
          <span className="pomodoro__stat-value">{formatDuration(stats.todayFocusSeconds)}</span>
          <span className="pomodoro__stat-label">Focused</span>
        </div>
        <div className="pomodoro__stat">
          <span className="pomodoro__stat-value">{stats.tasksCompleted}</span>
          <span className="pomodoro__stat-label">Done</span>
        </div>
      </div>
    </div>
  )
}

interface PomodoroSettingsProps {
  settings: Settings
  onChange: (patch: Partial<Settings>) => void
}

function PomodoroSettings({ settings, onChange }: PomodoroSettingsProps) {
  return (
    <div className="pomodoro__settings">
      <div className="pomodoro__settings-grid">
        <label className="pomodoro__setting">
          <span className="pomodoro__setting-label">Focus</span>
          <div className="pomodoro__setting-control">
            <Stepper
              value={settings.focusMinutes}
              min={1}
              max={90}
              label="Focus minutes"
              onChange={(value) => onChange({ focusMinutes: value })}
            />
            <span className="pomodoro__setting-label">min</span>
          </div>
        </label>

        <label className="pomodoro__setting">
          <span className="pomodoro__setting-label">Short break</span>
          <div className="pomodoro__setting-control">
            <Stepper
              value={settings.shortBreakMinutes}
              min={1}
              max={30}
              label="Short break minutes"
              onChange={(value) => onChange({ shortBreakMinutes: value })}
            />
            <span className="pomodoro__setting-label">min</span>
          </div>
        </label>

        <label className="pomodoro__setting">
          <span className="pomodoro__setting-label">Long break</span>
          <div className="pomodoro__setting-control">
            <Stepper
              value={settings.longBreakMinutes}
              min={1}
              max={60}
              label="Long break minutes"
              onChange={(value) => onChange({ longBreakMinutes: value })}
            />
            <span className="pomodoro__setting-label">min</span>
          </div>
        </label>

        <label className="pomodoro__setting">
          <span className="pomodoro__setting-label">Long break every</span>
          <div className="pomodoro__setting-control">
            <Stepper
              value={settings.longBreakEveryCycles}
              min={2}
              max={8}
              label="Cycles before a long break"
              onChange={(value) => onChange({ longBreakEveryCycles: value })}
            />
            <span className="pomodoro__setting-label">cycles</span>
          </div>
        </label>

        <div className="pomodoro__setting pomodoro__setting--wide">
          <span className="pomodoro__setting-label">Auto start breaks</span>
          <Switch
            checked={settings.autoStartBreaks}
            label="Auto start breaks"
            onChange={(checked) => onChange({ autoStartBreaks: checked })}
          />
        </div>
      </div>
    </div>
  )
}

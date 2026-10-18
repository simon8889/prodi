import { useMemo } from 'react'

import type { TimerView } from '@/store/useAppStore'
import type { Phase } from '@/types'

interface TimerRingProps {
  timer: TimerView
  remainingSeconds: number
  size?: number
}

const STROKE = 8

function phaseClass(phase: Phase): string {
  return `timer-ring__progress--${phase}`
}

export function TimerRing({ timer, remainingSeconds, size = 196 }: TimerRingProps) {
  const radius = (size - STROKE) / 2
  const circumference = 2 * Math.PI * radius
  const total = Math.max(1, timer.phaseDurationSeconds)
  const clampedRemaining = Math.min(Math.max(remainingSeconds, 0), total)
  const progress = (total - clampedRemaining) / total
  const offset = circumference * (1 - progress)
  const center = size / 2

  const label = useMemo(() => {
    const minutes = Math.floor(clampedRemaining / 60)
    const seconds = clampedRemaining % 60
    return `${String(minutes).padStart(2, '0')}:${String(seconds).padStart(2, '0')}`
  }, [clampedRemaining])

  const urgent = timer.phase === 'focus' && clampedRemaining <= 60 && timer.status === 'running'

  return (
    <div className="timer-ring" style={{ width: size, height: size }}>
      <div
        className={`timer-ring__glow timer-ring__glow--${timer.phase}${
          timer.status === 'running' ? '' : ' timer-ring__glow--paused'
        }`}
        style={{ inset: size * 0.14 }}
      />
      <svg
        className="timer-ring__svg"
        width={size}
        height={size}
        viewBox={`0 0 ${size} ${size}`}
        role="img"
        aria-label={`${timer.phase} phase, ${label} remaining`}
      >
        <circle
          className="timer-ring__track"
          cx={center}
          cy={center}
          r={radius}
          strokeWidth={STROKE}
        />
        <circle
          className={`timer-ring__progress ${phaseClass(timer.phase)}${
            urgent ? ' timer-ring__progress--urgent' : ''
          }`}
          cx={center}
          cy={center}
          r={radius}
          strokeWidth={STROKE}
          strokeDasharray={circumference}
          strokeDashoffset={offset}
        />
      </svg>
      <div className="timer-ring__content">
        <div className="timer-ring__time" style={urgent ? { color: 'var(--orange)' } : undefined}>
          {label}
        </div>
        <div className={`timer-ring__phase timer-ring__phase--${timer.phase}`}>
          {timer.status === 'paused' ? 'Paused' : timer.phase === 'focus' ? 'Focus' : timer.phase === 'shortBreak' ? 'Short' : 'Long'}
        </div>
      </div>
    </div>
  )
}

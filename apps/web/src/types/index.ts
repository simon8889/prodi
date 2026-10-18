export type Phase = 'focus' | 'shortBreak' | 'longBreak'

export type TimerStatus = 'idle' | 'running' | 'paused'

export type TaskStatus = 'pending' | 'done'

export interface Task {
  id: string
  title: string
  estimatedPomodoros: number
  completedPomodoros: number
  status: TaskStatus
  position: number
  createdAt: string
  completedAt?: string
}

export interface TimerState {
  phase: Phase
  status: TimerStatus
  phaseStartedAt: string
  phaseDurationSeconds: number
  remainingSeconds: number
  cycleCount: number
  activeTaskId?: string
}

export interface Settings {
  focusMinutes: number
  shortBreakMinutes: number
  longBreakMinutes: number
  longBreakEveryCycles: number
  autoStartBreaks: boolean
}

export interface Session {
  id: string
  taskId?: string
  taskTitle: string
  phase: Phase
  startedAt: string
  endedAt: string
  durationSeconds: number
  completed: boolean
  abandoned: boolean
}

export interface Stats {
  focusSessions: number
  focusSeconds: number
  tasksCompleted: number
  tasksTotal: number
  tasksPending: number
  currentStreakDays: number
  todayFocusSeconds: number
  todaySessions: number
  generatedAt: string
}

export interface Snapshot {
  tasks: Task[]
  timer: TimerState
  settings: Settings
  stats: Stats
  sessions: Session[]
  serverTime: string
  activeTask?: Task
}

export interface TickPayload {
  phase: Phase
  status: TimerStatus
  remainingSeconds: number
  phaseDurationSeconds: number
  progress: number
  cycleCount: number
  activeTaskId?: string
  activeTaskTitle: string
  taskCompletedPomodoros: number
  taskEstimatedPomodoros: number
  taskProgress: number
  serverTime: string
}

export interface ClientCounts {
  total: number
  web: number
  devices: number
}

export interface TaskCompletedPayload {
  id: string
  title: string
  completedAt: string
}

export interface PhaseChangedPayload {
  from: Phase
  to: Phase
  reason: string
  autoStart: boolean
  cycleCount: number
}

export interface NotifyPayload {
  level: 'success' | 'info' | 'warning' | 'error'
  title: string
  message: string
}

export interface CreateTaskInput {
  title: string
  estimatedPomodoros: number
}

export type SettingsUpdateInput = Settings

export type ServerMessageType =
  | 'snapshot'
  | 'tick'
  | 'timer.state'
  | 'tasks'
  | 'settings'
  | 'task.completed'
  | 'phase.changed'
  | 'notify'
  | 'hello'
  | 'error'
  | 'task.created'

export type ClientMessageType =
  | 'timer.start'
  | 'timer.pause'
  | 'timer.reset'
  | 'timer.skip'
  | 'task.create'
  | 'task.setActive'
  | 'task.complete'
  | 'settings.update'
  | 'ping'

export const PHASE_LABELS: Record<Phase, string> = {
  focus: 'Focus',
  shortBreak: 'Short Break',
  longBreak: 'Long Break',
}

export const PHASE_SHORT_LABELS: Record<Phase, string> = {
  focus: 'Focus',
  shortBreak: 'Short',
  longBreak: 'Long',
}

export function isBreakPhase(phase: Phase): boolean {
  return phase === 'shortBreak' || phase === 'longBreak'
}

export function taskProgress(task: Task): number {
  if (task.estimatedPomodoros <= 0) {
    return 0
  }
  const ratio = task.completedPomodoros / task.estimatedPomodoros
  return ratio > 1 ? 1 : ratio
}

export function taskRemaining(task: Task): number {
  return Math.max(0, task.estimatedPomodoros - task.completedPomodoros)
}

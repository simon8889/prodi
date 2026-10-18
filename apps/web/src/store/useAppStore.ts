import { create } from 'zustand'

import { ApiError, api } from '@/api/client'
import { RealtimeClient, type ConnectionStatus, type Envelope } from '@/api/realtime'
import { taskProgress } from '@/types'
import type {
  NotifyPayload,
  Phase,
  PhaseChangedPayload,
  Session,
  Settings,
  Snapshot,
  Stats,
  Task,
  TaskCompletedPayload,
  TimerStatus,
} from '@/types'

export interface TimerView {
  phase: Phase
  status: TimerStatus
  remainingSeconds: number
  phaseDurationSeconds: number
  cycleCount: number
  activeTaskId?: string
  activeTaskTitle: string
  taskCompletedPomodoros: number
  taskEstimatedPomodoros: number
  taskProgress: number
  progress: number
  receivedAtMs: number
  serverOffsetMs: number
  serverTime?: string
}

export interface AppNotification {
  id: string
  level: NotifyPayload['level']
  title: string
  message: string
  createdAtMs: number
}

interface AppStoreState {
  tasks: Task[]
  timer: TimerView
  settings: Settings
  stats: Stats
  sessions: Session[]
  connection: ConnectionStatus
  deviceCount: number
  webCount: number
  notifications: AppNotification[]
  loading: boolean
  error: string | null

  bootstrap: () => Promise<void>
  teardown: () => void
  handleEnvelope: (envelope: Envelope) => void
  reconnect: () => void

  createTask: (title: string, estimatedPomodoros: number) => Promise<boolean>
  renameTask: (id: string, title: string) => Promise<boolean>
  setTaskEstimate: (id: string, estimatedPomodoros: number) => Promise<boolean>
  setTaskDone: (id: string, done: boolean) => Promise<boolean>
  deleteTask: (id: string) => Promise<boolean>
  makeActive: (id: string) => Promise<boolean>

  startTimer: () => Promise<void>
  pauseTimer: () => Promise<void>
  resetTimer: () => Promise<void>
  skipTimer: () => Promise<void>

  saveSettings: (settings: Settings) => Promise<boolean>

  dismissNotification: (id: string) => void
  clearError: () => void
}

const DEFAULT_SETTINGS: Settings = {
  focusMinutes: 25,
  shortBreakMinutes: 5,
  longBreakMinutes: 15,
  longBreakEveryCycles: 4,
  autoStartBreaks: true,
}

const DEFAULT_STATS: Stats = {
  focusSessions: 0,
  focusSeconds: 0,
  tasksCompleted: 0,
  tasksTotal: 0,
  tasksPending: 0,
  currentStreakDays: 0,
  todayFocusSeconds: 0,
  todaySessions: 0,
  generatedAt: new Date(0).toISOString(),
}

const NOTIFICATION_TTL_MS = 6000

let socket: RealtimeClient | null = null
const notificationTimers = new Map<string, number>()

function defaultTimer(): TimerView {
  return {
    phase: 'focus',
    status: 'idle',
    remainingSeconds: DEFAULT_SETTINGS.focusMinutes * 60,
    phaseDurationSeconds: DEFAULT_SETTINGS.focusMinutes * 60,
    cycleCount: 0,
    activeTaskTitle: '',
    taskCompletedPomodoros: 0,
    taskEstimatedPomodoros: 0,
    taskProgress: 0,
    progress: 0,
    receivedAtMs: Date.now(),
    serverOffsetMs: 0,
  }
}

function errorMessage(error: unknown): string {
  if (error instanceof ApiError) {
    return error.message
  }
  if (error instanceof Error) {
    return error.message
  }
  return 'Something went wrong'
}

export const useAppStore = create<AppStoreState>((set, get) => ({
  tasks: [],
  timer: defaultTimer(),
  settings: DEFAULT_SETTINGS,
  stats: DEFAULT_STATS,
  sessions: [],
  connection: 'idle',
  deviceCount: 0,
  webCount: 0,
  notifications: [],
  loading: true,
  error: null,

  bootstrap: async () => {
    set({ loading: true, error: null })
    try {
      const snapshot = await api.state()
      applySnapshot(set, snapshot)
      set({ loading: false })
    } catch (error) {
      set({ loading: false, error: errorMessage(error) })
    }

    if (!socket) {
      socket = new RealtimeClient({
        onEnvelope: (envelope) => get().handleEnvelope(envelope),
        onStatus: (status) => set({ connection: status }),
      })
    }
    socket.connect()
  },

  teardown: () => {
    socket?.disconnect()
    socket = null
    notificationTimers.forEach((timer) => window.clearTimeout(timer))
    notificationTimers.clear()
  },

  reconnect: () => {
    socket?.reconnectNow()
  },

  handleEnvelope: (envelope) => {
    const payload = envelope.payload as Record<string, unknown> | undefined

    switch (envelope.type) {
      case 'snapshot': {
        applySnapshot(set, envelope.payload as unknown as Snapshot)
        void refreshClientCounts(set)
        return
      }
      case 'tick': {
        const tick = payload as unknown as TimerView
        if (!tick?.phase) {
          return
        }
        set((state) => ({
          timer: {
            ...state.timer,
            ...tick,
            receivedAtMs: Date.now(),
            serverOffsetMs: tick.serverTime
              ? new Date(tick.serverTime).getTime() - Date.now()
              : state.timer.serverOffsetMs,
          },
        }))
        return
      }
      case 'timer.state': {
        const state = payload as { phase: Phase; status: TimerStatus }
        set((prev) => ({
          timer: {
            ...prev.timer,
            phase: state.phase,
            status: state.status,
            receivedAtMs: Date.now(),
          },
        }))
        return
      }
      case 'tasks': {
        const tasks = payload as unknown as Task[]
        if (Array.isArray(tasks)) {
          set({ tasks })
        }
        return
      }
      case 'settings': {
        const settings = payload as unknown as Settings
        if (settings && typeof settings.focusMinutes === 'number') {
          set({ settings })
        }
        return
      }
      case 'stats': {
        const stats = payload as unknown as Stats
        if (stats && typeof stats.focusSessions === 'number') {
          set({ stats })
        }
        return
      }
      case 'task.completed': {
        const completed = payload as unknown as TaskCompletedPayload
        pushNotification(set, {
          level: 'success',
          title: 'Task completed',
          message: completed?.title ?? 'A focus block finished the job',
        })
        return
      }
      case 'phase.changed': {
        const changed = payload as unknown as PhaseChangedPayload
        if (!changed?.to) {
          return
        }
        pushNotification(set, {
          level: 'info',
          title: phaseTitle(changed.to),
          message: changed.autoStart ? 'Started automatically' : 'Ready when you are',
        })
        return
      }
      case 'notify': {
        const notify = payload as unknown as NotifyPayload
        if (notify?.title) {
          pushNotification(set, notify)
        }
        return
      }
      case 'hello': {
        void refreshClientCounts(set)
        return
      }
      case 'error': {
        pushNotification(set, {
          level: 'warning',
          title: 'Server error',
          message: (payload as unknown as NotifyPayload)?.message ?? 'The API rejected a request',
        })
        return
      }
      default:
        return
    }
  },

  createTask: async (title, estimatedPomodoros) => {
    const trimmed = title.trim()
    if (!trimmed) {
      return false
    }
    try {
      await api.createTask({ title: trimmed, estimatedPomodoros })
      set({ tasks: await api.listTasks(), error: null })
      return true
    } catch (error) {
      set({ error: errorMessage(error) })
      return false
    }
  },

  renameTask: async (id, title) => {
    const trimmed = title.trim()
    if (!trimmed) {
      return false
    }
    try {
      const updated = await api.updateTask(id, { title: trimmed })
      set((state) => ({
        tasks: state.tasks.map((task) => (task.id === updated.id ? updated : task)),
        error: null,
      }))
      return true
    } catch (error) {
      set({ error: errorMessage(error) })
      return false
    }
  },

  setTaskEstimate: async (id, estimatedPomodoros) => {
    const clamped = Math.min(24, Math.max(1, Math.round(estimatedPomodoros)))
    try {
      const updated = await api.updateTask(id, { estimatedPomodoros: clamped })
      set((state) => ({
        tasks: state.tasks.map((task) => (task.id === updated.id ? updated : task)),
        error: null,
      }))
      return true
    } catch (error) {
      set({ error: errorMessage(error) })
      return false
    }
  },

  setTaskDone: async (id, done) => {
    try {
      const updated = await api.updateTask(id, { status: done ? 'done' : 'pending' })
      set((state) => ({
        tasks: state.tasks.map((task) => (task.id === updated.id ? updated : task)),
        error: null,
      }))
      if (done) {
        pushNotification(set, {
          level: 'success',
          title: 'Task completed',
          message: updated.title,
        })
      }
      return true
    } catch (error) {
      set({ error: errorMessage(error) })
      return false
    }
  },

  deleteTask: async (id) => {
    const snapshot = get().tasks
    set((state) => ({ tasks: state.tasks.filter((task) => task.id !== id) }))
    try {
      await api.deleteTask(id)
      set({ error: null })
      return true
    } catch (error) {
      set({ tasks: snapshot, error: errorMessage(error) })
      return false
    }
  },

  makeActive: async (id) => {
    try {
      await api.setActiveTask(id)
      set({ error: null })
      return true
    } catch (error) {
      set({ error: errorMessage(error) })
      return false
    }
  },

  startTimer: async () => {
    await runTimerCommand(get, set, () => api.startTimer())
  },

  pauseTimer: async () => {
    await runTimerCommand(get, set, () => api.pauseTimer())
  },

  resetTimer: async () => {
    await runTimerCommand(get, set, () => api.resetTimer())
  },

  skipTimer: async () => {
    await runTimerCommand(get, set, () => api.skipTimer())
  },

  saveSettings: async (settings) => {
    const previous = get().settings
    set({ settings })
    try {
      const saved = await api.updateSettings(settings)
      set({ settings: saved, error: null })
      return true
    } catch (error) {
      set({ settings: previous, error: errorMessage(error) })
      return false
    }
  },

  dismissNotification: (id) => {
    const timer = notificationTimers.get(id)
    if (timer !== undefined) {
      window.clearTimeout(timer)
      notificationTimers.delete(id)
    }
    set((state) => ({ notifications: state.notifications.filter((item) => item.id !== id) }))
  },

  clearError: () => set({ error: null }),
}))

type SetState = (
  partial:
    | Partial<AppStoreState>
    | ((state: AppStoreState) => Partial<AppStoreState>),
) => void

function applySnapshot(set: SetState, snapshot: Snapshot): void {
  if (!snapshot || !Array.isArray(snapshot.tasks)) {
    return
  }
  const now = Date.now()
  set({
    tasks: snapshot.tasks,
    settings: snapshot.settings ?? DEFAULT_SETTINGS,
    stats: snapshot.stats ?? DEFAULT_STATS,
    sessions: snapshot.sessions ?? [],
    loading: false,
    error: null,
    timer: {
      phase: snapshot.timer.phase,
      status: snapshot.timer.status,
      remainingSeconds: snapshot.timer.remainingSeconds,
      phaseDurationSeconds: snapshot.timer.phaseDurationSeconds,
      cycleCount: snapshot.timer.cycleCount,
      activeTaskId: snapshot.timer.activeTaskId,
      activeTaskTitle: snapshot.activeTask?.title ?? '',
      taskCompletedPomodoros: snapshot.activeTask?.completedPomodoros ?? 0,
      taskEstimatedPomodoros: snapshot.activeTask?.estimatedPomodoros ?? 0,
      taskProgress: snapshot.activeTask
        ? taskProgress(snapshot.activeTask)
        : 0,
      progress: 0,
      receivedAtMs: now,
      serverOffsetMs: snapshot.serverTime ? new Date(snapshot.serverTime).getTime() - now : 0,
    },
  })
}

async function refreshClientCounts(set: SetState): Promise<void> {
  try {
    const counts = await api.clients()
    set({ deviceCount: counts.devices, webCount: counts.web })
  } catch {
    return
  }
}

async function runTimerCommand(
  get: () => AppStoreState,
  set: SetState,
  run: () => Promise<unknown>,
): Promise<void> {
  try {
    await run()
    set({ error: null })
  } catch (error) {
    set({ error: errorMessage(error) })
  }
  if (get().connection !== 'online') {
    return
  }
}

function phaseTitle(phase: Phase): string {
  switch (phase) {
    case 'focus':
      return 'Focus time'
    case 'shortBreak':
      return 'Short break'
    case 'longBreak':
      return 'Long break'
  }
}

function pushNotification(
  set: SetState,
  input: { level: NotifyPayload['level']; title: string; message: string },
): void {
  const id = `n-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 7)}`
  const notification: AppNotification = {
    id,
    level: input.level,
    title: input.title,
    message: input.message,
    createdAtMs: Date.now(),
  }

  set((state) => ({ notifications: [...state.notifications, notification].slice(-4) }))

  const timer = window.setTimeout(() => {
    notificationTimers.delete(id)
    set((state) => ({ notifications: state.notifications.filter((item) => item.id !== id) }))
  }, NOTIFICATION_TTL_MS)
  notificationTimers.set(id, timer)
}

export function useActiveTask(): Task | undefined {
  return useAppStore((state) =>
    state.tasks.find((task) => task.id === state.timer.activeTaskId),
  )
}

export function usePendingTasks(): Task[] {
  return useAppStore((state) => state.tasks.filter((task) => task.status === 'pending'))
}

export function useDoneTasks(): Task[] {
  return useAppStore((state) => state.tasks.filter((task) => task.status === 'done'))
}

export type { Session }

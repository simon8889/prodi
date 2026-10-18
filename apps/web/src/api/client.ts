import type {
  ClientCounts,
  Session,
  Settings,
  Snapshot,
  Stats,
  Task,
  TimerState,
} from '@/types'

const BASE = '/api'

export class ApiError extends Error {
  readonly status: number
  readonly code: string
  readonly field?: string

  constructor(status: number, code: string, message: string, field?: string) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.code = code
    this.field = field
  }
}

interface ErrorEnvelope {
  error?: {
    code?: string
    message?: string
    field?: string
  }
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  let response: Response
  try {
    response = await fetch(`${BASE}${path}`, {
      ...init,
      headers: {
        ...(init?.body ? { 'Content-Type': 'application/json' } : {}),
        ...init?.headers,
      },
    })
  } catch {
    throw new ApiError(0, 'network_error', 'Could not reach the prodi API')
  }

  if (response.status === 204) {
    return undefined as T
  }

  const raw = await response.text()
  let parsed: unknown = null
  if (raw) {
    try {
      parsed = JSON.parse(raw)
    } catch {
      parsed = null
    }
  }

  if (!response.ok) {
    const envelope = parsed as ErrorEnvelope | null
    throw new ApiError(
      response.status,
      envelope?.error?.code ?? 'unknown_error',
      envelope?.error?.message ?? `Request failed with status ${response.status}`,
      envelope?.error?.field,
    )
  }

  return parsed as T
}

export const api = {
  health: () => request<{ status: string; clients: ClientCounts }>('/health'),

  state: () => request<Snapshot>('/state'),

  clients: () => request<ClientCounts>('/clients'),

  stats: () => request<Stats>('/stats'),

  listTasks: () => request<Task[]>('/tasks'),

  createTask: (input: { title: string; estimatedPomodoros: number }) =>
    request<Task>('/tasks', {
      method: 'POST',
      body: JSON.stringify(input),
    }),

  updateTask: (
    id: string,
    patch: {
      title?: string
      estimatedPomodoros?: number
      completedPomodoros?: number
      status?: 'pending' | 'done'
    },
  ) =>
    request<Task>(`/tasks/${id}`, {
      method: 'PATCH',
      body: JSON.stringify(patch),
    }),

  deleteTask: (id: string) => request<void>(`/tasks/${id}`, { method: 'DELETE' }),

  reorderTasks: (ids: string[]) =>
    request<{ ok: boolean }>('/tasks/reorder', {
      method: 'POST',
      body: JSON.stringify({ ids }),
    }),

  listSessions: (limit = 20) => request<Session[]>(`/sessions?limit=${limit}`),

  getSettings: () => request<Settings>('/settings'),

  updateSettings: (settings: Settings) =>
    request<Settings>('/settings', {
      method: 'PUT',
      body: JSON.stringify(settings),
    }),

  startTimer: () => request<TimerState>('/timer/start', { method: 'POST' }),

  pauseTimer: () => request<TimerState>('/timer/pause', { method: 'POST' }),

  resetTimer: () => request<TimerState>('/timer/reset', { method: 'POST' }),

  skipTimer: () => request<TimerState>('/timer/skip', { method: 'POST' }),

  setActiveTask: (id: string) =>
    request<TimerState>('/timer/active-task', {
      method: 'POST',
      body: JSON.stringify({ id }),
    }),
}

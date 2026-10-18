import { create } from 'zustand'

export type WindowKind = 'todo' | 'pomodoro' | 'stats'

export interface WindowState {
  id: string
  kind: WindowKind
  x: number
  y: number
  width: number
  height: number
  z: number
  minimized: boolean
  collapsed: boolean
  previous?: { x: number; y: number; width: number; height: number }
}

interface WindowStoreState {
  windows: WindowState[]
  focusedId: string | null
  topZ: number
  cascadeIndex: number
  hydrated: boolean

  hydrate: () => void
  open: (kind: WindowKind) => string
  close: (id: string) => void
  closeKind: (kind: WindowKind) => void
  closeAll: () => void
  focus: (id: string) => void
  move: (id: string, x: number, y: number) => void
  resize: (id: string, width: number, height: number) => void
  toggleMaximize: (id: string) => void
  toggleMinimize: (id: string) => void
  toggleCollapse: (id: string) => void
  focusKind: (kind: WindowKind) => void
}

interface PersistedWindow {
  id: string
  kind: WindowKind
  x: number
  y: number
  width: number
  height: number
}

const STORAGE_KEY = 'prodi.windows.v1'

export const WINDOW_PRESETS: Record<
  WindowKind,
  { title: string; subtitle: string; width: number; height: number; minWidth: number; minHeight: number }
> = {
  todo: {
    title: 'To Do',
    subtitle: 'Tasks',
    width: 404,
    height: 496,
    minWidth: 320,
    minHeight: 300,
  },
  pomodoro: {
    title: 'Pomodoro',
    subtitle: 'Focus timer',
    width: 352,
    height: 496,
    minWidth: 300,
    minHeight: 380,
  },
  stats: {
    title: 'Insights',
    subtitle: 'Focus history',
    width: 380,
    height: 460,
    minWidth: 300,
    minHeight: 280,
  },
}

const DEFAULT_WINDOWS: PersistedWindow[] = [
  { id: 'todo-main', kind: 'todo', x: 92, y: 96, width: 404, height: 496 },
  { id: 'pomodoro-main', kind: 'pomodoro', x: 540, y: 132, width: 352, height: 496 },
]

const MENUBAR_HEIGHT = 26
const CASCADE_STEP = 30

function readStorage(): PersistedWindow[] {
  try {
    const raw = window.localStorage.getItem(STORAGE_KEY)
    if (!raw) {
      return DEFAULT_WINDOWS
    }
    const parsed = JSON.parse(raw) as unknown
    if (!Array.isArray(parsed)) {
      return DEFAULT_WINDOWS
    }
    const valid = parsed.filter(
      (entry): entry is PersistedWindow =>
        typeof entry === 'object' &&
        entry !== null &&
        typeof (entry as PersistedWindow).id === 'string' &&
        typeof (entry as PersistedWindow).kind === 'string' &&
        (entry as PersistedWindow).kind in WINDOW_PRESETS,
    )
    return valid.length > 0 ? valid : DEFAULT_WINDOWS
  } catch {
    return DEFAULT_WINDOWS
  }
}

function writeStorage(windows: WindowState[]): void {
  try {
    const payload: PersistedWindow[] = windows.map((win) => ({
      id: win.id,
      kind: win.kind,
      x: Math.round(win.x),
      y: Math.round(win.y),
      width: Math.round(win.width),
      height: Math.round(win.height),
    }))
    window.localStorage.setItem(STORAGE_KEY, JSON.stringify(payload))
  } catch {
    return
  }
}

function clampToViewport(
  value: { x: number; y: number; width: number; height: number },
): { x: number; y: number; width: number; height: number } {
  const viewportWidth = window.innerWidth
  const viewportHeight = window.innerHeight
  const width = Math.min(value.width, Math.max(value.width, viewportWidth - 32))
  const height = Math.min(value.height, Math.max(value.height, viewportHeight - MENUBAR_HEIGHT - 96))
  const maxX = Math.max(MENUBAR_HEIGHT + 8, viewportWidth - width + 24)
  const maxY = Math.max(MENUBAR_HEIGHT + 8, viewportHeight - 80)
  return {
    width,
    height,
    x: Math.min(Math.max(value.x, -width + 96), maxX),
    y: Math.min(Math.max(value.y, MENUBAR_HEIGHT + 6), maxY),
  }
}

export const useWindowStore = create<WindowStoreState>((set, get) => ({
  windows: [],
  focusedId: null,
  topZ: 10,
  cascadeIndex: 0,
  hydrated: false,

  hydrate: () => {
    if (get().hydrated) {
      return
    }
    const stored = readStorage()
    const topZ = stored.length + 10
    const windows = stored.map((entry, index) => {
      const preset = WINDOW_PRESETS[entry.kind]
      const geometry = clampToViewport({
        x: entry.x,
        y: entry.y,
        width: entry.width || preset.width,
        height: entry.height || preset.height,
      })
      return {
        id: entry.id,
        kind: entry.kind,
        ...geometry,
        z: index + 10,
        minimized: false,
        collapsed: false,
      }
    })
    set({
      windows,
      focusedId: windows[0]?.id ?? null,
      topZ,
      cascadeIndex: 0,
      hydrated: true,
    })
  },

  open: (kind) => {
    const state = get()
    const preset = WINDOW_PRESETS[kind]
    const existing = state.windows.find((win) => win.kind === kind)
    const topZ = state.topZ + 1

    if (existing) {
      set({
        topZ,
        focusedId: existing.id,
        windows: state.windows.map((win) =>
          win.id === existing.id ? { ...win, z: topZ, minimized: false } : win,
        ),
      })
      return existing.id
    }

    const id = `${kind}-${Date.now().toString(36)}`
    const offset = (state.cascadeIndex % 6) * CASCADE_STEP
    const geometry = clampToViewport({
      x: 120 + offset,
      y: 92 + offset,
      width: preset.width,
      height: preset.height,
    })

    const window: WindowState = {
      id,
      kind,
      ...geometry,
      z: topZ,
      minimized: false,
      collapsed: false,
    }

    const windows = [...state.windows, window]
    set({ windows, topZ, focusedId: id, cascadeIndex: state.cascadeIndex + 1 })
    writeStorage(windows)
    return id
  },

  close: (id) => {
    const state = get()
    const windows = state.windows.filter((win) => win.id !== id)
    const focusedId =
      state.focusedId === id
        ? (windows.filter((win) => !win.minimized).sort((a, b) => b.z - a.z)[0]?.id ?? null)
        : state.focusedId
    set({ windows, focusedId })
    writeStorage(windows)
  },

  closeKind: (kind) => {
    const ids = get()
      .windows.filter((win) => win.kind === kind)
      .map((win) => win.id)
    ids.forEach((id) => get().close(id))
  },

  closeAll: () => {
    set({ windows: [], focusedId: null })
    writeStorage([])
  },

  focus: (id) => {
    const state = get()
    const target = state.windows.find((win) => win.id === id)
    if (!target) {
      return
    }
    if (state.focusedId === id && target.z === state.topZ) {
      if (!target.minimized) {
        return
      }
    }
    const topZ = state.topZ + 1
    set({
      topZ,
      focusedId: id,
      windows: state.windows.map((win) =>
        win.id === id ? { ...win, z: topZ, minimized: false } : win,
      ),
    })
  },

  move: (id, x, y) => {
    set((state) => {
      const windows = state.windows.map((win) => {
        if (win.id !== id) {
          return win
        }
        const clamped = clampToViewport({ ...win, x, y })
        return { ...win, ...clamped }
      })
      writeStorage(windows)
      return { windows }
    })
  },

  resize: (id, width, height) => {
    set((state) => {
      const windows = state.windows.map((win) => {
        if (win.id !== id) {
          return win
        }
        const preset = WINDOW_PRESETS[win.kind]
        return {
          ...win,
          width: Math.max(preset.minWidth, Math.round(width)),
          height: Math.max(preset.minHeight, Math.round(height)),
        }
      })
      writeStorage(windows)
      return { windows }
    })
  },

  toggleMaximize: (id) => {
    set((state) => {
      const windows = state.windows.map((win) => {
        if (win.id !== id) {
          return win
        }
        if (win.previous) {
          return {
            ...win,
            ...win.previous,
            previous: undefined,
            collapsed: false,
          }
        }
        const previous = {
          x: win.x,
          y: win.y,
          width: win.width,
          height: win.height,
        }
        return {
          ...win,
          previous,
          x: 0,
          y: MENUBAR_HEIGHT,
          width: window.innerWidth,
          height: window.innerHeight - MENUBAR_HEIGHT,
          collapsed: false,
        }
      })
      writeStorage(windows)
      return { windows }
    })
  },

  toggleMinimize: (id) => {
    set((state) => {
      const windows = state.windows.map((win) =>
        win.id === id ? { ...win, minimized: !win.minimized } : win,
      )
      const focusedId =
        state.focusedId === id && windows.find((win) => win.id === id)?.minimized
          ? (windows
              .filter((win) => !win.minimized)
              .sort((a, b) => b.z - a.z)[0]?.id ?? null)
          : state.focusedId
      writeStorage(windows)
      return { windows, focusedId }
    })
  },

  toggleCollapse: (id) => {
    set((state) => {
      const windows = state.windows.map((win) =>
        win.id === id ? { ...win, collapsed: !win.collapsed } : win,
      )
      writeStorage(windows)
      return { windows }
    })
  },

  focusKind: (kind) => {
    const state = get()
    const target = state.windows
      .filter((win) => win.kind === kind && !win.minimized)
      .sort((a, b) => b.z - a.z)[0]
    if (target) {
      get().focus(target.id)
      return
    }
    get().open(kind)
  },
}))

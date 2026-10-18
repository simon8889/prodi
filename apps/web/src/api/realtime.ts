import type { ClientMessageType } from '@/types'

export type ConnectionStatus = 'idle' | 'connecting' | 'online' | 'offline'

export interface Envelope<T = unknown> {
  type: string
  payload?: T
  at: string
}

export interface SocketHandlers {
  onEnvelope: (envelope: Envelope) => void
  onStatus: (status: ConnectionStatus) => void
}

const MAX_BACKOFF_MS = 15000
const BASE_BACKOFF_MS = 600
const HEARTBEAT_MS = 25000
const PONG_TIMEOUT_MS = 12000

export class RealtimeClient {
  private socket: WebSocket | null = null
  private reconnectTimer: number | null = null
  private heartbeatTimer: number | null = null
  private pongTimer: number | null = null
  private attempt = 0
  private disposed = false
  private status: ConnectionStatus = 'idle'

  constructor(private readonly handlers: SocketHandlers) {}

  private url(): string {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    return `${protocol}//${window.location.host}/ws?kind=web&name=dashboard`
  }

  connect(): void {
    if (this.disposed) {
      return
    }
    this.clearReconnect()
    this.setStatus(this.attempt === 0 ? 'connecting' : 'connecting')

    let socket: WebSocket
    try {
      socket = new WebSocket(this.url())
    } catch {
      this.scheduleReconnect()
      return
    }

    this.socket = socket

    socket.onopen = () => {
      this.attempt = 0
      this.setStatus('online')
      this.startHeartbeat()
    }

    socket.onmessage = (event: MessageEvent<string>) => {
      if (typeof event.data !== 'string') {
        return
      }
      let envelope: Envelope
      try {
        envelope = JSON.parse(event.data) as Envelope
      } catch {
        return
      }
      if (typeof envelope?.type !== 'string') {
        return
      }
      this.handlers.onEnvelope(envelope)
    }

    socket.onerror = () => {
      this.clearHeartbeat()
    }

    socket.onclose = () => {
      this.socket = null
      this.clearHeartbeat()
      if (this.disposed) {
        this.setStatus('idle')
        return
      }
      this.setStatus('offline')
      this.scheduleReconnect()
    }
  }

  disconnect(): void {
    this.disposed = true
    this.clearReconnect()
    this.clearHeartbeat()
    this.socket?.close(1000, 'client disconnect')
    this.socket = null
    this.setStatus('idle')
  }

  reconnectNow(): void {
    if (this.disposed) {
      return
    }
    this.attempt = 0
    this.socket?.close(4000, 'forced reconnect')
    this.socket = null
    this.clearReconnect()
    this.connect()
  }

  send(type: ClientMessageType, payload?: unknown): boolean {
    const socket = this.socket
    if (!socket || socket.readyState !== WebSocket.OPEN) {
      return false
    }
    socket.send(JSON.stringify({ type, payload }))
    return true
  }

  get connected(): boolean {
    return this.socket?.readyState === WebSocket.OPEN
  }

  get currentStatus(): ConnectionStatus {
    return this.status
  }

  private setStatus(status: ConnectionStatus): void {
    if (this.status === status) {
      return
    }
    this.status = status
    this.handlers.onStatus(status)
  }

  private startHeartbeat(): void {
    this.clearHeartbeat()
    this.heartbeatTimer = window.setInterval(() => {
      this.send('ping')
      if (this.pongTimer !== null) {
        return
      }
      this.pongTimer = window.setTimeout(() => {
        this.pongTimer = null
        if (this.connected) {
          this.socket?.close(4001, 'heartbeat timeout')
        }
      }, PONG_TIMEOUT_MS)
    }, HEARTBEAT_MS)
  }

  private clearHeartbeat(): void {
    if (this.heartbeatTimer !== null) {
      window.clearInterval(this.heartbeatTimer)
      this.heartbeatTimer = null
    }
    if (this.pongTimer !== null) {
      window.clearTimeout(this.pongTimer)
      this.pongTimer = null
    }
  }

  private scheduleReconnect(): void {
    if (this.disposed || this.reconnectTimer !== null) {
      return
    }
    const delay = Math.min(BASE_BACKOFF_MS * 2 ** this.attempt, MAX_BACKOFF_MS)
    this.attempt += 1
    this.reconnectTimer = window.setTimeout(() => {
      this.reconnectTimer = null
      this.connect()
    }, delay)
  }

  private clearReconnect(): void {
    if (this.reconnectTimer !== null) {
      window.clearTimeout(this.reconnectTimer)
      this.reconnectTimer = null
    }
  }
}

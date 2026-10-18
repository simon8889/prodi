import { useEffect, useRef, useState } from 'react'

const FRAME_MS = 200

export function useNow(active = true): number {
  const [now, setNow] = useState(() => Date.now())

  useEffect(() => {
    if (!active) {
      return
    }
    const interval = window.setInterval(() => setNow(Date.now()), FRAME_MS)
    return () => window.clearInterval(interval)
  }, [active])

  return now
}

export function useServerClock(serverOffsetMs: number, active = true): number {
  const [now, setNow] = useState(() => Date.now() + serverOffsetMs)
  const offsetRef = useRef(serverOffsetMs)

  useEffect(() => {
    offsetRef.current = serverOffsetMs
  }, [serverOffsetMs])

  useEffect(() => {
    if (!active) {
      return
    }
    const interval = window.setInterval(
      () => setNow(Date.now() + offsetRef.current),
      FRAME_MS,
    )
    return () => window.clearInterval(interval)
  }, [active])

  return now
}

export function useClockLabel(): string {
  const now = useNow()
  return useMemoFormatTime(now)
}

function useMemoFormatTime(now: number): string {
  const date = new Date(now)
  const hours = date.getHours()
  const minutes = date.getMinutes()
  const suffix = hours >= 12 ? 'PM' : 'AM'
  const display = hours % 12 === 0 ? 12 : hours % 12
  return `${display}:${String(minutes).padStart(2, '0')} ${suffix}`
}

export function useWindowSize(): { width: number; height: number } {
  const [size, setSize] = useState(() => ({
    width: window.innerWidth,
    height: window.innerHeight,
  }))

  useEffect(() => {
    let frame = 0
    const handleResize = () => {
      window.cancelAnimationFrame(frame)
      frame = window.requestAnimationFrame(() => {
        setSize({ width: window.innerWidth, height: window.innerHeight })
      })
    }
    window.addEventListener('resize', handleResize)
    return () => {
      window.cancelAnimationFrame(frame)
      window.removeEventListener('resize', handleResize)
    }
  }, [])

  return size
}

import { useCallback, useEffect, useState, type ReactNode } from 'react'
import { Rnd, type ResizableDelta } from 'react-rnd'

import { Icon } from '@/components/common/Icon'
import { WINDOW_PRESETS, type WindowState } from '@/store/useWindowStore'

import '@/styles/window.css'

export const MENUBAR_HEIGHT = 26

const COLLAPSED_HEIGHT = 268

interface WindowFrameProps {
  window: WindowState
  active: boolean
  title: string
  subtitle?: ReactNode
  trailing?: ReactNode
  collapsible?: boolean
  children: ReactNode
  onClose: () => void
  onFocus: () => void
  onMove: (x: number, y: number) => void
  onResize: (width: number, height: number) => void
  onToggleMaximize: () => void
  onToggleCollapse: () => void
}

export function WindowFrame({
  window: win,
  active,
  title,
  subtitle,
  trailing,
  collapsible = false,
  children,
  onClose,
  onFocus,
  onMove,
  onResize,
  onToggleMaximize,
  onToggleCollapse,
}: WindowFrameProps) {
  const [trafficHovered, setTrafficHovered] = useState(false)

  const preset = WINDOW_PRESETS[win.kind]
  const maximized = win.previous !== undefined
  const collapsed = collapsible && win.collapsed
  const resizable = !maximized && !collapsed

  const handleDragStop = useCallback(
    (_event: MouseEvent | TouchEvent, data: { x: number; y: number }) => {
      onMove(Math.round(data.x), Math.round(data.y))
    },
    [onMove],
  )

  const handleResizeStop = useCallback(
    (
      _event: MouseEvent | TouchEvent,
      _dir: string,
      _elementRef: HTMLElement,
      delta: ResizableDelta,
      position: { x: number; y: number },
    ) => {
      onResize(delta.width, delta.height)
      if (maximized) {
        onMove(Math.round(position.x), Math.round(position.y))
      }
    },
    [maximized, onMove, onResize],
  )

  useEffect(() => {
    if (!trafficHovered) {
      return
    }
    const release = () => setTrafficHovered(false)
    window.addEventListener('mouseup', release)
    return () => window.removeEventListener('mouseup', release)
  }, [trafficHovered])

  if (win.minimized) {
    return null
  }

  return (
    <Rnd
      className={[
        'window',
        active ? 'window--active' : '',
        maximized ? 'window--maximized' : '',
        collapsed ? 'window--collapsed' : '',
      ]
        .filter(Boolean)
        .join(' ')}
      style={{ zIndex: win.z }}
      position={{ x: win.x, y: win.y }}
      size={{
        width: win.width,
        height: collapsed ? COLLAPSED_HEIGHT : win.height,
      }}
      minWidth={preset.minWidth}
      minHeight={preset.minHeight}
      bounds="parent"
      dragHandleClassName="window__titlebar"
      cancel="button, input, textarea, select, a, [role='button']"
      enableResizing={resizable}
      disableDragging={maximized || collapsed}
      resizeHandleComponent={{
        top: <ResizeHandle edge="n" />,
        bottom: <ResizeHandle edge="s" />,
        left: <ResizeHandle edge="w" />,
        right: <ResizeHandle edge="e" />,
        topLeft: <ResizeHandle edge="nw" />,
        topRight: <ResizeHandle edge="ne" />,
        bottomLeft: <ResizeHandle edge="sw" />,
        bottomRight: <ResizeHandle edge="se" />,
      }}
      onMouseDown={onFocus}
      onDragStop={handleDragStop}
      onResizeStop={handleResizeStop}
    >
      <div
        className="window__titlebar"
        onDoubleClick={onToggleMaximize}
        onMouseDown={onFocus}
      >
        <div
          className={`traffic${trafficHovered ? ' traffic--hovered' : ''}`}
          onMouseEnter={() => setTrafficHovered(true)}
          onMouseLeave={() => setTrafficHovered(false)}
          onDoubleClick={(event) => event.stopPropagation()}
        >
          <button
            type="button"
            className="traffic__button traffic__button--close"
            aria-label={`Close ${title}`}
            onClick={onClose}
          >
            <Icon name="close" size={7} strokeWidth={3} className="traffic__glyph" />
          </button>
          <button
            type="button"
            className="traffic__button traffic__button--minimize"
            aria-label={`Minimize ${title}`}
            onClick={onClose}
          >
            <Icon name="minus" size={7} strokeWidth={3} className="traffic__glyph" />
          </button>
          <button
            type="button"
            className="traffic__button traffic__button--maximize"
            aria-label={maximized ? `Restore ${title}` : `Maximize ${title}`}
            onClick={onToggleMaximize}
          >
            <Icon name="maximize" size={6} strokeWidth={2} className="traffic__glyph" />
          </button>
        </div>

        <div className="window__title-group">
          <div className="window__title">{title}</div>
          {subtitle ? <div className="window__subtitle">{subtitle}</div> : null}
        </div>

        {trailing ? <div className="window__trailing">{trailing}</div> : null}

        {collapsible ? (
          <button
            type="button"
            className="window__collapse-toggle"
            aria-label={win.collapsed ? `Expand ${title}` : `Collapse ${title}`}
            onClick={onToggleCollapse}
          >
            <Icon name={win.collapsed ? 'chevron-down' : 'chevron-up'} size={14} />
          </button>
        ) : null}
      </div>

      <div className="window__body">{children}</div>
    </Rnd>
  )
}

function ResizeHandle({ edge }: { edge: 'n' | 's' | 'w' | 'e' | 'nw' | 'ne' | 'sw' | 'se' }) {
  return <div className={`window__resize-handle window__resize-handle--${edge}`} />
}

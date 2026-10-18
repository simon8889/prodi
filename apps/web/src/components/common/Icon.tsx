import type { ReactElement } from 'react'

export type IconName =
  | 'plus'
  | 'check'
  | 'play'
  | 'pause'
  | 'reset'
  | 'skip'
  | 'close'
  | 'minus'
  | 'maximize'
  | 'chevron-down'
  | 'chevron-up'
  | 'trash'
  | 'target'
  | 'clock'
  | 'flame'
  | 'checkCircle'
  | 'list'
  | 'chart'
  | 'search'
  | 'wifi'
  | 'sparkle'
  | 'apple'
  | 'keyboard'
  | 'inbox'
  | 'focus'

interface IconProps {
  name: IconName
  size?: number
  strokeWidth?: number
  className?: string
}

const PATHS: Record<IconName, ReactElement> = {
  plus: <path d="M12 5v14M5 12h14" />,
  check: <path d="M20 6 9 17l-5-5" />,
  play: <path d="M7 4.5v15l12-7.5-12-7.5Z" fill="currentColor" stroke="none" />,
  pause: (
    <>
      <rect x="6" y="4.5" width="4" height="15" rx="1.2" fill="currentColor" stroke="none" />
      <rect x="14" y="4.5" width="4" height="15" rx="1.2" fill="currentColor" stroke="none" />
    </>
  ),
  reset: (
    <>
      <path d="M3 12a9 9 0 1 0 2.64-6.36" />
      <path d="M3 4v5h5" />
    </>
  ),
  skip: (
    <>
      <path d="M6 5.5v13l9-6.5-9-6.5Z" fill="currentColor" stroke="none" />
      <path d="M18 5v14" />
    </>
  ),
  close: <path d="M6 6l12 12M18 6 6 18" />,
  minus: <path d="M6 12h12" />,
  maximize: (
    <>
      <path d="M9 9h6v6H9z" fill="currentColor" stroke="none" />
    </>
  ),
  'chevron-down': <path d="m6 9.5 6 6 6-6" />,
  'chevron-up': <path d="m6 14.5 6-6 6 6" />,
  trash: (
    <>
      <path d="M4 7h16" />
      <path d="M10 11v6M14 11v6" />
      <path d="M6 7l1 12a2 2 0 0 0 2 2h6a2 2 0 0 0 2-2l1-12" />
      <path d="M9 7V5a2 2 0 0 1 2-2h2a2 2 0 0 1 2 2v2" />
    </>
  ),
  target: (
    <>
      <circle cx="12" cy="12" r="8.5" />
      <circle cx="12" cy="12" r="4.5" />
      <circle cx="12" cy="12" r="0.8" fill="currentColor" stroke="none" />
    </>
  ),
  clock: (
    <>
      <circle cx="12" cy="12" r="8.5" />
      <path d="M12 7.5V12l3 2" />
    </>
  ),
  flame: (
    <path d="M12 3s5 3.6 5 8.2A5 5 0 0 1 7 11.2C7 8 9 6 9 6s0 2 1.5 2.6C12 7.6 12 3 12 3Z" />
  ),
  checkCircle: (
    <>
      <circle cx="12" cy="12" r="8.5" />
      <path d="m8.5 12.2 2.4 2.4 4.6-5" />
    </>
  ),
  list: (
    <>
      <path d="M9 6.5h11M9 12h11M9 17.5h11" />
      <path d="M4.5 6.5h.01M4.5 12h.01M4.5 17.5h.01" />
    </>
  ),
  chart: (
    <>
      <path d="M4 20V10M10 20V4M16 20v-7M22 20H2" />
    </>
  ),
  search: (
    <>
      <circle cx="11" cy="11" r="6.5" />
      <path d="m16 16 4.5 4.5" />
    </>
  ),
  wifi: (
    <>
      <path d="M2.5 8.5a15 15 0 0 1 19 0" />
      <path d="M6 12.2a10 10 0 0 1 12 0" />
      <path d="M9.3 15.9a5 5 0 0 1 5.4 0" />
      <path d="M12 19.5h.01" />
    </>
  ),
  sparkle: (
    <path d="M12 3.5 13.7 9l5.5 1.7-5.5 1.7L12 18l-1.7-5.6L4.8 10.7 10.3 9 12 3.5Z" />
  ),
  apple: (
    <path
      d="M16.4 12.6c0-2.3 1.9-3.4 2-3.5-1.1-1.6-2.8-1.8-3.4-1.8-1.4-.2-2.8.9-3.5.9-.7 0-1.9-.9-3.1-.8-1.6 0-3.1.9-3.9 2.4-1.7 2.9-.4 7.2 1.2 9.5.8 1.2 1.7 2.4 3 2.4 1.2 0 1.6-.8 3.1-.8s1.9.8 3.1.8c1.3 0 2.1-1.2 2.9-2.3.9-1.3 1.3-2.6 1.3-2.7 0 0-2.5-1-2.6-3.9ZM14.2 5.6c.6-.8 1.1-1.9 1-3-1 0-2.1.7-2.8 1.4-.6.7-1.1 1.8-1 2.9 1.1.1 2.2-.6 2.8-1.3Z"
      fill="currentColor"
      stroke="none"
    />
  ),
  keyboard: (
    <>
      <rect x="2.5" y="6" width="19" height="12" rx="2.5" />
      <path d="M6 9.5h.01M9 9.5h.01M12 9.5h.01M15 9.5h.01M18 9.5h.01M6 13h.01M9 13h.01M12 13h.01M15 13h.01M18 13h.01M8 15.5h8" />
    </>
  ),
  inbox: (
    <>
      <path d="M3 13.5 5.5 5.5A2 2 0 0 1 7.4 4.2h9.2a2 2 0 0 1 1.9 1.3L21 13.5v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4Z" />
      <path d="M3 13.5h4.5l1 2.5h7l1-2.5H21" />
    </>
  ),
  focus: (
    <>
      <circle cx="12" cy="13" r="7.5" />
      <path d="M12 9.5V13l2.5 1.8" />
      <path d="M9 2.5h6" />
    </>
  ),
}

export function Icon({ name, size = 15, strokeWidth = 1.8, className }: IconProps) {
  return (
    <svg
      className={className}
      width={size}
      height={size}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth={strokeWidth}
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
      focusable="false"
    >
      {PATHS[name]}
    </svg>
  )
}

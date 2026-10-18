interface ProgressProps {
  value: number
  variant?: 'accent' | 'green' | 'orange' | 'purple'
  className?: string
}

const VARIANT_CLASS = {
  accent: '',
  green: 'progress__fill--green',
  orange: 'progress__fill--orange',
  purple: 'progress__fill--purple',
} as const

export function Progress({ value, variant = 'accent', className }: ProgressProps) {
  const clamped = Math.min(1, Math.max(0, Number.isFinite(value) ? value : 0))
  return (
    <div className={['progress', className].filter(Boolean).join(' ')}>
      <div
        className={['progress__fill', VARIANT_CLASS[variant]].filter(Boolean).join(' ')}
        style={{ width: `${clamped * 100}%` }}
      />
    </div>
  )
}

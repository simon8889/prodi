import { useState } from 'react'

interface StepperProps {
  value: number
  min: number
  max: number
  onChange: (value: number) => void
  label?: string
}

export function Stepper({ value, min, max, onChange, label }: StepperProps) {
  const [pending, setPending] = useState(false)

  const commit = (next: number) => {
    if (next < min || next > max || next === value) {
      return
    }
    setPending(true)
    onChange(next)
    window.setTimeout(() => setPending(false), 180)
  }

  return (
    <div className="stepper" role="group" aria-label={label}>
      <button
        type="button"
        className="stepper__button"
        onClick={() => commit(value - 1)}
        disabled={value <= min || pending}
        aria-label={`Decrease ${label ?? 'value'}`}
      >
        <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.4" strokeLinecap="round" aria-hidden="true">
          <path d="M6 12h12" />
        </svg>
      </button>
      <span className="stepper__value">{value}</span>
      <button
        type="button"
        className="stepper__button"
        onClick={() => commit(value + 1)}
        disabled={value >= max || pending}
        aria-label={`Increase ${label ?? 'value'}`}
      >
        <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.4" strokeLinecap="round" aria-hidden="true">
          <path d="M12 6v12M6 12h12" />
        </svg>
      </button>
    </div>
  )
}

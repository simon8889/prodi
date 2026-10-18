import type { ButtonHTMLAttributes, ReactNode } from 'react'

import { Icon, type IconName } from './Icon'

type Variant = 'default' | 'primary' | 'ghost' | 'danger'
type Size = 'default' | 'large'

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  icon?: IconName
  iconSize?: number
  variant?: Variant
  size?: Size
  round?: boolean
  children?: ReactNode
}

export function Button({
  icon,
  iconSize,
  variant = 'default',
  size = 'default',
  round = false,
  children,
  className,
  type = 'button',
  ...rest
}: ButtonProps) {
  const classes = [
    'control',
    variant !== 'default' ? `control--${variant}` : '',
    size === 'large' ? 'control--large' : '',
    round ? 'control--round' : '',
    !children && icon ? 'control--icon' : '',
    className ?? '',
  ]
    .filter(Boolean)
    .join(' ')

  return (
    <button type={type} className={classes} {...rest}>
      {icon ? <Icon name={icon} size={iconSize ?? (size === 'large' ? 16 : 14)} /> : null}
      {children}
    </button>
  )
}

import { useAppStore, type AppNotification } from '@/store/useAppStore'

import { Icon, type IconName } from '../common/Icon'

const ICONS: Record<AppNotification['level'], IconName> = {
  success: 'checkCircle',
  info: 'sparkle',
  warning: 'flame',
  error: 'close',
}

export function NotificationStack() {
  const notifications = useAppStore((state) => state.notifications)
  const dismiss = useAppStore((state) => state.dismissNotification)

  if (notifications.length === 0) {
    return null
  }

  return (
    <div className="notification-stack" role="status" aria-live="polite">
      {notifications.map((notification) => (
        <div key={notification.id} className="notification">
          <span className={`notification__icon notification__icon--${notification.level}`}>
            <Icon name={ICONS[notification.level]} size={14} />
          </span>
          <div className="notification__body">
            <div className="notification__title">{notification.title}</div>
            <div className="notification__message">{notification.message}</div>
          </div>
          <button
            type="button"
            className="control control--ghost control--icon"
            aria-label="Dismiss notification"
            onClick={() => dismiss(notification.id)}
          >
            <Icon name="close" size={12} strokeWidth={2.2} />
          </button>
        </div>
      ))}
    </div>
  )
}

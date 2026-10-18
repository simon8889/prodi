import { memo, useCallback, useEffect, useRef, useState } from 'react'

import { Icon } from '@/components/common/Icon'
import { Progress } from '@/components/common/Progress'
import { Stepper } from '@/components/common/Stepper'
import { taskProgress, type Task } from '@/types'

import '@/styles/todo.css'

interface TaskItemProps {
  task: Task
  active: boolean
  onToggle: (task: Task) => void
  onDelete: (task: Task) => void
  onMakeActive: (task: Task) => void
  onRename: (task: Task, title: string) => void
  onEstimate: (task: Task, estimate: number) => void
}

export const TaskItem = memo(function TaskItem({
  task,
  active,
  onToggle,
  onDelete,
  onMakeActive,
  onRename,
  onEstimate,
}: TaskItemProps) {
  const done = task.status === 'done'
  const [editing, setEditing] = useState(false)
  const [draft, setDraft] = useState(task.title)
  const inputRef = useRef<HTMLInputElement>(null)

  useEffect(() => {
    if (editing) {
      inputRef.current?.focus()
      inputRef.current?.select()
    }
  }, [editing])

  const commit = useCallback(() => {
    const trimmed = draft.trim()
    setEditing(false)
    if (trimmed && trimmed !== task.title) {
      onRename(task, trimmed)
      return
    }
    setDraft(task.title)
  }, [draft, onRename, task])

  const handleKeyDown = useCallback(
    (event: React.KeyboardEvent<HTMLInputElement>) => {
      event.stopPropagation()
      if (event.key === 'Enter') {
        event.preventDefault()
        commit()
        return
      }
      if (event.key === 'Escape') {
        event.preventDefault()
        setDraft(task.title)
        setEditing(false)
      }
    },
    [commit, task.title],
  )

  const dots = Array.from({ length: Math.min(task.estimatedPomodoros, 8) }, (_, index) => (
    <span
      key={index}
      className={`todo__pomo-dot${index < task.completedPomodoros ? ' todo__pomo-dot--filled' : ''}`}
    />
  ))

  return (
    <li
      className={[
        'todo__item',
        active ? 'todo__item--active' : '',
        done ? 'todo__item--done' : '',
      ]
        .filter(Boolean)
        .join(' ')}
      onDoubleClick={() => {
        if (!editing && !done) {
          setEditing(true)
        }
      }}
    >
      <button
        type="button"
        className={`todo__checkbox${done ? ' todo__checkbox--checked' : ''}`}
        aria-label={done ? `Mark ${task.title} as not done` : `Mark ${task.title} as done`}
        aria-pressed={done}
        onClick={() => onToggle(task)}
      >
        <Icon name="check" size={10} strokeWidth={3.2} />
      </button>

      <div className="todo__main">
        <div className="todo__title-row">
          {editing ? (
            <input
              ref={inputRef}
              className="field"
              value={draft}
              onChange={(event) => setDraft(event.target.value)}
              onBlur={commit}
              onKeyDown={handleKeyDown}
              aria-label="Task title"
            />
          ) : (
            <span className="todo__title">{task.title}</span>
          )}
          {active && !done ? (
            <span className="todo__active-flag">
              <Icon name="focus" size={9} strokeWidth={2.6} />
              Focus
            </span>
          ) : null}
        </div>

        <div className="todo__meta">
          <span className="todo__pomo-dots" aria-hidden="true">
            {dots}
          </span>
          <span>
            {task.completedPomodoros}/{task.estimatedPomodoros} pomodoro
            {task.estimatedPomodoros === 1 ? '' : 's'}
          </span>
          {!done ? (
            <span className="todo__meta-actions">
              <Stepper
                value={task.estimatedPomodoros}
                min={1}
                max={12}
                label={`Pomodoros for ${task.title}`}
                onChange={(value) => onEstimate(task, value)}
              />
            </span>
          ) : null}
        </div>

        {!done ? (
          <Progress
            value={taskProgress(task)}
            variant={active ? 'accent' : 'green'}
            className="todo__item-progress"
          />
        ) : null}
      </div>

      <div className="todo__actions">
        {!done ? (
          <button
            type="button"
            className={`todo__action${active ? ' todo__action--on' : ''}`}
            aria-label={active ? `${task.title} is active` : `Focus on ${task.title}`}
            title={active ? 'Active task' : 'Set as active'}
            disabled={active}
            onClick={() => onMakeActive(task)}
          >
            <Icon name="target" size={14} />
          </button>
        ) : null}
        <button
          type="button"
          className="todo__action todo__action--danger"
          aria-label={`Delete ${task.title}`}
          title="Delete task"
          onClick={() => onDelete(task)}
        >
          <Icon name="trash" size={14} />
        </button>
      </div>
    </li>
  )
})

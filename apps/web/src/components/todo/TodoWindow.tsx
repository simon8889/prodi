import { useCallback, useMemo, useState } from 'react'

import { Icon } from '@/components/common/Icon'
import { Stepper } from '@/components/common/Stepper'
import { TaskItem } from '@/components/todo/TaskItem'
import { useAppStore } from '@/store/useAppStore'
import type { Task } from '@/types'

import '@/styles/todo.css'

export function TodoWindow() {
  const tasks = useAppStore((state) => state.tasks)
  const activeTaskId = useAppStore((state) => state.timer.activeTaskId)
  const createTask = useAppStore((state) => state.createTask)
  const setTaskDone = useAppStore((state) => state.setTaskDone)
  const deleteTask = useAppStore((state) => state.deleteTask)
  const makeActive = useAppStore((state) => state.makeActive)
  const renameTask = useAppStore((state) => state.renameTask)
  const setTaskEstimate = useAppStore((state) => state.setTaskEstimate)

  const [query, setQuery] = useState('')
  const [title, setTitle] = useState('')
  const [estimate, setEstimate] = useState(1)
  const [showDone, setShowDone] = useState(true)

  const normalizedQuery = query.trim().toLowerCase()

  const { pending, done } = useMemo(() => {
    const matches = (task: Task) =>
      normalizedQuery === '' || task.title.toLowerCase().includes(normalizedQuery)

    const all = tasks.filter(matches)
    return {
      pending: all.filter((task) => task.status === 'pending'),
      done: all.filter((task) => task.status === 'done'),
    }
  }, [normalizedQuery, tasks])

  const completedPomodoros = useMemo(
    () => tasks.reduce((total, task) => total + task.completedPomodoros, 0),
    [tasks],
  )

  const submit = useCallback(
    async (event: React.FormEvent) => {
      event.preventDefault()
      const trimmed = title.trim()
      if (!trimmed) {
        return
      }
      setTitle('')
      const nextEstimate = estimate
      setEstimate(1)
      await createTask(trimmed, nextEstimate)
    },
    [createTask, estimate, title],
  )

  const handleToggle = useCallback(
    (task: Task) => {
      void setTaskDone(task.id, task.status !== 'done')
    },
    [setTaskDone],
  )

  const handleDelete = useCallback(
    (task: Task) => {
      void deleteTask(task.id)
    },
    [deleteTask],
  )

  const handleMakeActive = useCallback(
    (task: Task) => {
      void makeActive(task.id)
    },
    [makeActive],
  )

  const handleRename = useCallback(
    (task: Task, nextTitle: string) => {
      void renameTask(task.id, nextTitle)
    },
    [renameTask],
  )

  const handleEstimate = useCallback(
    (task: Task, nextEstimate: number) => {
      void setTaskEstimate(task.id, nextEstimate)
    },
    [setTaskEstimate],
  )

  const total = pending.length + done.length

  return (
    <div className="todo">
      <div className="todo__toolbar">
        <div className="todo__search">
          <Icon name="search" size={13} />
          <input
            className="field"
            type="search"
            value={query}
            placeholder="Filter tasks"
            aria-label="Filter tasks"
            onChange={(event) => setQuery(event.target.value)}
          />
        </div>
        <button
          type="button"
          className={`control control--icon${showDone ? '' : ' control--primary'}`}
          aria-pressed={showDone}
          aria-label={showDone ? 'Hide completed tasks' : 'Show completed tasks'}
          title={showDone ? 'Hide completed' : 'Show completed'}
          onClick={() => setShowDone((value) => !value)}
        >
          <Icon name="checkCircle" size={14} />
        </button>
      </div>

      <div className="todo__list desktop-scrollbar">
        {total === 0 ? (
          <div className="todo__empty">
            <div className="todo__empty-icon">
              <Icon name={normalizedQuery ? 'search' : 'inbox'} size={19} />
            </div>
            <div className="todo__empty-title">
              {normalizedQuery ? 'No matching tasks' : 'Nothing on the list'}
            </div>
            <div className="todo__empty-hint">
              {normalizedQuery
                ? `Nothing matches "${query.trim()}". Try a different word.`
                : 'Add a task below, then finish a focus block to tick it off.'}
            </div>
          </div>
        ) : null}

        {pending.length > 0 ? (
          <section className="todo__group">
            <header className="todo__group-header">
              <span>To do</span>
              <span className="todo__group-count">{pending.length}</span>
            </header>
            <ul>
              {pending.map((task) => (
                <TaskItem
                  key={task.id}
                  task={task}
                  active={task.id === activeTaskId}
                  onToggle={handleToggle}
                  onDelete={handleDelete}
                  onMakeActive={handleMakeActive}
                  onRename={handleRename}
                  onEstimate={handleEstimate}
                />
              ))}
            </ul>
          </section>
        ) : null}

        {showDone && done.length > 0 ? (
          <section className="todo__group">
            <header className="todo__group-header">
              <span>Completed</span>
              <span className="todo__group-count">{done.length}</span>
            </header>
            <ul>
              {done.map((task) => (
                <TaskItem
                  key={task.id}
                  task={task}
                  active={false}
                  onToggle={handleToggle}
                  onDelete={handleDelete}
                  onMakeActive={handleMakeActive}
                  onRename={handleRename}
                  onEstimate={handleEstimate}
                />
              ))}
            </ul>
          </section>
        ) : null}
      </div>

      <form className="todo__composer" onSubmit={submit}>
        <input
          className="field"
          value={title}
          placeholder="Add a task"
          aria-label="New task title"
          maxLength={200}
          onChange={(event) => setTitle(event.target.value)}
        />
        <div className="todo__estimate">
          <Stepper value={estimate} min={1} max={12} label="Estimated pomodoros" onChange={setEstimate} />
        </div>
        <button type="submit" className="control control--primary" disabled={title.trim() === ''}>
          <Icon name="plus" size={14} strokeWidth={2.2} />
          Add
        </button>
      </form>

      <footer className="todo__footer">
        <span>
          <strong>{pending.length}</strong> open
          {done.length > 0 ? (
            <>
              {' · '}
              <strong>{done.length}</strong> done
            </>
          ) : null}
        </span>
        <span>
          <strong>{completedPomodoros}</strong> focus block
          {completedPomodoros === 1 ? '' : 's'} logged
        </span>
      </footer>
    </div>
  )
}

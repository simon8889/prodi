package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"prodi/api/internal/domain"
)

const taskColumns = `id, title, estimated_pomodoros, completed_pomodoros, status, position, created_at, completed_at`

type TaskRepository struct {
	store *Store
}

func NewTaskRepository(s *Store) *TaskRepository {
	return &TaskRepository{store: s}
}

func (r *TaskRepository) Create(ctx context.Context, task *domain.Task) error {
	task.Normalize()
	if err := task.Validate(); err != nil {
		return err
	}
	if task.ID == "" {
		task.ID = uuid.NewString()
	}
	if task.Position == 0 {
		next, err := r.NextPosition(ctx)
		if err != nil {
			return err
		}
		task.Position = next
	}
	_, err := r.store.db.ExecContext(ctx, `
INSERT INTO tasks (id, title, estimated_pomodoros, completed_pomodoros, status, position, created_at, completed_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		task.ID, task.Title, task.EstimatedPomodoros, task.CompletedPomodoros,
		string(task.Status), task.Position, formatTime(task.CreatedAt), nullableTimeValue(task.CompletedAt),
	)
	if err != nil {
		return fmt.Errorf("insert task: %w", err)
	}
	return nil
}

func (r *TaskRepository) Get(ctx context.Context, id string) (domain.Task, error) {
	row := r.store.db.QueryRowContext(ctx, `SELECT `+taskColumns+` FROM tasks WHERE id = ?`, id)
	task, err := scanTask(row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Task{}, domain.ErrNotFound
	}
	return task, err
}

func (r *TaskRepository) List(ctx context.Context) ([]domain.Task, error) {
	rows, err := r.store.db.QueryContext(ctx,
		`SELECT `+taskColumns+` FROM tasks
		 ORDER BY status ASC, position ASC, created_at ASC`)
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}
	defer rows.Close()

	tasks := make([]domain.Task, 0)
	for rows.Next() {
		task, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	return tasks, rows.Err()
}

func (r *TaskRepository) ListByStatus(ctx context.Context, status domain.TaskStatus) ([]domain.Task, error) {
	rows, err := r.store.db.QueryContext(ctx,
		`SELECT `+taskColumns+` FROM tasks WHERE status = ? ORDER BY position ASC, created_at ASC`, string(status))
	if err != nil {
		return nil, fmt.Errorf("list tasks by status: %w", err)
	}
	defer rows.Close()

	tasks := make([]domain.Task, 0)
	for rows.Next() {
		task, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	return tasks, rows.Err()
}

func (r *TaskRepository) Update(ctx context.Context, task *domain.Task) error {
	task.Normalize()
	if err := task.Validate(); err != nil {
		return err
	}
	result, err := r.store.db.ExecContext(ctx, `
UPDATE tasks
SET title = ?, estimated_pomodoros = ?, completed_pomodoros = ?, status = ?, position = ?, completed_at = ?
WHERE id = ?`,
		task.Title, task.EstimatedPomodoros, task.CompletedPomodoros, string(task.Status),
		task.Position, nullableTimeValue(task.CompletedAt), task.ID,
	)
	if err != nil {
		return fmt.Errorf("update task: %w", err)
	}
	return requireAffected(result)
}

func (r *TaskRepository) Delete(ctx context.Context, id string) error {
	result, err := r.store.db.ExecContext(ctx, `DELETE FROM tasks WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete task: %w", err)
	}
	return requireAffected(result)
}

func (r *TaskRepository) Reorder(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	return withTx(ctx, r.store, func(tx *sql.Tx) error {
		for index, id := range ids {
			if _, err := tx.ExecContext(ctx, `UPDATE tasks SET position = ? WHERE id = ?`, index+1, id); err != nil {
				return fmt.Errorf("reorder task %s: %w", id, err)
			}
		}
		return nil
	})
}

func (r *TaskRepository) NextPosition(ctx context.Context) (int, error) {
	var next sql.NullInt64
	err := r.store.db.QueryRowContext(ctx, `SELECT MAX(position) FROM tasks`).Scan(&next)
	if err != nil {
		return 0, fmt.Errorf("max position: %w", err)
	}
	if !next.Valid {
		return 1, nil
	}
	return int(next.Int64) + 1, nil
}

func (r *TaskRepository) SetStatus(ctx context.Context, id string, status domain.TaskStatus, completedAt *time.Time) error {
	result, err := r.store.db.ExecContext(ctx,
		`UPDATE tasks SET status = ?, completed_at = ? WHERE id = ?`,
		string(status), nullableTimeValue(completedAt), id)
	if err != nil {
		return fmt.Errorf("set task status: %w", err)
	}
	return requireAffected(result)
}

func (r *TaskRepository) RegisterPomodoro(ctx context.Context, id string) (domain.Task, bool, error) {
	var (
		task           domain.Task
		completedCount int
		completedNow   bool
	)

	err := withTx(ctx, r.store, func(tx *sql.Tx) error {
		row := tx.QueryRowContext(ctx,
			`SELECT `+taskColumns+` FROM tasks WHERE id = ?`, id)
		current, scanErr := scanTask(row)
		if errors.Is(scanErr, sql.ErrNoRows) {
			return domain.ErrNotFound
		}
		if scanErr != nil {
			return scanErr
		}
		task = current
		if task.IsDone() {
			completedCount = task.CompletedPomodoros
			return nil
		}

		completedCount = task.CompletedPomodoros + 1
		var completedAt any
		status := string(domain.TaskStatusPending)
		if completedCount >= task.EstimatedPomodoros {
			status = string(domain.TaskStatusDone)
			completedNow = true
			now := r.store.now().UTC()
			completedAt = formatTime(now)
			task.CompletedAt = &now
		}
		task.CompletedPomodoros = completedCount
		task.Status = domain.TaskStatus(status)

		if _, err := tx.ExecContext(ctx,
			`UPDATE tasks SET completed_pomodoros = ?, status = ?, completed_at = ? WHERE id = ?`,
			completedCount, status, completedAt, id); err != nil {
			return fmt.Errorf("register pomodoro: %w", err)
		}
		return nil
	})
	if err != nil {
		return domain.Task{}, false, err
	}
	return task, completedNow, nil
}

func (r *TaskRepository) CountByStatus(ctx context.Context) (pending int, done int, err error) {
	rows, err := r.store.db.QueryContext(ctx, `SELECT status, COUNT(*) FROM tasks GROUP BY status`)
	if err != nil {
		return 0, 0, fmt.Errorf("count tasks: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var (
			status sql.NullString
			count  int
		)
		if err := rows.Scan(&status, &count); err != nil {
			return 0, 0, err
		}
		if domain.TaskStatus(status.String) == domain.TaskStatusDone {
			done = count
		} else {
			pending = count
		}
	}
	return pending, done, rows.Err()
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanTask(row rowScanner) (domain.Task, error) {
	var (
		task        domain.Task
		status      string
		createdAt   string
		completedAt sql.NullString
	)
	err := row.Scan(
		&task.ID,
		&task.Title,
		&task.EstimatedPomodoros,
		&task.CompletedPomodoros,
		&status,
		&task.Position,
		&createdAt,
		&completedAt,
	)
	if err != nil {
		return domain.Task{}, err
	}
	task.Status = domain.TaskStatus(status)
	task.CreatedAt = parseTime(createdAt)
	task.CompletedAt = nullableTime(completedAt)
	return task, nil
}

func requireAffected(result sql.Result) error {
	rows, err := result.RowsAffected()
	if err != nil {
		return nil
	}
	if rows == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func nullableTimeValue(value *time.Time) any {
	if value == nil {
		return nil
	}
	return formatTime(*value)
}

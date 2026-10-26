package tasks

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
)

const (
	TypeScanFile          = "file:scan"
	TypeDeleteExpiredFile = "file:delete_expired"

	maxRetry = 10

	// expiryGracePeriod delays the delete task slightly past expires_at so
	// the worker never runs before the database considers the file expired.
	expiryGracePeriod = time.Second
)

// Enqueuer is the part of *asynq.Client the application depends on.
type Enqueuer interface {
	EnqueueContext(
		ctx context.Context,
		task *asynq.Task,
		opts ...asynq.Option,
	) (*asynq.TaskInfo, error)
}

// FilePayload is the payload shared by all per-file tasks.
type FilePayload struct {
	FileID string `json:"file_id"`
}

func newFileTask(taskType, fileID string) (*asynq.Task, error) {
	payload, err := json.Marshal(FilePayload{FileID: fileID})
	if err != nil {
		return nil, fmt.Errorf("encoding %s payload: %w", taskType, err)
	}

	return asynq.NewTask(taskType, payload), nil
}

// ScheduleScan queues a virus scan for the file.
func ScheduleScan(
	ctx context.Context,
	queue Enqueuer,
	fileID string,
) error {
	task, err := newFileTask(TypeScanFile, fileID)
	if err != nil {
		return err
	}

	if _, err := queue.EnqueueContext(
		ctx,
		task,
		asynq.MaxRetry(maxRetry),
	); err != nil {
		return fmt.Errorf("scheduling scan task: %w", err)
	}

	return nil
}

// ScheduleExpiry queues deletion of the file for when it expires.
// It does nothing for files that never expire.
func ScheduleExpiry(
	ctx context.Context,
	queue Enqueuer,
	fileID string,
	expiresAt *time.Time,
) error {
	if expiresAt == nil {
		return nil
	}

	task, err := newFileTask(TypeDeleteExpiredFile, fileID)
	if err != nil {
		return err
	}

	if _, err := queue.EnqueueContext(
		ctx,
		task,
		asynq.ProcessAt(expiresAt.Add(expiryGracePeriod)),
		asynq.MaxRetry(maxRetry),
	); err != nil {
		return fmt.Errorf("scheduling expiry task: %w", err)
	}

	return nil
}

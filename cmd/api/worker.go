package main

import (
	"context"
	"encoding/json"
	"fmt"

	"parcel/internal/storage"

	"github.com/hibiken/asynq"
	"github.com/minio/minio-go/v7"
)

const TypeDeleteExpiredFile = "file:delete_expired"

type ExpiryPayload struct {
	FileID string `json:"file_id"`
}

func scheduleExpiry(
	ctx context.Context,
	client *asynq.Client,
	file *storage.File,
) error {
	if file.ExpiresAt == nil {
		return nil
	}

	payload, err := json.Marshal(ExpiryPayload{
		FileID: file.ID,
	})
	if err != nil {
		return fmt.Errorf("encoding expiry payload: %w", err)
	}

	task := asynq.NewTask(TypeDeleteExpiredFile, payload)

	_, err = client.EnqueueContext(
		ctx,
		task,
		asynq.ProcessAt(file.ExpiresAt.Add(1)),
		asynq.MaxRetry(10),
	)
	if err != nil {
		return fmt.Errorf("scheduling expiry task: %w", err)
	}

	return nil
}

func (a *app) handleDeleteExpiredFile(
	ctx context.Context,
	task *asynq.Task,
) error {
	var payload ExpiryPayload

	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("decoding expiry payload: %w", err)
	}

	storageKey, err := a.storage.Postgres.GetExpiredStorageKey(
		ctx,
		payload.FileID,
	)
	if err != nil {
		return err
	}

	if storageKey == "" {
		return nil
	}

	if err := a.storage.MinIO.RemoveObject(
		ctx,
		a.storage.Bucket,
		storageKey,
		minio.RemoveObjectOptions{},
	); err != nil {
		return fmt.Errorf("deleting MinIO object: %w", err)
	}

	if err := a.storage.Postgres.MarkExpired(ctx, payload.FileID); err != nil {
		return fmt.Errorf("marking file expired: %w", err)
	}

	return nil
}

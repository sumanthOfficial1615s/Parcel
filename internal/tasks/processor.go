package tasks

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/hibiken/asynq"

	"parcel/internal/scanner"
	"parcel/internal/storage"
)

// Processor handles background tasks. Its dependencies are injected so
// they can be swapped or faked in tests.
type Processor struct {
	files   storage.FileStore
	objects storage.ObjectStore
	scanner scanner.Scanner
}

func NewProcessor(
	files storage.FileStore,
	objects storage.ObjectStore,
	virusScanner scanner.Scanner,
) *Processor {
	return &Processor{
		files:   files,
		objects: objects,
		scanner: virusScanner,
	}
}

// Register wires every task handler into the mux.
func (p *Processor) Register(mux *asynq.ServeMux) {
	mux.HandleFunc(TypeDeleteExpiredFile, p.HandleDeleteExpiredFile)
	mux.HandleFunc(TypeScanFile, p.HandleScanFile)
}

func (p *Processor) HandleDeleteExpiredFile(
	ctx context.Context,
	task *asynq.Task,
) error {
	var payload FilePayload

	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("decoding expiry payload: %w", err)
	}

	storageKey, err := p.files.GetExpiredStorageKey(ctx, payload.FileID)
	if err != nil {
		return err
	}

	if storageKey == "" {
		return nil
	}

	if err := p.objects.Remove(ctx, storageKey); err != nil {
		return fmt.Errorf("deleting object: %w", err)
	}

	if err := p.files.MarkExpired(ctx, payload.FileID); err != nil {
		return fmt.Errorf("marking file expired: %w", err)
	}

	return nil
}

func (p *Processor) HandleScanFile(
	ctx context.Context,
	task *asynq.Task,
) error {
	var payload FilePayload

	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("decoding scan payload: %w", err)
	}

	storageKey, err := p.files.GetPendingStorageKey(ctx, payload.FileID)
	if err != nil {
		return err
	}

	object, err := p.objects.Get(ctx, storageKey)
	if err != nil {
		return err
	}
	defer object.Close()

	clean, err := p.scanner.Scan(ctx, object)
	if err != nil {
		return fmt.Errorf("scanning file: %w", err)
	}

	if clean {
		if err := p.files.MarkClean(ctx, payload.FileID); err != nil {
			return fmt.Errorf("marking file clean: %w", err)
		}

		return nil
	}

	if err := p.files.MarkInfected(ctx, payload.FileID); err != nil {
		return fmt.Errorf("marking file infected: %w", err)
	}

	// The file is already unreachable (downloads reject infected files);
	// delete the object too so malware is not kept in storage. A failure
	// here must not fail the task, because the status is already final
	// and a retry could not find the file in 'pending' again.
	if err := p.objects.Remove(ctx, storageKey); err != nil {
		log.Printf(
			"removing infected object for file %s: %v",
			payload.FileID,
			err,
		)
	}

	return nil
}

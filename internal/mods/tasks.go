package mods

import (
	"context"
	"errors"
	"sync"

	"github.com/AmadoMuerte/Waxlight-launcher/internal/errs"
)

// ModTaskManager tracks active ModDB download tasks separately from the
// persistent launcher operations subsystem. Each task owns the cancellation of
// its download and the reservation of the mod releases it downloads, so the
// frontend can cancel a download and the library never fetches the same
// release twice concurrently.
type ModTaskManager struct {
	publisher Publisher
	mu        sync.Mutex
	cancels   map[string]context.CancelFunc
	active    map[string]string
}

// NewModTaskManager creates a task manager publishing task events through the
// given publisher.
func NewModTaskManager(publisher Publisher) *ModTaskManager {
	return &ModTaskManager{
		publisher: publisher,
		cancels:   make(map[string]context.CancelFunc),
		active:    make(map[string]string),
	}
}

// Begin registers a task for the root release and returns a download context
// that Cancel will cancel. When the release is already downloading, the
// existing task ID is returned together with a mod-already-active error.
func (manager *ModTaskManager) Begin(
	ctx context.Context,
	taskID, modID, versionID string,
) (context.Context, string, error) {
	key := modDownloadKey(modID, versionID)
	downloadCtx, cancel := context.WithCancel(ctx)
	manager.mu.Lock()
	if existingTask, active := manager.active[key]; active {
		manager.mu.Unlock()
		cancel()
		return nil, existingTask, errs.NewError(ErrModAlreadyActive, "This mod is already downloading")
	}
	manager.active[key] = taskID
	manager.cancels[taskID] = cancel
	manager.mu.Unlock()
	return downloadCtx, "", nil
}

// Claim registers an additional release owned by an active task. The release
// stays reserved even when another task completes the same dependency first.
func (manager *ModTaskManager) Claim(taskID, modID, versionID string) error {
	key := modDownloadKey(modID, versionID)
	manager.mu.Lock()
	defer manager.mu.Unlock()
	if existingTask, active := manager.active[key]; active {
		if existingTask == taskID {
			return nil
		}
		return &ReleaseBusyError{TaskID: existingTask, Key: key}
	}
	manager.active[key] = taskID
	return nil
}

// IsDownloading reports whether any task is downloading the release.
func (manager *ModTaskManager) IsDownloading(modID, versionID string) bool {
	key := modDownloadKey(modID, versionID)
	manager.mu.Lock()
	defer manager.mu.Unlock()
	_, active := manager.active[key]
	return active
}

// Cancel cancels the task with the given ID.
func (manager *ModTaskManager) Cancel(taskID string) error {
	manager.mu.Lock()
	cancel, ok := manager.cancels[taskID]
	manager.mu.Unlock()
	if !ok {
		return errs.NewError(errs.ErrOperationNotFound, "Mod task not found")
	}
	cancel()
	return nil
}

// Release frees every reservation owned by the task and removes its
// cancellation. Stale releases never clear a newer task's reservation.
func (manager *ModTaskManager) Release(taskID string) {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	for key, owner := range manager.active {
		if owner == taskID {
			delete(manager.active, key)
		}
	}
	delete(manager.cancels, taskID)
}

// EmitProgress publishes a mods:task-progress event.
func (manager *ModTaskManager) EmitProgress(
	taskID, modID, phase string,
	downloadedBytes, totalBytes int64,
	progress float64,
	message string,
) {
	manager.publisher.Publish("mods:task-progress", map[string]any{
		"taskId": taskID, "modId": modID, "phase": phase,
		"downloadedBytes": downloadedBytes, "totalBytes": totalBytes,
		"progress": progress, "message": message,
	})
}

// EmitDownloadsChanged publishes a mods:downloads-changed event.
func (manager *ModTaskManager) EmitDownloadsChanged(
	taskID string,
	modID string,
	dependencies []modDownloadedDependencyEvent,
) {
	manager.publisher.Publish("mods:downloads-changed", map[string]any{
		"taskId":                 taskID,
		"modId":                  modID,
		"downloadedDependencies": dependencies,
	})
}

// modDownloadedDependencyEvent describes a dependency downloaded as part of a
// task.
type modDownloadedDependencyEvent struct {
	ModID   string `json:"modId"`
	Name    string `json:"name"`
	Version string `json:"version"`
}

// ReleaseBusyError reports that another task owns a release reservation.
type ReleaseBusyError struct {
	TaskID string
	Key    string
}

func (e *ReleaseBusyError) Error() string {
	return "task " + e.TaskID + " is downloading " + e.Key
}

// IsReleaseBusy reports whether the error chain contains a ReleaseBusyError.
func IsReleaseBusy(err error) bool {
	var busy *ReleaseBusyError
	return errors.As(err, &busy)
}

package instances

import (
	"context"
	"errors"
	"log/slog"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/waxlight/waxlight-launcher/internal/errs"
	"github.com/waxlight/waxlight-launcher/internal/operations"
)

const migrationSafetyMargin int64 = 64 << 20

type MigrationService struct {
	storage    MigrationStorage
	diskSpace  MigrationDiskSpace
	creator    PreparedInstanceCreator
	operations *operations.Manager
	reconcile  MigrationModReconciler
	now        Clock
	newID      IDGenerator
	dataRoot   string
}

func NewMigrationService(storage MigrationStorage, diskSpace MigrationDiskSpace, creator PreparedInstanceCreator,
	operationManager *operations.Manager, reconcile MigrationModReconciler, dataRoot string, now Clock, newID IDGenerator) *MigrationService {
	return &MigrationService{storage: storage, diskSpace: diskSpace, creator: creator,
		operations: operationManager, reconcile: reconcile, dataRoot: dataRoot, now: now, newID: newID}
}

func (service *MigrationService) Detect(ctx context.Context) ([]MigrationCandidate, error) {
	result := []MigrationCandidate{}
	for _, path := range service.storage.Discover() {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		candidate, err := service.storage.Inspect(path)
		if err == nil {
			result = append(result, candidate)
		}
	}
	return result, nil
}

func (service *MigrationService) Inspect(path string) (MigrationCandidate, error) {
	if strings.TrimSpace(path) == "" {
		return MigrationCandidate{}, errs.NewError(errs.ErrValidation, "Select a Vintage Story data directory")
	}
	return service.storage.Inspect(path)
}

func (service *MigrationService) Start(ctx context.Context, request MigrationImportRequest) (operations.Operation, error) {
	candidate, err := service.Inspect(request.SourcePath)
	if err != nil {
		return operations.Operation{}, err
	}
	if strings.TrimSpace(request.GameVersionID) == "" {
		return operations.Operation{}, errs.NewError(errs.ErrValidation, "Select an installed game version")
	}
	now := service.now().UTC()
	operation := operations.Operation{ID: service.newID(), Type: "existing_data_import", Title: "Importing Vintage Story data",
		TitleKey: "importing_existing_data", Status: operations.StatusQueued, TotalBytes: candidate.TotalBytes, CreatedAt: now}
	workerOperation := operation
	_, err = operations.Start(service.operations, ctx, operation, "existing-data-import", func(workerCtx context.Context) (Instance, error) {
		started := service.now().UTC()
		workerOperation.Status, workerOperation.StartedAt = operations.StatusRunning, &started
		service.operations.SaveBestEffort(workerOperation, operations.EventUpdated)
		instance, importErr := service.importData(workerCtx, request, candidate, func(instanceID string) {
			workerOperation.ResourceID = &instanceID
			service.operations.Persist(workerOperation)
		}, func(progress MigrationCopyProgress) {
			workerOperation.CurrentBytes = progress.Bytes
			if workerOperation.TotalBytes > 0 {
				workerOperation.Progress = min(1, float64(progress.Bytes)/float64(workerOperation.TotalBytes))
			}
			workerOperation.TitleParams = map[string]string{"files": strconv.FormatInt(progress.Files, 10), "totalFiles": strconv.FormatInt(candidate.TotalFiles, 10)}
			service.operations.Publish(operations.EventProgress, workerOperation)
			service.operations.Persist(workerOperation)
		})
		finished := service.now().UTC()
		workerOperation.FinishedAt = &finished
		if errors.Is(importErr, context.Canceled) {
			workerOperation.Status = operations.StatusCancelled
			service.operations.SaveBestEffort(workerOperation, operations.EventUpdated)
			return Instance{}, importErr
		}
		if importErr != nil {
			workerOperation.Status = operations.StatusFailed
			code, message := errs.ErrValidation, importErr.Error()
			workerOperation.ErrorCode, workerOperation.ErrorMessage = &code, &message
			service.operations.SaveBestEffort(workerOperation, operations.EventFailed)
			return Instance{}, importErr
		}
		workerOperation.Status, workerOperation.Progress = operations.StatusCompleted, 1
		service.operations.SaveBestEffort(workerOperation, operations.EventCompleted)
		return instance, nil
	})
	if err != nil {
		return operations.Operation{}, err
	}
	return operation, nil
}

func (service *MigrationService) importData(ctx context.Context, request MigrationImportRequest, candidate MigrationCandidate,
	created func(string), progress func(MigrationCopyProgress)) (instance Instance, err error) {
	destination := filepath.Join(service.dataRoot, "instances")
	if err := service.storage.ValidateTarget(candidate.Path, destination); err != nil {
		return Instance{}, err
	}
	available, err := service.diskSpace.Available(destination)
	if err != nil {
		return Instance{}, err
	}
	margin := max(migrationSafetyMargin, candidate.TotalBytes/20)
	if candidate.TotalBytes > available-margin {
		return Instance{}, errs.NewError(errs.ErrValidation, "Not enough free disk space to import the data")
	}
	instance, err = service.creator.CreatePrepared(ctx, CreateInput{Name: request.Name, Description: request.Description,
		GameVersionID: request.GameVersionID, GameClient: GameClientVanilla}, func(ctx context.Context, target string) error {
		_, copyErr := service.storage.Copy(ctx, candidate.Path, target, progress)
		return copyErr
	})
	if err != nil {
		return Instance{}, err
	}
	created(instance.ID)
	warnings := []string{}
	if service.reconcile != nil {
		warnings = append(warnings, service.reconcile(ctx, instance.ID)...)
	}
	if len(warnings) > 0 {
		slog.Info("Vintage Story data imported with warnings", "instanceId", instance.ID, "warningCount", len(warnings))
	}
	return instance, nil
}

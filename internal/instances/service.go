package instances

import (
	"context"
	"fmt"
	"github.com/waxlight/waxlight-launcher/internal/snapshots"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/waxlight/waxlight-launcher/internal/errs"
)

const (
	telemetryEventInstanceCreated = "instance_created"
	telemetryEventInstanceDeleted = "instance_deleted"
)

type QueryService struct {
	repository QueryRepository
}

func NewQueryService(repository QueryRepository) *QueryService {
	return &QueryService{repository: repository}
}

func (service *QueryService) List(ctx context.Context) ([]Instance, error) {
	return service.repository.ListInstances(ctx)
}

func (service *QueryService) Get(ctx context.Context, id string) (Instance, error) {
	return service.repository.GetInstance(ctx, id)
}

type CreateService struct {
	repository  CreateRepository
	versions    VersionReader
	accounts    AccountReader
	language    LanguageFunc
	gate        MutationGate
	directories DirectoryStorage
	events      Publisher
	telemetry   TelemetryFunc
	dataRoot    string
	now         Clock
	newID       IDGenerator
}

func NewCreateService(
	repository CreateRepository,
	versions VersionReader,
	accounts AccountReader,
	language LanguageFunc,
	gate MutationGate,
	directories DirectoryStorage,
	events Publisher,
	telemetryReporter TelemetryFunc,
	dataRoot string,
	now Clock,
	newID IDGenerator,
) *CreateService {
	return &CreateService{
		repository:  repository,
		versions:    versions,
		accounts:    accounts,
		language:    language,
		gate:        gate,
		directories: directories,
		events:      events,
		telemetry:   telemetryReporter,
		dataRoot:    dataRoot,
		now:         now,
		newID:       newID,
	}
}

func (service *CreateService) Create(ctx context.Context, input CreateInput) (Instance, error) {
	if err := service.gate.Begin(); err != nil {
		return Instance{}, err
	}
	defer service.gate.End()

	name := strings.TrimSpace(input.Name)
	var err error
	if name == "" {
		name, err = service.defaultInstanceName(ctx)
	} else {
		name, err = cleanName(name)
	}
	if err != nil {
		return Instance{}, err
	}

	slog.Info("creating instance", "name", name, "version", input.GameVersionID)
	if _, err := service.versions.Get(ctx, input.GameVersionID); err != nil {
		return Instance{}, err
	}
	gameClient, valid := NormalizeGameClient(input.GameClient)
	if !valid {
		return Instance{}, errs.NewError(errs.ErrValidation, "Game client must be Vanilla or Optimum")
	}
	environmentVariables, err := cleanEnvironmentVariables(input.EnvironmentVariables)
	if err != nil {
		return Instance{}, err
	}
	if input.DefaultAccountID != nil {
		if service.accounts == nil {
			return Instance{}, errs.NewError(errs.ErrAccountNotFound, "Account not found")
		}
		if _, err := service.accounts.GetAccount(ctx, *input.DefaultAccountID); err != nil {
			return Instance{}, err
		}
	}

	id := service.newID()
	directory := strings.TrimSpace(input.Directory)
	if directory == "" {
		directory = filepath.Join(service.dataRoot, "instances", id)
	}
	directory, err = filepath.Abs(directory)
	if err != nil {
		return Instance{}, err
	}
	used, err := service.repository.IsDirectoryUsed(ctx, directory, "")
	if err != nil {
		return Instance{}, err
	}
	if used {
		return Instance{}, errs.NewError(ErrDirectoryConflict, "The directory is already used by another instance")
	}

	allocation, err := service.directories.Allocate(directory, id)
	if err != nil {
		return Instance{}, err
	}
	committed := false
	defer func() {
		if !committed {
			if rollbackErr := allocation.Rollback(); rollbackErr != nil {
				slog.Warn("could not roll back instance directory allocation", "error", rollbackErr)
			}
		}
	}()

	directory = allocation.Directory()
	used, err = service.repository.IsDirectoryUsed(ctx, directory, "")
	if err != nil {
		return Instance{}, err
	}
	if used {
		return Instance{}, errs.NewError(ErrDirectoryConflict, "The directory is already used by another instance")
	}

	now := service.now().UTC()
	instance := Instance{
		ID:                   id,
		Name:                 name,
		Description:          strings.TrimSpace(input.Description),
		GameVersionID:        input.GameVersionID,
		GameClient:           gameClient,
		DefaultAccountID:     input.DefaultAccountID,
		Directory:            directory,
		Status:               StatusReady,
		LaunchArguments:      input.LaunchArguments,
		EnvironmentVariables: environmentVariables,
		CreatedAt:            now,
		UpdatedAt:            now,
	}
	if err := service.repository.SaveInstance(ctx, instance); err != nil {
		return instance, err
	}
	allocation.Commit()
	committed = true
	if service.events != nil {
		service.events.Publish("instance:created", instance)
	}
	if service.telemetry != nil {
		service.telemetry(ctx, telemetryEventInstanceCreated)
	}
	return instance, nil
}

type UpdateService struct {
	repository          UpdateRepository
	versions            VersionReader
	gate                MutationGate
	lock                MutationLock
	snapshotter         snapshots.SafetySnapshotter
	clearClientSettings ClientSettingsClearer
	events              Publisher
	now                 Clock
}

func NewUpdateService(
	repository UpdateRepository,
	versions VersionReader,
	gate MutationGate,
	lock MutationLock,
	snapshotter snapshots.SafetySnapshotter,
	clearClientSettings ClientSettingsClearer,
	events Publisher,
	now Clock,
) *UpdateService {
	return &UpdateService{
		repository:          repository,
		versions:            versions,
		gate:                gate,
		lock:                lock,
		snapshotter:         snapshotter,
		clearClientSettings: clearClientSettings,
		events:              events,
		now:                 now,
	}
}

func (service *UpdateService) Update(ctx context.Context, updated Instance) (Instance, error) {
	if err := service.gate.Begin(); err != nil {
		return updated, err
	}
	defer service.gate.End()

	previous, err := service.repository.GetInstance(ctx, updated.ID)
	if err != nil {
		return updated, err
	}
	updated.Name, err = cleanName(updated.Name)
	if err != nil {
		return updated, err
	}
	if _, err = service.versions.Get(ctx, updated.GameVersionID); err != nil {
		return updated, err
	}
	gameClient, valid := NormalizeGameClient(updated.GameClient)
	if !valid {
		return updated, errs.NewError(errs.ErrValidation, "Game client must be Vanilla or Optimum")
	}
	updated.GameClient = gameClient
	updated.EnvironmentVariables, err = cleanEnvironmentVariables(updated.EnvironmentVariables)
	if err != nil {
		return updated, err
	}
	if previous.GameVersionID != updated.GameVersionID {
		if service.lock == nil {
			return updated, errs.NewError(errs.ErrValidation, "Instance version changes are unavailable")
		}
		release, lockErr := service.lock.Lock(updated.ID, MutationMarker)
		if lockErr != nil {
			return updated, lockErr
		}
		if release == nil {
			return updated, errs.NewError(errs.ErrValidation, "Instance version change lock is unavailable")
		}
		defer release()
		if service.snapshotter != nil {
			toVersion := updated.GameVersionID
			if version, versionErr := service.versions.Get(ctx, updated.GameVersionID); versionErr == nil && strings.TrimSpace(version.Name) != "" {
				toVersion = version.Name
			}
			fromVersion := previous.GameVersionID
			if version, versionErr := service.versions.Get(ctx, previous.GameVersionID); versionErr == nil && strings.TrimSpace(version.Name) != "" {
				fromVersion = version.Name
			}
			if err := service.snapshotter.Create(ctx, updated.ID, snapshots.ReasonBeforeGameVersionChange, map[string]string{
				"fromGameVersion": fromVersion,
				"toGameVersion":   toVersion,
			}); err != nil {
				return updated, err
			}
		}
	}

	updated.Directory = previous.Directory
	updated.CreatedAt = previous.CreatedAt
	updated.LastPlayedAt = previous.LastPlayedAt
	updated.Status = previous.Status
	updated.UpdatedAt = service.now().UTC()
	if !sameOptionalString(previous.DefaultAccountID, updated.DefaultAccountID) && service.clearClientSettings != nil {
		if err = service.clearClientSettings(filepath.Join(previous.Directory, "clientsettings.json")); err != nil {
			return updated, err
		}
	}
	if err = service.repository.SaveInstance(ctx, updated); err != nil {
		return updated, err
	}
	if service.events != nil {
		service.events.Publish("instance:updated", updated)
	}
	return updated, nil
}

type DeleteService struct {
	repository          DeleteRepository
	gate                MutationGate
	lock                MutationLock
	removeDirectory     DirectoryRemover
	clearClientSettings ClientSettingsClearer
	cleanRecovery       RecoveryCleaner
	events              Publisher
	telemetry           TelemetryFunc
}

func NewDeleteService(
	repository DeleteRepository,
	gate MutationGate,
	lock MutationLock,
	removeDirectory DirectoryRemover,
	clearClientSettings ClientSettingsClearer,
	cleanRecovery RecoveryCleaner,
	events Publisher,
	telemetryReporter TelemetryFunc,
) *DeleteService {
	return &DeleteService{
		repository:          repository,
		gate:                gate,
		lock:                lock,
		removeDirectory:     removeDirectory,
		clearClientSettings: clearClientSettings,
		cleanRecovery:       cleanRecovery,
		events:              events,
		telemetry:           telemetryReporter,
	}
}

func (service *DeleteService) Delete(ctx context.Context, id string, deleteFiles bool) error {
	if err := service.gate.Begin(); err != nil {
		return err
	}
	defer service.gate.End()

	if service.lock == nil {
		return errs.NewError(errs.ErrValidation, "Instance deletion guard is unavailable")
	}
	guardRelease, err := service.lock.Guard(id, MutationMarker, "Stop the game before deleting this instance")
	if err != nil {
		return err
	}
	if guardRelease == nil {
		return errs.NewError(errs.ErrValidation, "Instance deletion reservation is unavailable")
	}
	defer guardRelease()
	instance, err := service.repository.GetInstance(ctx, id)
	if err != nil {
		return err
	}
	if deleteFiles {
		if service.removeDirectory == nil {
			return errs.NewError(errs.ErrValidation, "Instance directory removal is unavailable")
		}
		if err := service.removeDirectory(instance.Directory); err != nil {
			return err
		}
	}
	if !deleteFiles && service.clearClientSettings != nil {
		if err := service.clearClientSettings(filepath.Join(instance.Directory, "clientsettings.json")); err != nil {
			return err
		}
	}
	if err := service.repository.DeleteInstance(ctx, id); err != nil {
		return err
	}
	if service.cleanRecovery != nil {
		if err := service.cleanRecovery(ctx, id); err != nil {
			slog.Warn("could not clean up the last known good state of the deleted instance", "instanceId", id, "error", err)
		}
	}
	if service.events != nil {
		service.events.Publish("instance:deleted", map[string]string{"id": id})
	}
	if service.telemetry != nil {
		service.telemetry(ctx, telemetryEventInstanceDeleted)
	}
	slog.Info("instance deleted", "id", id)
	return nil
}

func cleanEnvironmentVariables(values map[string]string) (map[string]string, error) {
	if len(values) == 0 {
		return map[string]string{}, nil
	}

	cleaned := make(map[string]string, len(values))
	for rawKey, value := range values {
		key := strings.TrimSpace(rawKey)
		if !validEnvironmentVariableName(key) {
			return nil, errs.NewError(errs.ErrValidation, "Environment variable names must use letters, numbers, and underscores and cannot start with a number")
		}
		if key == "WAXLIGHT_INSTANCE_DIR" {
			return nil, errs.NewError(errs.ErrValidation, "WAXLIGHT_INSTANCE_DIR is reserved by Waxlight")
		}
		if strings.ContainsRune(value, '\x00') {
			return nil, errs.NewError(errs.ErrValidation, "Environment variable values cannot contain NUL characters")
		}
		cleaned[key] = value
	}
	return cleaned, nil
}

func validEnvironmentVariableName(value string) bool {
	if value == "" {
		return false
	}
	for index, char := range value {
		if char == '_' || char >= 'A' && char <= 'Z' || char >= 'a' && char <= 'z' {
			continue
		}
		if index > 0 && char >= '0' && char <= '9' {
			continue
		}
		return false
	}
	return true
}

func sameOptionalString(left, right *string) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

func cleanName(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", errs.NewError(errs.ErrValidation, "Name cannot be empty")
	}
	if len([]rune(value)) > 80 {
		return "", errs.NewError(errs.ErrValidation, "Name cannot exceed 80 characters")
	}
	return value, nil
}

func (service *CreateService) defaultInstanceName(ctx context.Context) (string, error) {
	language, err := service.language(ctx)
	if err != nil {
		return "", err
	}
	base := localizedInstanceName(language)
	stored, err := service.repository.ListInstances(ctx)
	if err != nil {
		return "", err
	}
	taken := make(map[string]bool, len(stored))
	for _, instance := range stored {
		taken[strings.ToLower(strings.TrimSpace(instance.Name))] = true
	}
	candidate := base
	for index := 2; ; index++ {
		if !taken[strings.ToLower(candidate)] {
			return candidate, nil
		}
		candidate = fmt.Sprintf("%s-%d", base, index)
	}
}

func localizedInstanceName(language string) string {
	switch strings.ToLower(strings.TrimSpace(language)) {
	case "ru":
		return "Сборка"
	case "be":
		return "Зборка"
	case "de":
		return "Instanz"
	case "es":
		return "Instancia"
	case "fr":
		return "Instance"
	case "kk":
		return "Жинақ"
	case "pl":
		return "Instancja"
	case "pt":
		return "Instância"
	case "sv":
		return "Instans"
	default:
		return "Instance"
	}
}

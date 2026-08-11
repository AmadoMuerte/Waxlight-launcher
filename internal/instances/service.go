package instances

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/waxlight/waxlight-launcher/internal/domain"
)

const telemetryEventInstanceCreated = "instance_created"

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
	if input.DefaultAccountID != nil {
		if service.accounts == nil {
			return Instance{}, domain.NewError(domain.ErrAccountNotFound, "Account not found")
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
		return Instance{}, domain.NewError(ErrDirectoryConflict, "The directory is already used by another instance")
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
		return Instance{}, domain.NewError(ErrDirectoryConflict, "The directory is already used by another instance")
	}

	now := service.now().UTC()
	instance := Instance{
		ID:               id,
		Name:             name,
		Description:      strings.TrimSpace(input.Description),
		GameVersionID:    input.GameVersionID,
		DefaultAccountID: input.DefaultAccountID,
		Directory:        directory,
		Status:           StatusReady,
		LaunchArguments:  input.LaunchArguments,
		CreatedAt:        now,
		UpdatedAt:        now,
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

func cleanName(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", domain.NewError(domain.ErrValidation, "Name cannot be empty")
	}
	if len([]rune(value)) > 80 {
		return "", domain.NewError(domain.ErrValidation, "Name cannot exceed 80 characters")
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

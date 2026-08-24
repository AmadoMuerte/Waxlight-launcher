package versions

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"github.com/AmadoMuerte/Waxlight-launcher/internal/errs"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/events"
)

type RemovalService struct {
	repository Repository
	references References
	filesystem Filesystem
	gate       MutationGate
	events     events.Publisher
}

func NewRemovalService(repository Repository, references References, filesystem Filesystem, gate MutationGate, publisher events.Publisher) *RemovalService {
	return &RemovalService{repository: repository, references: references, filesystem: filesystem, gate: gate, events: publisher}
}

func (service *RemovalService) Remove(ctx context.Context, id string, deleteFiles bool) error {
	if err := service.gate.Begin(); err != nil {
		return err
	}
	defer service.gate.End()
	id = strings.TrimSpace(id)
	version, err := service.repository.GetVersion(ctx, id)
	if err != nil {
		return err
	}
	name, found, err := service.references.VersionReference(ctx, id)
	if err != nil {
		return err
	}
	if found {
		return errs.NewError(errs.ErrValidation, "The version is used by instance \""+name+"\"")
	}
	slog.Info("removing game version", "version", version.Name)
	if deleteFiles {
		if err := service.filesystem.RemoveVersion(version.InstallationDir, id); err != nil {
			var appError *errs.AppError
			if errors.As(err, &appError) {
				return err
			}
			return &errs.AppError{Code: errs.ErrFilePermission, Message: "Could not remove the game version files. Close the game and try again", Cause: err}
		}
	}
	if err := service.repository.DeleteVersion(ctx, id); err != nil {
		return err
	}
	if deleteFiles {
		if err := service.filesystem.RemoveVersionsRootIfEmpty(version.InstallationDir); err != nil {
			slog.Debug("could not remove the empty versions root", "error", err)
		}
	}
	service.publish("version:removed", map[string]string{"id": id})
	return nil
}

func (service *RemovalService) publish(name string, payload any) {
	if service.events != nil {
		service.events.Publish(name, payload)
	}
}

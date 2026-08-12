package instances

import (
	"context"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	"github.com/waxlight/waxlight-launcher/internal/errs"
)

type CloneService struct {
	repository      CloneRepository
	mods            CloneModRepository
	creator         InstanceCreator
	gate            MutationGate
	lock            MutationLock
	storage         CloneStorage
	removeDirectory DirectoryRemover
	now             Clock
	newID           IDGenerator
}

func NewCloneService(
	repository CloneRepository,
	mods CloneModRepository,
	creator InstanceCreator,
	gate MutationGate,
	lock MutationLock,
	storage CloneStorage,
	removeDirectory DirectoryRemover,
	now Clock,
	newID IDGenerator,
) *CloneService {
	return &CloneService{
		repository:      repository,
		mods:            mods,
		creator:         creator,
		gate:            gate,
		lock:            lock,
		storage:         storage,
		removeDirectory: removeDirectory,
		now:             now,
		newID:           newID,
	}
}

func (service *CloneService) Clone(ctx context.Context, sourceID, name string) (Instance, error) {
	if err := service.gate.Begin(); err != nil {
		return Instance{}, err
	}
	defer service.gate.End()
	if service.storage == nil {
		return Instance{}, errs.NewError(errs.ErrValidation, "Instance clone storage is unavailable")
	}
	if service.removeDirectory == nil {
		return Instance{}, errs.NewError(errs.ErrValidation, "Instance clone cleanup is unavailable")
	}
	if service.lock == nil {
		return Instance{}, errs.NewError(errs.ErrValidation, "Instance clone guard is unavailable")
	}
	guardRelease, err := service.lock.Guard(sourceID, MutationMarker, "Stop the game before cloning this instance")
	if err != nil {
		return Instance{}, err
	}
	if guardRelease == nil {
		return Instance{}, errs.NewError(errs.ErrValidation, "Instance clone reservation is unavailable")
	}
	defer guardRelease()

	source, err := service.repository.GetInstance(ctx, sourceID)
	if err != nil {
		return Instance{}, err
	}
	slog.Info("cloning instance", "source", source.Name)
	clone, err := service.creator.Create(ctx, CreateInput{
		Name:             name,
		Description:      source.Description,
		GameVersionID:    source.GameVersionID,
		DefaultAccountID: source.DefaultAccountID,
		LaunchArguments:  append([]string(nil), source.LaunchArguments...),
	})
	if err != nil {
		return Instance{}, err
	}

	cleanup := func(cause error) (Instance, error) {
		if service.removeDirectory == nil {
			slog.Warn("could not remove the failed clone directory", "instance", clone.Name, "error", "directory remover is unavailable")
			return Instance{}, cause
		}
		if cleanupErr := service.removeDirectory(clone.Directory); cleanupErr != nil {
			slog.Warn("could not remove the failed clone directory", "instance", clone.Name, "error", cleanupErr)
			return Instance{}, cause
		}
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		defer cancel()
		if cleanupErr := service.repository.DeleteInstance(cleanupCtx, clone.ID); cleanupErr != nil {
			slog.Warn("could not delete the failed clone record", "instance", clone.Name, "error", cleanupErr)
		}
		return Instance{}, cause
	}

	if err := service.storage.Copy(ctx, source.Directory, clone.Directory); err != nil {
		return cleanup(err)
	}
	if err := service.replicateMods(ctx, source, clone); err != nil {
		return cleanup(err)
	}
	if source.CoverPath != nil && strings.TrimSpace(*source.CoverPath) != "" {
		if copiedCover, ok := service.storage.CopiedPath(source.Directory, clone.Directory, *source.CoverPath); ok {
			clone.CoverPath = &copiedCover
		}
	}
	clone.UpdatedAt = service.now().UTC()
	if err := service.repository.SaveInstance(ctx, clone); err != nil {
		return cleanup(err)
	}
	return clone, nil
}

func (service *CloneService) replicateMods(ctx context.Context, source, clone Instance) error {
	mods, err := service.mods.ListMods(ctx, source.ID)
	if err != nil {
		return err
	}
	for _, mod := range mods {
		copied := mod
		copied.ID = service.newID()
		copied.InstanceID = clone.ID
		relative, relErr := filepath.Rel(source.Directory, mod.FilePath)
		if relErr != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return errs.NewError(errs.ErrValidation, "Instance mod path is outside the instance directory")
		}
		copied.FilePath = filepath.Join(clone.Directory, relative)
		if err := service.mods.SaveMod(ctx, copied); err != nil {
			return err
		}
	}
	return nil
}

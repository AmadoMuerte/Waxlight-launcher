package application

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/waxlight/waxlight-launcher/internal/domain"
)

// CloneInstance creates a new, independent instance that duplicates the source
// instance's mods, configuration and client settings. World saves and logs are
// not copied, and authentication data is stripped from the copy.
func (s *Service) CloneInstance(
	ctx context.Context,
	sourceID string,
	name string,
) (domain.Instance, error) {
	if err := s.rejectIfRelocating(); err != nil {
		return domain.Instance{}, err
	}
	s.runningMu.Lock()
	_, running := s.running[sourceID]
	s.runningMu.Unlock()
	if running {
		return domain.Instance{}, domain.NewError(
			domain.ErrInstanceRunning,
			"Stop the game before cloning this instance",
		)
	}

	source, err := s.store.GetInstance(ctx, sourceID)
	if err != nil {
		return domain.Instance{}, err
	}

	clone, err := s.CreateInstance(ctx, CreateInstanceInput{
		Name:             name,
		Description:      source.Description,
		GameVersionID:    source.GameVersionID,
		DefaultAccountID: source.DefaultAccountID,
		LaunchArguments:  append([]string(nil), source.LaunchArguments...),
	})
	if err != nil {
		return domain.Instance{}, err
	}

	cleanup := func(cause error) (domain.Instance, error) {
		_ = safeRemoveAll(clone.Directory, s.dataRoot, ".waxlight-instance")
		_ = s.store.DeleteInstance(ctx, clone.ID)
		return domain.Instance{}, cause
	}

	if err := copyCloneDirectory(ctx, source.Directory, clone.Directory); err != nil {
		return cleanup(err)
	}

	if s.clientSettings != nil {
		if err := s.clientSettings.Clear(filepath.Join(clone.Directory, "clientsettings.json")); err != nil {
			return cleanup(err)
		}
	}

	if err := s.replicateMods(ctx, source, clone); err != nil {
		return cleanup(err)
	}

	if source.CoverPath != nil && *source.CoverPath != "" {
		if relative, relErr := filepath.Rel(source.Directory, *source.CoverPath); relErr == nil &&
			relative != "." && !strings.HasPrefix(relative, "..") {
			copiedCover := filepath.Join(clone.Directory, relative)
			if _, statErr := os.Lstat(copiedCover); statErr == nil {
				cover := copiedCover
				clone.CoverPath = &cover
			}
		}
	}
	clone.UpdatedAt = time.Now().UTC()
	if err := s.store.SaveInstance(ctx, clone); err != nil {
		return cleanup(err)
	}

	return clone, nil
}

// replicateMods writes the source instance's mod metadata for the clone. The
// files themselves already exist on disk after the directory copy; only the
// catalog source markers need to be carried over so updates keep working.
func (s *Service) replicateMods(
	ctx context.Context,
	source domain.Instance,
	clone domain.Instance,
) error {
	mods, err := s.store.ListMods(ctx, source.ID)
	if err != nil {
		return err
	}
	for _, mod := range mods {
		copied := mod
		copied.ID = newID()
		copied.InstanceID = clone.ID
		if relative, relErr := filepath.Rel(source.Directory, mod.FilePath); relErr == nil &&
			relative != "." && !strings.HasPrefix(relative, "..") {
			copied.FilePath = filepath.Join(clone.Directory, relative)
		}
		if err := s.store.SaveMod(ctx, copied); err != nil {
			return err
		}
	}
	return nil
}

// copyCloneDirectory copies the instance directory tree, keeping the fresh
// `.waxlight-instance` marker of the clone and skipping world saves, logs and
// any authentication journal. Symbolic links are rejected so the clone can
// never escape its own data directory.
func copyCloneDirectory(ctx context.Context, source string, target string) error {
	return filepath.Walk(source, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if path == source {
			return nil
		}

		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		if relative == "SaveGame" || relative == "Logs" {
			return filepath.SkipDir
		}
		switch {
		case info.Name() == ".waxlight-instance":
			return nil
		case strings.HasSuffix(info.Name(), ".waxlight-auth-injection"):
			return nil
		}

		destination := filepath.Join(target, relative)
		if info.IsDir() {
			return os.MkdirAll(destination, info.Mode().Perm())
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return errors.New("instance directory contains a symbolic link and cannot be cloned")
		}
		return copyCloneFile(path, destination, info.Mode().Perm())
	})
}

func copyCloneFile(source string, target string, mode os.FileMode) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()

	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	output, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(output, input); err != nil {
		_ = output.Close()
		return err
	}
	return output.Close()
}

package modstorage

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/waxlight/waxlight-launcher/internal/domain"
	"github.com/waxlight/waxlight-launcher/internal/infrastructure/atomicfile"
	"github.com/waxlight/waxlight-launcher/internal/mods"
)

const metadataFile = "metadata.json"

type Store struct {
	root string
}

func New(root string) *Store {
	return &Store{root: filepath.Join(root, "cache", "mods")}
}

func (store *Store) List(ctx context.Context) ([]mods.DownloadedMod, error) {
	result := []mods.DownloadedMod{}
	err := filepath.WalkDir(store.root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			if errors.Is(walkErr, os.ErrNotExist) {
				return nil
			}
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if entry.IsDir() || entry.Name() != metadataFile {
			return nil
		}
		value, err := readMetadata(path)
		if err != nil {
			return nil
		}
		result = append(result, value)
		return nil
	})
	if errors.Is(err, os.ErrNotExist) {
		return result, nil
	}
	return result, err
}

func (store *Store) Get(
	_ context.Context,
	modID string,
	versionID string,
) (mods.DownloadedMod, error) {
	path, err := store.metadataPath(modID, versionID)
	if err != nil {
		return mods.DownloadedMod{}, err
	}
	value, err := readMetadata(path)
	if errors.Is(err, os.ErrNotExist) {
		return value, domain.NewError(mods.ErrModVersionNotFound, "Downloaded mod version not found")
	}
	return value, err
}

func (store *Store) Save(_ context.Context, value mods.DownloadedMod) error {
	path, err := store.metadataPath(value.ModID, value.VersionID)
	if err != nil {
		return err
	}
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	slog.Info("cached mod saved", "modId", value.ModID, "versionId", value.VersionID)
	return atomicfile.Write(path, encoded, 0o600)
}

func (store *Store) Delete(_ context.Context, modID, versionID string) error {
	path, err := store.metadataPath(modID, versionID)
	if err != nil {
		return err
	}
	directory := filepath.Dir(path)
	root, _ := filepath.Abs(store.root)
	target, _ := filepath.Abs(directory)
	if filepath.Dir(filepath.Dir(target)) != root {
		return domain.NewError(domain.ErrValidation, "Unsafe mod cache path")
	}
	return os.RemoveAll(target)
}

func (store *Store) FilePath(modID, versionID, fileName string) (string, error) {
	path, err := store.metadataPath(modID, versionID)
	if err != nil {
		return "", err
	}
	fileName = filepath.Base(strings.TrimSpace(fileName))
	if fileName == "." || fileName == "" {
		return "", domain.NewError(mods.ErrInvalidModFile, "Invalid mod file name")
	}
	extension := strings.ToLower(filepath.Ext(fileName))
	if extension != ".zip" && extension != ".cs" && extension != ".dll" {
		return "", domain.NewError(mods.ErrInvalidModFile, "Unsupported mod file type")
	}
	return filepath.Join(filepath.Dir(path), fileName), nil
}

func (store *Store) metadataPath(modID, versionID string) (string, error) {
	modID, err := safeSegment(modID)
	if err != nil {
		return "", err
	}
	versionID, err = safeSegment(versionID)
	if err != nil {
		return "", err
	}
	return filepath.Join(store.root, modID, versionID, metadataFile), nil
}

func safeSegment(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" || value == "." || value == ".." || filepath.Base(value) != value || strings.ContainsAny(value, `/\\`) {
		return "", domain.NewError(domain.ErrValidation, "Invalid mod identifier")
	}
	return value, nil
}

func readMetadata(path string) (mods.DownloadedMod, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return mods.DownloadedMod{}, err
	}
	var value mods.DownloadedMod
	if err := json.Unmarshal(data, &value); err != nil {
		return value, err
	}
	if value.SchemaVersion != 1 {
		return value, domain.NewError(domain.ErrValidation, "Unsupported downloaded mod metadata")
	}
	return value, nil
}

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

	"github.com/AmadoMuerte/Waxlight-launcher/internal/errs"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/mods"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/platform/atomicfile"
)

const metadataFile = "metadata.json"

type Store struct {
	root string
}

func New(root string) *Store {
	return &Store{root: cacheRoot(root)}
}

func cacheRoot(dataRoot string) string {
	return filepath.Join(dataRoot, "cache", "mods")
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
		value.FilePath = store.resolvePath(value.FilePath)
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
		return value, errs.NewError(mods.ErrModVersionNotFound, "Downloaded mod version not found")
	}
	if err != nil {
		return value, err
	}
	value.FilePath = store.resolvePath(value.FilePath)
	return value, nil
}

func (store *Store) Save(_ context.Context, value mods.DownloadedMod) error {
	path, err := store.metadataPath(value.ModID, value.VersionID)
	if err != nil {
		return err
	}
	value.FilePath = store.storedPath(value.FilePath)
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	if err := atomicfile.Write(path, encoded, 0o600); err != nil {
		return err
	}
	slog.Info("cached mod saved", "modId", value.ModID, "versionId", value.VersionID)
	return nil
}

// storedPath converts an absolute file path inside the cache root into a
// root-relative one, so a data-root move never invalidates the cache. Files
// outside the root (local mods linked in place) stay absolute.
func (store *Store) storedPath(path string) string {
	if path == "" || !filepath.IsAbs(path) {
		return path
	}
	rel, err := filepath.Rel(store.root, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return path
	}
	return rel
}

func (store *Store) resolvePath(path string) string {
	if path == "" || filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(store.root, path)
}

// RelocateOldRoot rewrites cached metadata that still references the previous
// data root after a move, converting those paths to root-relative ones.
func (store *Store) RelocateOldRoot(ctx context.Context, oldRoot string) error {
	if oldRoot == "" {
		return nil
	}
	prefix := cacheRoot(oldRoot) + string(filepath.Separator)
	return filepath.WalkDir(store.root, func(path string, entry fs.DirEntry, walkErr error) error {
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
		if !strings.HasPrefix(value.FilePath, prefix) {
			return nil
		}
		value.FilePath = strings.TrimPrefix(value.FilePath, prefix)
		encoded, err := json.MarshalIndent(value, "", "  ")
		if err != nil {
			return err
		}
		return atomicfile.Write(path, encoded, 0o600)
	})
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
		return errs.NewError(errs.ErrValidation, "Unsafe mod cache path")
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
		return "", errs.NewError(mods.ErrInvalidModFile, "Invalid mod file name")
	}
	extension := strings.ToLower(filepath.Ext(fileName))
	if extension != ".zip" && extension != ".cs" && extension != ".dll" {
		return "", errs.NewError(mods.ErrInvalidModFile, "Unsupported mod file type")
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
		return "", errs.NewError(errs.ErrValidation, "Invalid mod identifier")
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
		return value, errs.NewError(errs.ErrValidation, "Unsupported downloaded mod metadata")
	}
	return value, nil
}

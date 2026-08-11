package snapshots

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type snapshotStats struct {
	sizeBytes  int64
	worldCount int
}

// copySnapshotData copies an instance data directory into destination.
// Symbolic links are rejected, launcher markers and authentication journals
// are skipped, and clientsettings.json is sanitized so temporary session
// credentials never become part of a snapshot. Files listed in skipPaths are
// not copied.
func copySnapshotData(
	ctx context.Context,
	source string,
	destination string,
	skipPaths map[string]struct{},
	progress func(int64),
	sanitizeSettings ClientSettingsSanitizer,
) (snapshotStats, error) {
	stats := snapshotStats{worldCount: countWorlds(filepath.Join(source, "SaveGame"))}
	err := filepath.Walk(source, func(path string, info os.FileInfo, walkErr error) error {
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
		switch {
		case info.Name() == instanceMarkerFile:
			return nil
		case strings.HasSuffix(info.Name(), ".waxlight-auth-injection"):
			return nil
		}
		if absolute, absErr := filepath.Abs(path); absErr == nil {
			if _, skipped := skipPaths[filepath.Clean(absolute)]; skipped {
				return nil
			}
		}

		target := filepath.Join(destination, relative)
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode().Perm())
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return errors.New("instance data contains a symbolic link and cannot be snapshotted")
		}
		if !info.Mode().IsRegular() {
			return errors.New("instance data contains a non-regular file and cannot be snapshotted")
		}
		if strings.EqualFold(info.Name(), "clientsettings.json") {
			size, err := sanitizeClientSettingsCopy(path, target, info.Mode().Perm(), sanitizeSettings)
			if err != nil {
				return err
			}
			stats.sizeBytes += size
			if progress != nil {
				progress(size)
			}
			return nil
		}
		size, err := copySnapshotFile(path, target, info.Mode().Perm())
		if err != nil {
			return err
		}
		stats.sizeBytes += size
		if progress != nil {
			progress(size)
		}
		return nil
	})
	return stats, err
}

// sanitizeClientSettingsCopy copies clientsettings.json with every temporary
// authentication property and machine specific mod path removed. It returns
// the number of bytes written.
func sanitizeClientSettingsCopy(source string, destination string, mode os.FileMode, sanitizeSettings ClientSettingsSanitizer) (int64, error) {
	file, err := os.Open(source)
	if err != nil {
		return 0, err
	}
	defer file.Close()
	contents, err := io.ReadAll(io.LimitReader(file, 8<<20))
	if err != nil {
		return 0, err
	}
	sanitized, err := sanitizeSettings(contents)
	if err != nil {
		return 0, err
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return 0, err
	}
	if err := os.WriteFile(destination, sanitized, mode); err != nil {
		return 0, err
	}
	return int64(len(sanitized)), nil
}

func copySnapshotFile(source string, destination string, mode os.FileMode) (int64, error) {
	return copySnapshotFileContext(context.Background(), source, destination, mode)
}

func copySnapshotFileContext(ctx context.Context, source string, destination string, mode os.FileMode) (int64, error) {
	input, err := os.Open(source)
	if err != nil {
		return 0, err
	}
	defer input.Close()
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return 0, err
	}
	output, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return 0, err
	}
	size, err := io.Copy(output, &contextReader{ctx: ctx, reader: input})
	if err != nil {
		_ = output.Close()
		return 0, err
	}
	return size, output.Close()
}

// contextReader wraps a reader with cancellation checking.
type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (reader *contextReader) Read(buffer []byte) (int, error) {
	if err := reader.ctx.Err(); err != nil {
		return 0, err
	}
	return reader.reader.Read(buffer)
}

// countWorlds counts the save data worlds directly under the SaveGame
// directory.
func countWorlds(saveGameDir string) int {
	entries, err := os.ReadDir(saveGameDir)
	if err != nil {
		return 0
	}
	count := 0
	for _, entry := range entries {
		if entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") {
			count++
		}
	}
	return count
}

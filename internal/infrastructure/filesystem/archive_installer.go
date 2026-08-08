package filesystem

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

type ArchiveInstaller struct{}

func (ArchiveInstaller) FindExecutable(
	rootPath string,
	relativePath string,
) (string, error) {
	return resolveExecutable(rootPath, relativePath)
}

func (ArchiveInstaller) Install(
	ctx context.Context,
	sourcePath string,
	targetPath string,
	executableRelativePath string,
	expectedSHA256 string,
	progress func(copied, total int64),
) (string, int64, error) {
	if expectedSHA256 != "" {
		actual, err := fileSHA256(ctx, sourcePath)
		if err != nil {
			return "", 0, err
		}
		if !strings.EqualFold(actual, expectedSHA256) {
			slog.Warn("game archive checksum mismatch")
			return "", 0, fmt.Errorf(
				"checksum mismatch: expected %s, got %s",
				expectedSHA256,
				actual,
			)
		}
	}
	slog.Info("installing game archive", "source", filepath.Base(sourcePath))

	partialPath := targetPath + ".partial"
	if err := os.RemoveAll(partialPath); err != nil {
		return "", 0, err
	}
	if err := os.MkdirAll(partialPath, 0o755); err != nil {
		return "", 0, err
	}

	installed := false
	defer func() {
		if !installed {
			_ = os.RemoveAll(partialPath)
		}
	}()

	if err := unpackSource(ctx, sourcePath, partialPath, progress); err != nil {
		return "", 0, err
	}

	executablePath, err := resolveExecutable(partialPath, executableRelativePath)
	if err != nil {
		return "", 0, err
	}
	if runtime.GOOS != "windows" {
		if err := os.Chmod(executablePath, 0o755); err != nil {
			return "", 0, err
		}
	}

	if err := os.RemoveAll(targetPath); err != nil {
		return "", 0, err
	}
	if err := os.Rename(partialPath, targetPath); err != nil {
		return "", 0, err
	}

	relativeExecutable, err := filepath.Rel(partialPath, executablePath)
	if err != nil {
		return "", 0, err
	}
	finalExecutable := filepath.Join(targetPath, relativeExecutable)

	size, err := directorySize(ctx, targetPath)
	if err != nil {
		return "", 0, err
	}

	installed = true
	slog.Info("game archive installed", "source", filepath.Base(sourcePath), "bytes", size)
	return finalExecutable, size, nil
}

// unpackState accumulates extracted bytes and reports progress.
type unpackState struct {
	mu     sync.Mutex
	copied int64
	total  int64
	report func(copied, total int64)
}

func (state *unpackState) add(n int64) {
	state.mu.Lock()
	state.copied += n
	copied, total := state.copied, state.total
	state.mu.Unlock()
	if state.report != nil {
		state.report(copied, total)
	}
}

type countingUnpackReader struct {
	reader io.Reader
	state  *unpackState
}

func (reader *countingUnpackReader) Read(buffer []byte) (int, error) {
	n, err := reader.reader.Read(buffer)
	if n > 0 {
		reader.state.add(int64(n))
	}
	return n, err
}

func unpackSource(
	ctx context.Context,
	sourcePath string,
	targetPath string,
	progress func(copied, total int64),
) error {
	info, err := os.Stat(sourcePath)
	if err != nil {
		return err
	}
	if info.IsDir() {
		total, err := directorySize(ctx, sourcePath)
		if err != nil {
			return err
		}
		return copyTree(ctx, sourcePath, targetPath, progress, total)
	}

	lowerName := strings.ToLower(sourcePath)
	switch {
	case strings.HasSuffix(lowerName, ".zip"):
		total, err := zipUncompressedSize(sourcePath)
		if err != nil {
			return err
		}
		return extractZip(ctx, sourcePath, targetPath, progress, total)
	case strings.HasSuffix(lowerName, ".tar.gz"), strings.HasSuffix(lowerName, ".tgz"):
		total, err := tarGzUncompressedSize(ctx, sourcePath)
		if err != nil {
			return err
		}
		return extractTarGz(ctx, sourcePath, targetPath, progress, total)
	default:
		return fmt.Errorf(
			"unsupported game archive %q; use .zip, .tar.gz, .tgz, or a directory",
			filepath.Base(sourcePath),
		)
	}
}

// zipUncompressedSize returns the total uncompressed size of a ZIP archive.
func zipUncompressedSize(archivePath string) (int64, error) {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return 0, fmt.Errorf("invalid ZIP archive: %w", err)
	}
	defer reader.Close()
	var total int64
	for _, entry := range reader.File {
		total += int64(entry.UncompressedSize64)
	}
	return total, nil
}

// tarGzUncompressedSize returns the total uncompressed size of a tar.gz
// archive by reading it once.
func tarGzUncompressedSize(ctx context.Context, archivePath string) (int64, error) {
	file, err := os.Open(archivePath)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return 0, fmt.Errorf("invalid gzip archive: %w", err)
	}
	defer gzipReader.Close()

	tarReader := tar.NewReader(gzipReader)
	var total int64
	for {
		if err := ctx.Err(); err != nil {
			return 0, err
		}
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return 0, fmt.Errorf("invalid tar archive: %w", err)
		}
		if header.Typeflag == tar.TypeReg || header.Typeflag == tar.TypeRegA {
			total += header.Size
		}
	}
	return total, nil
}

func extractZip(
	ctx context.Context,
	archivePath string,
	targetPath string,
	progress func(copied, total int64),
	total int64,
) error {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return fmt.Errorf("invalid ZIP archive: %w", err)
	}
	defer reader.Close()

	state := &unpackState{total: total, report: progress}
	for _, entry := range reader.File {
		if err := ctx.Err(); err != nil {
			return err
		}

		destination, err := safeArchiveDestination(targetPath, entry.Name)
		if err != nil {
			return err
		}
		if entry.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("symbolic links are not allowed in archives: %s", entry.Name)
		}
		if entry.FileInfo().IsDir() {
			if err := os.MkdirAll(destination, 0o755); err != nil {
				return err
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
			return err
		}

		source, err := entry.Open()
		if err != nil {
			return err
		}
		copyErr := copyArchiveFile(
			state,
			&countingUnpackReader{reader: source, state: state},
			destination,
			entry.Mode().Perm(),
		)
		closeErr := source.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
	}

	return nil
}

func extractTarGz(
	ctx context.Context,
	archivePath string,
	targetPath string,
	progress func(copied, total int64),
	total int64,
) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer file.Close()

	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("invalid gzip archive: %w", err)
	}
	defer gzipReader.Close()

	state := &unpackState{total: total, report: progress}
	tarReader := tar.NewReader(gzipReader)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("invalid tar archive: %w", err)
		}

		destination, err := safeArchiveDestination(targetPath, header.Name)
		if err != nil {
			return err
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(destination, 0o755); err != nil {
				return err
			}
		case tar.TypeReg, tar.TypeRegA:
			if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
				return err
			}
			if err := copyArchiveFile(
				state,
				&countingUnpackReader{reader: tarReader, state: state},
				destination,
				os.FileMode(header.Mode).Perm(),
			); err != nil {
				return err
			}
		case tar.TypeSymlink, tar.TypeLink:
			return fmt.Errorf("links are not allowed in archives: %s", header.Name)
		default:
			return fmt.Errorf("unsupported archive entry type for %s", header.Name)
		}
	}

	return nil
}

func safeArchiveDestination(targetPath string, entryName string) (string, error) {
	cleanTarget, err := filepath.Abs(targetPath)
	if err != nil {
		return "", err
	}

	destination := filepath.Join(cleanTarget, filepath.FromSlash(entryName))
	if destination != cleanTarget &&
		!strings.HasPrefix(destination, cleanTarget+string(os.PathSeparator)) {
		return "", fmt.Errorf("unsafe archive path: %s", entryName)
	}

	return destination, nil
}

func copyArchiveFile(
	state *unpackState,
	source io.Reader,
	destination string,
	mode os.FileMode,
) error {
	if mode == 0 {
		mode = 0o644
	}

	target, err := os.OpenFile(
		destination,
		os.O_CREATE|os.O_WRONLY|os.O_TRUNC,
		mode,
	)
	if err != nil {
		return err
	}

	_, copyErr := io.Copy(target, source)
	closeErr := target.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func copyTree(
	ctx context.Context,
	sourcePath string,
	targetPath string,
	progress func(copied, total int64),
	total int64,
) error {
	state := &unpackState{total: total, report: progress}
	return filepath.Walk(
		sourcePath,
		func(path string, info os.FileInfo, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if err := ctx.Err(); err != nil {
				return err
			}

			relativePath, err := filepath.Rel(sourcePath, path)
			if err != nil {
				return err
			}
			destination := filepath.Join(targetPath, relativePath)

			if info.IsDir() {
				return os.MkdirAll(destination, info.Mode().Perm())
			}
			if info.Mode()&os.ModeSymlink != 0 {
				return fmt.Errorf("symbolic links are not allowed: %s", path)
			}

			source, err := os.Open(path)
			if err != nil {
				return err
			}
			copyErr := copyArchiveFile(
				state,
				&countingUnpackReader{reader: source, state: state},
				destination,
				info.Mode().Perm(),
			)
			closeErr := source.Close()
			if copyErr != nil {
				return copyErr
			}
			return closeErr
		},
	)
}

func resolveExecutable(rootPath string, relativePath string) (string, error) {
	if relativePath != "" {
		path := filepath.Join(rootPath, filepath.Clean(relativePath))
		absoluteRoot, _ := filepath.Abs(rootPath)
		absolutePath, _ := filepath.Abs(path)
		if !strings.HasPrefix(absolutePath, absoluteRoot+string(os.PathSeparator)) {
			return "", fmt.Errorf("executable path escapes the installation directory")
		}

		info, err := os.Stat(path)
		if err == nil && !info.IsDir() {
			return path, nil
		}
		return "", fmt.Errorf("executable not found: %s", relativePath)
	}

	executableNames := map[string]struct{}{
		"vintagestory":     {},
		"vintagestory.exe": {},
	}

	var executablePath string
	err := filepath.Walk(
		rootPath,
		func(path string, info os.FileInfo, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if info.IsDir() {
				return nil
			}

			if _, matches := executableNames[strings.ToLower(info.Name())]; matches {
				executablePath = path
				return filepath.SkipAll
			}
			return nil
		},
	)
	if err != nil {
		return "", err
	}
	if executablePath == "" {
		return "", fmt.Errorf(
			"Vintagestory executable was not found; specify its path relative to the archive root",
		)
	}

	return executablePath, nil
}

func fileSHA256(ctx context.Context, path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	buffer := make([]byte, 128*1024)
	for {
		if err := ctx.Err(); err != nil {
			return "", err
		}

		readBytes, readErr := file.Read(buffer)
		if readBytes > 0 {
			_, _ = hash.Write(buffer[:readBytes])
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return "", readErr
		}
	}

	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

func directorySize(ctx context.Context, rootPath string) (int64, error) {
	var total int64
	err := filepath.Walk(
		rootPath,
		func(_ string, info os.FileInfo, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if err := ctx.Err(); err != nil {
				return err
			}
			if !info.IsDir() {
				total += info.Size()
			}
			return nil
		},
	)
	return total, err
}

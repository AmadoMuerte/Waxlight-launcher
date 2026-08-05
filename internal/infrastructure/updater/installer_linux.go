//go:build linux

package updater

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/waxlight/waxlight-launcher/internal/infrastructure/atomicfile"
)

const maximumExecutableBytes = 256 * 1024 * 1024

func (*Installer) Apply(ctx context.Context, archivePath string, _ int) error {
	if strings.HasSuffix(archivePath, ".deb") || strings.HasSuffix(archivePath, ".rpm") {
		return launchSystemPackageInstaller(archivePath)
	}
	if !strings.HasSuffix(archivePath, ".tar.gz") {
		return errors.New("unsupported Linux launcher update package")
	}
	return applyPortableUpdate(ctx, archivePath)
}

func applyPortableUpdate(ctx context.Context, archivePath string) error {
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	executable, err = filepath.EvalSymlinks(executable)
	if err != nil {
		return err
	}
	info, err := os.Lstat(executable)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("current launcher executable is not a safe regular file")
	}
	if info.Size() <= 0 || info.Size() > maximumExecutableBytes {
		return errors.New("current launcher executable has an unsafe size")
	}
	previousBinary, err := os.ReadFile(executable)
	if err != nil {
		return fmt.Errorf("read current launcher executable: %w", err)
	}
	binary, err := readPortableBinary(ctx, archivePath)
	if err != nil {
		return err
	}
	backupPath := executable + ".previous"
	if err := atomicfile.Write(backupPath, previousBinary, 0o755); err != nil {
		return fmt.Errorf("create recoverable launcher backup: %w", err)
	}
	if err := atomicfile.Write(executable, binary, 0o755); err != nil {
		return fmt.Errorf("atomically replace launcher executable: %w", err)
	}
	command := exec.Command(executable, "--update-wait-pid", strconv.Itoa(os.Getpid()))
	command.Dir = filepath.Dir(executable)
	if err := command.Start(); err != nil {
		if rollbackErr := atomicfile.Write(executable, previousBinary, 0o755); rollbackErr != nil {
			return fmt.Errorf("start updated launcher: %w; rollback failed: %v", err, rollbackErr)
		}
		return fmt.Errorf("start updated launcher; previous executable restored: %w", err)
	}
	return nil
}

func launchSystemPackageInstaller(packagePath string) error {
	opener, err := exec.LookPath("xdg-open")
	if err != nil {
		return errors.New("xdg-open is required to install a system package")
	}
	command := exec.Command(opener, packagePath)
	command.Dir = filepath.Dir(packagePath)
	if err := command.Start(); err != nil {
		return fmt.Errorf("open launcher package installer: %w", err)
	}
	return nil
}

func readPortableBinary(ctx context.Context, archivePath string) ([]byte, error) {
	archive, err := os.Open(archivePath)
	if err != nil {
		return nil, err
	}
	defer archive.Close()
	compressed, err := gzip.NewReader(archive)
	if err != nil {
		return nil, fmt.Errorf("open launcher update archive: %w", err)
	}
	defer compressed.Close()
	reader := tar.NewReader(compressed)
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read launcher update archive: %w", err)
		}
		if filepath.Base(filepath.ToSlash(header.Name)) != "waxlight" {
			continue
		}
		if header.Typeflag != tar.TypeReg || header.Size <= 0 || header.Size > maximumExecutableBytes {
			return nil, errors.New("launcher update contains an unsafe executable")
		}
		binary, err := io.ReadAll(io.LimitReader(reader, maximumExecutableBytes+1))
		if err != nil {
			return nil, err
		}
		if int64(len(binary)) != header.Size {
			return nil, errors.New("launcher update executable is truncated")
		}
		return binary, nil
	}
	return nil, errors.New("launcher update executable is missing")
}

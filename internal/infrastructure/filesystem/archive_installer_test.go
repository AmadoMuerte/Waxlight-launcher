package filesystem

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestArchiveInstallerInstallsDirectoryAtomically(t *testing.T) {
	sourceDirectory := t.TempDir()
	executableName := "Vintagestory"
	if err := os.WriteFile(
		filepath.Join(sourceDirectory, executableName),
		[]byte("game"),
		0o755,
	); err != nil {
		t.Fatal(err)
	}

	targetDirectory := filepath.Join(t.TempDir(), "1.0")
	executablePath, size, err := (ArchiveInstaller{}).Install(
		context.Background(),
		sourceDirectory,
		targetDirectory,
		"",
		"",
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}

	expectedExecutable := filepath.Join(targetDirectory, executableName)
	if executablePath != expectedExecutable {
		t.Fatalf("unexpected executable: %s", executablePath)
	}
	if size != 4 {
		t.Fatalf("expected 4 bytes, got %d", size)
	}
	if _, err := os.Stat(targetDirectory + ".partial"); !os.IsNotExist(err) {
		t.Fatal("the partial directory was not removed")
	}
}

func TestArchiveInstallerSupportsTarGzip(t *testing.T) {
	archivePath := filepath.Join(t.TempDir(), "vs_client_linux-x64_1.22.6.tar.gz")
	archiveFile, err := os.Create(archivePath)
	if err != nil {
		t.Fatal(err)
	}

	gzipWriter := gzip.NewWriter(archiveFile)
	tarWriter := tar.NewWriter(gzipWriter)
	content := []byte("game executable")
	header := &tar.Header{
		Name: "Vintagestory",
		Mode: 0o755,
		Size: int64(len(content)),
	}
	if err := tarWriter.WriteHeader(header); err != nil {
		t.Fatal(err)
	}
	if _, err := tarWriter.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := archiveFile.Close(); err != nil {
		t.Fatal(err)
	}

	targetDirectory := filepath.Join(t.TempDir(), "1.22.6")
	executablePath, size, err := (ArchiveInstaller{}).Install(
		context.Background(),
		archivePath,
		targetDirectory,
		"",
		"",
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}

	expectedExecutable := filepath.Join(targetDirectory, "Vintagestory")
	if executablePath != expectedExecutable {
		t.Fatalf("unexpected executable path %q", executablePath)
	}
	if size != int64(len(content)) {
		t.Fatalf("unexpected installed size %d", size)
	}

	fileInfo, err := os.Stat(executablePath)
	if err != nil {
		t.Fatal(err)
	}
	if fileInfo.Mode().Perm()&0o111 == 0 {
		t.Fatalf("the executable bit was not set: %v", fileInfo.Mode())
	}
}

func TestArchiveInstallerRejectsPathTraversal(t *testing.T) {
	archivePath := filepath.Join(t.TempDir(), "unsafe.zip")
	archiveFile, err := os.Create(archivePath)
	if err != nil {
		t.Fatal(err)
	}

	zipWriter := zip.NewWriter(archiveFile)
	entry, err := zipWriter.Create("../outside.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Write([]byte("bad")); err != nil {
		t.Fatal(err)
	}
	if err := zipWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := archiveFile.Close(); err != nil {
		t.Fatal(err)
	}

	_, _, err = (ArchiveInstaller{}).Install(
		context.Background(),
		archivePath,
		filepath.Join(t.TempDir(), "version"),
		"",
		"",
		nil,
	)
	if err == nil || !strings.Contains(err.Error(), "unsafe archive path") {
		t.Fatalf("expected a path traversal error, got %v", err)
	}
}

func TestArchiveInstallerRejectsChecksumMismatch(t *testing.T) {
	sourcePath := filepath.Join(t.TempDir(), "game.zip")
	if err := os.WriteFile(sourcePath, []byte("archive"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, _, err := (ArchiveInstaller{}).Install(
		context.Background(),
		sourcePath,
		filepath.Join(t.TempDir(), "version"),
		"",
		strings.Repeat("0", 64),
		nil,
	)
	if err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("expected a checksum error, got %v", err)
	}
}

package application

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/waxlight/waxlight-launcher/internal/domain"
)

type updateSourceStub struct {
	update domain.LauncherUpdate
	err    error
}

func (stub updateSourceStub) Check(context.Context, string, string) (domain.LauncherUpdate, error) {
	return stub.update, stub.err
}

type updateDownloaderStub struct {
	request DownloadRequest
	err     error
}

func (stub *updateDownloaderStub) Download(
	_ context.Context,
	request DownloadRequest,
	progress chan<- DownloadProgress,
) error {
	stub.request = request
	if progress != nil {
		progress <- DownloadProgress{DownloadedBytes: 50, TotalBytes: 100}
	}
	return stub.err
}

type updateInstallerStub struct {
	path string
	err  error
}

func (stub *updateInstallerStub) Apply(_ context.Context, path string) error {
	stub.path = path
	return stub.err
}

func TestLauncherUpdateDownloadsVerifiedOfficialAsset(t *testing.T) {
	downloader := &updateDownloaderStub{}
	installer := &updateInstallerStub{}
	service := NewLauncherUpdateService(updateSourceStub{update: domain.LauncherUpdate{
		InstalledVersion: "0.1.4",
		Version:          "0.1.5",
		Available:        true,
		AssetName:        "Waxlight-Launcher-v0.1.5-linux-amd64.tar.gz",
		DownloadURL:      "https://github.com/AmadoMuerte/Waxlight-launcher/releases/download/v0.1.5/Waxlight-Launcher-v0.1.5-linux-amd64.tar.gz",
		SHA256:           "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}}, downloader, installer, t.TempDir(), "0.1.4")

	var phases []string
	err := service.Install(context.Background(), "stable", func(progress domain.LauncherUpdateProgress) {
		phases = append(phases, progress.Phase)
	})
	if err != nil {
		t.Fatal(err)
	}
	if downloader.request.ChecksumAlgorithm != "sha256" || !downloader.request.Resume {
		t.Fatalf("update download was not verified and resumable: %+v", downloader.request)
	}
	if installer.path != downloader.request.DestinationPath {
		t.Fatalf("installer received %q, want %q", installer.path, downloader.request.DestinationPath)
	}
	if filepath.Base(installer.path) != "Waxlight-Launcher-v0.1.5-linux-amd64.tar.gz" {
		t.Fatalf("unexpected staged update path %q", installer.path)
	}
	if len(phases) < 3 || phases[len(phases)-1] != "restarting" {
		t.Fatalf("unexpected progress phases: %v", phases)
	}
}

func TestLauncherUpdatePreservesInstallationOnVerificationFailure(t *testing.T) {
	downloader := &updateDownloaderStub{err: errors.New("checksum mismatch")}
	installer := &updateInstallerStub{}
	service := NewLauncherUpdateService(updateSourceStub{update: domain.LauncherUpdate{
		Available:   true,
		AssetName:   "Waxlight-Launcher-v0.1.5-linux-amd64.tar.gz",
		DownloadURL: "https://github.com/example",
		SHA256:      "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}}, downloader, installer, t.TempDir(), "0.1.4")

	if err := service.Install(context.Background(), "stable", nil); err == nil {
		t.Fatal("expected verification failure")
	}
	if installer.path != "" {
		t.Fatal("installer ran after failed verification")
	}
}

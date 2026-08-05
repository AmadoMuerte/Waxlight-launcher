package application

import (
	"context"
	"errors"
	"path/filepath"
	"runtime"
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

func (stub *updateInstallerStub) Apply(_ context.Context, path string, _ int) error {
	stub.path = path
	return stub.err
}

type signatureVerifierStub struct {
	err error
}

func (stub *signatureVerifierStub) Verify(_ context.Context, _ string) error {
	return stub.err
}

func TestLauncherUpdateDownloadsVerifiedOfficialAsset(t *testing.T) {
	downloader := &updateDownloaderStub{}
	installer := &updateInstallerStub{}
	signatureVerifier := &signatureVerifierStub{}
	service := NewLauncherUpdateService(updateSourceStub{update: domain.LauncherUpdate{
		InstalledVersion: "0.1.4",
		Version:          "0.1.5",
		Available:        true,
		AssetName:        "Waxlight-Launcher-v0.1.5-linux-amd64.tar.gz",
		DownloadURL:      "https://github.com/AmadoMuerte/Waxlight-launcher/releases/download/v0.1.5/Waxlight-Launcher-v0.1.5-linux-amd64.tar.gz",
		SHA256:           "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}}, downloader, installer, signatureVerifier, t.TempDir(), "0.1.4")

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
	signatureVerifier := &signatureVerifierStub{}
	service := NewLauncherUpdateService(updateSourceStub{update: domain.LauncherUpdate{
		Available:   true,
		AssetName:   "Waxlight-Launcher-v0.1.5-linux-amd64.tar.gz",
		DownloadURL: "https://github.com/example",
		SHA256:      "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}}, downloader, installer, signatureVerifier, t.TempDir(), "0.1.4")

	if err := service.Install(context.Background(), "stable", nil); err == nil {
		t.Fatal("expected verification failure")
	}
	if installer.path != "" {
		t.Fatal("installer ran after failed verification")
	}
}

func TestLauncherUpdateRejectsUnsafeFilename(t *testing.T) {
	downloader := &updateDownloaderStub{}
	installer := &updateInstallerStub{}
	signatureVerifier := &signatureVerifierStub{}
	service := NewLauncherUpdateService(updateSourceStub{update: domain.LauncherUpdate{
		Available:   true,
		AssetName:   "../../etc/passwd",
		DownloadURL: "https://github.com/example",
		SHA256:      "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}}, downloader, installer, signatureVerifier, t.TempDir(), "0.1.4")

	err := service.Install(context.Background(), "stable", nil)
	if err == nil {
		t.Fatal("expected unsafe filename error")
	}
	if installer.path != "" {
		t.Fatal("installer ran after filename validation failure")
	}
}

func TestLauncherUpdateRejectsInvalidChecksumLength(t *testing.T) {
	downloader := &updateDownloaderStub{}
	installer := &updateInstallerStub{}
	signatureVerifier := &signatureVerifierStub{}
	service := NewLauncherUpdateService(updateSourceStub{update: domain.LauncherUpdate{
		Available:   true,
		AssetName:   "Waxlight-Launcher-v0.1.5-linux-amd64.tar.gz",
		DownloadURL: "https://github.com/example",
		SHA256:      "tooshort",
	}}, downloader, installer, signatureVerifier, t.TempDir(), "0.1.4")

	err := service.Install(context.Background(), "stable", nil)
	if err == nil {
		t.Fatal("expected checksum validation error")
	}
	if installer.path != "" {
		t.Fatal("installer ran after checksum validation failure")
	}
}

func TestLauncherUpdateRejectsSignatureFailure(t *testing.T) {
	downloader := &updateDownloaderStub{}
	installer := &updateInstallerStub{}
	signatureVerifier := &signatureVerifierStub{err: errors.New("signature invalid")}
	service := NewLauncherUpdateService(updateSourceStub{update: domain.LauncherUpdate{
		Available:   true,
		AssetName:   "Waxlight-Launcher-v0.1.5-linux-amd64.tar.gz",
		DownloadURL: "https://github.com/example",
		SHA256:      "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}}, downloader, installer, signatureVerifier, t.TempDir(), "0.1.4")

	err := service.Install(context.Background(), "stable", nil)
	if err == nil {
		t.Fatal("expected signature verification failure")
	}
	if installer.path != "" {
		t.Fatal("installer ran after signature verification failure")
	}
}

func TestLauncherUpdateRejectsInstallerFailure(t *testing.T) {
	downloader := &updateDownloaderStub{}
	installer := &updateInstallerStub{err: errors.New("installer failed")}
	signatureVerifier := &signatureVerifierStub{}
	service := NewLauncherUpdateService(updateSourceStub{update: domain.LauncherUpdate{
		Available:   true,
		AssetName:   "Waxlight-Launcher-v0.1.5-linux-amd64.tar.gz",
		DownloadURL: "https://github.com/example",
		SHA256:      "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}}, downloader, installer, signatureVerifier, t.TempDir(), "0.1.4")

	err := service.Install(context.Background(), "stable", nil)
	if err == nil {
		t.Fatal("expected installer failure")
	}
}

func TestLauncherUpdateRejectsConcurrentInstall(t *testing.T) {
	downloader := &updateDownloaderStub{}
	installer := &updateInstallerStub{}
	signatureVerifier := &signatureVerifierStub{}
	service := NewLauncherUpdateService(updateSourceStub{update: domain.LauncherUpdate{
		Available:   true,
		AssetName:   "Waxlight-Launcher-v0.1.5-linux-amd64.tar.gz",
		DownloadURL: "https://github.com/example",
		SHA256:      "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}}, downloader, installer, signatureVerifier, t.TempDir(), "0.1.4")

	service.mu.Lock()
	service.installing = true
	service.mu.Unlock()

	err := service.Install(context.Background(), "stable", nil)
	if err == nil {
		t.Fatal("expected concurrent install error")
	}
}

func TestLauncherUpdateRejectsInvalidChannel(t *testing.T) {
	downloader := &updateDownloaderStub{}
	installer := &updateInstallerStub{}
	signatureVerifier := &signatureVerifierStub{}
	service := NewLauncherUpdateService(updateSourceStub{}, downloader, installer, signatureVerifier, t.TempDir(), "0.1.4")

	err := service.Install(context.Background(), "invalid", nil)
	if err == nil {
		t.Fatal("expected invalid channel error")
	}
}

func TestLauncherUpdateRejectsNoUpdateAvailable(t *testing.T) {
	downloader := &updateDownloaderStub{}
	installer := &updateInstallerStub{}
	signatureVerifier := &signatureVerifierStub{}
	service := NewLauncherUpdateService(updateSourceStub{update: domain.LauncherUpdate{
		Available: false,
	}}, downloader, installer, signatureVerifier, t.TempDir(), "0.1.4")

	err := service.Install(context.Background(), "stable", nil)
	if err == nil {
		t.Fatal("expected no update available error")
	}
}

func TestLauncherUpdateAllowsPortableModeOnLinux(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux-specific portable update test")
	}

	downloader := &updateDownloaderStub{}
	installer := &updateInstallerStub{}
	signatureVerifier := &signatureVerifierStub{}

	service := NewLauncherUpdateService(
		updateSourceStub{
			update: domain.LauncherUpdate{
				Available:        true,
				AssetName:        "Waxlight-Launcher-v0.1.5-linux-amd64.tar.gz",
				DownloadURL:      "https://github.com/example",
				SHA256:           "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				InstallationMode: "portable",
			},
		},
		downloader,
		installer,
		signatureVerifier,
		t.TempDir(),
		"0.1.4",
	)

	err := service.Install(context.Background(), "stable", nil)
	if err != nil {
		t.Fatalf("portable Linux update failed: %v", err)
	}

	if downloader.request.DestinationPath == "" {
		t.Fatal("update was not downloaded")
	}

	if installer.path == "" {
		t.Fatal("installer was not called")
	}

	if installer.path != downloader.request.DestinationPath {
		t.Fatalf(
			"installer path = %q, want %q",
			installer.path,
			downloader.request.DestinationPath,
		)
	}
}

func TestLauncherUpdateRejectsPortableModeOnWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-specific portable update test")
	}

	downloader := &updateDownloaderStub{}
	installer := &updateInstallerStub{}
	signatureVerifier := &signatureVerifierStub{}

	service := NewLauncherUpdateService(
		updateSourceStub{
			update: domain.LauncherUpdate{
				Available:        true,
				AssetName:        "Waxlight-Launcher-v0.1.5-windows-amd64.zip",
				DownloadURL:      "https://github.com/example",
				SHA256:           "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				InstallationMode: "portable",
			},
		},
		downloader,
		installer,
		signatureVerifier,
		t.TempDir(),
		"0.1.4",
	)

	err := service.Install(context.Background(), "stable", nil)
	if err == nil {
		t.Fatal("expected portable Windows update error")
	}

	var appErr *domain.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("expected AppError, got %T: %v", err, err)
	}

	if appErr.Code != domain.ErrUpdateUnsupported {
		t.Fatalf(
			"error code = %q, want %q",
			appErr.Code,
			domain.ErrUpdateUnsupported,
		)
	}

	if downloader.request.DestinationPath != "" {
		t.Fatal("downloader ran for portable Windows mode")
	}

	if installer.path != "" {
		t.Fatal("installer ran for portable Windows mode")
	}
}

func TestLauncherUpdatePublishesProgressPhases(t *testing.T) {
	downloader := &updateDownloaderStub{}
	installer := &updateInstallerStub{}
	signatureVerifier := &signatureVerifierStub{}
	service := NewLauncherUpdateService(updateSourceStub{update: domain.LauncherUpdate{
		Available:   true,
		AssetName:   "Waxlight-Launcher-v0.1.5-linux-amd64.tar.gz",
		DownloadURL: "https://github.com/example",
		SHA256:      "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}}, downloader, installer, signatureVerifier, t.TempDir(), "0.1.4")

	var phases []string
	err := service.Install(context.Background(), "stable", func(progress domain.LauncherUpdateProgress) {
		phases = append(phases, progress.Phase)
	})
	if err != nil {
		t.Fatal(err)
	}

	expectedPhases := []string{"checking", "downloading", "downloading", "signature", "installing", "restarting"}
	if len(phases) != len(expectedPhases) {
		t.Fatalf("expected %d phases, got %d: %v", len(expectedPhases), len(phases), phases)
	}
	for i, expected := range expectedPhases {
		if phases[i] != expected {
			t.Fatalf("phase %d: expected %q, got %q", i, expected, phases[i])
		}
	}
}

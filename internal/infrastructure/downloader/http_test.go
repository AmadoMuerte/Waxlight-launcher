package downloader

import (
	"context"
	"crypto/md5"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/waxlight/waxlight-launcher/internal/application"
)

func TestDownloaderVerifiesOfficialMD5AndReportsProgress(t *testing.T) {
	content := []byte("waxlight download")
	server := httptest.NewTLSServer(http.HandlerFunc(func(
		writer http.ResponseWriter,
		_ *http.Request,
	) {
		writer.Header().Set("Content-Length", fmt.Sprint(len(content)))
		_, _ = writer.Write(content)
	}))
	defer server.Close()

	destination := filepath.Join(t.TempDir(), "game.tar.gz")
	progress := make(chan application.DownloadProgress, 2)
	downloader := &HTTPDownloader{Client: server.Client()}
	err := downloader.Download(
		context.Background(),
		application.DownloadRequest{
			URL:               server.URL,
			DestinationPath:   destination,
			ExpectedChecksum:  fmt.Sprintf("%x", md5.Sum(content)),
			ChecksumAlgorithm: "md5",
		},
		progress,
	)
	if err != nil {
		t.Fatal(err)
	}
	written, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	if string(written) != string(content) {
		t.Fatalf("unexpected downloaded content %q", written)
	}
	select {
	case update := <-progress:
		if update.DownloadedBytes != int64(len(content)) {
			t.Fatalf("unexpected progress: %+v", update)
		}
	default:
		t.Fatal("expected a progress update")
	}
}

func TestDownloaderRejectsChecksumMismatch(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(
		writer http.ResponseWriter,
		_ *http.Request,
	) {
		_, _ = writer.Write([]byte("different"))
	}))
	defer server.Close()

	destination := filepath.Join(t.TempDir(), "game.tar.gz")
	downloader := &HTTPDownloader{Client: server.Client()}
	err := downloader.Download(
		context.Background(),
		application.DownloadRequest{
			URL:               server.URL,
			DestinationPath:   destination,
			ExpectedChecksum:  "0123456789abcdef0123456789abcdef",
			ChecksumAlgorithm: "md5",
		},
		nil,
	)
	if err == nil {
		t.Fatal("expected checksum mismatch")
	}
	if _, statErr := os.Stat(destination); !os.IsNotExist(statErr) {
		t.Fatalf("final file must not exist after mismatch: %v", statErr)
	}
}

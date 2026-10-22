package downloader

import (
	"context"
	"crypto/md5"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/AmadoMuerte/Waxlight-launcher/internal/downloads"
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
	progress := make(chan downloads.Progress, 2)
	downloader := &HTTPDownloader{Client: server.Client()}
	err := downloader.Download(
		context.Background(),
		downloads.Request{
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

func TestNormalizeDownloadURL(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{
			name: "encodes raw spaces in query",
			raw:  "https://moddbcdn.vintagestory.at/Immersive+Light_0.2._abc.zip?dl=Immersive Light_0.2.5.zip",
			want: "https://moddbcdn.vintagestory.at/Immersive+Light_0.2._abc.zip?dl=Immersive+Light_0.2.5.zip",
		},
		{
			name: "keeps clean URL unchanged",
			raw:  "https://moddbcdn.vintagestory.at/ImmersiveMining_0.3._abc.zip?dl=ImmersiveMining_0.3.0.zip",
			want: "https://moddbcdn.vintagestory.at/ImmersiveMining_0.3._abc.zip?dl=ImmersiveMining_0.3.0.zip",
		},
		{
			name: "keeps URL without query unchanged",
			raw:  "https://example.com/files/mod.zip",
			want: "https://example.com/files/mod.zip",
		},
		{
			name: "preserves percent-encoded query values",
			raw:  "https://example.com/mod.zip?dl=a%20b.zip&x=1",
			want: "https://example.com/mod.zip?dl=a+b.zip&x=1",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := normalizeDownloadURL(test.raw)
			if err != nil {
				t.Fatalf("normalizeDownloadURL returned error: %v", err)
			}
			if got != test.want {
				t.Fatalf("normalizeDownloadURL(%q) = %q, want %q", test.raw, got, test.want)
			}
		})
	}
}

func TestDownloaderAcceptsURLWithRawSpacesInQuery(t *testing.T) {
	content := []byte("mod archive")
	server := httptest.NewTLSServer(http.HandlerFunc(func(
		writer http.ResponseWriter,
		request *http.Request,
	) {
		if got := request.URL.Query().Get("dl"); got != "Immersive Light_0.2.5.zip" {
			http.Error(writer, "unexpected dl parameter: "+got, http.StatusBadRequest)
			return
		}
		_, _ = writer.Write(content)
	}))
	defer server.Close()

	destination := filepath.Join(t.TempDir(), "Immersive Light_0.2.5.zip")
	downloader := &HTTPDownloader{Client: server.Client()}
	err := downloader.Download(
		context.Background(),
		downloads.Request{
			URL:             server.URL + "/mod.zip?dl=Immersive Light_0.2.5.zip",
			DestinationPath: destination,
		},
		nil,
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
		downloads.Request{
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

func TestContentLengthReturnsCorrectSize(t *testing.T) {
	content := []byte("test content for size")
	server := httptest.NewTLSServer(http.HandlerFunc(func(
		writer http.ResponseWriter,
		request *http.Request,
	) {
		writer.Header().Set("Content-Length", fmt.Sprint(len(content)))
		_, _ = writer.Write(content)
	}))
	defer server.Close()

	downloader := &HTTPDownloader{Client: server.Client()}
	size, err := downloader.ContentLength(context.Background(), server.URL+"/mod.zip")
	if err != nil {
		t.Fatalf("ContentLength returned error: %v", err)
	}
	if size != int64(len(content)) {
		t.Fatalf("ContentLength = %d, want %d", size, len(content))
	}
}

func TestContentLengthWithSpacesInURL(t *testing.T) {
	content := []byte("mod archive")
	receivedURL := ""
	server := httptest.NewTLSServer(http.HandlerFunc(func(
		writer http.ResponseWriter,
		request *http.Request,
	) {
		receivedURL = request.URL.String()
		writer.Header().Set("Content-Length", fmt.Sprint(len(content)))
		_, _ = writer.Write(content)
	}))
	defer server.Close()

	downloader := &HTTPDownloader{Client: server.Client()}
	// URL with raw space in query — should be normalized before HEAD request
	size, err := downloader.ContentLength(context.Background(),
		server.URL+"/mod.zip?dl=Immersive Light_0.2.5.zip")
	if err != nil {
		t.Fatalf("ContentLength returned error: %v", err)
	}
	if size != int64(len(content)) {
		t.Fatalf("ContentLength = %d, want %d", size, len(content))
	}
	// Verify the space was encoded (server received %20 or +)
	if !strings.Contains(receivedURL, "%20") && !strings.Contains(receivedURL, "+") {
		t.Fatalf("URL was not encoded, got: %s", receivedURL)
	}
}

func TestDownloaderRetriesTransportFailureAndSetsUserAgent(t *testing.T) {
	content := []byte("resilient download")
	var requests int
	var seenUserAgent string
	server := httptest.NewTLSServer(http.HandlerFunc(func(
		writer http.ResponseWriter,
		request *http.Request,
	) {
		requests++
		seenUserAgent = request.Header.Get("User-Agent")
		if requests == 1 {
			// Drop the connection before answering: the client sees a
			// transport error, not an HTTP status.
			conn, _, hijackErr := writer.(http.Hijacker).Hijack()
			if hijackErr != nil {
				t.Errorf("hijack failed: %v", hijackErr)
				return
			}
			_ = conn.Close()
			return
		}
		writer.Header().Set("Content-Length", fmt.Sprint(len(content)))
		_, _ = writer.Write(content)
	}))
	defer server.Close()

	downloader := &HTTPDownloader{Client: server.Client()}
	destination := filepath.Join(t.TempDir(), "mod.zip")
	err := downloader.Download(context.Background(), downloads.Request{
		URL:             server.URL,
		DestinationPath: destination,
	}, nil)
	if err != nil {
		t.Fatalf("expected the retry to succeed, got %v", err)
	}
	if requests != 2 {
		t.Fatalf("expected 2 requests (1 failed + 1 retry), got %d", requests)
	}
	if seenUserAgent != userAgent {
		t.Fatalf("expected User-Agent %q, got %q", userAgent, seenUserAgent)
	}
	written, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	if string(written) != string(content) {
		t.Fatalf("unexpected downloaded content %q", written)
	}
}

func TestDownloaderGivesUpAfterRepeatedTransportFailures(t *testing.T) {
	var requests int
	server := httptest.NewTLSServer(http.HandlerFunc(func(
		writer http.ResponseWriter,
		_ *http.Request,
	) {
		requests++
		conn, _, hijackErr := writer.(http.Hijacker).Hijack()
		if hijackErr != nil {
			t.Errorf("hijack failed: %v", hijackErr)
			return
		}
		_ = conn.Close()
	}))
	defer server.Close()

	// Keep the retry pause from slowing the test run down.
	previous := downloadRetryDelay
	downloadRetryDelay = 0
	defer func() { downloadRetryDelay = previous }()

	downloader := &HTTPDownloader{Client: server.Client()}
	err := downloader.Download(context.Background(), downloads.Request{
		URL:             server.URL,
		DestinationPath: filepath.Join(t.TempDir(), "mod.zip"),
	}, nil)
	if err == nil {
		t.Fatal("expected the download to fail after exhausting the retries")
	}
	if requests != maxDownloadAttempts {
		t.Fatalf("expected %d attempts, got %d", maxDownloadAttempts, requests)
	}
	if _, statErr := os.Stat(filepath.Join(t.TempDir(), "mod.zip.partial")); statErr == nil {
		t.Fatal("expected no partial file to be left behind")
	}
}

func TestDownloaderDoesNotRetryCancelledDownload(t *testing.T) {
	var requests int
	server := httptest.NewTLSServer(http.HandlerFunc(func(
		writer http.ResponseWriter,
		_ *http.Request,
	) {
		requests++
		conn, _, hijackErr := writer.(http.Hijacker).Hijack()
		if hijackErr != nil {
			t.Errorf("hijack failed: %v", hijackErr)
			return
		}
		_ = conn.Close()
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	previous := downloadRetryDelay
	downloadRetryDelay = time.Minute
	defer func() { downloadRetryDelay = previous }()

	downloader := &HTTPDownloader{Client: server.Client()}
	err := downloader.Download(ctx, downloads.Request{
		URL:             server.URL,
		DestinationPath: filepath.Join(t.TempDir(), "mod.zip"),
	}, nil)
	if err == nil {
		t.Fatal("expected the cancelled download to fail")
	}
	if requests > 1 {
		t.Fatalf("expected no retry for a cancelled download, got %d requests", requests)
	}
}

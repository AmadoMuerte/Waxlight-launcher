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

	"github.com/waxlight/waxlight-launcher/internal/downloads"
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

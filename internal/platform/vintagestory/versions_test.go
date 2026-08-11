package vintagestory

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestVersionCatalogSelectsPlatformDistributionAndSortsVersions(t *testing.T) {
	payload := `{
  "1.22.0-rc.2": {
    "linux": {
      "filename": "vs_client_linux-x64_1.22.0-rc.2.tar.gz",
      "filesize": "590.5 MB",
      "md5": "0123456789abcdef0123456789abcdef",
      "urls": {"cdn": "https://cdn.vintagestory.at/gamefiles/unstable/vs_client_linux-x64_1.22.0-rc.2.tar.gz"},
      "latest": 1
    }
  },
  "1.22.0": {
    "linux": {
      "filename": "vs_client_linux-x64_1.22.0.tar.gz",
      "filesize": "591 MB",
      "md5": "abcdef0123456789abcdef0123456789",
      "urls": {"cdn": "https://cdn.vintagestory.at/gamefiles/stable/vs_client_linux-x64_1.22.0.tar.gz"},
      "latest": 1
    },
    "windows": {
      "filename": "vs_install_win-x64_1.22.0.exe",
      "filesize": "570 MB",
      "md5": "abcdef0123456789abcdef0123456789",
      "urls": {"cdn": "https://cdn.vintagestory.at/gamefiles/stable/vs_install_win-x64_1.22.0.exe"}
    }
  }
}`
	server := httptest.NewServer(http.HandlerFunc(func(
		writer http.ResponseWriter,
		_ *http.Request,
	) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(payload))
	}))
	defer server.Close()

	catalog := NewVersionCatalogForPlatform(
		server.Client(),
		server.URL,
		"linux",
		"amd64",
	)
	versions, err := catalog.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(versions) != 2 {
		t.Fatalf("expected two versions, got %d", len(versions))
	}
	if versions[0].ID != "1.22.0" || versions[0].Channel != "stable" {
		t.Fatalf("unexpected first release: %+v", versions[0])
	}
	if versions[1].ID != "1.22.0-rc.2" ||
		versions[1].Channel != "unstable" ||
		versions[1].DownloadSize != 590_500_000 {
		t.Fatalf("unexpected preview release: %+v", versions[1])
	}
}

func TestVersionCatalogRejectsUntrustedDownloadURLs(t *testing.T) {
	payload := `{"1.22.0":{"linux":{"filename":"game.tar.gz","filesize":"1 MB","md5":"0123456789abcdef0123456789abcdef","urls":{"cdn":"https://example.com/game.tar.gz"}}}}`
	server := httptest.NewServer(http.HandlerFunc(func(
		writer http.ResponseWriter,
		_ *http.Request,
	) {
		_, _ = writer.Write([]byte(payload))
	}))
	defer server.Close()

	catalog := NewVersionCatalogForPlatform(
		server.Client(),
		server.URL,
		"linux",
		"amd64",
	)
	versions, err := catalog.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(versions) != 0 {
		t.Fatalf("expected the untrusted release to be rejected: %+v", versions)
	}
}

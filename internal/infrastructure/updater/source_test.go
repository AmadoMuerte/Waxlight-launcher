package updater

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func TestSourceSelectsLatestChannelAssetAndChecksum(t *testing.T) {
	assetName, err := expectedAssetName("0.1.5")
	if err != nil {
		t.Skip(err)
	}
	releases := []githubRelease{
		releaseFixture("v0.2.0-beta.1", true, assetName),
		releaseFixture("v0.1.5", false, assetName),
	}
	payload, err := json.Marshal(releases)
	if err != nil {
		t.Fatal(err)
	}
	checksum := strings.Repeat("a", 64)
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		body := payload
		if strings.HasSuffix(request.URL.Path, "/SHA256SUMS") {
			body = []byte(checksum + "  " + assetName + "\n")
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(string(body))),
			Header:     make(http.Header),
			Request:    request,
		}, nil
	})}

	update, err := NewSource(client).Check(context.Background(), "0.1.4", "stable")
	if err != nil {
		t.Fatal(err)
	}
	if !update.Available || update.Version != "0.1.5" || update.Prerelease {
		t.Fatalf("unexpected stable update: %+v", update)
	}
	if update.AssetName != assetName || update.SHA256 != checksum {
		t.Fatalf("unexpected verified asset: %+v", update)
	}
}

func TestSourceRejectsUntrustedAssetURL(t *testing.T) {
	assetName, err := expectedAssetName("0.1.5")
	if err != nil {
		t.Skip(err)
	}
	release := releaseFixture("v0.1.5", false, assetName)
	release.Assets[0].BrowserDownloadURL = "https://example.com/update"
	payload, err := json.Marshal([]githubRelease{release})
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(string(payload))),
			Header:     make(http.Header),
			Request:    request,
		}, nil
	})}

	if _, err := NewSource(client).Check(context.Background(), "0.1.4", "stable"); err == nil ||
		!strings.Contains(err.Error(), "untrusted") {
		t.Fatalf("expected untrusted URL error, got %v", err)
	}
}

func TestAssetNameForSupportedPlatforms(t *testing.T) {
	cases := map[string]string{
		"linux":   "Waxlight-Launcher-v0.1.5-linux-amd64.tar.gz",
		"windows": "Waxlight-Launcher-v0.1.5-windows-amd64-installer.exe",
	}
	for platform, expected := range cases {
		actual, err := assetNameForPlatform("0.1.5", platform, "amd64", "tar.gz")
		if err != nil {
			t.Fatal(err)
		}
		if actual != expected {
			t.Fatalf("%s: got %q, want %q", platform, actual, expected)
		}
	}
	if _, err := assetNameForPlatform("0.1.5", "linux", "arm64", "tar.gz"); err == nil {
		t.Fatal("expected unsupported architecture error")
	}
	deb, err := assetNameForPlatform("0.1.5", "linux", "amd64", "deb")
	if err != nil || !strings.HasSuffix(deb, ".deb") {
		t.Fatalf("unexpected Debian asset: %q, %v", deb, err)
	}
}

func TestUpdateHTTPClientRejectsUntrustedRedirect(t *testing.T) {
	client := NewHTTPClient()
	request, err := http.NewRequest(http.MethodGet, "https://example.com/update", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := client.CheckRedirect(request, nil); err == nil {
		t.Fatal("expected untrusted redirect to be rejected")
	}
}

func TestSourceOffersDowngradeWhenSwitchingToStable(t *testing.T) {
	stableAsset, err := expectedAssetName("0.1.5")
	if err != nil {
		t.Skip(err)
	}
	releases := []githubRelease{
		releaseFixture("v0.1.5", false, stableAsset),
	}
	payload, err := json.Marshal(releases)
	if err != nil {
		t.Fatal(err)
	}
	checksum := strings.Repeat("a", 64)
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		body := payload
		if strings.HasSuffix(request.URL.Path, "/SHA256SUMS") {
			body = []byte(checksum + "  " + stableAsset + "\n")
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(string(body))),
			Header:     make(http.Header),
			Request:    request,
		}, nil
	})}

	update, err := NewSource(client).Check(context.Background(), "0.2.0-beta.1", "stable")
	if err != nil {
		t.Fatal(err)
	}
	if !update.Available || !update.Downgrade {
		t.Fatalf("expected downgrade to be available, got: %+v", update)
	}
	if update.Version != "0.1.5" {
		t.Fatalf("expected version 0.1.5, got %q", update.Version)
	}
}

func TestSourceNoDowngradeWhenCurrentIsStable(t *testing.T) {
	assetName, err := expectedAssetName("0.1.5")
	if err != nil {
		t.Skip(err)
	}
	releases := []githubRelease{
		releaseFixture("v0.1.5", false, assetName),
	}
	payload, err := json.Marshal(releases)
	if err != nil {
		t.Fatal(err)
	}
	checksum := strings.Repeat("a", 64)
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		body := payload
		if strings.HasSuffix(request.URL.Path, "/SHA256SUMS") {
			body = []byte(checksum + "  " + assetName + "\n")
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(string(body))),
			Header:     make(http.Header),
			Request:    request,
		}, nil
	})}

	update, err := NewSource(client).Check(context.Background(), "0.1.5", "stable")
	if err != nil {
		t.Fatal(err)
	}
	if update.Available || update.Downgrade {
		t.Fatalf("expected no update available, got: %+v", update)
	}
}

func TestSourceNoDowngradeWhenVersionIsNewer(t *testing.T) {
	assetName, err := expectedAssetName("0.3.0")
	if err != nil {
		t.Skip(err)
	}
	releases := []githubRelease{
		releaseFixture("v0.3.0", false, assetName),
	}
	payload, err := json.Marshal(releases)
	if err != nil {
		t.Fatal(err)
	}
	checksum := strings.Repeat("a", 64)
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		body := payload
		if strings.HasSuffix(request.URL.Path, "/SHA256SUMS") {
			body = []byte(checksum + "  " + assetName + "\n")
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(string(body))),
			Header:     make(http.Header),
			Request:    request,
		}, nil
	})}

	update, err := NewSource(client).Check(context.Background(), "0.2.0-beta.9", "stable")
	if err != nil {
		t.Fatal(err)
	}
	if !update.Available || update.Downgrade {
		t.Fatalf("expected normal update (not downgrade), got: %+v", update)
	}
	if update.Version != "0.3.0" {
		t.Fatalf("expected version 0.3.0, got %q", update.Version)
	}
}

func releaseFixture(tag string, prerelease bool, assetName string) githubRelease {
	base := "https://github.com/AmadoMuerte/Waxlight-launcher/releases"
	return githubRelease{
		TagName:    tag,
		HTMLURL:    base + "/tag/" + tag,
		Body:       "Release notes",
		Prerelease: prerelease,
		Assets: []githubAsset{
			{
				Name:               assetName,
				BrowserDownloadURL: base + "/download/" + tag + "/" + assetName,
				Size:               1024,
			},
			{
				Name:               "SHA256SUMS",
				BrowserDownloadURL: base + "/download/" + tag + "/SHA256SUMS",
				Size:               128,
			},
		},
	}
}

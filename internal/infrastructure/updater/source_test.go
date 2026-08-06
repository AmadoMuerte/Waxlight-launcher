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
	update, err := sourceForReleases(t, releases, assetName).Check(
		context.Background(),
		"0.1.4",
		"stable",
	)
	if err != nil {
		t.Fatal(err)
	}
	if !update.Available || update.Version != "0.1.5" || update.Prerelease {
		t.Fatalf("unexpected stable update: %+v", update)
	}
	if update.AssetName != assetName || update.SHA256 != strings.Repeat("a", 64) {
		t.Fatalf("unexpected verified asset: %+v", update)
	}
}

func TestSourceSelectsPrereleaseAheadOfStable(t *testing.T) {
	assetName, err := expectedAssetName("0.2.1-beta.3")
	if err != nil {
		t.Skip(err)
	}
	releases := []githubRelease{
		releaseFixture("v0.2.0", false, assetName),
		releaseFixture("v0.2.1-beta.3", true, assetName),
	}
	update, err := sourceForReleases(t, releases, assetName).Check(
		context.Background(),
		"0.2.0",
		"prerelease",
	)
	if err != nil {
		t.Fatal(err)
	}
	if !update.Available || update.Downgrade || !update.Prerelease || update.Version != "0.2.1-beta.3" {
		t.Fatalf("unexpected prerelease channel result: %+v", update)
	}
}

func TestPrereleaseChannelIgnoresNewerStable(t *testing.T) {
	assetName, err := expectedAssetName("0.2.1-beta.3")
	if err != nil {
		t.Skip(err)
	}
	releases := []githubRelease{
		releaseFixture("v0.2.1-beta.3", true, assetName),
		releaseFixture("v0.2.1", false, assetName),
	}
	update, err := sourceForReleases(t, releases, assetName).Check(
		context.Background(),
		"0.2.1-beta.3",
		"prerelease",
	)
	if err != nil {
		t.Fatal(err)
	}
	if update.Available || update.Version != "0.2.1-beta.3" {
		t.Fatalf("prerelease channel must stay on the newest prerelease, got: %+v", update)
	}
}

func TestPrereleaseChannelOffersNewestPrereleaseFromStable(t *testing.T) {
	assetName, err := expectedAssetName("0.2.1-beta.3")
	if err != nil {
		t.Skip(err)
	}
	releases := []githubRelease{
		releaseFixture("v0.2.0", false, assetName),
		releaseFixture("v0.2.1-beta.3", true, assetName),
	}
	update, err := sourceForReleases(t, releases, assetName).Check(
		context.Background(),
		"0.2.0",
		"prerelease",
	)
	if err != nil {
		t.Fatal(err)
	}
	if !update.Available || update.Downgrade || !update.Prerelease || update.Version != "0.2.1-beta.3" {
		t.Fatalf("expected newest prerelease after switching from stable, got: %+v", update)
	}
}

func TestPrereleaseChannelOffersDowngradeBelowInstalledStable(t *testing.T) {
	assetName, err := expectedAssetName("0.2.1-beta.3")
	if err != nil {
		t.Skip(err)
	}
	releases := []githubRelease{
		releaseFixture("v0.2.1-beta.3", true, assetName),
		releaseFixture("v0.2.2", false, assetName),
	}
	update, err := sourceForReleases(t, releases, assetName).Check(
		context.Background(),
		"0.2.2",
		"prerelease",
	)
	if err != nil {
		t.Fatal(err)
	}
	if !update.Available || !update.Downgrade || !update.Prerelease || update.Version != "0.2.1-beta.3" {
		t.Fatalf("expected prerelease downgrade below installed stable, got: %+v", update)
	}
}

func TestPrereleaseChannelRejectsStableOnlyReleases(t *testing.T) {
	assetName, err := expectedAssetName("0.2.0")
	if err != nil {
		t.Skip(err)
	}
	releases := []githubRelease{releaseFixture("v0.2.0", false, assetName)}
	_, err = sourceForReleases(t, releases, assetName).Check(
		context.Background(),
		"0.1.9",
		"prerelease",
	)
	if err == nil || !strings.Contains(err.Error(), "no trusted launcher release is available") {
		t.Fatalf("expected no trusted prerelease error, got %v", err)
	}
}

func TestSourceOffersDowngradeWhenSwitchingToStable(t *testing.T) {
	stableAsset, err := expectedAssetName("0.2.0")
	if err != nil {
		t.Skip(err)
	}
	releases := []githubRelease{
		releaseFixture("v0.2.1-beta.3", true, stableAsset),
		releaseFixture("v0.2.0", false, stableAsset),
	}
	update, err := sourceForReleases(t, releases, stableAsset).Check(
		context.Background(),
		"0.2.1-beta.3",
		"stable",
	)
	if err != nil {
		t.Fatal(err)
	}
	if !update.Available || !update.Downgrade || update.Prerelease || update.Version != "0.2.0" {
		t.Fatalf("expected stable channel downgrade, got: %+v", update)
	}
}

func TestSourceReconcilesToSelectedChannelEvenWithoutPrereleaseMarker(t *testing.T) {
	stableAsset, err := expectedAssetName("0.2.0")
	if err != nil {
		t.Skip(err)
	}
	releases := []githubRelease{
		releaseFixture("v0.2.0", false, stableAsset),
	}
	update, err := sourceForReleases(t, releases, stableAsset).Check(
		context.Background(),
		"0.2.1",
		"stable",
	)
	if err != nil {
		t.Fatal(err)
	}
	if !update.Available || !update.Downgrade || update.Version != "0.2.0" {
		t.Fatalf("expected channel reconciliation downgrade, got: %+v", update)
	}
}

func TestSourceDoesNotOfferUpdateWhenAlreadyOnChannelHead(t *testing.T) {
	assetName, err := expectedAssetName("0.2.0")
	if err != nil {
		t.Skip(err)
	}
	releases := []githubRelease{releaseFixture("v0.2.0", false, assetName)}
	update, err := sourceForReleases(t, releases, assetName).Check(
		context.Background(),
		"0.2.0",
		"stable",
	)
	if err != nil {
		t.Fatal(err)
	}
	if update.Available || update.Downgrade {
		t.Fatalf("expected no update available, got: %+v", update)
	}
}

func TestStableChannelRejectsSemverPrereleaseEvenWhenGitHubFlagIsWrong(t *testing.T) {
	assetName, err := expectedAssetName("0.2.0")
	if err != nil {
		t.Skip(err)
	}
	releases := []githubRelease{
		releaseFixture("v0.2.1-beta.3", false, assetName),
		releaseFixture("v0.2.0", false, assetName),
	}
	update, err := sourceForReleases(t, releases, assetName).Check(
		context.Background(),
		"0.1.9",
		"stable",
	)
	if err != nil {
		t.Fatal(err)
	}
	if update.Version != "0.2.0" || update.Prerelease {
		t.Fatalf("stable channel accepted a prerelease tag: %+v", update)
	}
}

func TestSourceRejectsInvalidChannel(t *testing.T) {
	assetName, err := expectedAssetName("0.2.0")
	if err != nil {
		t.Skip(err)
	}
	releases := []githubRelease{releaseFixture("v0.2.0", false, assetName)}
	_, err = sourceForReleases(t, releases, assetName).Check(
		context.Background(),
		"0.1.9",
		"nightly",
	)
	if err == nil || !strings.Contains(err.Error(), "invalid launcher update channel") {
		t.Fatalf("expected invalid channel error, got %v", err)
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

func sourceForReleases(t *testing.T, releases []githubRelease, assetName string) *Source {
	t.Helper()
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
	return NewSource(client)
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

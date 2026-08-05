package updater

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/waxlight/waxlight-launcher/internal/domain"
	"golang.org/x/mod/semver"
)

const (
	officialAPIURL       = "https://api.github.com/repos/AmadoMuerte/Waxlight-launcher/releases?per_page=20"
	officialRepository   = "AmadoMuerte/Waxlight-launcher"
	maximumAPIBytes      = 2 * 1024 * 1024
	maximumChecksumBytes = 1024 * 1024
)

type Source struct {
	client   *http.Client
	endpoint string
}

type githubRelease struct {
	TagName    string        `json:"tag_name"`
	HTMLURL    string        `json:"html_url"`
	Body       string        `json:"body"`
	Draft      bool          `json:"draft"`
	Prerelease bool          `json:"prerelease"`
	Assets     []githubAsset `json:"assets"`
}

type githubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

func NewSource(client *http.Client) *Source {
	if client == nil {
		client = NewHTTPClient()
	}
	return &Source{client: client, endpoint: officialAPIURL}
}

func NewHTTPClient() *http.Client {
	return &http.Client{
		Timeout: 30 * time.Minute,
		CheckRedirect: func(request *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return errors.New("too many update download redirects")
			}
			host := strings.ToLower(request.URL.Hostname())
			if request.URL.Scheme != "https" ||
				(host != "github.com" && !strings.HasSuffix(host, ".githubusercontent.com")) {
				return errors.New("update download redirected to an untrusted host")
			}
			return nil
		},
	}
}

func (source *Source) Check(
	ctx context.Context,
	currentVersion string,
	channel string,
) (domain.LauncherUpdate, error) {
	releases, err := source.releases(ctx)
	if err != nil {
		return domain.LauncherUpdate{}, err
	}
	current := canonicalVersion(currentVersion)
	if current == "" {
		return domain.LauncherUpdate{}, fmt.Errorf("invalid installed version %q", currentVersion)
	}

	var selected *githubRelease
	for index := range releases {
		release := &releases[index]
		version := canonicalVersion(release.TagName)
		if release.Draft || version == "" || (channel == "stable" && release.Prerelease) {
			continue
		}
		if selected == nil || semver.Compare(version, canonicalVersion(selected.TagName)) > 0 {
			selected = release
		}
	}
	if selected == nil {
		return domain.LauncherUpdate{}, errors.New("no trusted launcher release is available")
	}

	version := canonicalVersion(selected.TagName)
	newer := semver.Compare(version, current) > 0
	isDowngrade := channel == "stable" && semver.Prerelease(current) != "" && semver.Compare(version, current) < 0
	result := domain.LauncherUpdate{
		InstalledVersion: strings.TrimPrefix(current, "v"),
		Version:          strings.TrimPrefix(version, "v"),
		Available:        newer || isDowngrade,
		Downgrade:        isDowngrade && !newer,
		Prerelease:       selected.Prerelease,
		ReleaseNotes:     strings.TrimSpace(selected.Body),
		ReleasePageURL:   selected.HTMLURL,
	}
	if err := validateReleasePageURL(result.ReleasePageURL, selected.TagName); err != nil {
		return domain.LauncherUpdate{}, err
	}
	if !result.Available {
		return result, nil
	}

	assetName, err := expectedAssetName(result.Version)
	if err != nil {
		return domain.LauncherUpdate{}, err
	}
	asset, ok := findAsset(selected.Assets, assetName)
	if !ok || asset.Size <= 0 {
		return domain.LauncherUpdate{}, fmt.Errorf("release asset %q is unavailable", assetName)
	}
	if err := validateAssetURL(asset.BrowserDownloadURL, selected.TagName, assetName); err != nil {
		return domain.LauncherUpdate{}, err
	}
	checksumAsset, ok := findAsset(selected.Assets, "SHA256SUMS")
	if !ok {
		return domain.LauncherUpdate{}, errors.New("release checksum manifest is unavailable")
	}
	if err := validateAssetURL(checksumAsset.BrowserDownloadURL, selected.TagName, "SHA256SUMS"); err != nil {
		return domain.LauncherUpdate{}, err
	}
	checksum, err := source.checksum(ctx, checksumAsset.BrowserDownloadURL, assetName)
	if err != nil {
		return domain.LauncherUpdate{}, err
	}
	result.AssetName = assetName
	result.AssetSize = asset.Size
	result.DownloadURL = asset.BrowserDownloadURL
	result.SHA256 = checksum
	result.InstallationMode = string(DetectInstallationMode())
	return result, nil
}

func (source *Source) releases(ctx context.Context) ([]githubRelease, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, source.endpoint, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("User-Agent", "Waxlight-Launcher-Updater")
	response, err := source.client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub Releases returned HTTP %d", response.StatusCode)
	}
	var releases []githubRelease
	decoder := json.NewDecoder(io.LimitReader(response.Body, maximumAPIBytes))
	if err := decoder.Decode(&releases); err != nil {
		return nil, fmt.Errorf("decode GitHub Releases response: %w", err)
	}
	return releases, nil
}

func (source *Source) checksum(ctx context.Context, manifestURL, assetName string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, manifestURL, nil)
	if err != nil {
		return "", err
	}
	request.Header.Set("User-Agent", "Waxlight-Launcher-Updater")
	response, err := source.client.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("release checksums returned HTTP %d", response.StatusCode)
	}
	manifest, err := io.ReadAll(io.LimitReader(response.Body, maximumChecksumBytes))
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(manifest), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 || strings.TrimPrefix(fields[1], "*") != assetName {
			continue
		}
		checksum := strings.ToLower(fields[0])
		if isSHA256(checksum) {
			return checksum, nil
		}
	}
	return "", fmt.Errorf("SHA-256 checksum for %q is unavailable", assetName)
}

func canonicalVersion(value string) string {
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(value, "v") {
		value = "v" + value
	}
	if !semver.IsValid(value) {
		return ""
	}
	return semver.Canonical(value)
}

func expectedAssetName(version string) (string, error) {
	return assetNameForPlatform(version, runtime.GOOS, runtime.GOARCH, linuxUpdateFormat())
}

func assetNameForPlatform(version, platform, architecture, linuxFormat string) (string, error) {
	if architecture != "amd64" {
		return "", fmt.Errorf("launcher updates are unsupported on architecture %s", architecture)
	}
	switch platform {
	case "linux":
		switch linuxFormat {
		case "deb", "rpm":
			return fmt.Sprintf("Waxlight-Launcher-v%s-linux-amd64.%s", version, linuxFormat), nil
		default:
			return fmt.Sprintf("Waxlight-Launcher-v%s-linux-amd64.tar.gz", version), nil
		}
	case "windows":
		return fmt.Sprintf("Waxlight-Launcher-v%s-windows-amd64-installer.exe", version), nil
	default:
		return "", fmt.Errorf("launcher updates are unsupported on platform %s", platform)
	}
}

func linuxUpdateFormat() string {
	if runtime.GOOS != "linux" {
		return ""
	}
	executable, err := os.Executable()
	if err != nil {
		return "tar.gz"
	}
	executable, err = filepath.EvalSymlinks(executable)
	if err == nil && directoryWritable(filepath.Dir(executable)) {
		return "tar.gz"
	}
	if _, err := exec.LookPath("dpkg"); err == nil {
		return "deb"
	}
	if _, err := exec.LookPath("rpm"); err == nil {
		return "rpm"
	}
	return "tar.gz"
}

func findAsset(assets []githubAsset, name string) (githubAsset, bool) {
	for _, asset := range assets {
		if asset.Name == name {
			return asset, true
		}
	}
	return githubAsset{}, false
}

func validateReleasePageURL(value, tag string) error {
	return validateGitHubURL(value, fmt.Sprintf("/%s/releases/tag/%s", officialRepository, tag))
}

func validateAssetURL(value, tag, name string) error {
	return validateGitHubURL(
		value,
		fmt.Sprintf("/%s/releases/download/%s/%s", officialRepository, tag, name),
	)
}

func validateGitHubURL(value, expectedPath string) error {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme != "https" || !strings.EqualFold(parsed.Hostname(), "github.com") ||
		parsed.RawQuery != "" || parsed.Fragment != "" || parsed.Path != expectedPath {
		return fmt.Errorf("untrusted GitHub release URL")
	}
	return nil
}

func isSHA256(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, character := range value {
		if !strings.ContainsRune("0123456789abcdef", character) {
			return false
		}
	}
	return true
}

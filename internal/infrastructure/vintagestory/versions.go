package vintagestory

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"path"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/waxlight/waxlight-launcher/internal/domain"
)

const OfficialVersionCatalogURL = "https://api.vintagestory.at/stable-unstable.json"

type VersionCatalog struct {
	client       *http.Client
	endpoint     string
	platform     string
	architecture string

	cacheMu  sync.Mutex
	cachedAt time.Time
	cached   []domain.AvailableGameVersion
}

func NewVersionCatalog(client *http.Client) *VersionCatalog {
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	return &VersionCatalog{
		client:       client,
		endpoint:     OfficialVersionCatalogURL,
		platform:     runtime.GOOS,
		architecture: runtime.GOARCH,
	}
}

func NewVersionCatalogForPlatform(
	client *http.Client,
	endpoint string,
	platform string,
	architecture string,
) *VersionCatalog {
	catalog := NewVersionCatalog(client)
	catalog.endpoint = endpoint
	catalog.platform = platform
	catalog.architecture = architecture
	return catalog
}

type catalogFile struct {
	Filename string `json:"filename"`
	FileSize string `json:"filesize"`
	MD5      string `json:"md5"`
	URLs     struct {
		CDN string `json:"cdn"`
	} `json:"urls"`
	Latest int `json:"latest"`
}

func (catalog *VersionCatalog) List(
	ctx context.Context,
) ([]domain.AvailableGameVersion, error) {
	catalog.cacheMu.Lock()
	if len(catalog.cached) > 0 && time.Since(catalog.cachedAt) < 5*time.Minute {
		result := append([]domain.AvailableGameVersion(nil), catalog.cached...)
		catalog.cacheMu.Unlock()
		return result, nil
	}
	catalog.cacheMu.Unlock()

	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		catalog.endpoint,
		nil,
	)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "Waxlight-Launcher/0.1")

	response, err := catalog.client.Do(request)
	if err != nil {
		slog.Warn("game version catalog request failed", "error", err)
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		slog.Warn("game version catalog returned an error status", "status", response.StatusCode)
		return nil, fmt.Errorf(
			"version catalog returned HTTP %d",
			response.StatusCode,
		)
	}

	var payload map[string]map[string]catalogFile
	decoder := json.NewDecoder(io.LimitReader(response.Body, 8<<20))
	if err := decoder.Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode version catalog: %w", err)
	}

	distributionKey, architecture, ok := distributionFor(
		catalog.platform,
		catalog.architecture,
	)
	if !ok {
		return []domain.AvailableGameVersion{}, nil
	}

	result := make([]domain.AvailableGameVersion, 0, len(payload))
	for versionID, files := range payload {
		file, exists := files[distributionKey]
		if !exists {
			continue
		}
		release, err := parseRelease(
			versionID,
			file,
			catalog.platform,
			architecture,
		)
		if err != nil {
			continue
		}
		result = append(result, release)
	}

	sort.SliceStable(result, func(left, right int) bool {
		return compareVersions(result[left].ID, result[right].ID) > 0
	})

	catalog.cacheMu.Lock()
	catalog.cached = append([]domain.AvailableGameVersion(nil), result...)
	catalog.cachedAt = time.Now()
	catalog.cacheMu.Unlock()
	return result, nil
}

func distributionFor(platform string, architecture string) (string, string, bool) {
	switch {
	case platform == "linux" && architecture == "amd64":
		return "linux", "amd64", true
	case platform == "windows" && architecture == "amd64":
		return "windows", "amd64", true
	default:
		return "", "", false
	}
}

func parseRelease(
	versionID string,
	file catalogFile,
	platform string,
	architecture string,
) (domain.AvailableGameVersion, error) {
	versionID = strings.TrimSpace(versionID)
	if versionID == "" || strings.TrimSpace(file.Filename) == "" {
		return domain.AvailableGameVersion{}, fmt.Errorf("missing release identity")
	}
	if path.Base(file.Filename) != file.Filename ||
		strings.Contains(file.Filename, "\\") {
		return domain.AvailableGameVersion{}, fmt.Errorf("unsafe release filename")
	}

	downloadURL, err := url.Parse(file.URLs.CDN)
	if err != nil || downloadURL.Scheme != "https" ||
		downloadURL.Hostname() != "cdn.vintagestory.at" {
		return domain.AvailableGameVersion{}, fmt.Errorf("untrusted release URL")
	}
	if path.Base(downloadURL.Path) != file.Filename {
		return domain.AvailableGameVersion{}, fmt.Errorf("release filename does not match URL")
	}

	checksum := strings.ToLower(strings.TrimSpace(file.MD5))
	if len(checksum) != 32 {
		return domain.AvailableGameVersion{}, fmt.Errorf("invalid MD5 checksum")
	}
	for _, character := range checksum {
		if !strings.ContainsRune("0123456789abcdef", character) {
			return domain.AvailableGameVersion{}, fmt.Errorf("invalid MD5 checksum")
		}
	}

	channel := "unknown"
	switch {
	case strings.Contains(downloadURL.Path, "/stable/"):
		channel = "stable"
	case strings.Contains(downloadURL.Path, "/unstable/"):
		channel = "unstable"
	}

	return domain.AvailableGameVersion{
		ID:                versionID,
		Name:              versionID,
		Channel:           channel,
		Platform:          platform,
		Architecture:      architecture,
		Filename:          file.Filename,
		DownloadURL:       downloadURL.String(),
		DownloadSize:      parseFileSize(file.FileSize),
		Checksum:          checksum,
		ChecksumAlgorithm: "md5",
		Latest:            file.Latest == 1,
	}, nil
}

func parseFileSize(value string) int64 {
	parts := strings.Fields(strings.TrimSpace(value))
	if len(parts) != 2 {
		return 0
	}
	number, err := strconv.ParseFloat(parts[0], 64)
	if err != nil || number < 0 {
		return 0
	}
	multipliers := map[string]float64{
		"B":  1,
		"KB": 1_000,
		"MB": 1_000_000,
		"GB": 1_000_000_000,
	}
	multiplier, ok := multipliers[strings.ToUpper(parts[1])]
	if !ok {
		return 0
	}
	return int64(number * multiplier)
}

type versionPart struct {
	number *int
	text   string
}

func compareVersions(left string, right string) int {
	leftParts := splitVersion(left)
	rightParts := splitVersion(right)
	length := len(leftParts)
	if len(rightParts) > length {
		length = len(rightParts)
	}
	for index := 0; index < length; index++ {
		if index >= len(leftParts) {
			return trailingVersionOrder(rightParts[index:]) * -1
		}
		if index >= len(rightParts) {
			return trailingVersionOrder(leftParts[index:])
		}
		leftPart, rightPart := leftParts[index], rightParts[index]
		if leftPart.number != nil && rightPart.number != nil {
			if *leftPart.number > *rightPart.number {
				return 1
			}
			if *leftPart.number < *rightPart.number {
				return -1
			}
			continue
		}
		if leftPart.number != nil {
			return 1
		}
		if rightPart.number != nil {
			return -1
		}
		if leftPart.text != rightPart.text {
			if leftPart.text > rightPart.text {
				return 1
			}
			return -1
		}
	}
	return 0
}

func trailingVersionOrder(parts []versionPart) int {
	for _, part := range parts {
		if part.number != nil && *part.number != 0 {
			return 1
		}
		if part.text != "" {
			return -1
		}
	}
	return 0
}

func splitVersion(value string) []versionPart {
	value = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(value)), "v")
	parts := make([]versionPart, 0, 8)
	for index := 0; index < len(value); {
		if unicode.IsDigit(rune(value[index])) {
			end := index + 1
			for end < len(value) && unicode.IsDigit(rune(value[end])) {
				end++
			}
			number, _ := strconv.Atoi(value[index:end])
			parts = append(parts, versionPart{number: &number})
			index = end
			continue
		}
		if unicode.IsLetter(rune(value[index])) {
			end := index + 1
			for end < len(value) && unicode.IsLetter(rune(value[end])) {
				end++
			}
			parts = append(parts, versionPart{text: value[index:end]})
			index = end
			continue
		}
		index++
	}
	return parts
}

package modcatalog

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/waxlight/waxlight-launcher/internal/domain"
)

const (
	DefaultBaseURL  = "https://mods.vintagestory.at/api"
	maxReplyBytes   = 16 << 20
	catalogCacheTTL = 10 * time.Minute
	detailsCacheTTL = 30 * time.Minute
)

type Client struct {
	httpClient *http.Client
	baseURL    string
	mu         sync.RWMutex
	catalog    []domain.ModSummary
	catalogAt  time.Time
	details    map[string]cachedDetails
}

type cachedDetails struct {
	value     domain.ModDetails
	fetchedAt time.Time
}

func NewClient(httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	return &Client{
		httpClient: httpClient,
		baseURL:    DefaultBaseURL,
		details:    make(map[string]cachedDetails),
	}
}

func NewClientWithURL(httpClient *http.Client, baseURL string) *Client {
	client := NewClient(httpClient)
	client.baseURL = strings.TrimRight(baseURL, "/")
	return client
}

type modsResponse struct {
	StatusCode string       `json:"statuscode"`
	Mods       []apiSummary `json:"mods"`
}

type apiSummary struct {
	ModID        int64    `json:"modid"`
	AssetID      int64    `json:"assetid"`
	Downloads    int64    `json:"downloads"`
	Name         string   `json:"name"`
	Summary      string   `json:"summary"`
	ModIDStrings []string `json:"modidstrs"`
	Author       string   `json:"author"`
	URLAlias     *string  `json:"urlalias"`
	Side         string   `json:"side"`
	Type         string   `json:"type"`
	Logo         *string  `json:"logo"`
	Tags         []string `json:"tags"`
	LastReleased string   `json:"lastreleased"`
}

type modResponse struct {
	StatusCode string    `json:"statuscode"`
	Mod        apiDetail `json:"mod"`
}

type apiDetail struct {
	ModID         int64             `json:"modid"`
	AssetID       int64             `json:"assetid"`
	Name          string            `json:"name"`
	Text          string            `json:"text"`
	Author        string            `json:"author"`
	URLAlias      string            `json:"urlalias"`
	Logo          string            `json:"logofile"`
	HomepageURL   string            `json:"homepageurl"`
	SourceCodeURL string            `json:"sourcecodeurl"`
	Downloads     int64             `json:"downloads"`
	Side          string            `json:"side"`
	Type          string            `json:"type"`
	Created       string            `json:"created"`
	LastReleased  string            `json:"lastreleased"`
	Tags          []string          `json:"tags"`
	Releases      []apiRelease      `json:"releases"`
	Screenshots   []json.RawMessage `json:"screenshots"`
}

type apiRelease struct {
	ReleaseID  int64    `json:"releaseid"`
	MainFile   string   `json:"mainfile"`
	Filename   string   `json:"filename"`
	Tags       []string `json:"tags"`
	ModID      string   `json:"modidstr"`
	ModVersion string   `json:"modversion"`
	Created    string   `json:"created"`
	Changelog  string   `json:"changelog"`
}

func (client *Client) List(ctx context.Context) ([]domain.ModSummary, error) {
	items, err := client.list(ctx)
	if err != nil {
		return nil, err
	}
	return append([]domain.ModSummary(nil), items...), nil
}

func (client *Client) Search(
	ctx context.Context,
	query domain.ModSearchQuery,
) (domain.ModSearchResult, error) {
	items, err := client.list(ctx)
	if err != nil {
		return domain.ModSearchResult{}, err
	}

	filtered := make([]domain.ModSummary, 0, len(items))
	textQuery := strings.ToLower(strings.TrimSpace(query.Text))
	for _, item := range items {
		if textQuery != "" && !matchesText(item, textQuery) {
			continue
		}
		if query.Side != "" && query.Side != domain.ModSideUnknown && item.Side != query.Side {
			continue
		}
		if query.UpdatedAfter != nil && (item.UpdatedAt == nil || item.UpdatedAt.Before(*query.UpdatedAfter)) {
			continue
		}
		if len(query.Tags) > 0 && !containsAllFold(item.Tags, query.Tags) {
			continue
		}
		filtered = append(filtered, item)
	}

	sortMods(filtered, query.Sort, textQuery != "")
	if query.GameVersion != "" {
		filtered = client.enrichCompatible(ctx, filtered, query.GameVersion, query.PageSize)
	}

	page := query.Page
	if page < 1 {
		page = 1
	}
	pageSize := query.PageSize
	if pageSize < 1 || pageSize > 60 {
		pageSize = 24
	}
	total := len(filtered)
	start := (page - 1) * pageSize
	if start > total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	totalPages := (total + pageSize - 1) / pageSize
	return domain.ModSearchResult{
		Items:      append([]domain.ModSummary(nil), filtered[start:end]...),
		Page:       page,
		PageSize:   pageSize,
		TotalItems: total,
		TotalPages: totalPages,
		HasNext:    end < total,
	}, nil
}

func (client *Client) ListTags(ctx context.Context) ([]domain.ModTag, error) {
	items, err := client.list(ctx)
	if err != nil {
		return nil, err
	}
	byKey := make(map[string]*domain.ModTag, len(items))
	for _, item := range items {
		for _, tag := range item.Tags {
			key := strings.ToLower(tag)
			entry := byKey[key]
			if entry == nil {
				byKey[key] = &domain.ModTag{Name: tag, Count: 1}
				continue
			}
			entry.Count++
		}
	}
	tags := make([]domain.ModTag, 0, len(byKey))
	for _, tag := range byKey {
		tags = append(tags, *tag)
	}
	sort.Slice(tags, func(left, right int) bool {
		return strings.ToLower(tags[left].Name) < strings.ToLower(tags[right].Name)
	})
	return tags, nil
}

func (client *Client) list(ctx context.Context) ([]domain.ModSummary, error) {
	client.mu.RLock()
	if len(client.catalog) > 0 && time.Since(client.catalogAt) < catalogCacheTTL {
		items := append([]domain.ModSummary(nil), client.catalog...)
		client.mu.RUnlock()
		return items, nil
	}
	client.mu.RUnlock()

	var response modsResponse
	if err := client.getJSON(ctx, client.baseURL+"/mods", &response); err != nil {
		return nil, err
	}
	if response.StatusCode != "200" {
		return nil, domain.NewError(domain.ErrModCatalog, "The mod catalog is unavailable")
	}
	items := make([]domain.ModSummary, 0, len(response.Mods))
	for _, raw := range response.Mods {
		if raw.Type != "mod" || raw.ModID == 0 {
			continue
		}
		updated := parseDate(raw.LastReleased)
		slug := ""
		if raw.URLAlias != nil {
			slug = *raw.URLAlias
		}
		if slug == "" && len(raw.ModIDStrings) > 0 {
			slug = raw.ModIDStrings[0]
		}
		imageURL := ""
		if raw.Logo != nil {
			imageURL = *raw.Logo
		}
		items = append(items, domain.ModSummary{
			ID:           strconv.FormatInt(raw.ModID, 10),
			Slug:         slug,
			Name:         raw.Name,
			AuthorName:   raw.Author,
			Summary:      raw.Summary,
			ImageURL:     imageURL,
			Side:         normalizeSide(raw.Side),
			Downloads:    raw.Downloads,
			UpdatedAt:    updated,
			Tags:         nonEmpty(raw.Tags),
			ModIDStrings: nonEmpty(raw.ModIDStrings),
			GameVersions: []string{},
		})
	}
	client.mu.Lock()
	client.catalog = append([]domain.ModSummary(nil), items...)
	client.catalogAt = time.Now()
	client.mu.Unlock()
	return items, nil
}

func (client *Client) Get(ctx context.Context, id string) (domain.ModDetails, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return domain.ModDetails{}, domain.NewError(domain.ErrModNotFound, "Mod not found")
	}
	client.mu.RLock()
	if cached, ok := client.details[id]; ok && time.Since(cached.fetchedAt) < detailsCacheTTL {
		client.mu.RUnlock()
		return cached.value, nil
	}
	client.mu.RUnlock()

	var response modResponse
	endpoint := client.baseURL + "/mod/" + url.PathEscape(id)
	if err := client.getJSON(ctx, endpoint, &response); err != nil {
		return domain.ModDetails{}, err
	}
	if response.StatusCode != "200" || response.Mod.ModID == 0 {
		return domain.ModDetails{}, domain.NewError(domain.ErrModNotFound, "Mod not found")
	}
	details := mapDetails(response.Mod)
	client.mu.Lock()
	client.details[id] = cachedDetails{value: details, fetchedAt: time.Now()}
	client.details[details.ID] = cachedDetails{value: details, fetchedAt: time.Now()}
	if details.Slug != "" {
		client.details[details.Slug] = cachedDetails{value: details, fetchedAt: time.Now()}
	}
	client.mu.Unlock()
	return details, nil
}

func mapDetails(raw apiDetail) domain.ModDetails {
	versions := make([]domain.ModVersion, 0, len(raw.Releases))
	gameVersionSet := map[string]struct{}{}
	for _, release := range raw.Releases {
		for _, gameVersion := range release.Tags {
			gameVersionSet[gameVersion] = struct{}{}
		}
		versions = append(versions, domain.ModVersion{
			ID:           strconv.FormatInt(release.ReleaseID, 10),
			Version:      release.ModVersion,
			GameVersions: nonEmpty(release.Tags),
			ReleaseType:  releaseType(release.ModVersion),
			FileName:     release.Filename,
			DownloadURL:  release.MainFile,
			PublishedAt:  parseDate(release.Created),
			Changelog:    release.Changelog,
		})
	}
	gameVersions := make([]string, 0, len(gameVersionSet))
	for version := range gameVersionSet {
		gameVersions = append(gameVersions, version)
	}
	sort.Strings(gameVersions)
	latestVersion := ""
	if len(versions) > 0 {
		latestVersion = versions[0].Version
	}
	details := domain.ModDetails{
		ModSummary: domain.ModSummary{
			ID:            strconv.FormatInt(raw.ModID, 10),
			Slug:          raw.URLAlias,
			Name:          raw.Name,
			AuthorName:    raw.Author,
			ImageURL:      raw.Logo,
			Side:          normalizeSide(raw.Side),
			LatestVersion: latestVersion,
			GameVersions:  gameVersions,
			Downloads:     raw.Downloads,
			CreatedAt:     parseDate(raw.Created),
			UpdatedAt:     parseDate(raw.LastReleased),
			Tags:          nonEmpty(raw.Tags),
		},
		Description: raw.Text,
		Versions:    versions,
		WebsiteURL:  raw.HomepageURL,
		SourceURL:   raw.SourceCodeURL,
	}
	for _, screenshot := range raw.Screenshots {
		if parsed, ok := parseScreenshot(screenshot); ok {
			details.Screenshots = append(details.Screenshots, parsed)
		}
	}
	return details
}

func (client *Client) enrichCompatible(
	ctx context.Context,
	items []domain.ModSummary,
	gameVersion string,
	pageSize int,
) []domain.ModSummary {
	if pageSize < 1 {
		pageSize = 24
	}
	limit := pageSize * 6
	if limit > 180 {
		limit = 180
	}
	if limit > len(items) {
		limit = len(items)
	}
	type result struct {
		index   int
		details domain.ModDetails
	}
	jobs := make(chan int)
	results := make(chan result, limit)
	workers := 8
	if workers > limit {
		workers = limit
	}
	var wait sync.WaitGroup
	for range workers {
		wait.Add(1)
		go func() {
			defer wait.Done()
			for index := range jobs {
				details, err := client.Get(ctx, items[index].ID)
				if err == nil {
					results <- result{index: index, details: details}
				}
			}
		}()
	}
	go func() {
		for index := 0; index < limit; index++ {
			select {
			case jobs <- index:
			case <-ctx.Done():
				close(jobs)
				wait.Wait()
				close(results)
				return
			}
		}
		close(jobs)
		wait.Wait()
		close(results)
	}()
	byIndex := make(map[int]domain.ModDetails, limit)
	for result := range results {
		byIndex[result.index] = result.details
	}
	compatible := make([]domain.ModSummary, 0, limit)
	for index := 0; index < limit; index++ {
		details, ok := byIndex[index]
		if !ok || !supportsVersion(details.GameVersions, gameVersion) {
			continue
		}
		summary := details.ModSummary
		// The details endpoint has the full description, but no short summary.
		// Keep the summary returned by the catalog endpoint when enriching cards.
		summary.Summary = items[index].Summary
		compatible = append(compatible, summary)
	}
	return compatible
}

func (client *Client) getJSON(ctx context.Context, endpoint string, target any) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return domain.NewError(domain.ErrModCatalog, "Could not prepare the mod catalog request")
	}
	request.Header.Set("Accept", "application/json")
	response, err := client.httpClient.Do(request)
	if err != nil {
		slog.Warn("mod catalog request failed", "endpoint", endpoint, "error", err)
		return &domain.AppError{Code: domain.ErrModCatalog, Message: "Could not connect to the mod catalog", Retryable: true, Cause: err}
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound {
		return domain.NewError(domain.ErrModNotFound, "Mod not found")
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		slog.Warn("mod catalog returned an error status", "endpoint", endpoint, "status", response.StatusCode)
		return &domain.AppError{Code: domain.ErrModCatalog, Message: "The mod catalog is temporarily unavailable", Retryable: true}
	}
	decoder := json.NewDecoder(io.LimitReader(response.Body, maxReplyBytes))
	if err := decoder.Decode(target); err != nil {
		return &domain.AppError{Code: domain.ErrModCatalog, Message: "The mod catalog returned an invalid response", Cause: err}
	}
	return nil
}

func matchesText(item domain.ModSummary, query string) bool {
	values := []string{item.ID, item.Slug, item.Name, item.AuthorName, item.Summary, strings.Join(item.Tags, " ")}
	return strings.Contains(strings.ToLower(strings.Join(values, " ")), query)
}

func containsAllFold(values, required []string) bool {
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		set[strings.ToLower(value)] = struct{}{}
	}
	for _, value := range required {
		if _, ok := set[strings.ToLower(value)]; !ok {
			return false
		}
	}
	return true
}

func sortMods(items []domain.ModSummary, sortBy string, hasQuery bool) {
	sort.SliceStable(items, func(left, right int) bool {
		a, b := items[left], items[right]
		switch sortBy {
		case "downloads":
			return a.Downloads > b.Downloads
		case "name_asc":
			return strings.ToLower(a.Name) < strings.ToLower(b.Name)
		case "name_desc":
			return strings.ToLower(a.Name) > strings.ToLower(b.Name)
		case "newest", "updated":
			return dateAfter(a.UpdatedAt, b.UpdatedAt)
		default:
			if hasQuery {
				return false
			}
			return dateAfter(a.UpdatedAt, b.UpdatedAt)
		}
	})
}

func dateAfter(left, right *time.Time) bool {
	if left == nil {
		return false
	}
	if right == nil {
		return true
	}
	return left.After(*right)
}

func parseDate(value string) *time.Time {
	value = strings.TrimSpace(value)
	for _, layout := range []string{"2006-01-02 15:04:05", time.RFC3339, time.RFC3339Nano} {
		if parsed, err := time.ParseInLocation(layout, value, time.UTC); err == nil {
			return &parsed
		}
	}
	return nil
}

func normalizeSide(value string) domain.ModSide {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "client":
		return domain.ModSideClient
	case "server":
		return domain.ModSideServer
	case "both":
		return domain.ModSideBoth
	default:
		return domain.ModSideUnknown
	}
}

func nonEmpty(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			result = append(result, value)
		}
	}
	return result
}

func releaseType(version string) string {
	lower := strings.ToLower(version)
	if strings.Contains(lower, "alpha") {
		return "alpha"
	}
	if strings.Contains(lower, "beta") || strings.Contains(lower, "rc") || strings.Contains(lower, "pre") {
		return "beta"
	}
	return "stable"
}

func supportsVersion(versions []string, requested string) bool {
	requested = strings.TrimSpace(requested)
	series := strings.TrimSuffix(requested, ".x")
	for _, version := range versions {
		if version == requested || (series != "" && version == series) {
			return true
		}
		if series != "" && strings.HasPrefix(version, series+".") {
			return true
		}
	}
	return false
}

func parseScreenshot(raw json.RawMessage) (domain.ModScreenshot, bool) {
	var direct string
	if json.Unmarshal(raw, &direct) == nil && strings.HasPrefix(direct, "https://") {
		return domain.ModScreenshot{URL: direct}, true
	}
	var object map[string]any
	if json.Unmarshal(raw, &object) != nil {
		return domain.ModScreenshot{}, false
	}
	for _, key := range []string{"url", "file", "filename", "image"} {
		if value, ok := object[key].(string); ok && strings.HasPrefix(value, "https://") {
			caption, _ := object["caption"].(string)
			return domain.ModScreenshot{URL: value, Caption: caption}, true
		}
	}
	return domain.ModScreenshot{}, false
}

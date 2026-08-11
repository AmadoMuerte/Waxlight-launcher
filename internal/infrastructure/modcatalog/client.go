package modcatalog

import (
	"context"
	"errors"
	"net/http"

	"github.com/AmadoMuerte/vintagestory-go/moddb"
	"github.com/waxlight/waxlight-launcher/internal/domain"
	"github.com/waxlight/waxlight-launcher/internal/mods"
)

// Client adapts the generic ModDB API client to Waxlight's domain models and
// user-facing catalog errors. Local installation enrichment remains in application.
type Client struct {
	client *moddb.Client
}

func NewClient(httpClient *http.Client) *Client {
	return &Client{client: moddb.NewClient(httpClient)}
}

func NewClientWithURL(httpClient *http.Client, baseURL string) *Client {
	return &Client{client: moddb.NewClientWithURL(httpClient, baseURL)}
}

func (client *Client) List(ctx context.Context) ([]mods.ModSummary, error) {
	items, err := client.client.List(ctx)
	if err != nil {
		return nil, mapError(err)
	}
	result := make([]mods.ModSummary, 0, len(items))
	for _, item := range items {
		result = append(result, mapMod(item))
	}
	return result, nil
}

func (client *Client) Search(ctx context.Context, query mods.ModSearchQuery) (mods.ModSearchResult, error) {
	result, err := client.client.Search(ctx, moddb.SearchOptions{
		Text: query.Text, GameVersion: query.GameVersion, Side: mapSide(query.Side),
		UpdatedAfter: query.UpdatedAfter, Tags: query.Tags, Sort: query.Sort,
		Page: query.Page, PageSize: query.PageSize,
	})
	if err != nil {
		return mods.ModSearchResult{}, mapError(err)
	}
	items := make([]mods.ModSummary, 0, len(result.Items))
	for _, item := range result.Items {
		items = append(items, mapMod(item))
	}
	return mods.ModSearchResult{Items: items, Page: result.Page, PageSize: result.PageSize, TotalItems: result.TotalItems, TotalPages: result.TotalPages, HasNext: result.HasNext}, nil
}

func (client *Client) Get(ctx context.Context, id string) (mods.ModDetails, error) {
	details, err := client.client.Get(ctx, id)
	if err != nil {
		return mods.ModDetails{}, mapError(err)
	}
	return mapDetails(details), nil
}

func (client *Client) ListTags(ctx context.Context) ([]mods.ModTag, error) {
	tags, err := client.client.ListTags(ctx)
	if err != nil {
		return nil, mapError(err)
	}
	result := make([]mods.ModTag, 0, len(tags))
	for _, tag := range tags {
		result = append(result, mods.ModTag{Name: tag.Name, Count: tag.Count})
	}
	return result, nil
}

func mapMod(mod moddb.Mod) mods.ModSummary {
	return mods.ModSummary{ID: mod.ID, Slug: mod.Slug, Name: mod.Name, AuthorName: mod.AuthorName, Summary: mod.Summary, ImageURL: mod.ImageURL, LatestVersion: mod.LatestVersion, Side: mods.ModSide(mod.Side), GameVersions: mod.GameVersions, ModIDStrings: mod.ModIDStrings, Downloads: mod.Downloads, CreatedAt: mod.CreatedAt, UpdatedAt: mod.UpdatedAt, Tags: mod.Tags}
}

func mapDetails(mod moddb.ModDetails) mods.ModDetails {
	versions := make([]mods.ModVersion, 0, len(mod.Releases))
	for _, release := range mod.Releases {
		versions = append(versions, mods.ModVersion{ID: release.ID, Version: release.Version, GameVersions: release.GameVersions, ReleaseType: release.ReleaseType, FileName: release.FileName, FileSize: release.FileSize, DownloadURL: release.DownloadURL, Checksum: release.Checksum, PublishedAt: release.PublishedAt, Changelog: release.Changelog})
	}
	dependencies := make([]mods.ModDependency, 0, len(mod.Dependencies))
	for _, dependency := range mod.Dependencies {
		dependencies = append(dependencies, mods.ModDependency{ModID: dependency.ModID, Name: dependency.Name, Version: dependency.Version, Requirement: dependency.Requirement})
	}
	screenshots := make([]mods.ModScreenshot, 0, len(mod.Screenshots))
	for _, screenshot := range mod.Screenshots {
		screenshots = append(screenshots, mods.ModScreenshot{URL: screenshot.URL, Caption: screenshot.Caption})
	}
	return mods.ModDetails{ModSummary: mapMod(mod.Mod), Description: mod.Description, Screenshots: screenshots, Versions: versions, Dependencies: dependencies, WebsiteURL: mod.WebsiteURL, SourceURL: mod.SourceURL, License: mod.License}
}

func mapSide(side mods.ModSide) moddb.Side { return moddb.Side(side) }

func mapError(err error) error {
	switch {
	case errors.Is(err, moddb.ErrNotFound):
		return domain.NewError(mods.ErrModNotFound, "Mod not found")
	case errors.Is(err, moddb.ErrInvalidResponse):
		return &domain.AppError{Code: mods.ErrModCatalog, Message: "The mod catalog returned an invalid response", Cause: err}
	default:
		return &domain.AppError{Code: mods.ErrModCatalog, Message: "The mod catalog is temporarily unavailable", Retryable: true, Cause: err}
	}
}

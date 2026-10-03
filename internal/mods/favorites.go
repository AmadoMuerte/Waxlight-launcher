package mods

import (
	"context"
	"strings"

	"github.com/AmadoMuerte/Waxlight-launcher/internal/errs"
)

// ListFavoriteModIDs returns locally stored catalog IDs without contacting ModDB.
func (service *CatalogService) ListFavoriteModIDs(ctx context.Context) ([]string, error) {
	return service.favorites.ListFavoriteModIDs(ctx)
}

// ListFavoriteMods resolves saved IDs against one catalog listing. IDs missing
// from that listing remain stored so they can still be removed while offline.
func (service *CatalogService) ListFavoriteMods(ctx context.Context) ([]ModSummary, error) {
	ids, err := service.ListFavoriteModIDs(ctx)
	if err != nil || len(ids) == 0 {
		return []ModSummary{}, err
	}
	items, err := service.catalog.List(ctx)
	if err != nil {
		return []ModSummary{}, err
	}
	wanted := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		wanted[id] = struct{}{}
	}
	result := make([]ModSummary, 0, len(ids))
	for _, item := range items {
		if _, ok := wanted[item.ID]; ok {
			result = append(result, item)
		}
	}
	return service.enrichModSummaries(ctx, result), nil
}

// SetModFavorite explicitly adds or removes a locally stored catalog ID.
func (service *CatalogService) SetModFavorite(ctx context.Context, modID string, favorite bool) error {
	modID = strings.TrimSpace(modID)
	if modID == "" {
		return errs.NewError(errs.ErrValidation, "Mod ID must not be empty")
	}
	if favorite {
		details, err := service.catalog.Get(ctx, modID)
		if err != nil {
			return err
		}
		if strings.TrimSpace(details.ID) == "" {
			return errs.NewError(errs.ErrValidation, "Catalog returned an empty mod ID")
		}
		modID = details.ID
	}
	if err := service.gate.Begin(); err != nil {
		return err
	}
	defer service.gate.End()
	return service.favorites.SetModFavorite(ctx, modID, favorite)
}

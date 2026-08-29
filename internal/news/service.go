package news

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/AmadoMuerte/Waxlight-launcher/internal/errs"
)

const (
	maximumItems    = 50
	maximumStateIDs = 256
)

type Service struct {
	mu       sync.Mutex
	source   Source
	cache    Cache
	state    StateRepository
	cacheTTL time.Duration
	now      func() time.Time
}

func NewService(source Source, cache Cache, state StateRepository, cacheTTL time.Duration, now func() time.Time) *Service {
	return &Service{source: source, cache: cache, state: state, cacheTTL: cacheTTL, now: now}
}

func (service *Service) Sync(ctx context.Context, force bool) (Feed, error) {
	service.mu.Lock()
	defer service.mu.Unlock()

	now := service.now().UTC()
	cached, cacheErr := service.cache.Load()
	hasCache := cacheErr == nil && len(cached.Items) > 0
	if cacheErr != nil && !errors.Is(cacheErr, ErrCacheMiss) {
		slog.Warn("news cache could not be read", "error", cacheErr)
	}

	entry := cached
	refreshFailed := false
	cacheAge := now.Sub(cached.FetchedAt)
	if force || !hasCache || cacheAge < 0 || cacheAge >= service.cacheTTL {
		items, err := service.source.List(ctx)
		if err != nil {
			if !hasCache {
				errs.LogFailure("official news refresh failed", err)
				return Feed{}, &errs.AppError{Code: errs.ErrNewsUnavailable, Message: "Unable to load Vintage Story news", Cause: err}
			}
			refreshFailed = true
			errs.LogFailure("official news refresh failed; using cache", err)
		} else {
			if len(items) > maximumItems {
				items = items[:maximumItems]
			}
			entry = CacheEntry{Items: append([]Item(nil), items...), FetchedAt: now}
			if err := service.cache.Save(entry); err != nil {
				slog.Warn("news cache could not be saved", "error", err)
			}
		}
	}

	state, initialized, err := service.state.LoadNewsState(ctx)
	if err != nil {
		return Feed{}, err
	}
	currentIDs := itemIDs(entry.Items)
	if !initialized || len(state.KnownItemIDs) == 0 {
		state = State{KnownItemIDs: currentIDs, SeenItemIDs: currentIDs}
		if err := service.state.SaveNewsState(ctx, state); err != nil {
			return Feed{}, err
		}
		return Feed{Items: entry.Items, FetchedAt: entry.FetchedAt, RefreshFailed: refreshFailed}, nil
	}

	known := idSet(state.KnownItemIDs)
	newItems := make([]Item, 0)
	for _, item := range entry.Items {
		if _, exists := known[item.ID]; !exists {
			newItems = append(newItems, item)
		}
	}
	if len(newItems) > 0 {
		state.KnownItemIDs = mergeIDs(currentIDs, state.KnownItemIDs)
		if err := service.state.SaveNewsState(ctx, state); err != nil {
			return Feed{}, err
		}
	}
	seen := idSet(state.SeenItemIDs)
	unreadCount := 0
	for _, id := range currentIDs {
		if _, exists := seen[id]; !exists {
			unreadCount++
		}
	}
	return Feed{
		Items: entry.Items, NewItems: newItems, FetchedAt: entry.FetchedAt,
		UnreadCount: unreadCount, RefreshFailed: refreshFailed,
	}, nil
}

func (service *Service) MarkSeen(ctx context.Context, ids []string) error {
	service.mu.Lock()
	defer service.mu.Unlock()
	state, initialized, err := service.state.LoadNewsState(ctx)
	if err != nil {
		return err
	}
	if !initialized {
		state.KnownItemIDs = mergeIDs(ids, nil)
	}
	state.SeenItemIDs = mergeIDs(ids, state.SeenItemIDs)
	return service.state.SaveNewsState(ctx, state)
}

func (service *Service) IsOfficialArticleURL(value string) bool {
	return service.source.IsOfficialArticleURL(value)
}

func itemIDs(items []Item) []string {
	result := make([]string, 0, len(items))
	for _, item := range items {
		if item.ID != "" {
			result = append(result, item.ID)
		}
	}
	return result
}

func idSet(ids []string) map[string]struct{} {
	result := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		result[id] = struct{}{}
	}
	return result
}

func mergeIDs(current, previous []string) []string {
	result := make([]string, 0, min(len(current)+len(previous), maximumStateIDs))
	seen := make(map[string]struct{}, cap(result))
	for _, ids := range [][]string{current, previous} {
		for _, id := range ids {
			if id == "" {
				continue
			}
			if _, exists := seen[id]; exists {
				continue
			}
			seen[id] = struct{}{}
			result = append(result, id)
			if len(result) == maximumStateIDs {
				return result
			}
		}
	}
	return result
}

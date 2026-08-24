package newscache

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/AmadoMuerte/Waxlight-launcher/internal/news"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/platform/atomicfile"
)

type Cache struct {
	path string
}

func New(dataRoot string) *Cache {
	return &Cache{path: filepath.Join(dataRoot, "cache", "news.json")}
}

func (cache *Cache) Load() (news.CacheEntry, error) {
	data, err := os.ReadFile(cache.path)
	if errors.Is(err, os.ErrNotExist) {
		return news.CacheEntry{}, news.ErrCacheMiss
	}
	if err != nil {
		return news.CacheEntry{}, err
	}
	var entry news.CacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return news.CacheEntry{}, err
	}
	if entry.FetchedAt.IsZero() {
		return news.CacheEntry{}, errors.New("news cache has no fetch timestamp")
	}
	for _, item := range entry.Items {
		if item.ID == "" || item.Title == "" || item.URL == "" || item.PublishedAt.IsZero() {
			return news.CacheEntry{}, errors.New("news cache contains an invalid item")
		}
	}
	return entry, nil
}

func (cache *Cache) Save(entry news.CacheEntry) error {
	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	return atomicfile.Write(cache.path, data, 0o600)
}

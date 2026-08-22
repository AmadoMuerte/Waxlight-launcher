package newscache

import (
	"errors"
	"testing"
	"time"

	"github.com/waxlight/waxlight-launcher/internal/news"
)

func TestCacheRoundTrip(t *testing.T) {
	cache := New(t.TempDir())
	if _, err := cache.Load(); !errors.Is(err, news.ErrCacheMiss) {
		t.Fatalf("initial Load() error = %v", err)
	}
	want := news.CacheEntry{
		Items: []news.Item{{
			ID: "A", Title: "Post", URL: "https://www.vintagestory.at/blog.html/news/post/",
			PublishedAt: time.Now().UTC(),
		}},
		FetchedAt: time.Now().UTC(),
	}
	if err := cache.Save(want); err != nil {
		t.Fatal(err)
	}
	got, err := cache.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Items) != 1 || got.Items[0].ID != "A" || !got.FetchedAt.Equal(want.FetchedAt) {
		t.Fatalf("Load() = %+v", got)
	}
}

package news

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
)

type sourceStub struct {
	items []Item
	err   error
}

func (source *sourceStub) List(context.Context) ([]Item, error) { return source.items, source.err }
func (*sourceStub) IsOfficialArticleURL(value string) bool      { return value == "official" }

type cacheStub struct {
	entry CacheEntry
	err   error
	saved CacheEntry
}

func (cache *cacheStub) Load() (CacheEntry, error) { return cache.entry, cache.err }
func (cache *cacheStub) Save(entry CacheEntry) error {
	cache.entry = entry
	cache.saved = entry
	cache.err = nil
	return nil
}

type stateStub struct {
	state       State
	initialized bool
}

func (state *stateStub) LoadNewsState(context.Context) (State, bool, error) {
	return state.state, state.initialized, nil
}
func (state *stateStub) SaveNewsState(_ context.Context, value State) error {
	state.state = value
	state.initialized = true
	return nil
}

func newsItems(ids ...string) []Item {
	items := make([]Item, len(ids))
	for index, id := range ids {
		items[index] = Item{ID: id, Title: id}
	}
	return items
}

func TestInitialSyncEstablishesBaseline(t *testing.T) {
	state := &stateStub{}
	service := testService(&sourceStub{items: newsItems("A", "B", "C")}, &cacheStub{err: ErrCacheMiss}, state)
	feed, err := service.Sync(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	if len(feed.NewItems) != 0 || feed.UnreadCount != 0 || !reflect.DeepEqual(state.state.KnownItemIDs, []string{"A", "B", "C"}) || !reflect.DeepEqual(state.state.SeenItemIDs, []string{"A", "B", "C"}) {
		t.Fatalf("unexpected baseline: feed=%+v state=%+v", feed, state.state)
	}
}

func TestSyncDetectsNewItemsOnlyOnce(t *testing.T) {
	state := &stateStub{initialized: true, state: State{KnownItemIDs: []string{"A"}, SeenItemIDs: []string{"A"}}}
	source := &sourceStub{items: newsItems("D", "C", "B", "A")}
	cache := &cacheStub{err: ErrCacheMiss}
	service := testService(source, cache, state)
	feed, err := service.Sync(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(itemIDs(feed.NewItems), []string{"D", "C", "B"}) || feed.UnreadCount != 3 {
		t.Fatalf("unexpected new items: %+v", feed)
	}
	feed, err = service.Sync(context.Background(), true)
	if err != nil {
		t.Fatal(err)
	}
	if len(feed.NewItems) != 0 {
		t.Fatalf("repeat sync returned new items: %+v", feed.NewItems)
	}
}

func TestSyncDetectsOneNewItem(t *testing.T) {
	state := &stateStub{initialized: true, state: State{KnownItemIDs: []string{"A"}, SeenItemIDs: []string{"A"}}}
	service := testService(&sourceStub{items: newsItems("B", "A")}, &cacheStub{err: ErrCacheMiss}, state)
	feed, err := service.Sync(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(itemIDs(feed.NewItems), []string{"B"}) || feed.UnreadCount != 1 {
		t.Fatalf("unexpected new items: %+v", feed)
	}
}

func TestSyncUsesCachedItemsAfterNetworkError(t *testing.T) {
	now := time.Date(2026, 8, 23, 0, 0, 0, 0, time.UTC)
	cache := &cacheStub{entry: CacheEntry{Items: newsItems("A"), FetchedAt: now.Add(-2 * time.Hour)}}
	state := &stateStub{initialized: true, state: State{KnownItemIDs: []string{"A"}, SeenItemIDs: []string{"A"}}}
	service := NewService(&sourceStub{err: errors.New("offline")}, cache, state, time.Hour, func() time.Time { return now })
	feed, err := service.Sync(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	if !feed.RefreshFailed || len(feed.Items) != 1 {
		t.Fatalf("unexpected cached fallback: %+v", feed)
	}
}

func TestSyncRefreshesCacheWithFutureTimestamp(t *testing.T) {
	now := time.Date(2026, 8, 23, 0, 0, 0, 0, time.UTC)
	cache := &cacheStub{entry: CacheEntry{Items: newsItems("A"), FetchedAt: now.Add(time.Hour)}}
	state := &stateStub{initialized: true, state: State{KnownItemIDs: []string{"A"}, SeenItemIDs: []string{"A"}}}
	source := &sourceStub{items: newsItems("B", "A")}
	service := NewService(source, cache, state, time.Hour, func() time.Time { return now })
	feed, err := service.Sync(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	if len(feed.Items) != 2 || !cache.saved.FetchedAt.Equal(now) {
		t.Fatalf("future cache timestamp was not refreshed: feed=%+v cache=%+v", feed, cache.saved)
	}
}

func TestMarkSeenClearsCurrentUnreadSet(t *testing.T) {
	state := &stateStub{initialized: true, state: State{KnownItemIDs: []string{"B", "A"}, SeenItemIDs: []string{"A"}}}
	service := testService(&sourceStub{}, &cacheStub{}, state)
	if err := service.MarkSeen(context.Background(), []string{"B", "A"}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(state.state.SeenItemIDs[:2], []string{"B", "A"}) {
		t.Fatalf("unexpected seen IDs: %v", state.state.SeenItemIDs)
	}
}

func testService(source Source, cache Cache, state StateRepository) *Service {
	now := time.Date(2026, 8, 23, 0, 0, 0, 0, time.UTC)
	return NewService(source, cache, state, time.Hour, func() time.Time { return now })
}

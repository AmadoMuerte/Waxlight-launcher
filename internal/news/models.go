package news

import "time"

type Item struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	URL         string    `json:"url"`
	Summary     string    `json:"summary"`
	ImageURL    string    `json:"imageUrl,omitempty"`
	PublishedAt time.Time `json:"publishedAt"`
	Category    string    `json:"category"`
}

type CacheEntry struct {
	Items     []Item    `json:"items"`
	FetchedAt time.Time `json:"fetchedAt"`
}

type State struct {
	KnownItemIDs []string `json:"knownItemIds"`
	SeenItemIDs  []string `json:"seenItemIds"`
}

type Feed struct {
	Items         []Item
	NewItems      []Item
	FetchedAt     time.Time
	UnreadCount   int
	RefreshFailed bool
}

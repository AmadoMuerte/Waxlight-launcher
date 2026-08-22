package news

import (
	"context"
	"errors"
)

var ErrCacheMiss = errors.New("news cache not found")

type Source interface {
	List(context.Context) ([]Item, error)
	IsOfficialArticleURL(string) bool
}

type Cache interface {
	Load() (CacheEntry, error)
	Save(CacheEntry) error
}

type StateRepository interface {
	LoadNewsState(context.Context) (State, bool, error)
	SaveNewsState(context.Context, State) error
}

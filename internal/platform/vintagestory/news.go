package vintagestory

import (
	"context"
	"net/http"

	vsnews "github.com/AmadoMuerte/vintagestory-go/news"
	"github.com/waxlight/waxlight-launcher/internal/news"
)

type NewsSource struct {
	client *vsnews.Client
}

func NewNewsSource(httpClient *http.Client, userAgent string) *NewsSource {
	return &NewsSource{client: vsnews.NewClient(httpClient, userAgent)}
}

func (source *NewsSource) List(ctx context.Context) ([]news.Item, error) {
	items, err := source.client.List(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]news.Item, 0, len(items))
	for _, item := range items {
		result = append(result, news.Item{
			ID: item.ID, Title: item.Title, URL: item.URL, Summary: item.Summary,
			ImageURL: item.ImageURL, PublishedAt: item.PublishedAt, Category: string(item.Category),
		})
	}
	return result, nil
}

func (*NewsSource) IsOfficialArticleURL(value string) bool {
	return vsnews.IsOfficialArticleURL(value)
}

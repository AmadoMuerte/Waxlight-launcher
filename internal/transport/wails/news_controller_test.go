package wails

import (
	"context"
	"testing"
	"time"

	"github.com/waxlight/waxlight-launcher/internal/news"
)

type newsSourceStub struct{}

func (newsSourceStub) List(context.Context) ([]news.Item, error) { return nil, nil }
func (newsSourceStub) IsOfficialArticleURL(value string) bool {
	return value == "https://www.vintagestory.at/blog.html/news/post-r1/"
}

func TestNewsControllerRejectsUntrustedArticleURL(t *testing.T) {
	service := news.NewService(newsSourceStub{}, nil, nil, time.Hour, time.Now)
	controller := NewNewsController(service, newTestLifecycle())
	if err := controller.OpenArticle("https://example.com/post"); err == nil {
		t.Fatal("expected untrusted URL to be rejected")
	}
}

func TestNewsFeedDTO(t *testing.T) {
	publishedAt := time.Date(2026, 8, 23, 1, 2, 3, 0, time.UTC)
	dto := newsFeedDTO(news.Feed{
		Items:     []news.Item{{ID: "A", Title: "Post", PublishedAt: publishedAt}},
		FetchedAt: publishedAt, UnreadCount: 1, RefreshFailed: true,
	})
	if len(dto.Items) != 1 || dto.Items[0].PublishedAt != "2026-08-23T01:02:03Z" ||
		dto.UnreadCount != 1 || !dto.RefreshFailed {
		t.Fatalf("unexpected DTO: %+v", dto)
	}
}

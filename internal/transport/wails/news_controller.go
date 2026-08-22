package wails

import (
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"github.com/waxlight/waxlight-launcher/internal/errs"
	"github.com/waxlight/waxlight-launcher/internal/news"
)

type NewsItemDTO struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	URL         string `json:"url"`
	Summary     string `json:"summary"`
	ImageURL    string `json:"imageUrl,omitempty"`
	PublishedAt string `json:"publishedAt"`
	Category    string `json:"category"`
}

type NewsFeedDTO struct {
	Items         []NewsItemDTO `json:"items"`
	NewItems      []NewsItemDTO `json:"newItems"`
	FetchedAt     string        `json:"fetchedAt"`
	UnreadCount   int           `json:"unreadCount"`
	RefreshFailed bool          `json:"refreshFailed"`
}

type NewsController struct {
	service   *news.Service
	lifecycle lifecycle
}

func NewNewsController(service *news.Service, lifecycle lifecycle) *NewsController {
	return &NewsController{service: service, lifecycle: lifecycle}
}

func (controller *NewsController) Sync(force bool) (NewsFeedDTO, error) {
	feed, err := controller.service.Sync(controller.lifecycle.Context(), force)
	return newsFeedDTO(feed), err
}

func (controller *NewsController) MarkSeen(ids []string) error {
	return controller.service.MarkSeen(controller.lifecycle.Context(), ids)
}

func (controller *NewsController) OpenArticle(rawURL string) error {
	if !controller.service.IsOfficialArticleURL(rawURL) {
		return errs.NewError(errs.ErrInvalidURL, "Only official Vintage Story news links can be opened")
	}
	runtime.BrowserOpenURL(controller.lifecycle.Context(), rawURL)
	return nil
}

func newsFeedDTO(feed news.Feed) NewsFeedDTO {
	return NewsFeedDTO{
		Items: newsItemDTOs(feed.Items), NewItems: newsItemDTOs(feed.NewItems),
		FetchedAt:   feed.FetchedAt.UTC().Format(time.RFC3339Nano),
		UnreadCount: feed.UnreadCount, RefreshFailed: feed.RefreshFailed,
	}
}

func newsItemDTOs(items []news.Item) []NewsItemDTO {
	result := make([]NewsItemDTO, 0, len(items))
	for _, item := range items {
		result = append(result, NewsItemDTO{
			ID: item.ID, Title: item.Title, URL: item.URL, Summary: item.Summary,
			ImageURL: item.ImageURL, PublishedAt: item.PublishedAt.UTC().Format(time.RFC3339Nano), Category: item.Category,
		})
	}
	return result
}

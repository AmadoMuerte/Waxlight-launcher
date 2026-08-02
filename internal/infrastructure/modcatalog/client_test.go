package modcatalog

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/waxlight/waxlight-launcher/internal/domain"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func jsonResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}

func TestClientMapsFiltersAndSortsCatalog(t *testing.T) {
	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path != "/mods" {
			t.Fatalf("unexpected path %s", request.URL.Path)
		}
		return jsonResponse(`{"statuscode":"200","mods":[
			{"modid":1,"downloads":20,"name":"Server Tools","summary":"Admin helpers","author":"Ada","side":"server","type":"mod","tags":["Utility"],"lastreleased":"2026-08-01 12:00:00"},
			{"modid":2,"downloads":200,"name":"Warm Light","summary":"Soft lamps","author":"Bea","side":"client","type":"mod","tags":["Graphics"],"lastreleased":"2026-08-02 12:00:00"},
			{"modid":3,"downloads":999,"name":"External","summary":"skip","author":"Cat","side":"both","type":"externaltool","tags":[],"lastreleased":"2026-08-03 12:00:00"}
		]}`), nil
	})
	client := NewClientWithURL(&http.Client{Transport: transport}, "https://catalog.test")
	result, err := client.Search(context.Background(), domain.ModSearchQuery{
		Text: "ada", Side: domain.ModSideServer, Sort: "downloads", Page: 1, PageSize: 24,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.TotalItems != 1 || result.Items[0].ID != "1" {
		t.Fatalf("unexpected search result: %#v", result)
	}
}

func TestClientMapsDetailsWithoutComments(t *testing.T) {
	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return jsonResponse(`{"statuscode":"200","mod":{
			"modid":51,"name":"Player Corpse","text":"<p>Safe description</p>","author":"Ada","side":"both","type":"mod","downloads":42,
			"created":"2026-01-01 10:00:00","lastreleased":"2026-08-01 10:00:00","tags":["Utility"],"comments":[{"text":"must not map"}],
			"releases":[{"releaseid":7,"mainfile":"https://cdn.test/mod.zip","filename":"mod.zip","tags":["1.21.0"],"modversion":"2.0.0","created":"2026-08-01 10:00:00","changelog":"<p>New</p>"}],
			"screenshots":["https://cdn.test/screen.png"]
		}}`), nil
	})
	client := NewClientWithURL(&http.Client{Transport: transport}, "https://catalog.test")
	details, err := client.Get(context.Background(), "51")
	if err != nil {
		t.Fatal(err)
	}
	if details.LatestVersion != "2.0.0" || len(details.Versions) != 1 || len(details.Screenshots) != 1 {
		t.Fatalf("unexpected details: %#v", details)
	}
	if details.Versions[0].DownloadURL != "https://cdn.test/mod.zip" {
		t.Fatal("download URL was not mapped")
	}
}

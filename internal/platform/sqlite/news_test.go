package sqlite_test

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/AmadoMuerte/Waxlight-launcher/internal/news"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/platform/sqlite"
)

func TestNewsStateRoundTrip(t *testing.T) {
	store, err := sqlite.Open(filepath.Join(t.TempDir(), "news.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if _, initialized, err := store.LoadNewsState(context.Background()); err != nil || initialized {
		t.Fatalf("initial LoadNewsState() initialized=%v err=%v", initialized, err)
	}
	want := news.State{KnownItemIDs: []string{"B", "A"}, SeenItemIDs: []string{"A"}}
	if err := store.SaveNewsState(context.Background(), want); err != nil {
		t.Fatal(err)
	}
	got, initialized, err := store.LoadNewsState(context.Background())
	if err != nil || !initialized || !reflect.DeepEqual(got, want) {
		t.Fatalf("LoadNewsState() = %+v, %v, %v", got, initialized, err)
	}
}

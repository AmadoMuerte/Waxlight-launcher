package wails

import (
	"context"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/AmadoMuerte/Waxlight-launcher/internal/instances"
)

type coverQueries struct {
	instance instances.Instance
}

func (queries coverQueries) List(context.Context) ([]instances.Instance, error) {
	return []instances.Instance{queries.instance}, nil
}

func (queries coverQueries) Get(context.Context, string) (instances.Instance, error) {
	return queries.instance, nil
}

func TestInstanceCoverHandlerServesOnlyImageInsideInstance(t *testing.T) {
	directory := t.TempDir()
	coverPath := filepath.Join(directory, "cover.png")
	file, err := os.Create(coverPath)
	if err != nil {
		t.Fatal(err)
	}
	if err = png.Encode(file, image.NewRGBA(image.Rect(0, 0, 1, 1))); err != nil {
		t.Fatal(err)
	}
	if err = file.Close(); err != nil {
		t.Fatal(err)
	}
	handler := NewInstanceCoverHandler(coverQueries{instance: instances.Instance{
		ID: "instance", Directory: directory, CoverPath: &coverPath,
	}})

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/instance-covers/instance", nil))
	if response.Code != http.StatusOK || response.Header().Get("Content-Type") != "image/png" {
		t.Fatalf("response = %d %q", response.Code, response.Header().Get("Content-Type"))
	}

	handler = NewInstanceCoverHandler(coverQueries{instance: instances.Instance{
		ID: "instance", Directory: t.TempDir(), CoverPath: &coverPath,
	}})
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/instance-covers/instance", nil))
	if response.Code != http.StatusNotFound {
		t.Fatalf("outside response = %d", response.Code)
	}
}

package instances

import (
	"context"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestUpdateServiceSetCoverCopiesAndReplacesManagedImage(t *testing.T) {
	directory := t.TempDir()
	oldPath := filepath.Join(directory, ".waxlight-cover-old")
	if err := os.WriteFile(oldPath, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	sourcePath := filepath.Join(t.TempDir(), "cover.png")
	source, err := os.Create(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	coverImage := image.NewRGBA(image.Rect(0, 0, 1, 1))
	coverImage.Set(0, 0, color.RGBA{R: 255, A: 255})
	if err = png.Encode(source, coverImage); err != nil {
		t.Fatal(err)
	}
	if err = source.Close(); err != nil {
		t.Fatal(err)
	}

	var calls []string
	repository := &mutationRepository{
		instance: Instance{
			ID: "instance", Name: "Instance", GameVersionID: "1.20", Directory: directory,
			CoverPath: &oldPath, CreatedAt: time.Now(),
		},
		calls: &calls,
	}
	service := NewUpdateService(repository, versionReader{}, &testGate{}, &testLock{}, nil, nil, nil, time.Now)
	updated, err := service.UpdateWithCover(context.Background(), repository.instance, &sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	if updated.CoverPath == nil || *updated.CoverPath == sourcePath {
		t.Fatalf("cover path = %v", updated.CoverPath)
	}
	if _, err = os.Stat(*updated.CoverPath); err != nil {
		t.Fatalf("copied cover: %v", err)
	}
	if _, err = os.Stat(oldPath); !os.IsNotExist(err) {
		t.Fatalf("old cover still exists: %v", err)
	}
}

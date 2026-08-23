package instances

import (
	"context"
	"errors"
	"testing"
	"time"
)

type migrationStorageFake struct {
	copyErr error
}

func (migrationStorageFake) Discover() []string { return nil }
func (migrationStorageFake) Inspect(path string) (MigrationCandidate, error) {
	return MigrationCandidate{Path: path, TotalBytes: 10}, nil
}
func (migrationStorageFake) ValidateTarget(string, string) error { return nil }
func (storage migrationStorageFake) Copy(context.Context, string, string, func(MigrationCopyProgress)) (MigrationCopyResult, error) {
	return MigrationCopyResult{}, storage.copyErr
}

type migrationSpaceFake int64

func (space migrationSpaceFake) Available(string) (int64, error) { return int64(space), nil }

type migrationCreatorFake struct{ directory string }

func (creator migrationCreatorFake) CreatePrepared(ctx context.Context, _ CreateInput, prepare func(context.Context, string) error) (Instance, error) {
	if err := prepare(ctx, creator.directory); err != nil {
		return Instance{}, err
	}
	return Instance{ID: "created", Directory: creator.directory}, nil
}

func TestMigrationStopsBeforeCreatingInstanceAfterCopyFailure(t *testing.T) {
	for _, copyErr := range []error{errors.New("copy failed"), context.Canceled} {
		directory := t.TempDir()
		service := NewMigrationService(migrationStorageFake{copyErr: copyErr}, migrationSpaceFake(1<<30),
			migrationCreatorFake{directory}, nil, nil, t.TempDir(), time.Now, func() string { return "id" })
		_, err := service.importData(context.Background(), MigrationImportRequest{SourcePath: "source", GameVersionID: "version"},
			MigrationCandidate{Path: "source", TotalBytes: 10}, func(string) {}, nil)
		if !errors.Is(err, copyErr) {
			t.Fatalf("expected copy error, got %v", err)
		}
	}
}

func TestMigrationReconcilesModsBestEffort(t *testing.T) {
	directory := t.TempDir()
	reconciled := false
	service := NewMigrationService(migrationStorageFake{}, migrationSpaceFake(1<<30), migrationCreatorFake{directory},
		nil, func(context.Context, string) []string { reconciled = true; return []string{"unmatched"} },
		t.TempDir(), time.Now, func() string { return "id" })
	instance, err := service.importData(context.Background(), MigrationImportRequest{SourcePath: "source", GameVersionID: "version"},
		MigrationCandidate{Path: "source", TotalBytes: 10}, func(string) {}, nil)
	if err != nil || instance.ID != "created" || !reconciled {
		t.Fatalf("unexpected import: %#v err=%v reconciled=%v", instance, err, reconciled)
	}
}

func TestMigrationRejectsInsufficientDiskSpaceBeforeCreation(t *testing.T) {
	directory := t.TempDir()
	service := NewMigrationService(migrationStorageFake{}, migrationSpaceFake(64<<20), migrationCreatorFake{directory},
		nil, nil, t.TempDir(), time.Now, func() string { return "id" })
	_, err := service.importData(context.Background(), MigrationImportRequest{SourcePath: "source", GameVersionID: "version"},
		MigrationCandidate{Path: "source", TotalBytes: 1}, func(string) {}, nil)
	if err == nil {
		t.Fatal("insufficient disk space was accepted")
	}
}

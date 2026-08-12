//go:build linux

package updater

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestReadPortableBinaryRejectsLinksAndExtractsExecutable(t *testing.T) {
	archivePath := filepath.Join(t.TempDir(), "update.tar.gz")
	file, err := os.Create(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	gzipWriter := gzip.NewWriter(file)
	tarWriter := tar.NewWriter(gzipWriter)
	binary := []byte("verified launcher")
	if err := tarWriter.WriteHeader(&tar.Header{
		Name:     "Waxlight-Launcher-v0.1.5-linux-amd64/waxlight",
		Mode:     0o755,
		Size:     int64(len(binary)),
		Typeflag: tar.TypeReg,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := tarWriter.Write(binary); err != nil {
		t.Fatal(err)
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	extracted, err := readPortableBinary(context.Background(), archivePath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(extracted, binary) {
		t.Fatalf("unexpected extracted executable %q", extracted)
	}
}

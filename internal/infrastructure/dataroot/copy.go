package dataroot

import (
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/otiai10/copy"
)

// reservedNames are the launcher's own files that always stay in the home
// directory and are never copied with the data root.
var reservedNames = map[string]bool{
	pointerFile:  true,
	pendingFile:  true,
	previousFile: true,
	errorFile:    true,
	databaseName: true,
	databaseWAL:  true,
	databaseSHM:  true,
}

// CopyData copies the contents of src into dst, excluding the launcher's
// reserved files, and reports byte progress through progress(copied, total).
// The destination directory is created if missing.
func CopyData(src, dst string, progress func(copied, total int64)) error {
	total, err := TotalSize(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dst, 0o700); err != nil {
		return err
	}
	counter := &progressCounter{
		onUpdate: func(copied int64) {
			if progress != nil {
				progress(copied, total)
			}
		},
	}
	err = copy.Copy(src, dst, copy.Options{
		OnSymlink: func(string) copy.SymlinkAction { return copy.Shallow },
		Skip: func(info os.FileInfo, sourcePath, _ string) (bool, error) {
			if info.IsDir() {
				return false, nil
			}
			rel, err := filepath.Rel(src, sourcePath)
			if err != nil {
				return false, nil
			}
			depth := strings.Count(filepath.ToSlash(rel), "/")
			return depth == 0 && reservedNames[info.Name()], nil
		},
		WrapReader: func(reader io.Reader) io.Reader {
			return &countingReader{reader: reader, counter: counter}
		},
		Sync:         true,
		NumOfWorkers: int64(runtime.NumCPU()),
	})
	return err
}

// TotalSize returns the number of bytes that CopyData would copy, excluding
// the launcher's reserved files at the top level.
func TotalSize(root string) (int64, error) {
	var total int64
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		depth := strings.Count(filepath.ToSlash(rel), "/")
		if depth == 0 && reservedNames[entry.Name()] {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.Type().IsRegular() {
			info, err := entry.Info()
			if err != nil {
				return err
			}
			total += info.Size()
		}
		return nil
	})
	return total, err
}

type progressCounter struct {
	mu       sync.Mutex
	copied   int64
	onUpdate func(copied int64)
}

func (counter *progressCounter) add(n int64) {
	counter.mu.Lock()
	counter.copied += n
	copied := counter.copied
	counter.mu.Unlock()
	if counter.onUpdate != nil {
		counter.onUpdate(copied)
	}
}

type countingReader struct {
	reader  io.Reader
	counter *progressCounter
}

func (reader *countingReader) Read(p []byte) (int, error) {
	n, err := reader.reader.Read(p)
	if n > 0 {
		reader.counter.add(int64(n))
	}
	return n, err
}

// copyFile copies a single file with durable sync.
func copyFile(source, destination string) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := os.OpenFile(destination, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	if _, err := io.Copy(output, input); err != nil {
		_ = output.Close()
		return err
	}
	if err := output.Sync(); err != nil {
		_ = output.Close()
		return err
	}
	return output.Close()
}

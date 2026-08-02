package downloader

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/waxlight/waxlight-launcher/internal/application"
)

type HTTPDownloader struct{ Client *http.Client }

func NewHTTPDownloader() *HTTPDownloader {
	return &HTTPDownloader{Client: &http.Client{Timeout: 30 * time.Minute}}
}

func (d *HTTPDownloader) Download(ctx context.Context, in application.DownloadRequest, progress chan<- application.DownloadProgress) error {
	if !strings.HasPrefix(in.URL, "https://") {
		return fmt.Errorf("only HTTPS downloads are allowed")
	}
	if err := os.MkdirAll(filepath.Dir(in.DestinationPath), 0o755); err != nil {
		return err
	}
	partial := in.DestinationPath + ".partial"
	var offset int64
	flags := os.O_CREATE | os.O_WRONLY | os.O_TRUNC
	if in.Resume {
		if info, err := os.Stat(partial); err == nil {
			offset = info.Size()
			flags = os.O_CREATE | os.O_WRONLY | os.O_APPEND
		}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, in.URL, nil)
	if err != nil {
		return err
	}
	if offset > 0 {
		req.Header.Set("Range", "bytes="+strconv.FormatInt(offset, 10)+"-")
	}
	resp, err := d.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("download returned HTTP %d", resp.StatusCode)
	}
	if offset > 0 && resp.StatusCode != http.StatusPartialContent {
		offset = 0
		flags = os.O_CREATE | os.O_WRONLY | os.O_TRUNC
	}
	out, err := os.OpenFile(partial, flags, 0o644)
	if err != nil {
		return err
	}
	total := resp.ContentLength
	if total > 0 {
		total += offset
	}
	started, downloaded := time.Now(), offset
	buf := make([]byte, 128*1024)
	for {
		if err = ctx.Err(); err != nil {
			out.Close()
			return err
		}
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, err = out.Write(buf[:n]); err != nil {
				out.Close()
				return err
			}
			downloaded += int64(n)
			elapsed := time.Since(started).Seconds()
			var speed int64
			if elapsed > 0 {
				speed = int64(float64(downloaded-offset) / elapsed)
			}
			if progress != nil {
				select {
				case progress <- application.DownloadProgress{DownloadedBytes: downloaded, TotalBytes: total, BytesPerSecond: speed}:
				default:
				}
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			out.Close()
			return readErr
		}
	}
	if err = out.Close(); err != nil {
		return err
	}
	if in.ExpectedSHA256 != "" {
		actual, err := sha256File(partial)
		if err != nil {
			return err
		}
		if !strings.EqualFold(actual, in.ExpectedSHA256) {
			return fmt.Errorf("checksum mismatch")
		}
	}
	return os.Rename(partial, in.DestinationPath)
}

func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err = io.Copy(h, f); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

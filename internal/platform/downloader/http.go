package downloader

import (
	"context"
	"crypto/md5"
	"crypto/sha256"
	"fmt"
	"hash"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/AmadoMuerte/Waxlight-launcher/internal/downloads"
)

type HTTPDownloader struct{ Client *http.Client }

func NewHTTPDownloader() *HTTPDownloader {
	return &HTTPDownloader{Client: &http.Client{Timeout: 30 * time.Minute}}
}

func (d *HTTPDownloader) Download(ctx context.Context, in downloads.Request, progress chan<- downloads.Progress) error {
	if !strings.HasPrefix(in.URL, "https://") {
		return fmt.Errorf("only HTTPS downloads are allowed")
	}
	// Some catalog entries (for example Vintage Story ModDB) embed file names
	// with raw spaces in the query string. Written verbatim into the request
	// line, such URLs are rejected by servers with HTTP 400.
	normalizedURL, err := normalizeDownloadURL(in.URL)
	if err != nil {
		return fmt.Errorf("invalid download URL: %w", err)
	}
	in.URL = normalizedURL
	fileName := filepath.Base(in.DestinationPath)
	slog.Info("download started", "file", fileName, "resume", in.Resume)
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
		slog.Warn("download rejected by server", "file", fileName, "status", resp.StatusCode)
		return fmt.Errorf("download returned HTTP %d", resp.StatusCode)
	}
	if in.MaxBytes > 0 && resp.ContentLength > in.MaxBytes {
		return fmt.Errorf("download exceeds the maximum allowed size")
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
			if in.MaxBytes > 0 && downloaded+int64(n) > in.MaxBytes {
				out.Close()
				_ = os.Remove(partial)
				return fmt.Errorf("download exceeds the maximum allowed size")
			}
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
				case progress <- downloads.Progress{DownloadedBytes: downloaded, TotalBytes: total, BytesPerSecond: speed}:
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
	if in.ExpectedChecksum != "" {
		actual, err := checksumFile(partial, in.ChecksumAlgorithm)
		if err != nil {
			return err
		}
		if !strings.EqualFold(actual, in.ExpectedChecksum) {
			slog.Warn("download checksum mismatch", "file", fileName)
			return fmt.Errorf(
				"checksum mismatch: expected %s, got %s",
				in.ExpectedChecksum,
				actual,
			)
		}
	}
	slog.Info("download completed", "file", fileName, "bytes", downloaded)
	return os.Rename(partial, in.DestinationPath)
}

func (d *HTTPDownloader) ContentLength(ctx context.Context, rawURL string) (int64, error) {
	if !strings.HasPrefix(rawURL, "https://") {
		return 0, fmt.Errorf("only HTTPS URLs are allowed")
	}
	normalizedURL, err := normalizeDownloadURL(rawURL)
	if err != nil {
		return 0, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, normalizedURL, nil)
	if err != nil {
		return 0, err
	}
	resp, err := d.Client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return 0, fmt.Errorf("HEAD returned HTTP %d", resp.StatusCode)
	}
	return resp.ContentLength, nil
}

func normalizeDownloadURL(raw string) (string, error) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	parsed.RawQuery = parsed.Query().Encode()
	return parsed.String(), nil
}

func checksumFile(path string, algorithm string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	var h hash.Hash
	switch strings.ToLower(strings.TrimSpace(algorithm)) {
	case "md5":
		h = md5.New()
	case "sha256", "sha-256", "":
		h = sha256.New()
	default:
		return "", fmt.Errorf("unsupported checksum algorithm %q", algorithm)
	}
	if _, err = io.Copy(h, f); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

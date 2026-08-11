// Package downloads defines transport-neutral download contracts.
package downloads

import "context"

type Request struct {
	URL               string
	DestinationPath   string
	ExpectedChecksum  string
	ChecksumAlgorithm string
	Resume            bool
	MaxBytes          int64
}

type Progress struct {
	DownloadedBytes int64
	TotalBytes      int64
	BytesPerSecond  int64
}

type Downloader interface {
	Download(context.Context, Request, chan<- Progress) error
	ContentLength(context.Context, string) (int64, error)
}

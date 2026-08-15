package wails

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type InstanceCoverHandler struct {
	queries instanceQueries
}

func NewInstanceCoverHandler(queries instanceQueries) *InstanceCoverHandler {
	return &InstanceCoverHandler{queries: queries}
}

func (handler *InstanceCoverHandler) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	id := strings.TrimPrefix(request.URL.Path, "/instance-covers/")
	if id == request.URL.Path || id == "" || strings.Contains(id, "/") {
		http.NotFound(response, request)
		return
	}
	instance, err := handler.queries.Get(request.Context(), id)
	if err != nil || instance.CoverPath == nil || !insideDirectory(instance.Directory, *instance.CoverPath) {
		http.NotFound(response, request)
		return
	}
	pathInfo, err := os.Lstat(*instance.CoverPath)
	if err != nil || !pathInfo.Mode().IsRegular() {
		http.NotFound(response, request)
		return
	}
	file, err := os.Open(*instance.CoverPath)
	if err != nil {
		http.NotFound(response, request)
		return
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > maxCoverResponseSize {
		http.NotFound(response, request)
		return
	}
	prefix := make([]byte, 512)
	read, _ := io.ReadFull(file, prefix)
	contentType := http.DetectContentType(prefix[:read])
	if contentType != "image/png" && contentType != "image/jpeg" && contentType != "image/gif" {
		http.NotFound(response, request)
		return
	}
	_, _ = file.Seek(0, io.SeekStart)
	response.Header().Set("Content-Type", contentType)
	response.Header().Set("X-Content-Type-Options", "nosniff")
	response.Header().Set("Cache-Control", "private, max-age=31536000, immutable")
	http.ServeContent(response, request, filepath.Base(*instance.CoverPath), info.ModTime(), file)
}

const maxCoverResponseSize = 8 << 20

func insideDirectory(directory, path string) bool {
	relative, err := filepath.Rel(directory, path)
	return err == nil && relative != ".." && !filepath.IsAbs(relative) && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

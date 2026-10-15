package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"mime"
	"path/filepath"
	"strings"
	"time"
)

type progressFn func(progress int, message string)

const (
	maxArchiveAnalyzeBytes  int64 = 1 << 40 // 1 TiB
	maxArchiveAnalyzeEntries      = 10000
)

type archiveAnalyzeLimits struct {
	bytesRead int64
}

type archiveByteLimiter struct {
	lim *archiveAnalyzeLimits
	r   io.Reader
}

func (l *archiveByteLimiter) Read(p []byte) (int, error) {
	n, err := l.r.Read(p)
	if n > 0 {
		l.lim.bytesRead += int64(n)
		if l.lim.bytesRead > maxArchiveAnalyzeBytes {
			return n, fmt.Errorf("archive exceeds %d byte analysis limit", maxArchiveAnalyzeBytes)
		}
	}
	return n, err
}

func (lim *archiveAnalyzeLimits) streamSHA256(r io.Reader) (string, int64, error) {
	h := sha256.New()
	n, err := io.Copy(h, &archiveByteLimiter{lim: lim, r: r})
	if err != nil {
		return "", n, err
	}
	return hex.EncodeToString(h.Sum(nil)), n, nil
}

func addArchiveEntry(files map[string]fileInfo, name string, info fileInfo) error {
	if len(files) >= maxArchiveAnalyzeEntries {
		return fmt.Errorf("archive exceeds %d entry analysis limit", maxArchiveAnalyzeEntries)
	}
	files[name] = info
	return nil
}

func mimeForPath(name string) string {
	ext := filepath.Ext(name)
	if ext == "" {
		return "application/octet-stream"
	}
	if t := mime.TypeByExtension(ext); t != "" {
		return t
	}
	return "application/octet-stream"
}

func formatModTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

func finalizeArchive(files map[string]fileInfo, format archiveFormat) analysisOutput {
	var totalSize, totalCompressed int64
	for _, f := range files {
		if f.IsDir {
			continue
		}
		totalSize += f.Size
		compressed := f.SizeCompressed
		if compressed == 0 {
			compressed = f.Size
		}
		totalCompressed += compressed
	}

	archive := archiveInfo{
		Kind:        format.Kind,
		Description: format.Description,
		FileCount:   len(files),
		TotalSize:   totalSize,
	}
	if totalSize > 0 {
		archive.Ratio = float64(totalCompressed) / float64(totalSize)
	}
	return analysisOutput{Files: files, Archive: archive}
}

func stripCompressedExt(name string) string {
	lower := strings.ToLower(name)
	for _, suffix := range []string{
		".tar.zst", ".tzst", ".tar.lz4", ".tar.bz2", ".tbz2", ".tbz",
		".tar.xz", ".tar.gz", ".tgz", ".txz", ".tar",
		".zip", ".7z", ".iso", ".img", ".rar",
		".zst", ".lz4", ".bz2", ".gz", ".xz",
	} {
		if strings.HasSuffix(lower, suffix) {
			return name[:len(name)-len(suffix)]
		}
	}
	return name
}

func memberName(archivePath, headerName, fallbackExt string) string {
	if headerName != "" {
		return filepath.ToSlash(headerName)
	}
	base := stripCompressedExt(filepath.Base(archivePath))
	if base == "" {
		base = "content"
	}
	if fallbackExt != "" && !strings.HasSuffix(strings.ToLower(base), fallbackExt) {
		return base + fallbackExt
	}
	return base
}

package main

import (
	"fmt"
	"path/filepath"

	"github.com/bodgit/sevenzip"
)

func analyze7z(path string, format archiveFormat, progress progressFn, lim *archiveAnalyzeLimits) (analysisOutput, error) {
	r, err := sevenzip.OpenReader(path)
	if err != nil {
		return analysisOutput{}, err
	}
	defer r.Close()

	files := make(map[string]fileInfo, len(r.File))
	for i, f := range r.File {
		name := filepath.ToSlash(f.Name)
		info := fileInfo{
			Size:     int64(f.UncompressedSize),
			MimeType: mimeForPath(name),
			ModTime:  formatModTime(f.Modified),
			IsDir:    f.FileInfo().IsDir(),
		}
		if !info.IsDir && f.UncompressedSize > 0 {
			rc, err := f.Open()
			if err != nil {
				return analysisOutput{}, fmt.Errorf("%s: %w", name, err)
			}
			sum, _, err := lim.streamSHA256(rc)
			rc.Close()
			if err != nil {
				return analysisOutput{}, fmt.Errorf("%s: %w", name, err)
			}
			info.SHA256 = sum
		}
		if err := addArchiveEntry(files, name, info); err != nil {
			return analysisOutput{}, err
		}
		if len(r.File) > 0 {
			p := 20 + (i+1)*70/len(r.File)
			progress(p, "reading "+name)
		}
	}
	return finalizeArchive(files, format), nil
}

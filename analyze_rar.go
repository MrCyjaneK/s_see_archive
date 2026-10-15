package main

import (
	"fmt"
	"io"
	"path/filepath"

	"github.com/nwaples/rardecode/v2"
)

func analyzeRar(path string, format archiveFormat, progress progressFn, lim *archiveAnalyzeLimits) (analysisOutput, error) {
	rc, err := rardecode.OpenReader(path)
	if err != nil {
		return analysisOutput{}, err
	}
	defer rc.Close()

	files := map[string]fileInfo{}
	i := 0
	for {
		hdr, err := rc.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return analysisOutput{}, err
		}

		name := filepath.ToSlash(hdr.Name)
		info := fileInfo{
			Size:           hdr.UnPackedSize,
			SizeCompressed: hdr.PackedSize,
			MimeType:       mimeForPath(name),
			ModTime:        formatModTime(hdr.ModificationTime),
			IsDir:          hdr.IsDir,
		}
		if !hdr.IsDir {
			sum, size, err := lim.streamSHA256(rc)
			if err != nil {
				return analysisOutput{}, fmt.Errorf("%s: %w", name, err)
			}
			info.SHA256 = sum
			if info.Size <= 0 {
				info.Size = size
			}
		}
		if err := addArchiveEntry(files, name, info); err != nil {
			return analysisOutput{}, err
		}
		i++
		progress(20+i%70, "reading "+name)
	}
	return finalizeArchive(files, format), nil
}

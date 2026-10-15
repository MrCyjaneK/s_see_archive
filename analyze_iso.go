package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/kdomanski/iso9660"
)

type offsetReaderAt struct {
	r io.ReaderAt
	o int64
}

func (o offsetReaderAt) ReadAt(p []byte, off int64) (int, error) {
	return o.r.ReadAt(p, off+o.o)
}

func analyzeISO(path string, format archiveFormat, progress progressFn, lim *archiveAnalyzeLimits) (analysisOutput, error) {
	f, err := os.Open(path)
	if err != nil {
		return analysisOutput{}, err
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return analysisOutput{}, err
	}

	base, err := findISO9660Base(f, fi.Size())
	if err != nil {
		return analysisOutput{}, err
	}

	image, err := iso9660.OpenImage(offsetReaderAt{r: f, o: base})
	if err != nil {
		return analysisOutput{}, err
	}
	root, err := image.RootDir()
	if err != nil {
		return analysisOutput{}, err
	}

	files := map[string]fileInfo{}
	if err := walkISO(root, "", files, progress, lim); err != nil {
		return analysisOutput{}, err
	}
	return finalizeArchive(files, format), nil
}

func findISO9660Base(ra io.ReaderAt, size int64) (int64, error) {
	sig := make([]byte, 6)
	for _, base := range []int64{0, 32768, 2048, 65536, 512} {
		off := base + 16*2048
		if off+6 > size {
			continue
		}
		if _, err := ra.ReadAt(sig, off); err != nil {
			continue
		}
		if sig[0] == 1 && string(sig[1:6]) == "CD001" {
			return base, nil
		}
	}
	return 0, fmt.Errorf("no ISO9660 filesystem found in image")
}

func walkISO(dir *iso9660.File, prefix string, files map[string]fileInfo, progress progressFn, lim *archiveAnalyzeLimits) error {
	children, err := dir.GetChildren()
	if err != nil {
		return err
	}
	for _, child := range children {
		name := child.Name()
		path := name
		if prefix != "" {
			path = prefix + "/" + name
		}
		path = filepath.ToSlash(path)

		if child.IsDir() {
			if err := addArchiveEntry(files, path, fileInfo{
				IsDir:    true,
				MimeType: "application/x-directory",
				ModTime:  formatModTime(child.ModTime()),
			}); err != nil {
				return err
			}
			if err := walkISO(child, path, files, progress, lim); err != nil {
				return err
			}
			continue
		}

		info := fileInfo{
			Size:     child.Size(),
			MimeType: mimeForPath(path),
			ModTime:  formatModTime(child.ModTime()),
		}
		reader := child.Reader()
		if reader != nil {
			sum, size, err := lim.streamSHA256(reader)
			if err != nil {
				return fmt.Errorf("%s: %w", path, err)
			}
			info.SHA256 = sum
			if info.Size == 0 {
				info.Size = size
			}
		}
		if err := addArchiveEntry(files, path, info); err != nil {
			return err
		}
		progress(30, "reading "+path)
	}
	return nil
}

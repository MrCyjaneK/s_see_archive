package main

import (
	"archive/tar"
	"archive/zip"
	"compress/bzip2"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/klauspost/compress/zstd"
	"github.com/pierrec/lz4/v4"
	"github.com/ulikunitz/xz"
)

func analyzeArchive(path string, progress progressFn) (analysisOutput, error) {
	lim := &archiveAnalyzeLimits{}
	f, err := os.Open(path)
	if err != nil {
		return analysisOutput{}, err
	}
	defer f.Close()

	head, err := readHead(f)
	if err != nil {
		return analysisOutput{}, err
	}
	format := detectFormat(path, head)

	progress(10, "detected "+format.Kind)

	switch format.Kind {
	case "zip":
		return analyzeZip(path, format, progress, lim)
	case "tar":
		return analyzeTar(f, format, progress, lim)
	case "tar.gz":
		return analyzeTarGzip(f, format, progress, lim)
	case "tar.xz":
		return analyzeTarXz(f, format, progress, lim)
	case "tar.bz2":
		return analyzeTarBzip2(f, format, progress, lim)
	case "tar.zst":
		return analyzeTarZstd(f, format, progress, lim)
	case "tar.lz4":
		return analyzeTarLz4(f, format, progress, lim)
	case "gzip":
		return analyzeGzip(f, path, format, progress, lim)
	case "xz":
		return analyzeXz(f, path, format, progress, lim)
	case "bzip2":
		return analyzeBzip2(f, path, format, progress, lim)
	case "zstd":
		return analyzeZstd(f, path, format, progress, lim)
	case "lz4":
		return analyzeLz4(f, path, format, progress, lim)
	case "7z":
		return analyze7z(path, format, progress, lim)
	case "iso", "img":
		return analyzeISO(path, format, progress, lim)
	case "rar":
		return analyzeRar(path, format, progress, lim)
	default:
		return analysisOutput{}, fmt.Errorf("unsupported archive format: %s", format.Kind)
	}
}

func analyzeZip(path string, format archiveFormat, progress progressFn, lim *archiveAnalyzeLimits) (analysisOutput, error) {
	f, err := os.Open(path)
	if err != nil {
		return analysisOutput{}, err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return analysisOutput{}, err
	}
	zr, err := zip.NewReader(f, fi.Size())
	if err != nil {
		return analysisOutput{}, err
	}

	files := make(map[string]fileInfo, len(zr.File))
	for i, zf := range zr.File {
		name := filepath.ToSlash(zf.Name)
		info := fileInfo{
			Size:           int64(zf.UncompressedSize64),
			SizeCompressed: int64(zf.CompressedSize64),
			MimeType:       mimeForPath(name),
			ModTime:        formatModTime(zf.Modified),
			IsDir:          zf.FileInfo().IsDir(),
		}
		if !info.IsDir {
			rc, err := zf.Open()
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
		if len(zr.File) > 0 {
			p := 20 + (i+1)*70/len(zr.File)
			progress(p, "reading "+name)
		}
	}
	return finalizeArchive(files, format), nil
}

func analyzeTar(r io.ReadSeeker, format archiveFormat, progress progressFn, lim *archiveAnalyzeLimits) (analysisOutput, error) {
	return analyzeTarReader(r, format, progress, lim)
}

func analyzeTarGzip(r io.ReadSeeker, format archiveFormat, progress progressFn, lim *archiveAnalyzeLimits) (analysisOutput, error) {
	return analyzeTarCompressed(r, format, func(r io.Reader) (io.Reader, error) {
		return gzip.NewReader(r)
	}, progress, lim)
}

func analyzeTarXz(r io.ReadSeeker, format archiveFormat, progress progressFn, lim *archiveAnalyzeLimits) (analysisOutput, error) {
	return analyzeTarCompressed(r, format, func(r io.Reader) (io.Reader, error) {
		return xz.NewReader(r)
	}, progress, lim)
}

func analyzeTarBzip2(r io.ReadSeeker, format archiveFormat, progress progressFn, lim *archiveAnalyzeLimits) (analysisOutput, error) {
	return analyzeTarCompressed(r, format, func(r io.Reader) (io.Reader, error) {
		return bzip2.NewReader(r), nil
	}, progress, lim)
}

func analyzeTarZstd(r io.ReadSeeker, format archiveFormat, progress progressFn, lim *archiveAnalyzeLimits) (analysisOutput, error) {
	return analyzeTarCompressed(r, format, func(r io.Reader) (io.Reader, error) {
		return zstd.NewReader(r)
	}, progress, lim)
}

func analyzeTarLz4(r io.ReadSeeker, format archiveFormat, progress progressFn, lim *archiveAnalyzeLimits) (analysisOutput, error) {
	return analyzeTarCompressed(r, format, func(r io.Reader) (io.Reader, error) {
		return lz4.NewReader(r), nil
	}, progress, lim)
}

func analyzeTarCompressed(r io.ReadSeeker, format archiveFormat, open func(io.Reader) (io.Reader, error), progress progressFn, lim *archiveAnalyzeLimits) (analysisOutput, error) {
	if _, err := r.Seek(0, io.SeekStart); err != nil {
		return analysisOutput{}, err
	}
	inner, err := open(r)
	if err != nil {
		return analysisOutput{}, err
	}
	if c, ok := inner.(io.Closer); ok {
		defer c.Close()
	}
	return analyzeTarReader(inner, format, progress, lim)
}

func analyzeTarReader(r io.Reader, format archiveFormat, progress progressFn, lim *archiveAnalyzeLimits) (analysisOutput, error) {
	tr := tar.NewReader(r)
	files := map[string]fileInfo{}
	i := 0
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return analysisOutput{}, err
		}
		name := filepath.ToSlash(hdr.Name)
		info := fileInfo{
			Size:     hdr.Size,
			MimeType: mimeForPath(name),
			ModTime:  formatModTime(hdr.ModTime),
			IsDir:    hdr.Typeflag == tar.TypeDir,
		}
		if !info.IsDir && hdr.Size > 0 {
			sum, _, err := lim.streamSHA256(tr)
			if err != nil {
				return analysisOutput{}, fmt.Errorf("%s: %w", name, err)
			}
			info.SHA256 = sum
		}
		if err := addArchiveEntry(files, name, info); err != nil {
			return analysisOutput{}, err
		}
		i++
		progress(20+i%70, "reading "+name)
	}
	return finalizeArchive(files, format), nil
}

func analyzeGzip(r io.ReadSeeker, path string, format archiveFormat, progress progressFn, lim *archiveAnalyzeLimits) (analysisOutput, error) {
	if _, err := r.Seek(0, io.SeekStart); err != nil {
		return analysisOutput{}, err
	}
	gr, err := gzip.NewReader(r)
	if err != nil {
		return analysisOutput{}, err
	}
	defer gr.Close()

	name := memberName(path, gr.Name, "")
	progress(40, "reading "+name)
	sum, size, err := lim.streamSHA256(gr)
	if err != nil {
		return analysisOutput{}, err
	}

	files := map[string]fileInfo{
		name: {
			Size:     size,
			MimeType: mimeForPath(name),
			SHA256:   sum,
			ModTime:  formatModTime(gr.ModTime),
		},
	}
	return finalizeArchive(files, format), nil
}

func analyzeXz(r io.ReadSeeker, path string, format archiveFormat, progress progressFn, lim *archiveAnalyzeLimits) (analysisOutput, error) {
	return analyzeSingleCompressed(r, path, format, func(r io.Reader) (io.Reader, error) {
		return xz.NewReader(r)
	}, progress, lim)
}

func analyzeBzip2(r io.ReadSeeker, path string, format archiveFormat, progress progressFn, lim *archiveAnalyzeLimits) (analysisOutput, error) {
	return analyzeSingleCompressed(r, path, format, func(r io.Reader) (io.Reader, error) {
		return bzip2.NewReader(r), nil
	}, progress, lim)
}

func analyzeZstd(r io.ReadSeeker, path string, format archiveFormat, progress progressFn, lim *archiveAnalyzeLimits) (analysisOutput, error) {
	return analyzeSingleCompressed(r, path, format, func(r io.Reader) (io.Reader, error) {
		return zstd.NewReader(r)
	}, progress, lim)
}

func analyzeLz4(r io.ReadSeeker, path string, format archiveFormat, progress progressFn, lim *archiveAnalyzeLimits) (analysisOutput, error) {
	return analyzeSingleCompressed(r, path, format, func(r io.Reader) (io.Reader, error) {
		return lz4.NewReader(r), nil
	}, progress, lim)
}

func analyzeSingleCompressed(r io.ReadSeeker, path string, format archiveFormat, open func(io.Reader) (io.Reader, error), progress progressFn, lim *archiveAnalyzeLimits) (analysisOutput, error) {
	if _, err := r.Seek(0, io.SeekStart); err != nil {
		return analysisOutput{}, err
	}
	inner, err := open(r)
	if err != nil {
		return analysisOutput{}, err
	}
	if c, ok := inner.(io.Closer); ok {
		defer c.Close()
	}

	name := memberName(path, "", "")
	progress(40, "reading "+name)
	sum, size, err := lim.streamSHA256(inner)
	if err != nil {
		return analysisOutput{}, err
	}

	files := map[string]fileInfo{
		name: {
			Size:     size,
			MimeType: mimeForPath(name),
			SHA256:   sum,
		},
	}
	return finalizeArchive(files, format), nil
}

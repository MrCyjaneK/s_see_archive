package main

import (
	"bytes"
	"io"
	"path/filepath"
	"strings"
)

type archiveFormat struct {
	Kind        string
	Description string
}

func detectFormat(path string, head []byte) archiveFormat {
	if kind, desc, ok := formatFromName(path); ok {
		return archiveFormat{Kind: kind, Description: desc}
	}
	return formatFromMagic(head)
}

func formatFromName(path string) (kind, description string, ok bool) {
	lower := strings.ToLower(filepath.Base(path))
	switch {
	case strings.HasSuffix(lower, ".tar.zst"), strings.HasSuffix(lower, ".tzst"):
		return "tar.zst", "POSIX tar archive compressed with Zstandard", true
	case strings.HasSuffix(lower, ".tar.lz4"):
		return "tar.lz4", "POSIX tar archive compressed with LZ4", true
	case strings.HasSuffix(lower, ".tar.bz2"), strings.HasSuffix(lower, ".tbz2"), strings.HasSuffix(lower, ".tbz"):
		return "tar.bz2", "POSIX tar archive compressed with bzip2", true
	case strings.HasSuffix(lower, ".tar.xz"), strings.HasSuffix(lower, ".txz"):
		return "tar.xz", "POSIX tar archive compressed with XZ", true
	case strings.HasSuffix(lower, ".tar.gz"), strings.HasSuffix(lower, ".tgz"):
		return "tar.gz", "POSIX tar archive compressed with gzip", true
	case strings.HasSuffix(lower, ".tar"):
		return "tar", "POSIX tar archive", true
	case strings.HasSuffix(lower, ".zip"):
		return "zip", "ZIP archive", true
	case strings.HasSuffix(lower, ".7z"):
		return "7z", "7-Zip archive", true
	case strings.HasSuffix(lower, ".iso"):
		return "iso", "ISO 9660 disc image", true
	case strings.HasSuffix(lower, ".img"):
		return "img", "disc image (ISO 9660)", true
	case strings.HasSuffix(lower, ".rar"):
		return "rar", "RAR archive", true
	case strings.HasSuffix(lower, ".zst"):
		return "zstd", "Zstandard-compressed data", true
	case strings.HasSuffix(lower, ".lz4"):
		return "lz4", "LZ4-compressed data", true
	case strings.HasSuffix(lower, ".bz2"):
		return "bzip2", "bzip2-compressed data", true
	case strings.HasSuffix(lower, ".gz"):
		return "gzip", "gzip-compressed data", true
	case strings.HasSuffix(lower, ".xz"):
		return "xz", "XZ-compressed data", true
	default:
		return "", "", false
	}
}

func formatFromMagic(head []byte) archiveFormat {
	switch {
	case len(head) >= 6 && bytes.Equal(head[:6], []byte{0x37, 0x7a, 0xbc, 0xaf, 0x27, 0x1c}):
		return archiveFormat{Kind: "7z", Description: "7-Zip archive"}
	case len(head) >= 7 && bytes.Equal(head[:6], []byte("Rar!\x1a\x07")):
		return archiveFormat{Kind: "rar", Description: "RAR archive"}
	case len(head) >= 4 && head[0] == 'P' && head[1] == 'K' && (head[2] == 0x03 || head[2] == 0x05 || head[2] == 0x07):
		return archiveFormat{Kind: "zip", Description: "ZIP archive"}
	case len(head) >= 263 && bytes.Equal(head[257:262], []byte("ustar")):
		return archiveFormat{Kind: "tar", Description: "POSIX tar archive"}
	case len(head) >= 3 && head[0] == 'B' && head[1] == 'Z':
		return archiveFormat{Kind: "bzip2", Description: "bzip2-compressed data"}
	case len(head) >= 4 && bytes.Equal(head[:4], []byte{0x28, 0xb5, 0x2f, 0xfd}):
		return archiveFormat{Kind: "zstd", Description: "Zstandard-compressed data"}
	case len(head) >= 4 && bytes.Equal(head[:4], []byte{0x04, 0x22, 0x4d, 0x18}):
		return archiveFormat{Kind: "lz4", Description: "LZ4 frame-compressed data"}
	case len(head) >= 2 && head[0] == 0x1f && head[1] == 0x8b:
		return archiveFormat{Kind: "gzip", Description: "gzip-compressed data"}
	case len(head) >= 6 && bytes.Equal(head[:6], []byte{0xfd, '7', 'z', 'X', 'Z', 0x00}):
		return archiveFormat{Kind: "xz", Description: "XZ-compressed data"}
	case isoMagicAt(head, 0) || isoMagicAt(head, 32768):
		return archiveFormat{Kind: "iso", Description: "ISO 9660 disc image"}
	default:
		return archiveFormat{Kind: "unknown", Description: "unknown archive format"}
	}
}

func isoMagicAt(head []byte, base int64) bool {
	off := int(base + 16*2048)
	if off < 0 || off+6 > len(head) {
		return false
	}
	return head[off] == 1 && string(head[off+1:off+6]) == "CD001"
}

func readHead(r io.ReadSeeker) ([]byte, error) {
	if _, err := r.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	buf := make([]byte, 64*1024)
	n, err := io.ReadFull(r, buf)
	if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
		return nil, err
	}
	if _, err := r.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	return buf[:n], nil
}

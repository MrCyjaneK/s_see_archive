package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	bzip2w "github.com/dsnet/compress/bzip2"

	"github.com/kdomanski/iso9660"
	"github.com/klauspost/compress/zstd"
	"github.com/ulikunitz/xz"
)

func TestAnalyzeZip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sample.zip")
	content := []byte("hello from zip")
	writeZip(t, path, map[string][]byte{"a/a/a.txt": content})

	out, err := analyzeArchive(path, func(int, string) {})
	if err != nil {
		t.Fatal(err)
	}
	if out.Archive.Kind != "zip" {
		t.Fatalf("kind=%q", out.Archive.Kind)
	}
	f, ok := out.Files["a/a/a.txt"]
	if !ok {
		t.Fatalf("files=%v", out.Files)
	}
	if f.Size != int64(len(content)) {
		t.Fatalf("size=%d", f.Size)
	}
	if !strings.HasPrefix(f.MimeType, "text/plain") {
		t.Fatalf("mime=%q", f.MimeType)
	}
	wantSHA := sha256Hex(content)
	if f.SHA256 != wantSHA {
		t.Fatalf("sha256=%q want %q", f.SHA256, wantSHA)
	}
	if out.Archive.FileCount != 1 {
		t.Fatalf("fileCount=%d", out.Archive.FileCount)
	}
}

func TestAnalyzeTarGz(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sample.tar.gz")
	content := []byte("tar.gz payload")
	writeTarGz(t, path, map[string][]byte{"dir/readme.txt": content})

	out, err := analyzeArchive(path, func(int, string) {})
	if err != nil {
		t.Fatal(err)
	}
	if out.Archive.Kind != "tar.gz" {
		t.Fatalf("kind=%q", out.Archive.Kind)
	}
	f := out.Files["dir/readme.txt"]
	if f.SHA256 != sha256Hex(content) {
		t.Fatalf("sha256=%q", f.SHA256)
	}
}

func TestAnalyzeTarXz(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sample.tar.xz")
	content := []byte("tar.xz payload")
	writeTarXz(t, path, map[string][]byte{"nested/data.bin": content})

	out, err := analyzeArchive(path, func(int, string) {})
	if err != nil {
		t.Fatal(err)
	}
	if out.Archive.Kind != "tar.xz" {
		t.Fatalf("kind=%q", out.Archive.Kind)
	}
	if out.Files["nested/data.bin"].SHA256 != sha256Hex(content) {
		t.Fatalf("sha256=%q", out.Files["nested/data.bin"].SHA256)
	}
}

func TestDetectFormatFromMagic(t *testing.T) {
	cases := []struct {
		head []byte
		kind string
	}{
		{[]byte{0x50, 0x4b, 0x03, 0x04}, "zip"},
		{[]byte{0x37, 0x7a, 0xbc, 0xaf, 0x27, 0x1c}, "7z"},
		{[]byte{'B', 'Z', 'h', '9'}, "bzip2"},
		{[]byte{0x28, 0xb5, 0x2f, 0xfd}, "zstd"},
	}
	for _, tc := range cases {
		if got := detectFormat("unknown", tc.head); got.Kind != tc.kind {
			t.Fatalf("head=% x kind=%q want %q", tc.head, got.Kind, tc.kind)
		}
	}
}

func TestAnalyzeTarBz2(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sample.tar.bz2")
	content := []byte("tar.bz2 payload")
	writeTarBz2(t, path, map[string][]byte{"pkg/data.txt": content})

	out, err := analyzeArchive(path, func(int, string) {})
	if err != nil {
		t.Fatal(err)
	}
	if out.Archive.Kind != "tar.bz2" {
		t.Fatalf("kind=%q", out.Archive.Kind)
	}
	if out.Files["pkg/data.txt"].SHA256 != sha256Hex(content) {
		t.Fatalf("sha256=%q", out.Files["pkg/data.txt"].SHA256)
	}
}

func TestAnalyzeTarZst(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sample.tar.zst")
	content := []byte("tar.zst payload")
	writeTarZst(t, path, map[string][]byte{"z/file.bin": content})

	out, err := analyzeArchive(path, func(int, string) {})
	if err != nil {
		t.Fatal(err)
	}
	if out.Archive.Kind != "tar.zst" {
		t.Fatalf("kind=%q", out.Archive.Kind)
	}
	if out.Files["z/file.bin"].SHA256 != sha256Hex(content) {
		t.Fatalf("sha256=%q", out.Files["z/file.bin"].SHA256)
	}
}

// note: no binary file for testing.
// func TestAnalyze7z(t *testing.T) {
// 	path := filepath.Join("testdata", "sample.7z")
// 	out, err := analyzeArchive(path, func(int, string) {})
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	if out.Archive.Kind != "7z" {
// 		t.Fatalf("kind=%q", out.Archive.Kind)
// 	}
// 	if out.Archive.FileCount == 0 {
// 		t.Fatal("expected files in 7z archive")
// 	}
// }

func TestAnalyzeISO(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sample.iso")
	content := []byte("iso payload")
	writeISO(t, path, "HELLO.TXT", content)

	out, err := analyzeArchive(path, func(int, string) {})
	if err != nil {
		t.Fatal(err)
	}
	if out.Archive.Kind != "iso" {
		t.Fatalf("kind=%q", out.Archive.Kind)
	}
	f, ok := out.Files["hello.txt"]
	if !ok {
		t.Fatalf("files=%v", out.Files)
	}
	if f.SHA256 != sha256Hex(content) {
		t.Fatalf("sha256=%q", f.SHA256)
	}
}

func TestAnalyzeIMG(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "disk.img")
	content := []byte("img payload")
	writeISO(t, path, "DATA.BIN", content)

	out, err := analyzeArchive(path, func(int, string) {})
	if err != nil {
		t.Fatal(err)
	}
	if out.Archive.Kind != "img" {
		t.Fatalf("kind=%q", out.Archive.Kind)
	}
	f, ok := out.Files["data.bin"]
	if !ok {
		t.Fatalf("files=%v", out.Files)
	}
	if f.SHA256 != sha256Hex(content) {
		t.Fatalf("sha256=%q", f.SHA256)
	}
}

func writeZip(t *testing.T, path string, files map[string][]byte) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zw := zip.NewWriter(f)
	for name, data := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
}

func writeTarGz(t *testing.T, path string, files map[string][]byte) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	gw := gzip.NewWriter(f)
	writeTarBytes(t, gw, files)
	if err := gw.Close(); err != nil {
		t.Fatal(err)
	}
}

func writeTarXz(t *testing.T, path string, files map[string][]byte) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	xw, err := xz.NewWriter(f)
	if err != nil {
		t.Fatal(err)
	}
	writeTarBytes(t, xw, files)
	if err := xw.Close(); err != nil {
		t.Fatal(err)
	}
}

func writeTarBz2(t *testing.T, path string, files map[string][]byte) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	bw, err := bzip2w.NewWriter(f, nil)
	if err != nil {
		t.Fatal(err)
	}
	writeTarBytes(t, bw, files)
	if err := bw.Close(); err != nil {
		t.Fatal(err)
	}
}

func writeTarZst(t *testing.T, path string, files map[string][]byte) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zw, err := zstd.NewWriter(f)
	if err != nil {
		t.Fatal(err)
	}
	writeTarBytes(t, zw, files)
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
}

func writeISO(t *testing.T, path, name string, content []byte) {
	t.Helper()
	writer, err := iso9660.NewWriter()
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Cleanup()
	if err := writer.AddFile(bytes.NewReader(content), name); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := writer.WriteTo(f, "TEST"); err != nil {
		t.Fatal(err)
	}
}

func writeTarBytes(t *testing.T, w interface{ Write([]byte) (int, error) }, files map[string][]byte) {
	t.Helper()
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for name, data := range files {
		if err := tw.WriteHeader(&tar.Header{
			Name: name,
			Mode: 0o644,
			Size: int64(len(data)),
		}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(buf.Bytes()); err != nil {
		t.Fatal(err)
	}
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

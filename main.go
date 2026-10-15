package main

/*
#include <stdlib.h>
*/
import "C"

import (
	"encoding/json"
	"os"
	"path/filepath"
	"unsafe"
)

const defaultMaxInputSize = 16 * 1024 * 1024 * 1024 // 16 GiB

const moduleVersion = "1.1.0"
const moduleExtension = "json"

var (
	loadErr string
	tempDir string
)

//export SPACESEE_archive_load
func SPACESEE_archive_load(workDir, assetsDir *C.char) C.int {
	loadErr = ""
	path := C.GoString(workDir)
	if path == "" {
		loadErr = "work dir is empty"
		return 1
	}
	if err := os.MkdirAll(path, 0o700); err != nil {
		loadErr = err.Error()
		return 1
	}
	tempDir = path
	_ = assetsDir
	return 0
}

//export SPACESEE_archive_unload
func SPACESEE_archive_unload() {}

//export SPACESEE_archive_isHeavy
func SPACESEE_archive_isHeavy() C.int { return 0 }

//export SPACESEE_archive_assetCount
func SPACESEE_archive_assetCount() C.int { return 0 }

//export SPACESEE_archive_asset
func SPACESEE_archive_asset(index C.int) *C.char {
	_ = index
	return nil
}

//export SPACESEE_archive_loadError
func SPACESEE_archive_loadError() *C.char {
	if loadErr == "" {
		return nil
	}
	return C.CString(loadErr)
}

//export SPACESEE_archive_version
func SPACESEE_archive_version() *C.char {
	return C.CString(moduleVersion)
}

//export SPACESEE_archive_extension
func SPACESEE_archive_extension() *C.char {
	return C.CString(moduleExtension)
}

//export SPACESEE_archive_filetypes
func SPACESEE_archive_filetypes() *C.char {
	return C.CString(".zip,.tar,.tar.gz,.tgz,.tar.bz2,.tbz2,.tbz,.tar.xz,.txz,.tar.zst,.tzst,.tar.lz4,.gz,.bz2,.xz,.zst,.lz4,.7z,.iso,.img,.rar")
}

//export SPACESEE_archive_maxInputSize
func SPACESEE_archive_maxInputSize() C.longlong {
	return C.longlong(defaultMaxInputSize)
}

//export SPACESEE_archive_analyze
func SPACESEE_archive_analyze(inputPath *C.char, outputPath *C.char, dataDir *C.char) C.longlong {
	_ = dataDir
	in := C.GoString(inputPath)
	out := C.GoString(outputPath)
	jobID := newJob()
	go runAnalyze(jobID, in, out)
	return C.longlong(jobID)
}

func runAnalyze(jobID int64, inputPath, outputPath string) {
	setJobProgress(jobID, 5, "opening archive")
	result, err := analyzeArchive(inputPath, func(p int, msg string) {
		setJobProgress(jobID, p, msg)
	})
	if err != nil {
		setJobError(jobID, err)
		return
	}

	setJobProgress(jobID, 90, "writing output")
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o700); err != nil {
		setJobError(jobID, err)
		return
	}
	b, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		setJobError(jobID, err)
		return
	}
	if err := os.WriteFile(outputPath, b, 0o600); err != nil {
		setJobError(jobID, err)
		return
	}
	setJobDone(jobID, "done")
}

//export SPACESEE_archive_progress
func SPACESEE_archive_progress(jobID C.longlong, scale C.int) C.int {
	return C.int(jobProgress(int64(jobID), int(scale)))
}

//export SPACESEE_archive_progressString
func SPACESEE_archive_progressString(jobID C.longlong) *C.char {
	return C.CString(jobProgressString(int64(jobID)))
}

//export SPACESEE_archive_free
func SPACESEE_archive_free(ptr unsafe.Pointer) {
	C.free(ptr)
}

func main() {}

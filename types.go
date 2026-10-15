package main

type analysisOutput struct {
	Files   map[string]fileInfo `json:"files"`
	Archive archiveInfo         `json:"archive"`
}

type fileInfo struct {
	Size           int64  `json:"size"`
	SizeCompressed int64  `json:"sizeCompressed,omitempty"`
	MimeType       string `json:"mimeType,omitempty"`
	SHA256         string `json:"sha256,omitempty"`
	ModTime        string `json:"modTime,omitempty"`
	IsDir          bool   `json:"isDir,omitempty"`
}

type archiveInfo struct {
	Kind        string  `json:"kind"`
	Description string  `json:"description"`
	Ratio       float64 `json:"ratio,omitempty"`
	FileCount   int     `json:"fileCount,omitempty"`
	TotalSize   int64   `json:"totalSize,omitempty"`
}

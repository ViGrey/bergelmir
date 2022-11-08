package main

import (
	"mime"
	"path/filepath"
)

func getMIMEType(path string) (mimeType string) {
	mimeType = "application/octet-stream"
	extension := filepath.Ext(path)
	if len(extension) > 0 {
		mimeType = mime.TypeByExtension(extension)
		return
	}
	return
}

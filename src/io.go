package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

func fileExists(path string) bool {
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return false
	}
	return true
}

func createFileDirectory(filePath string) {
	dir := filepath.Dir(filePath)
	handleErr(os.MkdirAll(dir, 0700),
		fmt.Sprintf("Unable to create directory %s", dir))
}

func writeFile(filePath string, content []byte) {
	handleErr(os.WriteFile(filePath, content, 0600),
		fmt.Sprintf("Unable to write content to file %s", filePath))
}

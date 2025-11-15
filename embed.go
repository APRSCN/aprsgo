package main

import (
	"embed"
	"io/fs"
	"os"
	"path/filepath"
)

//go:embed static/*
var embeddedResources embed.FS

//go:embed config.yaml
var defaultConfig []byte

// InitEmbed inits embed to generate basic static resources
func InitEmbed() {
	// Default config
	if _, err := os.Stat("config.yaml"); os.IsNotExist(err) {
		if err = os.WriteFile("config.yaml", defaultConfig, 0644); err != nil {
			panic(err)
		}
	}

	// Static of web
	staticDir := "static"

	if _, err := os.Stat(staticDir); os.IsNotExist(err) {
		err := copyEmbeddedResources(embeddedResources, "static", staticDir)
		if err != nil {
			panic(err)
		}
	}
}

// copyEmbeddedResources copies resources from embed to disk
func copyEmbeddedResources(embedFS embed.FS, embedPath, targetPath string) error {
	return fs.WalkDir(embedFS, embedPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(embedPath, path)
		if err != nil {
			return err
		}
		targetFile := filepath.Join(targetPath, relPath)

		if d.IsDir() {
			return os.MkdirAll(targetFile, 0755)
		} else {
			data, err := embedFS.ReadFile(path)
			if err != nil {
				return err
			}
			return os.WriteFile(targetFile, data, 0644)
		}
	})
}

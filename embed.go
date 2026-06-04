package main

import (
	"embed"
	"io/fs"
	"os"
	"path/filepath"
)

// webRuntimeDir is the on-disk directory the web bundle is released to on first
// run. Serving from here (when present) lets operators replace/customise the UI
// without rebuilding the binary.
const webRuntimeDir = "static"

// webDist holds the prerendered Nuxt SSG bundle (built with `pnpm generate`
// in web/). It is embedded into the binary; on first run it is extracted to
// webRuntimeDir so it can be customised, and served from there if present.
//
//go:embed all:web/dist
var webDist embed.FS

//go:embed config.yaml
var defaultConfig []byte

// embeddedWebFS returns the embedded web bundle rooted at web/dist.
func embeddedWebFS() fs.FS {
	sub, err := fs.Sub(webDist, "web/dist")
	if err != nil {
		// This only fails if the embed path is wrong, which is a build-time
		// guarantee; panic to surface it immediately.
		panic(err)
	}
	return sub
}

// WebFS returns the filesystem to serve the web UI from: the on-disk runtime
// directory if it exists (so user edits take effect), otherwise the embedded
// bundle.
func WebFS() fs.FS {
	if st, err := os.Stat(webRuntimeDir); err == nil && st.IsDir() {
		return os.DirFS(webRuntimeDir)
	}
	return embeddedWebFS()
}

// InitEmbed writes the default configuration file and releases the web bundle
// to disk on first run.
func InitEmbed() {
	// Default config
	if _, err := os.Stat("config.yaml"); os.IsNotExist(err) {
		if err = os.WriteFile("config.yaml", defaultConfig, 0644); err != nil {
			panic(err)
		}
	}

	// Release the web bundle to disk if the runtime dir does not exist yet.
	if _, err := os.Stat(webRuntimeDir); os.IsNotExist(err) {
		if err = releaseWeb(); err != nil {
			panic(err)
		}
	}
}

// releaseWeb copies the embedded web bundle into webRuntimeDir.
func releaseWeb() error {
	src := embeddedWebFS()
	return fs.WalkDir(src, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		target := filepath.Join(webRuntimeDir, filepath.FromSlash(path))
		if d.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		data, err := fs.ReadFile(src, path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		return os.WriteFile(target, data, 0644)
	})
}

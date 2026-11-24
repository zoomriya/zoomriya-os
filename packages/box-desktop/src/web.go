// The desktop's page, built by Vite from ../web into dist/ and put into the
// program, so the box serves exactly what was built with it.
package main

import (
	"embed"
	"io/fs"
	"os"
)

//go:embed all:dist
var built embed.FS

// webFiles: the built-in page, or a folder of one (development).
func webFiles(folder string) (fs.FS, error) {
	if folder != "" {
		if _, err := os.Stat(folder); err != nil {
			return nil, err
		}
		return os.DirFS(folder), nil
	}
	return fs.Sub(built, "dist")
}

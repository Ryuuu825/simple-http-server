package admin

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed static/*
var staticFiles embed.FS

// GetStaticHandler returns a handler for serving embedded static files
func GetStaticHandler() http.Handler {
	// Get the static subdirectory
	fsys, err := fs.Sub(staticFiles, "static")
	if err != nil {
		panic(err)
	}
	
	return http.FileServer(http.FS(fsys))
}

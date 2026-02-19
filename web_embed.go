package main

import (
	"embed"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"regexp"
	"strings"
)

//go:embed frontend/dist
var embeddedFrontend embed.FS

var frontendFS fs.FS
var hashedAssetPattern = regexp.MustCompile(`-[a-zA-Z0-9]{8,}\.`)

func init() {
	var err error
	frontendFS, err = fs.Sub(embeddedFrontend, "frontend/dist")
	if err != nil {
		panic(err)
	}
}

func shouldServeFrontend(pathname string) bool {
	if pathname == "/" {
		return true
	}
	if strings.HasPrefix(pathname, "/assets/") {
		return true
	}
	switch pathname {
	case "/favicon.ico", "/vite.svg", "/manifest.webmanifest":
		return true
	default:
		return false
	}
}

func serveFrontend(w http.ResponseWriter, r *http.Request) bool {
	if !shouldServeFrontend(r.URL.Path) {
		return false
	}

	name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
	if r.URL.Path == "/" || name == "." {
		name = "index.html"
	}

	content, err := fs.ReadFile(frontendFS, name)
	if err != nil {
		return false
	}

	if contentType := mime.TypeByExtension(path.Ext(name)); contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}
	setStaticCacheHeader(w, name)
	_, _ = w.Write(content)
	return true
}

func setStaticCacheHeader(w http.ResponseWriter, name string) {
	switch {
	case strings.HasPrefix(name, "assets/") && hashedAssetPattern.MatchString(name):
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	case name == "index.html":
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	default:
		w.Header().Set("Cache-Control", "public, max-age=3600")
	}
}

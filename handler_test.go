package main

import (
	"TheOnlyMirror/config"
	"TheOnlyMirror/plugins"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestMatchSourceDeterministicAndSpecific(t *testing.T) {
	if err := config.Load(); err != nil {
		t.Fatalf("load config.json failed: %v", err)
	}

	cases := []struct {
		path string
		ua   string
		want string
	}{
		{"/go/mod", "Go-http-client/1.1", "golang"},
		{"/pkg/mod", "Go-http-client/1.1", "go"},
		{"/ubuntu-security/dists/jammy-security/Release", "Debian APT-HTTP/1.3 (2.4.12)", "ubuntu-security"},
		{"/ubuntu/dists/jammy/Release", "Debian APT-HTTP/1.3 (2.4.12)", "ubuntu"},
		{"/crates-index/config.json", "cargo 1.83.0", "crates-index"},
		{"/crates-index/se/rd/serde", "cargo 1.83.0", "crates-index"},
		{"/crates/serde/1.0.219/download", "cargo 1.83.0", "crates-dl"},
		{"/whl/cpu/torch/", "pip/24.0", "pytorch"},
		{"/whl/cpu/torch-2.4.0%2Bcpu-cp310-cp310-linux_x86_64.whl", "pip/24.0", "pytorch"},
		{"/rustup/dist/x86_64-unknown-linux-gnu/rustup-init", "curl/8.5.0", "rustup"},
		{"/dist/channel-rust-stable.toml", "rustup/1.29.1", "rust-dist"},
		{"/openclaw.ai/install.sh", "curl/8.5.0", "openclaw"},
		{"/debian/dists/bookworm/Release", "Debian APT-HTTP/1.3 (2.6.1)", "debian"},
		{"/simple/requests/", "pip/24.0", "pypi_web"},
		{"/packages/x/y.whl", "pip/24.0", "pypi_package"},
		{"/express", "npm/10.0.0", "npm"},
		{"/v2/library/ubuntu/manifests/latest", "docker/24.0.7", "docker_registry"},
		{"/token", "docker/24.0.7", "docker_auth"},
	}

	for _, tc := range cases {
		source, key, ok := matchSource(tc.path, tc.ua)
		if !ok {
			t.Fatalf("path %s ua %s: no source matched, want %s", tc.path, tc.ua, tc.want)
		}
		if key != tc.want {
			t.Fatalf("path %s ua %s: got %s, want %s", tc.path, tc.ua, key, tc.want)
		}
		if source == nil {
			t.Fatalf("path %s: nil source", tc.path)
		}
	}

	// Map iteration order must never change routing decisions.
	for round := 0; round < 50; round++ {
		_, key, ok := matchSource("/go/mod", "Go-http-client/1.1")
		if !ok || key != "golang" {
			t.Fatalf("round %d: unstable routing, got %s ok=%v", round, key, ok)
		}
	}
}

func TestHandlerProxyAliasRedirectPreservesPort(t *testing.T) {
	if err := config.Load(); err != nil {
		t.Fatalf("load config.json failed: %v", err)
	}

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", "https://ghcr.io/token?scope=repository:homebrew/core:pull&service=ghcr.io")
		w.WriteHeader(http.StatusFound)
	}))
	defer upstream.Close()

	targetURL, err := url.Parse(upstream.URL)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "http://ghcr.mirrors.test:8080/v2/", nil)
	rec := httptest.NewRecorder()
	plugins.HandlerProxy(rec, req, targetURL, "ghcr.mirrors.test")

	if rec.Code != http.StatusFound {
		t.Fatalf("status %d", rec.Code)
	}
	want := "http://ghcr.mirrors.test:8080/token?scope=repository:homebrew/core:pull&service=ghcr.io"
	if got := rec.Header().Get("Location"); got != want {
		t.Fatalf("alias redirect lost port or alias:\nwant %s\ngot  %s", want, got)
	}
}

func TestHandlerProxyAliasRedirectForwardedProto(t *testing.T) {
	if err := config.Load(); err != nil {
		t.Fatalf("load config.json failed: %v", err)
	}

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", "https://ghcr.io/token")
		w.WriteHeader(http.StatusFound)
	}))
	defer upstream.Close()

	targetURL, err := url.Parse(upstream.URL)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "http://ghcr.mirrors.test/v2/", nil)
	req.Header.Set("X-Forwarded-Proto", "https")
	rec := httptest.NewRecorder()
	plugins.HandlerProxy(rec, req, targetURL, "ghcr.mirrors.test")

	want := "https://ghcr.mirrors.test/token"
	if got := rec.Header().Get("Location"); got != want {
		t.Fatalf("alias redirect must honor X-Forwarded-Proto:\nwant %s\ngot  %s", want, got)
	}
}

func TestHandlerProxyPathModeRelativeRedirect(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", "/login?return_to=%2Fhome")
		w.WriteHeader(http.StatusFound)
	}))
	defer upstream.Close()

	targetURL, err := url.Parse(upstream.URL)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "http://mirror.example/github.com/settings", nil)
	rec := httptest.NewRecorder()
	plugins.HandlerProxy(rec, req, targetURL, "")

	// Relative redirects in path mode must be prefixed with the normalized
	// upstream host so the client stays on the mirror.
	want := "/127.0.0.1/login?return_to=%2Fhome"
	if got := rec.Header().Get("Location"); got != want {
		t.Fatalf("relative redirect mismatch:\nwant %s\ngot  %s", want, got)
	}
}

package plugins

import (
	"TheOnlyMirror/config"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

func TestRequestScheme(t *testing.T) {
	plain := httptest.NewRequest(http.MethodGet, "http://mirror.example/x", nil)
	if got := requestScheme(plain); got != "http" {
		t.Fatalf("plain request: want http, got %s", got)
	}

	forwarded := httptest.NewRequest(http.MethodGet, "http://mirror.example/x", nil)
	forwarded.Header.Set("X-Forwarded-Proto", "https")
	if got := requestScheme(forwarded); got != "https" {
		t.Fatalf("forwarded request: want https, got %s", got)
	}

	multi := httptest.NewRequest(http.MethodGet, "http://mirror.example/x", nil)
	multi.Header.Set("X-Forwarded-Proto", "https, http")
	if got := requestScheme(multi); got != "https" {
		t.Fatalf("multi forwarded proto: want https, got %s", got)
	}

	bogus := httptest.NewRequest(http.MethodGet, "http://mirror.example/x", nil)
	bogus.Header.Set("X-Forwarded-Proto", "ftp")
	if got := requestScheme(bogus); got != "http" {
		t.Fatalf("bogus forwarded proto: want http fallback, got %s", got)
	}
}

func TestResolveReplaceTarget(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "http://mirror.example:8080/crates-index/config.json", nil)

	if got, ok := resolveReplaceTarget("<HOST>", req); !ok || got != "mirror.example:8080" {
		t.Fatalf("<HOST>: got %q ok=%v", got, ok)
	}
	if got, ok := resolveReplaceTarget("<TLS_SCHEME>", req); !ok || got != "http://" {
		t.Fatalf("<TLS_SCHEME> on plain request: got %q ok=%v", got, ok)
	}
	if got, ok := resolveReplaceTarget("<TLS_SCHEME><HOST>/crates", req); !ok || got != "http://mirror.example:8080/crates" {
		t.Fatalf("composite dst: got %q ok=%v", got, ok)
	}

	forwarded := httptest.NewRequest(http.MethodGet, "http://mirror.example/x", nil)
	forwarded.Header.Set("X-Forwarded-Proto", "https")
	if got, ok := resolveReplaceTarget("<TLS_SCHEME><HOST>", forwarded); !ok || got != "https://mirror.example" {
		t.Fatalf("composite dst forwarded: got %q ok=%v", got, ok)
	}
}

func TestRewriteBodyUnknownLength(t *testing.T) {
	body := `{"dl":"https://static.crates.io/crates","api":"https://crates.io"}`
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type": []string{"application/octet-stream"},
		},
		Body:          io.NopCloser(strings.NewReader(body)),
		ContentLength: -1,
	}
	resp.Header.Set("ETag", `"etag-value"`)
	resp.Header.Set("Digest", "sha-256=abcd")
	resp.Header.Set("Content-MD5", "md5value")

	replaces := []config.Replace{{Src: "https://static.crates.io/crates", Dst: "http://mirror.example/crates"}}
	if err := rewriteBody(resp, replaces, []string{"application/octet-stream"}); err != nil {
		t.Fatalf("rewriteBody failed: %v", err)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read rewritten body failed: %v", err)
	}
	want := `{"dl":"http://mirror.example/crates","api":"https://crates.io"}`
	if string(data) != want {
		t.Fatalf("rewritten body mismatch:\nwant %s\ngot  %s", want, string(data))
	}
	if resp.ContentLength != int64(len(data)) {
		t.Fatalf("ContentLength not updated: %d", resp.ContentLength)
	}
	if resp.Header.Get("Content-Length") != strconv.Itoa(len(data)) {
		t.Fatalf("Content-Length header not updated: %q", resp.Header.Get("Content-Length"))
	}
	for _, header := range []string{"ETag", "Digest", "Content-MD5"} {
		if resp.Header.Get(header) != "" {
			t.Fatalf("stale %s header not cleared", header)
		}
	}
}

func TestRewriteBodyTooLargePassthrough(t *testing.T) {
	original := "https://static.crates.io/crates" + strings.Repeat("a", maxRewriteBodySize+1024)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type": []string{"text/plain"},
		},
		Body:          io.NopCloser(strings.NewReader(original)),
		ContentLength: -1,
	}

	replaces := []config.Replace{{Src: "https://static.crates.io/crates", Dst: "http://mirror.example/crates"}}
	if err := rewriteBody(resp, replaces, nil); err != nil {
		t.Fatalf("rewriteBody should pass through, got error: %v", err)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body failed: %v", err)
	}
	if string(data) != original {
		t.Fatal("oversized body must pass through unmodified")
	}
}

func TestRewriteBodyGzipRoundTrip(t *testing.T) {
	var compressed bytes.Buffer
	gw := gzip.NewWriter(&compressed)
	if _, err := gw.Write([]byte(`{"dl":"https://static.crates.io/crates"}`)); err != nil {
		t.Fatal(err)
	}
	if err := gw.Close(); err != nil {
		t.Fatal(err)
	}

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type":     []string{"application/json"},
			"Content-Encoding": []string{"gzip"},
		},
		Body:          io.NopCloser(bytes.NewReader(compressed.Bytes())),
		ContentLength: int64(compressed.Len()),
	}

	replaces := []config.Replace{{Src: "https://static.crates.io/crates", Dst: "http://mirror.example/crates"}}
	if err := rewriteBody(resp, replaces, nil); err != nil {
		t.Fatalf("rewriteBody failed: %v", err)
	}
	if resp.Header.Get("Content-Encoding") != "gzip" {
		t.Fatal("Content-Encoding must be preserved")
	}

	gr, err := gzip.NewReader(resp.Body)
	if err != nil {
		t.Fatalf(" rewritten body is not gzip: %v", err)
	}
	data, err := io.ReadAll(gr)
	if err != nil {
		t.Fatalf("gunzip failed: %v", err)
	}
	if string(data) != `{"dl":"http://mirror.example/crates"}` {
		t.Fatalf("unexpected rewritten body: %s", string(data))
	}
	if resp.ContentLength <= 0 || resp.Header.Get("Content-Length") != strconv.FormatInt(resp.ContentLength, 10) {
		t.Fatalf("ContentLength/Header mismatch: %d / %q", resp.ContentLength, resp.Header.Get("Content-Length"))
	}
	if resp.Header.Get("ETag") != "" {
		t.Fatal("ETag must be cleared after gzip rewrite")
	}
}

func TestRewriteBodyGzipBombPassthrough(t *testing.T) {
	var compressed bytes.Buffer
	gw := gzip.NewWriter(&compressed)
	if _, err := gw.Write(bytes.Repeat([]byte{0}, maxDecompressedBodySize+1)); err != nil {
		t.Fatal(err)
	}
	if err := gw.Close(); err != nil {
		t.Fatal(err)
	}
	original := compressed.Bytes()

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type":     []string{"text/plain"},
			"Content-Encoding": []string{"gzip"},
		},
		Body:          io.NopCloser(bytes.NewReader(original)),
		ContentLength: int64(len(original)),
	}

	replaces := []config.Replace{{Src: "x", Dst: "y"}}
	if err := rewriteBody(resp, replaces, nil); err != nil {
		t.Fatalf("rewriteBody should pass through, got error: %v", err)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body failed: %v", err)
	}
	if !bytes.Equal(data, original) {
		t.Fatal("gzip bomb must pass through as original compressed bytes")
	}
}

func TestRewriteBodySkipsBinaryContentType(t *testing.T) {
	original := []byte("https://static.crates.io/crates")
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type": []string{"application/gzip"},
		},
		Body:          io.NopCloser(bytes.NewReader(original)),
		ContentLength: int64(len(original)),
	}

	replaces := []config.Replace{{Src: "https://static.crates.io/crates", Dst: "http://mirror.example/crates"}}
	if err := rewriteBody(resp, replaces, nil); err != nil {
		t.Fatalf("rewriteBody failed: %v", err)
	}
	data, _ := io.ReadAll(resp.Body)
	if !bytes.Equal(data, original) {
		t.Fatal("binary content type must not be rewritten")
	}
}

func TestRewriteBodyNoMatchKeepsDigest(t *testing.T) {
	original := []byte(`{"unrelated": true}`)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type": []string{"application/json"},
		},
		Body:          io.NopCloser(bytes.NewReader(original)),
		ContentLength: int64(len(original)),
	}
	resp.Header.Set("ETag", `"keep-me"`)

	replaces := []config.Replace{{Src: "https://static.crates.io/crates", Dst: "http://mirror.example/crates"}}
	if err := rewriteBody(resp, replaces, nil); err != nil {
		t.Fatalf("rewriteBody failed: %v", err)
	}
	data, _ := io.ReadAll(resp.Body)
	if !bytes.Equal(data, original) {
		t.Fatal("unmatched body must stay byte-identical")
	}
	if resp.Header.Get("ETag") != `"keep-me"` {
		t.Fatal("ETag must be kept when body is unchanged")
	}
}

func TestHandlerReverseStripPathAndRewrite(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/config.json" {
			t.Errorf("upstream got path %s, want /config.json", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		fmt.Fprint(w, `{"dl":"https://static.crates.io/crates","api":"https://crates.io"}`)
	}))
	defer upstream.Close()

	source := config.Source{
		Path:                "/crates-index",
		StripPath:           true,
		RewriteContentTypes: []string{"application/octet-stream"},
		Replaces:            []config.Replace{{Src: "https://static.crates.io/crates", Dst: "<TLS_SCHEME><HOST>/crates"}},
		Mirror:              upstream.URL,
	}

	req := httptest.NewRequest(http.MethodGet, "http://mirror.example/crates-index/config.json", nil)
	rec := httptest.NewRecorder()
	HandlerReverse(rec, req, source)

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	want := `{"dl":"http://mirror.example/crates","api":"https://crates.io"}`
	if rec.Body.String() != want {
		t.Fatalf("body mismatch:\nwant %s\ngot  %s", want, rec.Body.String())
	}
}

func TestHandlerReverseStripPathWithPrefix(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ubuntu/dists/jammy-security/Release" {
			t.Errorf("upstream got path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprint(w, "release file")
	}))
	defer upstream.Close()

	source := config.Source{
		Path:      "/ubuntu-security",
		Prefix:    "/ubuntu",
		StripPath: true,
		Mirror:    upstream.URL,
	}

	req := httptest.NewRequest(http.MethodGet, "http://mirror.example/ubuntu-security/dists/jammy-security/Release", nil)
	rec := httptest.NewRecorder()
	HandlerReverse(rec, req, source)

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	if rec.Body.String() != "release file" {
		t.Fatalf("unexpected body: %s", rec.Body.String())
	}
}

func TestHandlerReversePrefixKept(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/simple/requests/" {
			t.Errorf("upstream got path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<a href="https://files.pythonhosted.org/packages/x">x</a>`)
	}))
	defer upstream.Close()

	source := config.Source{
		Prefix: "/simple",
		Replaces: []config.Replace{
			{Src: "https://files.pythonhosted.org", Dst: ""},
		},
		Mirror: upstream.URL,
	}

	req := httptest.NewRequest(http.MethodGet, "http://mirror.example/simple/requests/", nil)
	rec := httptest.NewRecorder()
	HandlerReverse(rec, req, source)

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `href="/packages/x"`) {
		t.Fatalf("body not rewritten: %s", rec.Body.String())
	}
}

func TestHandlerReverseOpenClawScriptRewrite(t *testing.T) {
	script := `registry="https://registry.npmjs.org"
pin_url="https://raw.githubusercontent.com/openclaw/openclaw/v1.0.0/scripts/lib/openclaw-bun.json"
git clone https://github.com/openclaw/openclaw.git
setup_url="https://deb.nodesource.com/setup_22.x"
base="https://github.com/charmbracelet/gum/releases/download/v2.0.0"
doc="https://docs.openclaw.ai/start/faq"
`
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/install.sh" {
			t.Errorf("upstream got path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/x-sh")
		fmt.Fprint(w, script)
	}))
	defer upstream.Close()

	source := config.Source{
		Path:      "/openclaw.ai",
		StripPath: true,
		Replaces: []config.Replace{
			{Src: "https://openclaw.ai", Dst: "<TLS_SCHEME><HOST>/openclaw.ai"},
			{Src: "https://raw.githubusercontent.com", Dst: "<TLS_SCHEME><HOST>/raw.githubusercontent.com"},
			{Src: "https://github.com", Dst: "<TLS_SCHEME><HOST>/github.com"},
			{Src: "https://registry.npmjs.org", Dst: "<TLS_SCHEME><HOST>/registry.npmjs.org"},
			{Src: "https://deb.nodesource.com", Dst: "<TLS_SCHEME><HOST>/deb.nodesource.com"},
			{Src: "https://rpm.nodesource.com", Dst: "<TLS_SCHEME><HOST>/rpm.nodesource.com"},
			{Src: "https://nodejs.org", Dst: "<TLS_SCHEME><HOST>/nodejs.org"},
		},
		Mirror: upstream.URL,
	}

	req := httptest.NewRequest(http.MethodGet, "http://mirror.example/openclaw.ai/install.sh", nil)
	rec := httptest.NewRecorder()
	HandlerReverse(rec, req, source)

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{
		`registry="http://mirror.example/registry.npmjs.org"`,
		`pin_url="http://mirror.example/raw.githubusercontent.com/openclaw/openclaw/v1.0.0/scripts/lib/openclaw-bun.json"`,
		`git clone http://mirror.example/github.com/openclaw/openclaw.git`,
		`setup_url="http://mirror.example/deb.nodesource.com/setup_22.x"`,
		`base="http://mirror.example/github.com/charmbracelet/gum/releases/download/v2.0.0"`,
		`doc="https://docs.openclaw.ai/start/faq"`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("rewritten script missing %q:\n%s", want, body)
		}
	}
}

func TestSplitAndApplyHeaderReplacesMultiValue(t *testing.T) {
	resp := &http.Response{Header: http.Header{}}
	resp.Header.Add("Www-Authenticate", `Bearer realm="https://auth.docker.io/token",service="registry.docker.io"`)
	resp.Header.Add("Www-Authenticate", `Bearer realm="https://auth.docker.io/token2",service="registry.docker.io"`)

	req := httptest.NewRequest(http.MethodGet, "http://mirror.example/v2/", nil)
	replaces := []config.Replace{
		{Type: "header", Header: "www-authenticate", Src: "https://", Dst: "<TLS_SCHEME>"},
		{Type: "header", Header: "www-authenticate", Src: "auth.docker.io", Dst: "<HOST>"},
	}
	bodyReplaces := splitAndApplyHeaderReplaces(resp, req, replaces)
	if len(bodyReplaces) != 0 {
		t.Fatalf("header replaces must not produce body replaces: %v", bodyReplaces)
	}

	values := resp.Header.Values("www-authenticate")
	if len(values) != 2 {
		t.Fatalf("multi-value header must keep both values, got %v", values)
	}
	if values[0] != `Bearer realm="http://mirror.example/token",service="registry.docker.io"` {
		t.Fatalf("first value not rewritten: %s", values[0])
	}
	if values[1] != `Bearer realm="http://mirror.example/token2",service="registry.docker.io"` {
		t.Fatalf("second value not rewritten: %s", values[1])
	}
}

func TestSplitAndApplyHeaderReplacesSkipsAbsentHeader(t *testing.T) {
	resp := &http.Response{Header: http.Header{}}
	req := httptest.NewRequest(http.MethodGet, "http://mirror.example/v2/library/ubuntu/blobs/sha256:abc", nil)
	replaces := []config.Replace{
		{Type: "header", Header: "location", Src: "https://", Dst: "<TLS_SCHEME>"},
		{Type: "header", Header: "www-authenticate", Src: "auth.docker.io", Dst: "<HOST>"},
	}
	splitAndApplyHeaderReplaces(resp, req, replaces)

	if len(resp.Header) != 0 {
		t.Fatalf("absent headers must not be injected, got %v", resp.Header)
	}
}

func TestHandlerReverseDockerAuthHeaders(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Docker-Distribution-Api-Version", "registry/2.0")
		w.Header().Set("Www-Authenticate", `Bearer realm="https://auth.docker.io/token",service="registry.docker.io"`)
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"errors":[{"code":"UNAUTHORIZED","message":"authentication required"}]}`)
	}))
	defer upstream.Close()

	source := config.Source{
		UA: "docker",
		Replaces: []config.Replace{
			{Type: "header", Header: "www-authenticate", Src: "https://", Dst: "<TLS_SCHEME>"},
			{Type: "header", Header: "www-authenticate", Src: "auth.docker.io", Dst: "<HOST>"},
		},
		Mirror: upstream.URL,
	}

	req := httptest.NewRequest(http.MethodGet, "http://mirror.example/v2/", nil)
	req.Header.Set("User-Agent", "docker/24.0.7")
	rec := httptest.NewRecorder()
	HandlerReverse(rec, req, source)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status %d", rec.Code)
	}
	if got := rec.Header().Get("Www-Authenticate"); got != `Bearer realm="http://mirror.example/token",service="registry.docker.io"` {
		t.Fatalf("www-authenticate not rewritten: %s", got)
	}
	if got := rec.Header().Get("Docker-Distribution-Api-Version"); got != "registry/2.0" {
		t.Fatalf("Docker-Distribution-Api-Version must pass through, got %q", got)
	}
}

func TestHandlerReverseLinkHeaderRewrite(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Link", `<https://registry-1.docker.io/v2/_catalog?n=100&last=library%2Fubuntu>; rel="next"`)
		fmt.Fprint(w, `{"repositories":[]}`)
	}))
	defer upstream.Close()

	source := config.Source{
		UA: "docker",
		Replaces: []config.Replace{
			{Type: "header", Header: "link", Src: "https://registry-1.docker.io", Dst: "<TLS_SCHEME><HOST>"},
		},
		Mirror: upstream.URL,
	}

	req := httptest.NewRequest(http.MethodGet, "http://mirror.example/v2/_catalog?n=100", nil)
	req.Header.Set("User-Agent", "docker/24.0.7")
	rec := httptest.NewRecorder()
	HandlerReverse(rec, req, source)

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	want := `<http://mirror.example/v2/_catalog?n=100&last=library%2Fubuntu>; rel="next"`
	if got := rec.Header().Get("Link"); got != want {
		t.Fatalf("link header mismatch:\nwant %s\ngot  %s", want, got)
	}
}

func TestHandlerReversePreservesEscapedPath(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.RequestURI, "%2B") {
			t.Errorf("upstream lost original escaping, RequestURI=%s", r.RequestURI)
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	source := config.Source{
		UA:     "pip",
		Path:   "/whl",
		Mirror: upstream.URL,
	}

	req := httptest.NewRequest(http.MethodGet, "http://mirror.example/whl/cpu/torch-2.4.0%2Bcpu-cp310-cp310-linux_x86_64.whl", nil)
	rec := httptest.NewRecorder()
	HandlerReverse(rec, req, source)

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestHandlerReverseStripPathDropsStaleRawPath(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/config.json" {
			t.Errorf("upstream got path %s, want /config.json", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	source := config.Source{
		Path:      "/crates-index",
		StripPath: true,
		Mirror:    upstream.URL,
	}

	req := httptest.NewRequest(http.MethodGet, "http://mirror.example/crates-index/config.json", nil)
	rec := httptest.NewRecorder()
	HandlerReverse(rec, req, source)

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
}

package plugins

import (
	"TheOnlyMirror/config"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
)

const (
	maxRewriteBodySize      = 8 * 1024 * 1024  // on-wire body limit for rewriting
	maxDecompressedBodySize = 64 * 1024 * 1024 // gzip decompression bomb limit
)

// requestScheme reports the scheme the client used to reach the mirror,
// honoring TLS termination at a reverse proxy (e.g. Caddy X-Forwarded-Proto).
func requestScheme(r *http.Request) string {
	if r.TLS != nil {
		return "https"
	}
	if forwarded := r.Header.Get("X-Forwarded-Proto"); forwarded != "" {
		proto := strings.ToLower(strings.TrimSpace(strings.Split(forwarded, ",")[0]))
		if proto == "http" || proto == "https" {
			return proto
		}
	}
	if config.GetTls() {
		return "https"
	}
	return "http"
}

func resolveReplaceTarget(value string, r *http.Request) (string, bool) {
	scheme := requestScheme(r) + "://"
	switch value {
	case "<HOST>":
		return r.Host, true
	case "<TLS_SCHEME>":
		return scheme, true
	default:
		value = strings.ReplaceAll(value, "<TLS_SCHEME>", scheme)
		value = strings.ReplaceAll(value, "<HOST>", r.Host)
		return value, true
	}
}

func splitAndApplyHeaderReplaces(resp *http.Response, r *http.Request, replaces []config.Replace) []config.Replace {
	bodyReplaces := make([]config.Replace, 0, len(replaces))
	for _, replace := range replaces {
		dst, ok := resolveReplaceTarget(replace.Dst, r)
		if !ok {
			continue
		}

		if replace.Type == "header" {
			values := resp.Header.Values(replace.Header)
			if len(values) == 0 {
				continue
			}
			for i, value := range values {
				values[i] = strings.ReplaceAll(value, replace.Src, dst)
			}
			resp.Header.Del(replace.Header)
			for _, value := range values {
				resp.Header.Add(replace.Header, value)
			}
			continue
		}

		replace.Dst = dst
		bodyReplaces = append(bodyReplaces, replace)
	}
	return bodyReplaces
}

func baseContentType(contentType string) string {
	if idx := strings.Index(contentType, ";"); idx >= 0 {
		contentType = contentType[:idx]
	}
	return strings.ToLower(strings.TrimSpace(contentType))
}

func shouldRewriteBody(resp *http.Response, extraContentTypes []string) bool {
	contentType := baseContentType(resp.Header.Get("Content-Type"))
	if contentType == "" {
		return true
	}
	if strings.HasPrefix(contentType, "text/") {
		return true
	}
	if strings.Contains(contentType, "json") || strings.Contains(contentType, "xml") ||
		strings.Contains(contentType, "javascript") || strings.Contains(contentType, "x-sh") {
		return true
	}
	for _, extra := range extraContentTypes {
		if contentType == baseContentType(extra) {
			return true
		}
	}
	return false
}

// readBodyLimited reads up to maxBytes+1 bytes. When the limit is exceeded the
// response body is restored to a pass-through stream so the caller can skip
// rewriting without corrupting the payload.
func readBodyLimited(resp *http.Response, maxBytes int64) ([]byte, bool, error) {
	limited := io.LimitReader(resp.Body, maxBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		_ = resp.Body.Close()
		return nil, false, err
	}
	if int64(len(data)) > maxBytes {
		resp.Body = struct {
			io.Reader
			io.Closer
		}{io.MultiReader(bytes.NewReader(data), resp.Body), resp.Body}
		return nil, true, nil
	}
	_ = resp.Body.Close()
	return data, false, nil
}

// decodeBody decompresses bodyBytes when needed. The second return value is
// false when the payload cannot be decoded or exceeds the decompression limit;
// callers should then pass the original bytes through unchanged. Unknown
// encodings are never treated as identity: replacing inside compressed bytes
// would silently skip or corrupt the payload.
func decodeBody(encoding string, bodyBytes []byte) ([]byte, bool) {
	switch strings.ToLower(strings.TrimSpace(encoding)) {
	case "":
		return bodyBytes, true
	case "gzip":
		reader, err := gzip.NewReader(bytes.NewReader(bodyBytes))
		if err != nil {
			return nil, false
		}
		defer reader.Close()
		dataDecompressed, err := io.ReadAll(io.LimitReader(reader, maxDecompressedBodySize+1))
		if err != nil || int64(len(dataDecompressed)) > maxDecompressedBodySize {
			return nil, false
		}
		return dataDecompressed, true
	default:
		return nil, false
	}
}

func encodeBody(encoding string, body []byte) ([]byte, error) {
	switch strings.ToLower(strings.TrimSpace(encoding)) {
	case "gzip":
		var b bytes.Buffer
		gw := gzip.NewWriter(&b)
		if _, err := gw.Write(body); err != nil {
			_ = gw.Close()
			return nil, fmt.Errorf("gzip encode failed: %w", err)
		}
		if err := gw.Close(); err != nil {
			return nil, fmt.Errorf("gzip close failed: %w", err)
		}
		return b.Bytes(), nil
	default:
		return body, nil
	}
}

// restoreOriginalBody puts the untouched payload back on the response so a
// failed or oversized rewrite degrades to a plain pass-through.
func restoreOriginalBody(resp *http.Response, bodyBytes []byte) {
	resp.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	resp.ContentLength = int64(len(bodyBytes))
	resp.Header.Set("Content-Length", strconv.Itoa(len(bodyBytes)))
}

func rewriteBody(resp *http.Response, bodyReplaces []config.Replace, extraContentTypes []string) error {
	if !shouldRewriteBody(resp, extraContentTypes) {
		return nil
	}
	if resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusNotModified {
		return nil
	}
	// Ranged responses must pass through byte-identical: rewriting the body
	// without fixing Content-Range would corrupt resumed transfers.
	if resp.StatusCode == http.StatusPartialContent || resp.Header.Get("Content-Range") != "" {
		return nil
	}
	if resp.Request != nil && resp.Request.Method == http.MethodHead {
		return nil
	}
	if resp.ContentLength == 0 {
		return nil
	}
	if resp.ContentLength > maxRewriteBodySize {
		return nil
	}

	bodyBytes, tooLarge, err := readBodyLimited(resp, maxRewriteBodySize)
	if err != nil {
		return err
	}
	if tooLarge {
		return nil
	}

	contentEncoding := resp.Header.Get("Content-Encoding")
	decodedBody, ok := decodeBody(contentEncoding, bodyBytes)
	if !ok {
		restoreOriginalBody(resp, bodyBytes)
		return nil
	}

	modifiedBodyString := string(decodedBody)
	for _, replace := range bodyReplaces {
		modifiedBodyString = strings.ReplaceAll(modifiedBodyString, replace.Src, replace.Dst)
	}
	if modifiedBodyString == string(decodedBody) {
		restoreOriginalBody(resp, bodyBytes)
		return nil
	}

	finalBody, err := encodeBody(contentEncoding, []byte(modifiedBodyString))
	if err != nil {
		restoreOriginalBody(resp, bodyBytes)
		return nil
	}

	resp.Body = io.NopCloser(bytes.NewReader(finalBody))
	resp.ContentLength = int64(len(finalBody))
	resp.Header.Set("Content-Length", strconv.Itoa(len(finalBody)))
	// Payload digests no longer match the rewritten body.
	resp.Header.Del("ETag")
	resp.Header.Del("Digest")
	resp.Header.Del("Content-MD5")
	return nil
}

// upstreamPath maps a request path to the upstream mirror path. By default
// the prefix is kept as-is; with strip_path the matched path is replaced by
// the configured prefix instead.
func upstreamPath(source config.Source, requestPath string) string {
	if source.StripPath {
		trimmed := strings.TrimPrefix(requestPath, source.Path)
		if trimmed == requestPath {
			return requestPath
		}
		if trimmed != "" && !strings.HasPrefix(trimmed, "/") {
			return requestPath
		}
		return source.Prefix + trimmed
	}
	return source.Prefix + strings.TrimPrefix(requestPath, source.Prefix)
}

func hasBodyReplaces(replaces []config.Replace) bool {
	for _, replace := range replaces {
		if replace.Type != "header" {
			return true
		}
	}
	return false
}

func HandlerReverse(w http.ResponseWriter, r *http.Request, source config.Source) {
	targetUrl, err := url.Parse(source.Mirror)
	if err != nil || targetUrl.Host == "" {
		http.Error(w, "invalid upstream mirror", http.StatusBadGateway)
		return
	}
	proxy := httputil.NewSingleHostReverseProxy(targetUrl)
	proxy.Transport = upstreamTransport()
	proxy.Director = func(req *http.Request) {
		req.URL.Scheme = targetUrl.Scheme
		req.URL.Host = targetUrl.Host
		// Only drop the escaped path when the path is actually rewritten;
		// untouched paths must keep the client's original escaping (e.g. %2B
		// in PyTorch wheel names, which upstream CDNs require verbatim).
		if rewritten := upstreamPath(source, req.URL.Path); rewritten != req.URL.Path {
			req.URL.Path = rewritten
			req.URL.RawPath = ""
		}
		if hasBodyReplaces(source.Replaces) {
			// Sources needing body rewrites must get a decodable encoding from
			// upstream. Dropping the client's Accept-Encoding lets the Go
			// transport negotiate gzip and decompress it transparently, so
			// Brotli and other unsupported encodings can neither bypass nor
			// corrupt the rewrite.
			req.Header.Del("Accept-Encoding")
		}
		req.Host = targetUrl.Host
	}
	proxy.ModifyResponse = func(resp *http.Response) error {
		if len(source.Replaces) == 0 {
			return nil
		}

		bodyReplaces := splitAndApplyHeaderReplaces(resp, r, source.Replaces)
		if len(bodyReplaces) == 0 {
			return nil
		}
		return rewriteBody(resp, bodyReplaces, source.RewriteContentTypes)
	}
	proxy.ServeHTTP(w, r)
}

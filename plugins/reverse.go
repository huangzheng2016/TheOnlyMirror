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

const maxRewriteBodySize = 8 * 1024 * 1024 // 8 MiB

func resolveReplaceTarget(value string, r *http.Request) (string, bool) {
	switch value {
	case "<HOST>":
		return r.Host, true
	case "<TLS_SCHEME>":
		if !config.GetTlsRedirect() {
			return "", false
		}
		if config.GetTls() {
			return "https://", true
		}
		return "http://", true
	default:
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
			header := resp.Header.Get(replace.Header)
			header = strings.ReplaceAll(header, replace.Src, dst)
			resp.Header.Set(replace.Header, header)
			continue
		}

		replace.Dst = dst
		bodyReplaces = append(bodyReplaces, replace)
	}
	return bodyReplaces
}

func shouldRewriteBody(resp *http.Response) bool {
	contentType := strings.ToLower(resp.Header.Get("Content-Type"))
	if contentType == "" {
		return true
	}
	if strings.HasPrefix(contentType, "text/") {
		return true
	}
	return strings.Contains(contentType, "json") || strings.Contains(contentType, "xml") || strings.Contains(contentType, "javascript")
}

func readBodyLimited(body io.ReadCloser, maxBytes int64) ([]byte, bool, error) {
	defer body.Close()
	limited := io.LimitReader(body, maxBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, false, err
	}
	if int64(len(data)) > maxBytes {
		return nil, true, nil
	}
	return data, false, nil
}

func decodeBody(encoding string, bodyBytes []byte) ([]byte, error) {
	switch strings.ToLower(strings.TrimSpace(encoding)) {
	case "gzip":
		reader, err := gzip.NewReader(bytes.NewBuffer(bodyBytes))
		if err != nil {
			return nil, fmt.Errorf("gzip decode failed: %w", err)
		}
		defer reader.Close()
		dataDecompressed, err := io.ReadAll(reader)
		if err != nil {
			return nil, fmt.Errorf("gzip read failed: %w", err)
		}
		return dataDecompressed, nil
	default:
		return bodyBytes, nil
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

func rewriteBody(resp *http.Response, bodyReplaces []config.Replace) error {
	if !shouldRewriteBody(resp) {
		return nil
	}
	if resp.ContentLength < 0 {
		return nil
	}
	if resp.ContentLength > maxRewriteBodySize {
		return nil
	}

	bodyBytes, tooLarge, err := readBodyLimited(resp.Body, maxRewriteBodySize)
	if err != nil {
		return err
	}
	if tooLarge {
		return fmt.Errorf("response body exceeds rewrite limit")
	}

	decodedBody, err := decodeBody(resp.Header.Get("Content-Encoding"), bodyBytes)
	if err != nil {
		return err
	}

	modifiedBodyString := string(decodedBody)
	for _, replace := range bodyReplaces {
		modifiedBodyString = strings.ReplaceAll(modifiedBodyString, replace.Src, replace.Dst)
	}

	finalBody, err := encodeBody(resp.Header.Get("Content-Encoding"), []byte(modifiedBodyString))
	if err != nil {
		return err
	}

	resp.Body = io.NopCloser(bytes.NewReader(finalBody))
	resp.ContentLength = int64(len(finalBody))
	resp.Header.Set("Content-Length", strconv.Itoa(len(finalBody)))
	return nil
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
		req.URL.Path = source.Prefix + strings.TrimPrefix(req.URL.Path, source.Prefix)
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
		return rewriteBody(resp, bodyReplaces)
	}
	proxy.ServeHTTP(w, r)
}

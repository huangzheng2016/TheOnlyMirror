package test

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

const (
	defaultBaseURL      = "https://127.0.0.1:443"
	defaultTimeoutMS    = 8000
	defaultAliasDomain  = "mirrors.test"
	maxResponseReadSize = 2048
)

type configFile struct {
	HostAliases     map[string]string `json:"host_aliases"`
	SourceTemplates map[string]source `json:"source_templates"`
	Sources         map[string]source `json:"sources"`
	Proxy           []string          `json:"proxy"`
}

type source struct {
	Template bool     `json:"template"`
	BaseName string   `json:"base_name"`
	Targets  []string `json:"targets"`
	UA       string   `json:"ua"`
	Path     string   `json:"path"`
	Prefix   string   `json:"prefix"`
	Mirror   string   `json:"mirror"`
}

type connectivityCase struct {
	category string
	name     string
	path     string
	host     string
	ua       string
}

func TestAllMirrorsConnectivity(t *testing.T) {
	cfg := mustLoadConfig(t)
	allSources := expandSourcesForTest(t, cfg)

	client := buildTestClient()
	base := mustParseBaseURL(t)
	timeout := readTimeout(t)

	if !isServiceReachable(client, base, timeout) {
		t.Skipf("mirror service is not reachable at %s, set MIRROR_BASE_URL and ensure service is running", base.String())
	}

	runCases(t, client, base, timeout, buildSourceCases(allSources))
	runCases(t, client, base, timeout, buildProxyCases(t, cfg.Proxy))
	runCases(t, client, base, timeout, buildHostAliasCases(cfg.HostAliases))
}

func mustLoadConfig(t *testing.T) configFile {
	t.Helper()

	pathCandidates := []string{"config.json", "../config.json"}
	var data []byte
	var err error
	for _, candidate := range pathCandidates {
		data, err = os.ReadFile(candidate)
		if err == nil {
			break
		}
	}
	if err != nil {
		t.Fatalf("read config.json failed: %v", err)
	}

	var cfg configFile
	if err = json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("parse config.json failed: %v", err)
	}
	return cfg
}

func expandSourcesForTest(t *testing.T, cfg configFile) map[string]source {
	t.Helper()

	expanded := map[string]source{}
	for key, value := range cfg.Sources {
		expanded[key] = value
	}

	for templateName, template := range cfg.SourceTemplates {
		if !template.Template {
			t.Fatalf("source_templates.%s must set template=true", templateName)
		}
		if template.BaseName == "" || !strings.Contains(template.BaseName, "{target}") {
			t.Fatalf("source_templates.%s.base_name must contain {target}", templateName)
		}
		if len(template.Targets) == 0 {
			t.Fatalf("source_templates.%s.targets is empty", templateName)
		}

		for _, target := range template.Targets {
			target = strings.TrimSpace(target)
			if target == "" {
				t.Fatalf("source_templates.%s.targets contains empty item", templateName)
			}
			key := strings.ReplaceAll(template.BaseName, "{target}", target)
			if _, exists := expanded[key]; exists {
				t.Fatalf("source_templates.%s generated duplicate source key: %s", templateName, key)
			}
			expanded[key] = source{
				Template: false,
				UA:       strings.ReplaceAll(template.UA, "{target}", target),
				Path:     strings.ReplaceAll(template.Path, "{target}", target),
				Prefix:   strings.ReplaceAll(template.Prefix, "{target}", target),
				Mirror:   strings.ReplaceAll(template.Mirror, "{target}", target),
			}
		}
	}

	return expanded
}

func buildSourceCases(sources map[string]source) []connectivityCase {
	keys := make([]string, 0, len(sources))
	for key := range sources {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	cases := make([]connectivityCase, 0, len(keys))
	for _, key := range keys {
		item := sources[key]
		path := strings.TrimSpace(item.Path)
		if path == "" {
			path = "/"
		}
		if !strings.HasPrefix(path, "/") {
			path = "/" + path
		}
		cases = append(cases, connectivityCase{
			category: "sources",
			name:     key,
			path:     path,
			ua:       strings.TrimSpace(item.UA),
		})
	}
	return cases
}

func buildProxyCases(t *testing.T, proxyList []string) []connectivityCase {
	t.Helper()

	sorted := append([]string{}, proxyList...)
	sort.Strings(sorted)

	cases := make([]connectivityCase, 0, len(sorted))
	for _, raw := range sorted {
		targetURL, err := url.Parse(raw)
		if err != nil || targetURL.Host == "" {
			t.Fatalf("invalid proxy url in config: %s", raw)
		}
		host := strings.ToLower(targetURL.Host)
		cases = append(cases, connectivityCase{
			category: "proxy",
			name:     host,
			path:     "/" + host + "/",
		})
	}
	return cases
}

func buildHostAliasCases(hostAliases map[string]string) []connectivityCase {
	aliases := make([]string, 0, len(hostAliases))
	for alias := range hostAliases {
		aliases = append(aliases, alias)
	}
	sort.Strings(aliases)

	cases := make([]connectivityCase, 0, len(aliases))
	for _, alias := range aliases {
		cases = append(cases, connectivityCase{
			category: "host_aliases",
			name:     alias,
			path:     "/",
			host:     alias + "." + defaultAliasDomain,
		})
	}
	return cases
}

func buildTestClient() *http.Client {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	return &http.Client{
		Transport: transport,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

func mustParseBaseURL(t *testing.T) *url.URL {
	t.Helper()

	raw := strings.TrimSpace(os.Getenv("MIRROR_BASE_URL"))
	if raw == "" {
		raw = defaultBaseURL
	}
	base, err := url.Parse(raw)
	if err != nil || base.Scheme == "" || base.Host == "" {
		t.Fatalf("invalid MIRROR_BASE_URL: %s", raw)
	}
	return base
}

func readTimeout(t *testing.T) time.Duration {
	t.Helper()

	raw := strings.TrimSpace(os.Getenv("MIRROR_TEST_TIMEOUT_MS"))
	if raw == "" {
		return time.Duration(defaultTimeoutMS) * time.Millisecond
	}
	ms, err := strconv.Atoi(raw)
	if err != nil || ms <= 0 {
		t.Fatalf("invalid MIRROR_TEST_TIMEOUT_MS: %s", raw)
	}
	return time.Duration(ms) * time.Millisecond
}

func isServiceReachable(client *http.Client, base *url.URL, timeout time.Duration) bool {
	ctx, cancel := context.WithTimeout(context.Background(), timeout/2)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base.String(), nil)
	if err != nil {
		return false
	}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	_, _ = io.CopyN(io.Discard, resp.Body, 1)
	_ = resp.Body.Close()
	return true
}

func runCases(t *testing.T, client *http.Client, base *url.URL, timeout time.Duration, cases []connectivityCase) {
	t.Helper()
	for _, tc := range cases {
		tc := tc
		t.Run(tc.category+"/"+tc.name, func(t *testing.T) {
			checkConnectivity(t, client, base, timeout, tc)
		})
	}
}

func checkConnectivity(t *testing.T, client *http.Client, base *url.URL, timeout time.Duration, tc connectivityCase) {
	t.Helper()

	headStatus, headErr := doProbe(client, base, timeout, http.MethodHead, tc)
	if headErr == nil && isAcceptableStatus(headStatus) {
		return
	}

	getStatus, getErr := doProbe(client, base, timeout, http.MethodGet, tc)
	if getErr != nil {
		t.Fatalf(
			"connectivity failed: category=%s name=%s path=%s host=%s ua=%s head_status=%d head_err=%v get_err=%v",
			tc.category, tc.name, tc.path, tc.host, tc.ua, headStatus, headErr, getErr,
		)
	}
	if !isAcceptableStatus(getStatus) {
		t.Fatalf(
			"unexpected status: category=%s name=%s path=%s host=%s ua=%s head_status=%d get_status=%d",
			tc.category, tc.name, tc.path, tc.host, tc.ua, headStatus, getStatus,
		)
	}
}

func doProbe(client *http.Client, base *url.URL, timeout time.Duration, method string, tc connectivityCase) (int, error) {
	targetURL := *base
	pathPart := tc.path
	if pathPart == "" {
		pathPart = "/"
	}
	ref, err := url.Parse(pathPart)
	if err != nil {
		return 0, err
	}
	finalURL := targetURL.ResolveReference(ref)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, method, finalURL.String(), nil)
	if err != nil {
		return 0, err
	}
	if tc.host != "" {
		req.Host = tc.host
	}
	if tc.ua != "" {
		req.Header.Set("User-Agent", tc.ua)
	}
	if method == http.MethodGet {
		req.Header.Set("Range", fmt.Sprintf("bytes=0-%d", maxResponseReadSize-1))
	}

	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if method == http.MethodGet {
		_, _ = io.CopyN(io.Discard, resp.Body, maxResponseReadSize)
	}
	return resp.StatusCode, nil
}

func isAcceptableStatus(code int) bool {
	if code >= 200 && code <= 399 {
		return true
	}
	switch code {
	case http.StatusUnauthorized, http.StatusForbidden, http.StatusMethodNotAllowed:
		return true
	default:
		return false
	}
}

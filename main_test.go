package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

// lockedBuffer is a goroutine-safe log sink for tests whose handlers run on
// other goroutines (e.g. real httptest servers).
type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func captureLog(t *testing.T, fn func()) string {
	t.Helper()
	var buf lockedBuffer
	origOut, origFlags := log.Writer(), log.Flags()
	log.SetOutput(&buf)
	log.SetFlags(0)
	defer func() {
		log.SetOutput(origOut)
		log.SetFlags(origFlags)
	}()
	fn()
	return buf.String()
}

func TestNewServerTimeouts(t *testing.T) {
	srv := newServer(":8080", nil)
	if srv.Addr != ":8080" {
		t.Fatalf("unexpected addr: %s", srv.Addr)
	}
	if srv.ReadHeaderTimeout != readHeaderTimeout {
		t.Errorf("ReadHeaderTimeout = %v, want %v", srv.ReadHeaderTimeout, readHeaderTimeout)
	}
	if srv.IdleTimeout != idleTimeout {
		t.Errorf("IdleTimeout = %v, want %v", srv.IdleTimeout, idleTimeout)
	}
	if srv.WriteTimeout != 0 {
		t.Errorf("WriteTimeout = %v, want 0 so large streaming proxy responses are never truncated", srv.WriteTimeout)
	}
	if srv.ReadTimeout != 0 {
		t.Errorf("ReadTimeout = %v, want 0 so slow streaming clients are not cut off", srv.ReadTimeout)
	}
}

func TestAccessLogFieldsAndQueryRedaction(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})
	req := httptest.NewRequest(http.MethodGet, "http://mirror.example.com/token?token=supersecret&scope=repository:pull", nil)
	rec := httptest.NewRecorder()

	line := captureLog(t, func() {
		accessLog(handler).ServeHTTP(rec, req)
	})

	for _, want := range []string{"GET", "mirror.example.com", "/token", "418"} {
		if !strings.Contains(line, want) {
			t.Errorf("access log missing %q: %q", want, line)
		}
	}
	for _, secret := range []string{"supersecret", "token=", "scope=", "repository:pull"} {
		if strings.Contains(line, secret) {
			t.Errorf("query string leaked into access log (%q): %q", secret, line)
		}
	}

	fields := strings.Fields(line)
	if len(fields) != 5 {
		t.Fatalf("expected 5 log fields, got %d: %q", len(fields), line)
	}
	if d, err := time.ParseDuration(fields[4]); err != nil || d < 0 {
		t.Errorf("last field should be a duration, got %q", fields[4])
	}
}

func TestAccessLogImplicitStatus(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})
	req := httptest.NewRequest(http.MethodGet, "http://mirror.example.com/debian", nil)
	rec := httptest.NewRecorder()

	line := captureLog(t, func() {
		accessLog(handler).ServeHTTP(rec, req)
	})

	if !strings.Contains(line, " 200 ") {
		t.Errorf("handler writing body without WriteHeader should log 200: %q", line)
	}
}

func TestStatusRecorder(t *testing.T) {
	underlying := httptest.NewRecorder()
	rec := &statusRecorder{ResponseWriter: underlying}

	if rec.Unwrap() != underlying {
		t.Fatal("Unwrap should return the underlying ResponseWriter")
	}

	if _, err := rec.Write([]byte("hello")); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	if rec.status != http.StatusOK {
		t.Errorf("first Write should record implicit 200, got %d", rec.status)
	}

	rec2 := &statusRecorder{ResponseWriter: httptest.NewRecorder()}
	rec2.WriteHeader(http.StatusBadGateway)
	if rec2.status != http.StatusBadGateway {
		t.Errorf("WriteHeader(502) recorded %d", rec2.status)
	}
	if rec2.ResponseWriter.(*httptest.ResponseRecorder).Code != http.StatusBadGateway {
		t.Errorf("status not forwarded to underlying writer, got %d", rec2.ResponseWriter.(*httptest.ResponseRecorder).Code)
	}
}

func TestStatusRecorderFlushStreaming(t *testing.T) {
	underlying := httptest.NewRecorder()
	rec := &statusRecorder{ResponseWriter: underlying}

	if err := http.NewResponseController(rec).Flush(); err != nil {
		t.Fatalf("flush through recorder failed, ReverseProxy streaming would break: %v", err)
	}
	if !underlying.Flushed {
		t.Fatal("flush was not propagated to the underlying writer")
	}
}

func TestServeGracefulShutdown(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen failed: %v", err)
	}

	handlerStarted := make(chan struct{})
	handlerDone := make(chan struct{})
	mux := http.NewServeMux()
	mux.HandleFunc("/slow", func(w http.ResponseWriter, r *http.Request) {
		close(handlerStarted)
		time.Sleep(300 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		close(handlerDone)
	})

	srv := newServer(ln.Addr().String(), mux)
	ctx, cancel := context.WithCancel(context.Background())
	serveErr := make(chan error, 1)
	go func() { serveErr <- serve(ctx, srv, ln, false, "", "") }()

	reqErr := make(chan error, 1)
	go func() {
		resp, err := http.Get("http://" + ln.Addr().String() + "/slow")
		if err == nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
		}
		reqErr <- err
	}()

	<-handlerStarted
	cancel() // shutdown signal while the request is still in flight

	err = <-serveErr
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		t.Fatalf("serve returned unexpected error: %v", err)
	}

	select {
	case <-handlerDone:
	case <-time.After(2 * time.Second):
		t.Fatal("in-flight request was not drained before shutdown returned")
	}
	if err := <-reqErr; err != nil {
		t.Fatalf("in-flight request failed: %v", err)
	}

	if _, err := http.Get("http://" + ln.Addr().String() + "/slow"); err == nil {
		t.Fatal("server still accepts new connections after shutdown")
	}
}

func TestServeListenError(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen failed: %v", err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()

	srv := newServer(addr, http.NewServeMux())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	serveErr := make(chan error, 1)
	go func() { serveErr <- serve(ctx, srv, ln, false, "", "") }()

	select {
	case err := <-serveErr:
		if err == nil || errors.Is(err, http.ErrServerClosed) {
			t.Fatalf("expected listen error, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("serve did not return promptly on listener failure")
	}
}

type headerSpy struct {
	*httptest.ResponseRecorder
	codes []int
}

func (s *headerSpy) WriteHeader(code int) {
	s.codes = append(s.codes, code)
	s.ResponseRecorder.WriteHeader(code)
}

func TestStatusRecorderDuplicateFinalWriteHeader(t *testing.T) {
	spy := &headerSpy{ResponseRecorder: httptest.NewRecorder()}
	rec := &statusRecorder{ResponseWriter: spy}

	rec.WriteHeader(http.StatusOK)
	rec.WriteHeader(http.StatusInternalServerError)

	if rec.status != http.StatusOK {
		t.Errorf("duplicate final WriteHeader must not override the latched status, got %d", rec.status)
	}
	if !slices.Equal(spy.codes, []int{http.StatusOK}) {
		t.Errorf("duplicate final WriteHeader must not reach the underlying writer, forwarded %v", spy.codes)
	}
}

func TestStatusRecorderInformationalThenFinal(t *testing.T) {
	spy := &headerSpy{ResponseRecorder: httptest.NewRecorder()}
	rec := &statusRecorder{ResponseWriter: spy}

	rec.WriteHeader(http.StatusContinue)
	rec.WriteHeader(http.StatusEarlyHints)
	if rec.status != 0 {
		t.Fatalf("1xx responses must not latch a final status, got %d", rec.status)
	}

	rec.WriteHeader(http.StatusCreated)
	if rec.status != http.StatusCreated {
		t.Errorf("final status after 1xx = %d, want 201", rec.status)
	}
	if !slices.Equal(spy.codes, []int{http.StatusContinue, http.StatusEarlyHints, http.StatusCreated}) {
		t.Errorf("forwarded codes = %v, want [100 103 201]", spy.codes)
	}
}

func TestStatusRecorderSwitchingProtocols(t *testing.T) {
	spy := &headerSpy{ResponseRecorder: httptest.NewRecorder()}
	rec := &statusRecorder{ResponseWriter: spy}

	rec.WriteHeader(http.StatusSwitchingProtocols)
	if rec.status != http.StatusSwitchingProtocols {
		t.Errorf("101 must latch as the final status, got %d", rec.status)
	}

	rec.WriteHeader(http.StatusOK)
	if rec.status != http.StatusSwitchingProtocols {
		t.Errorf("status after 101 = %d, want 101", rec.status)
	}
}

func TestAccessLogPanicStillLogs(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("partial"))
		panic(http.ErrAbortHandler)
	})
	req := httptest.NewRequest(http.MethodGet, "http://mirror.example.com/blob", nil)
	rec := httptest.NewRecorder()

	var panicked any
	line := captureLog(t, func() {
		defer func() { panicked = recover() }()
		accessLog(handler).ServeHTTP(rec, req)
	})

	if panicked != http.ErrAbortHandler {
		t.Errorf("panic must propagate unchanged, got %v", panicked)
	}
	if !strings.Contains(line, "/blob") || !strings.Contains(line, " 200 ") {
		t.Errorf("stream aborted by panic is missing from the access log: %q", line)
	}
}

func TestAccessLogEscapedPath(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	req := httptest.NewRequest(http.MethodGet, "http://mirror.example.com/%0AFORGED%20STATUS%20499", nil)
	rec := httptest.NewRecorder()

	line := captureLog(t, func() {
		accessLog(handler).ServeHTTP(rec, req)
	})

	if strings.Count(line, "\n") != 1 || !strings.HasSuffix(line, "\n") {
		t.Errorf("encoded newline must not forge extra log lines: %q", line)
	}
	if !strings.Contains(line, "/%0AFORGED%20STATUS%20499") {
		t.Errorf("access log should carry the escaped path, got %q", line)
	}
}

func TestAccessLogReverseProxyTruncatedStream(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "1024")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("short"))
	}))
	defer backend.Close()

	backendURL, err := url.Parse(backend.URL)
	if err != nil {
		t.Fatalf("parse backend url: %v", err)
	}
	proxy := httputil.NewSingleHostReverseProxy(backendURL)

	var logged lockedBuffer
	origOut, origFlags := log.Writer(), log.Flags()
	log.SetOutput(&logged)
	log.SetFlags(0)
	defer func() {
		log.SetOutput(origOut)
		log.SetFlags(origFlags)
	}()

	frontend := httptest.NewServer(accessLog(proxy))
	defer frontend.Close()

	resp, err := http.Get(frontend.URL + "/blob")
	if err != nil {
		// The panic aborted the handler before the buffered headers were
		// flushed, so the client sees a bare connection error.
		t.Logf("client saw aborted stream as transport error: %v", err)
	} else {
		if _, readErr := io.ReadAll(resp.Body); readErr == nil {
			t.Error("truncated upstream body should fail the client body read")
		}
		_ = resp.Body.Close()
	}

	deadline := time.Now().Add(2 * time.Second)
	for !strings.Contains(logged.String(), "/blob") {
		if time.Now().After(deadline) {
			t.Fatal("aborted reverse proxy stream left no access log entry")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if line := logged.String(); !strings.Contains(line, " 200 ") {
		t.Errorf("aborted stream should log the committed status 200: %q", line)
	}
}

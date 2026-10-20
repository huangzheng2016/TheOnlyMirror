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
	"strings"
	"testing"
	"time"
)

func captureLog(t *testing.T, fn func()) string {
	t.Helper()
	var buf bytes.Buffer
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

package main

import (
	"TheOnlyMirror/config"
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

const (
	// readHeaderTimeout bounds header reads against slowloris-style clients.
	readHeaderTimeout = 10 * time.Second
	// idleTimeout reaps keep-alive connections idling between requests.
	idleTimeout = 120 * time.Second
	// shutdownTimeout is how long graceful shutdown waits for in-flight
	// streams (e.g. registry blob pulls) to finish before force-closing.
	shutdownTimeout = 30 * time.Second
)

// statusRecorder captures the response status for access logging.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

// Unwrap exposes the underlying writer to http.ResponseController, so
// ReverseProxy streaming (Flush) and protocol switches (Hijack) keep
// working through the recorder.
func (r *statusRecorder) Unwrap() http.ResponseWriter { return r.ResponseWriter }

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	return r.ResponseWriter.Write(b)
}

// accessLog logs method, host, path, status and duration per request.
// Only r.URL.Path is logged: the query string can carry registry tokens
// and must never end up in logs.
func accessLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w}
		next.ServeHTTP(rec, r)
		status := rec.status
		if status == 0 {
			status = http.StatusOK
		}
		log.Printf("%s %s %s %d %s", r.Method, r.Host, r.URL.Path, status, time.Since(start).Round(time.Millisecond))
	})
}

// newServer builds the mirror HTTP server. ReadTimeout and WriteTimeout stay
// zero on purpose: responses are large streaming proxy bodies (registry
// blobs), and a fixed read/write deadline would truncate long downloads.
// ReadHeaderTimeout plus IdleTimeout are the timeouts that matter here.
func newServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: readHeaderTimeout,
		IdleTimeout:       idleTimeout,
	}
}

// serve runs srv until it fails or ctx is canceled. On cancellation it
// gracefully drains in-flight requests for up to shutdownTimeout, then
// force-closes whatever is left.
func serve(ctx context.Context, srv *http.Server, ln net.Listener, useTLS bool, crt, key string) error {
	errCh := make(chan error, 1)
	go func() {
		if useTLS {
			errCh <- srv.ServeTLS(ln, crt, key)
			return
		}
		errCh <- srv.Serve(ln)
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		log.Printf("shutdown signal received, draining in-flight requests for up to %s", shutdownTimeout)
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			_ = srv.Close()
			return fmt.Errorf("graceful shutdown failed, connections force-closed: %w", err)
		}
		return <-errCh
	}
}

func main() {
	if err := config.Load(); err != nil {
		log.Fatalf("load config error: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", handler)
	mux.HandleFunc("/{path}", handler)

	srv := newServer(":"+config.GetPort(), accessLog(mux))

	ln, err := net.Listen("tcp", srv.Addr)
	if err != nil {
		log.Fatalf("Failed to listen on %s: %v", srv.Addr, err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if config.GetTls() {
		log.Println("Starting proxy server with tls on :" + config.GetPort())
		crt, key := config.GetCert()
		err = serve(ctx, srv, ln, true, crt, key)
	} else {
		log.Println("Starting proxy server on :" + config.GetPort())
		err = serve(ctx, srv, ln, false, "", "")
	}
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("Failed to start server: %v", err)
	}
}

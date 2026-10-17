// SPDX-License-Identifier: GPL-3.0-or-later
package httpx

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/runwisp/runwisp-manager/internal/api"
	"github.com/runwisp/runwisp-manager/internal/auth"
	"github.com/runwisp/runwisp-manager/internal/cfgtree"
	"github.com/runwisp/runwisp-manager/internal/daemon"
	"github.com/runwisp/runwisp-manager/internal/proxy"
	"github.com/runwisp/runwisp-manager/internal/ui"
)

// NewRouter builds the manager HTTP router. The version and commit values are
// returned by the health endpoint for operators and orchestration checks.
func NewRouter(version, commit string, logger *slog.Logger) http.Handler {
	return NewRouterWithDiscovery(version, commit, logger, nil, nil)
}

func NewRouterWithDiscovery(version, commit string, logger *slog.Logger, discovery *daemon.Discovery, networks []*net.IPNet) http.Handler {
	return NewRouterWithAuth(version, commit, logger, discovery, networks, nil)
}

func NewRouterWithAuth(version, commit string, logger *slog.Logger, discovery *daemon.Discovery, networks []*net.IPNet, authentication *auth.Authenticator) http.Handler {
	r := chi.NewRouter()
	r.Use(RequestID, SecurityHeaders, Recoverer, AccessLog(logger))
	if networks != nil {
		r.Use(RealIP(networks))
	}
	r.Get("/manager/healthz", health(version, commit))
	status := http.Handler(http.HandlerFunc(statusHandler(discovery)))
	if authentication != nil {
		status = authentication.RequireRole("viewer")(status)
	}
	r.Method(http.MethodGet, "/manager/api/status", status)
	r.Get("/manager/readyz", readyHandler(discovery))
	if authentication != nil {
		if discovery != nil && discovery.Capabilities.ConfigPath != "" {
			watcher, err := cfgtree.NewWatcher(cfgtree.Options{Root: discovery.Capabilities.ConfigPath, ReadOnly: !discovery.Capabilities.CanEdit, ReadOnlyReason: discovery.Capabilities.ReadOnlyReason}, os.Getenv("RWM_FS_POLL") == "true")
			if err == nil {
				watcher.Start(context.Background())
				configAPI := &api.ConfigAPI{Watcher: watcher, Auth: authentication}
				viewer := authentication.RequireRole("viewer")(configAPI.Routes())
				r.Mount("/manager/api", viewer)
			}
		}
		r.Mount("/manager", authentication.Routes())
		if discovery != nil {
			r.Get("/manager/bridge.js", ui.BridgeHandler(discovery.Config))
		}
	}
	r.Mount("/manager", http.StripPrefix("/manager", ui.Handler()))
	if authentication != nil && discovery != nil {
		if discovery.Config.Landing == "manager" {
			r.Get("/", func(w http.ResponseWriter, req *http.Request) { http.Redirect(w, req, "/manager/", http.StatusFound) })
		}
		r.Handle("/*", proxy.New(discovery.Client, authentication, discovery.Config, logger))
	}
	return r
}

// Serve listens for HTTP requests until the process is interrupted.
func Serve(address, version, commit string, logger *slog.Logger) error {
	logger.Info("starting manager", "address", address, "version", version, "commit", commit)
	return http.ListenAndServe(address, NewRouter(version, commit, logger))
}

func ServeWithDiscovery(address, version, commit string, logger *slog.Logger, discovery *daemon.Discovery, networks []*net.IPNet) error {
	logger.Info("starting manager", "address", address, "version", version, "commit", commit)
	return http.ListenAndServe(address, NewRouterWithDiscovery(version, commit, logger, discovery, networks))
}

func ServeWithAuth(address, version, commit string, logger *slog.Logger, discovery *daemon.Discovery, networks []*net.IPNet, authentication *auth.Authenticator) error {
	logger.Info("starting manager", "address", address, "version", version, "commit", commit)
	return http.ListenAndServe(address, NewRouterWithAuth(version, commit, logger, discovery, networks, authentication))
}

func health(version, commit string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok", "version": version, "commit": commit})
	}
}

func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if id == "" {
			id = newRequestID()
		}
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), requestIDKey{}, id)))
	})
}

func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		w.Header().Set("Content-Security-Policy", "img-src 'self' data:; connect-src 'self'; font-src 'self'; object-src 'none'; base-uri 'self'; form-action 'self'; frame-ancestors 'none'")
		if r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		next.ServeHTTP(w, r)
	})
}

func Recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recover() != nil {
				problem(w, http.StatusInternalServerError, "internal server error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func AccessLog(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			started := time.Now()
			rw := &statusWriter{ResponseWriter: w}
			next.ServeHTTP(rw, r)
			logger.Info("http request", "method", r.Method, "path", r.URL.Path, "status", rw.status(), "duration", time.Since(started), "request_id", requestID(r))
		})
	}
}

type statusWriter struct {
	http.ResponseWriter
	code int
}

func (w *statusWriter) WriteHeader(code int) { w.code = code; w.ResponseWriter.WriteHeader(code) }
func (w *statusWriter) Write(p []byte) (int, error) {
	if w.code == 0 {
		w.code = http.StatusOK
	}
	return w.ResponseWriter.Write(p)
}
func (w *statusWriter) status() int {
	if w.code == 0 {
		return http.StatusOK
	}
	return w.code
}

type requestIDKey struct{}

// requestID is intentionally header-based for now; middleware keeps the value
// on the request while avoiding query strings in access logs.
func requestID(r *http.Request) string {
	if id, ok := r.Context().Value(requestIDKey{}).(string); ok {
		return id
	}
	return r.Header.Get("X-Request-ID")
}

func newRequestID() string {
	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "unknown"
	}
	return hex.EncodeToString(b[:])
}

func problem(w http.ResponseWriter, status int, detail string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"type": "about:blank", "title": http.StatusText(status), "status": status, "detail": detail})
}

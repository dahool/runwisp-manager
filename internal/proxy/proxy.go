package proxy

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/runwisp/runwisp-manager/internal/audit"
	"github.com/runwisp/runwisp-manager/internal/auth"
	"github.com/runwisp/runwisp-manager/internal/config"
	"github.com/runwisp/runwisp-manager/internal/daemon"
)

const maxRequestBody = 16 << 20
const maxHTMLBody = 8 << 20

type Handler struct {
	client daemon.Client
	auth   *auth.Authenticator
	config config.Config
	logger *slog.Logger
	audit  audit.Emitter
}

func New(client daemon.Client, authentication *auth.Authenticator, c config.Config, logger *slog.Logger) http.Handler {
	return NewWithAudit(client, authentication, c, logger, nil)
}

func NewWithAudit(client daemon.Client, authentication *auth.Authenticator, c config.Config, logger *slog.Logger, emitter audit.Emitter) http.Handler {
	return &Handler{client: client, auth: authentication, config: c, logger: logger, audit: emitter}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// SecurityHeaders runs before the catch-all route. RunWisp owns its own CSP;
	// do not apply the manager CSP to proxied origin pages.
	w.Header().Del("Content-Security-Policy")
	public := splitPaths(h.config.ProxyPublicPaths)
	if len(public) == 0 {
		public = []string{"/health", "/api/hooks/"}
	}
	class := classify(r.Method, r.URL.Path, public, h.config.ProxyExposeMetrics)
	if class == Denied {
		h.logDenied(r)
		h.emit(r, audit.Event{Action: "proxy.denied", RemoteIP: r.RemoteAddr, Outcome: "denied", Success: false, Reason: "path or method denied", TargetFile: r.URL.Path})
		problem(w, http.StatusForbidden, "RunWisp path is not available through the manager")
		return
	}
	identity, authenticated := h.auth.IdentityFromRequest(r)
	if class != Public && !authenticated {
		h.unauthenticated(w, r)
		return
	}
	if minimum := minimumRole(class); minimum != "" && rank(identity.Role) < rank(minimum) {
		h.emit(r, audit.Event{Action: "proxy.denied", Subject: identity.Sub, Username: identity.Username, Role: identity.Role, RemoteIP: r.RemoteAddr, Outcome: "denied", Success: false, Reason: "insufficient role", TargetFile: r.URL.Path})
		problem(w, http.StatusForbidden, "insufficient role")
		return
	}
	if class == Synth {
		h.status(w, r, identity)
		return
	}
	if r.Body != nil {
		r.Body = http.MaxBytesReader(w, r.Body, maxRequestBody)
	}
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	if strings.HasPrefix(r.Header.Get("Accept"), "text/event-stream") {
		go h.expiryCloser(ctx, cancel, r)
	}
	r = r.WithContext(ctx)
	proxy := h.reverseProxy()
	proxy.ServeHTTP(w, r)
}

func (h *Handler) reverseProxy() *httputil.ReverseProxy {
	target := h.client.BaseURL()
	p := &httputil.ReverseProxy{
		Director: func(r *http.Request) {
			r.URL.Scheme, r.URL.Host = target.Scheme, target.Host
			r.URL.Path = cleanPath(r.URL.Path)
			for _, key := range []string{"Authorization", "Cookie", "X-Forwarded-For", "X-Forwarded-Proto", "X-Forwarded-Host", "Forwarded", "X-Real-IP"} {
				r.Header.Del(key)
			}
			if r.Header.Get("Accept") == "" || strings.Contains(r.Header.Get("Accept"), "text/html") {
				r.Header.Set("Accept-Encoding", "identity")
			}
			r.Header.Set("X-Forwarded-Proto", requestProto(r))
			r.Header.Set("X-Forwarded-Host", r.Host)
			r.Header.Set("X-Accel-Buffering", "no")
		},
		Transport:     h.client.RoundTripper(),
		FlushInterval: -1,
		ModifyResponse: func(resp *http.Response) error {
			if resp.Request != nil && classForPath(resp.Request.Method, resp.Request.URL.Path) == Operate {
				h.emit(resp.Request, audit.Event{Action: operateAction(resp.Request.Method, resp.Request.URL.Path), RemoteIP: resp.Request.RemoteAddr, Outcome: outcome(resp.StatusCode), Success: resp.StatusCode < 400, TargetFile: resp.Request.URL.Path})
			}
			resp.Header.Del("Set-Cookie")
			addMissingHeaders(resp.Header, resp.Request != nil && resp.Request.URL.Scheme == "https")
			if resp.Request != nil && strings.HasPrefix(resp.Request.Header.Get("Accept"), "text/event-stream") {
				resp.Header.Set("X-Accel-Buffering", "no")
				resp.Header.Del("Content-Encoding")
				return nil
			}
			if resp.StatusCode == http.StatusOK && resp.Request != nil && resp.Request.Method != http.MethodHead && strings.HasPrefix(strings.ToLower(resp.Header.Get("Content-Type")), "text/html") {
				return injectHTML(resp)
			}
			return nil
		},
		ErrorHandler: func(w http.ResponseWriter, _ *http.Request, err error) {
			if h.logger != nil {
				h.logger.Error("RunWisp proxy failed", "error", err)
			}
			problem(w, http.StatusBadGateway, "RunWisp is unavailable")
		},
	}
	return p
}

func (h *Handler) status(w http.ResponseWriter, r *http.Request, identity auth.Identity) {
	var value map[string]any
	if err := h.client.Do(r.Context(), http.MethodGet, "/api/auth/status", nil, &value); err != nil {
		problem(w, http.StatusBadGateway, "RunWisp is unavailable")
		return
	}
	for key := range value {
		lower := strings.ToLower(key)
		if strings.Contains(lower, "password") || strings.Contains(lower, "chap") || strings.Contains(lower, "credential") || strings.Contains(lower, "token") {
			delete(value, key)
		}
	}
	value["authenticated"], value["readOnly"] = true, rank(identity.Role) < rank("operator")
	if _, ok := value["name"]; ok {
		value["name"] = identity.Name
	}
	writeJSON(w, http.StatusOK, value)
}

func (h *Handler) unauthenticated(w http.ResponseWriter, r *http.Request) {
	if strings.Contains(r.Header.Get("Accept"), "text/html") && !strings.HasPrefix(r.URL.Path, "/api/") {
		href := "/manager/auth/login?return_to=" + url.QueryEscape(r.URL.RequestURI())
		http.Redirect(w, r, href, http.StatusFound)
		return
	}
	problem(w, http.StatusUnauthorized, "authentication required")
}

func (h *Handler) expiryCloser(ctx context.Context, cancel context.CancelFunc, r *http.Request) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if _, ok := h.auth.IdentityFromRequest(r); !ok {
				cancel()
				return
			}
		}
	}
}

func injectHTML(resp *http.Response) error {
	var reader io.Reader = resp.Body
	if strings.EqualFold(resp.Header.Get("Content-Encoding"), "gzip") {
		decoded, err := gzip.NewReader(resp.Body)
		if err != nil {
			return err
		}
		defer decoded.Close()
		reader = decoded
	}
	data, err := io.ReadAll(io.LimitReader(reader, maxHTMLBody+1))
	if err != nil {
		return err
	}
	_ = resp.Body.Close()
	if int64(len(data)) > maxHTMLBody {
		return fmt.Errorf("upstream HTML exceeds limit")
	}
	script := []byte(`<script src="/manager/bridge.js" defer></script>`)
	lower := bytes.ToLower(data)
	marker := bytes.Index(lower, []byte("</head>"))
	if marker < 0 {
		marker = bytes.Index(lower, []byte("</body>"))
	}
	if marker < 0 {
		data = append(data, script...)
	} else {
		data = append(data[:marker], append(script, data[marker:]...)...)
	}
	resp.Body, resp.ContentLength = io.NopCloser(bytes.NewReader(data)), int64(len(data))
	resp.Header.Set("Content-Length", fmt.Sprint(len(data)))
	resp.Header.Del("Content-Encoding")
	if resp.Header.Get("Cache-Control") == "" {
		resp.Header.Set("Cache-Control", "no-store")
	}
	return nil
}

func addMissingHeaders(header http.Header, https bool) {
	if header.Get("X-Content-Type-Options") == "" {
		header.Set("X-Content-Type-Options", "nosniff")
	}
	if header.Get("Referrer-Policy") == "" {
		header.Set("Referrer-Policy", "strict-origin-when-cross-origin")
	}
	if header.Get("X-Frame-Options") == "" {
		header.Set("X-Frame-Options", "DENY")
	}
	if https && header.Get("Strict-Transport-Security") == "" {
		header.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
	}
}

func cleanPath(value string) string {
	if value == "" {
		return "/"
	}
	return value
}
func splitPaths(value string) []string {
	var out []string
	for _, item := range strings.Split(value, ",") {
		if item = strings.TrimSpace(item); item != "" {
			out = append(out, item)
		}
	}
	return out
}
func requestProto(r *http.Request) string {
	if r.TLS != nil {
		return "https"
	}
	if value := r.Header.Get("X-Forwarded-Proto"); value == "https" {
		return "https"
	}
	return "http"
}
func (h *Handler) logDenied(r *http.Request) {
	if h.logger != nil {
		h.logger.Warn("proxy path denied", "method", r.Method, "path", r.URL.Path)
	}
}

func (h *Handler) emit(r *http.Request, event audit.Event) {
	if h.audit != nil {
		h.audit.Emit(r.Context(), event)
	}
}
func classForPath(method, path string) Class { return classify(method, path, nil, false) }
func outcome(status int) string {
	if status < 400 {
		return "ok"
	}
	if status == http.StatusForbidden || status == http.StatusUnauthorized {
		return "denied"
	}
	return "error"
}
func operateAction(method, requestPath string) string {
	parts := strings.Split(strings.Trim(requestPath, "/"), "/")
	verb := "operate"
	if len(parts) > 0 {
		verb = parts[len(parts)-1]
	}
	if method == http.MethodDelete {
		verb = "delete"
	}
	return "run." + verb
}
func problem(w http.ResponseWriter, status int, detail string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"type": "about:blank", "title": http.StatusText(status), "status": status, "detail": detail})
}
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

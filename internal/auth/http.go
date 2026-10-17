package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/runwisp/runwisp-manager/internal/audit"
	"github.com/runwisp/runwisp-manager/internal/config"
)

type contextKey struct{}

type Authenticator struct {
	Config   config.Config
	Store    *Store
	Provider *Provider
	Disabled bool
	Audit    audit.Emitter
	mu       sync.Mutex
	attempts map[string][]time.Time
}

func NewAuthenticator(c config.Config, store *Store, provider *Provider) *Authenticator {
	return &Authenticator{Config: c, Store: store, Provider: provider, Disabled: c.AuthMode == "disabled", attempts: make(map[string][]time.Time)}
}

func (a *Authenticator) SetAudit(emitter audit.Emitter) { a.Audit = emitter }
func (a *Authenticator) StorePtr() *Store               { return a.Store }

func (a *Authenticator) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/auth/login", a.login)
	mux.HandleFunc("/auth/callback", a.callback)
	mux.HandleFunc("/auth/logout", a.logout)
	mux.Handle("/api/me", a.RequireRole("viewer")(http.HandlerFunc(a.me)))
	return mux
}

func (a *Authenticator) RequireRole(minimum string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			identity, ok := a.identity(r)
			if !ok {
				problemJSON(w, http.StatusUnauthorized, "authentication required")
				return
			}
			if rank(identity.Role) < rank(minimum) {
				problemJSON(w, http.StatusForbidden, "insufficient role")
				return
			}
			if !a.Disabled && isUnsafe(r.Method) && !a.csrfOK(r) {
				problemJSON(w, http.StatusForbidden, "csrf validation failed")
				return
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), contextKey{}, identity)))
		})
	}
}

// IdentityFromRequest returns the authenticated browser identity, if any.
// It is intentionally read-only so downstream handlers cannot forge roles.
func (a *Authenticator) IdentityFromRequest(r *http.Request) (Identity, bool) {
	return a.identity(r)
}

func (a *Authenticator) login(w http.ResponseWriter, r *http.Request) {
	if !a.allow(r.RemoteAddr) {
		browserError(w, http.StatusTooManyRequests, "too many login attempts")
		return
	}
	returnTo, err := validateReturnTo(r.URL.Query().Get("return_to"), landingPath(a.Config.Landing))
	if err != nil {
		browserError(w, http.StatusBadRequest, err.Error())
		return
	}
	if a.Disabled {
		http.Redirect(w, r, returnTo, http.StatusFound)
		return
	}
	if a.Provider == nil || a.Store == nil {
		browserError(w, http.StatusInternalServerError, "authentication is not configured")
		return
	}
	state, err := randomString(32)
	if err != nil {
		browserError(w, http.StatusInternalServerError, "could not start login")
		return
	}
	nonce, err := randomString(32)
	if err != nil {
		browserError(w, http.StatusInternalServerError, "could not start login")
		return
	}
	verifier, err := randomString(48)
	if err != nil {
		browserError(w, http.StatusInternalServerError, "could not start login")
		return
	}
	flow := Flow{State: state, Nonce: nonce, CodeVerifier: verifier, ReturnTo: returnTo, ExpiresAt: time.Now().Add(10 * time.Minute)}
	if err := a.Store.CreateFlow(r.Context(), flow); err != nil {
		browserError(w, http.StatusInternalServerError, "could not start login")
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "rwm_oidc_flow", Value: state, Path: "/manager/auth", HttpOnly: true, Secure: !a.Config.InsecureCookies, SameSite: http.SameSiteLaxMode, MaxAge: 600})
	extra, err := url.ParseQuery(a.Config.OIDCExtraAuthParams)
	if err != nil {
		browserError(w, http.StatusInternalServerError, "invalid authentication parameters")
		return
	}
	http.Redirect(w, r, a.Provider.AuthCodeURL(state, nonce, verifier, extra), http.StatusFound)
}

func (a *Authenticator) callback(w http.ResponseWriter, r *http.Request) {
	if !a.allow(r.RemoteAddr) {
		browserError(w, http.StatusTooManyRequests, "too many callback attempts")
		return
	}
	if a.Disabled {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	state := r.URL.Query().Get("state")
	cookie, err := r.Cookie("rwm_oidc_flow")
	if err != nil || state == "" || cookie.Value != state {
		browserError(w, http.StatusForbidden, "invalid login state")
		return
	}
	flow, err := a.Store.ConsumeFlow(r.Context(), state)
	if err != nil {
		a.emit(r, audit.Event{Action: "auth.denied", RemoteIP: r.RemoteAddr, Outcome: "denied", Success: false, Reason: "invalid state"})
		browserError(w, http.StatusForbidden, "invalid or expired login state")
		return
	}
	if providerError := r.URL.Query().Get("error"); providerError != "" {
		browserError(w, http.StatusForbidden, "login was denied")
		return
	}
	claims, hint, err := a.Provider.Exchange(r.Context(), r.URL.Query().Get("code"), flow.CodeVerifier, flow.Nonce)
	if err != nil {
		a.emit(r, audit.Event{Action: "auth.denied", RemoteIP: r.RemoteAddr, Outcome: "denied", Success: false, Reason: "token verification failed"})
		browserError(w, http.StatusForbidden, "login could not be verified")
		return
	}
	identity, err := ResolveIdentity(a.Config, claims)
	if err != nil {
		a.emit(r, audit.Event{Action: "auth.denied", Subject: identity.Sub, Username: identity.Username, RemoteIP: r.RemoteAddr, Outcome: "denied", Success: false, Reason: "identity denied"})
		browserError(w, http.StatusForbidden, "login was denied")
		return
	}
	ttl, _ := time.ParseDuration(a.Config.SessionTTL)
	now := time.Now()
	sessionID, err := a.Store.CreateSession(r.Context(), Session{Sub: identity.Sub, Username: identity.Username, Name: identity.Name, Email: identity.Email, Role: identity.Role, Groups: identity.Groups, CreatedAt: now, LastSeenAt: now, ExpiresAt: now.Add(ttl), IP: r.RemoteAddr, UserAgent: r.UserAgent(), IDTokenHint: hint})
	if err != nil {
		browserError(w, http.StatusInternalServerError, "could not create session")
		return
	}
	setSessionCookie(w, a.Config, sessionID, ttl)
	a.emit(r, audit.Event{Action: "auth.login", Subject: identity.Sub, Username: identity.Username, Role: identity.Role, RemoteIP: r.RemoteAddr, Outcome: "ok", Success: true})
	http.SetCookie(w, &http.Cookie{Name: "rwm_oidc_flow", Value: "", Path: "/manager/auth", HttpOnly: true, MaxAge: -1})
	http.Redirect(w, r, flow.ReturnTo, http.StatusFound)
}

func (a *Authenticator) logout(w http.ResponseWriter, r *http.Request) {
	session, ok := a.identity(r)
	if !ok {
		http.Redirect(w, r, "/manager/auth/login", http.StatusFound)
		return
	}
	if !a.Disabled && !a.csrfOK(r) {
		problemJSON(w, http.StatusForbidden, "csrf validation failed")
		return
	}
	if cookie, err := r.Cookie(sessionCookieName(a.Config)); err == nil && a.Store != nil {
		_ = a.Store.DeleteSession(r.Context(), cookie.Value)
	}
	a.emit(r, audit.Event{Action: "auth.logout", Subject: session.Sub, Username: session.Username, Role: session.Role, RemoteIP: r.RemoteAddr, Outcome: "ok", Success: true})
	clearSessionCookie(w, a.Config)
	if a.Provider != nil && a.Provider.EndSessionURL != "" {
		target, _ := url.Parse(a.Provider.EndSessionURL)
		query := target.Query()
		query.Set("post_logout_redirect_uri", strings.TrimRight(a.Config.ExternalURL, "/")+"/manager/auth/login")
		if session.IDTokenHint != "" {
			query.Set("id_token_hint", session.IDTokenHint)
		}
		target.RawQuery = query.Encode()
		http.Redirect(w, r, target.String(), http.StatusFound)
		return
	}
	http.Redirect(w, r, "/manager/auth/login", http.StatusFound)
}

func (a *Authenticator) me(w http.ResponseWriter, r *http.Request) {
	identity := r.Context().Value(contextKey{}).(Identity)
	session, _ := a.identity(r)
	writeJSON(w, http.StatusOK, map[string]any{"username": identity.Username, "name": identity.Name, "email": identity.Email, "role": identity.Role, "groups": identity.Groups, "expiresAt": session.ExpiresAt, "devMode": a.Disabled})
}

func (a *Authenticator) identity(r *http.Request) (Identity, bool) {
	if a.Disabled {
		return Identity{Sub: "dev", Username: "dev", Name: "Development", Role: "admin"}, true
	}
	cookie, err := r.Cookie(sessionCookieName(a.Config))
	if err != nil || a.Store == nil {
		return Identity{}, false
	}
	session, err := a.Store.GetSession(r.Context(), cookie.Value, durationOr(a.Config.SessionIdle, time.Hour))
	if err != nil {
		return Identity{}, false
	}
	return Identity{Sub: session.Sub, Username: session.Username, Name: session.Name, Email: session.Email, Role: session.Role, Groups: session.Groups, ExpiresAt: session.ExpiresAt, IDTokenHint: session.IDTokenHint}, true
}

func (a *Authenticator) csrfOK(r *http.Request) bool {
	if !isUnsafe(r.Method) {
		return true
	}
	if r.Header.Get("Content-Type") != "" && !strings.HasPrefix(strings.ToLower(r.Header.Get("Content-Type")), "application/json") {
		return false
	}
	reference := r.Header.Get("Origin")
	if reference == "" {
		reference = r.Header.Get("Referer")
	}
	if reference == "" {
		return false
	}
	got, err := url.Parse(reference)
	expected, expectedErr := url.Parse(a.Config.ExternalURL)
	if err != nil || expectedErr != nil {
		return false
	}
	return strings.EqualFold(got.Scheme, expected.Scheme) && strings.EqualFold(got.Host, expected.Host)
}

func (a *Authenticator) allow(remote string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	now := time.Now()
	cutoff := now.Add(-time.Minute)
	attempts := a.attempts[remote][:0]
	for _, at := range attempts {
		if at.After(cutoff) {
			attempts = append(attempts, at)
		}
	}
	if len(attempts) >= 20 {
		a.attempts[remote] = attempts
		return false
	}
	a.attempts[remote] = append(attempts, now)
	return true
}

func validateReturnTo(value, fallback string) (string, error) {
	if value == "" {
		value = fallback
	}
	if value == "" {
		value = "/"
	}
	parsed, err := url.Parse(value)
	if err != nil || !strings.HasPrefix(value, "/") || strings.HasPrefix(value, "//") || strings.Contains(value, "\\") || parsed.IsAbs() || parsed.Host != "" {
		return "", errors.New("return_to must be a relative path")
	}
	return value, nil
}
func landingPath(value string) string {
	switch value {
	case "manager":
		return "/manager/"
	case "runwisp", "":
		return "/"
	default:
		return value
	}
}
func randomString(size int) (string, error) {
	data := make([]byte, size)
	if _, err := rand.Read(data); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(data), nil
}
func isUnsafe(method string) bool {
	return method != http.MethodGet && method != http.MethodHead && method != http.MethodOptions
}
func durationOr(value string, fallback time.Duration) time.Duration {
	parsed, err := time.ParseDuration(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}
func sessionCookieName(c config.Config) string {
	if c.InsecureCookies {
		return "rwm_session"
	}
	return "__Host-rwm_session"
}
func setSessionCookie(w http.ResponseWriter, c config.Config, value string, ttl time.Duration) {
	http.SetCookie(w, &http.Cookie{Name: sessionCookieName(c), Value: value, Path: "/", HttpOnly: true, Secure: !c.InsecureCookies, SameSite: http.SameSiteLaxMode, MaxAge: int(ttl.Seconds())})
}
func clearSessionCookie(w http.ResponseWriter, c config.Config) {
	http.SetCookie(w, &http.Cookie{Name: sessionCookieName(c), Value: "", Path: "/", HttpOnly: true, Secure: !c.InsecureCookies, SameSite: http.SameSiteLaxMode, MaxAge: -1})
}
func browserError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = fmt.Fprintf(w, "<!doctype html><title>Authentication error</title><p>%s</p>", template.HTMLEscapeString(message))
}
func problemJSON(w http.ResponseWriter, status int, detail string) {
	writeJSON(w, status, map[string]any{"type": "about:blank", "title": http.StatusText(status), "status": status, "detail": detail})
}
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func (a *Authenticator) emit(r *http.Request, event audit.Event) {
	if a.Audit != nil {
		a.Audit.Emit(r.Context(), event)
	}
}

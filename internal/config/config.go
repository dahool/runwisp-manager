// SPDX-License-Identifier: GPL-3.0-or-later
// Package config loads and validates RunWisp Manager configuration.
package config

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Config is the complete Phase 02 environment configuration.
type Config struct {
	Listen                    string `env:"RWM_LISTEN" default:"127.0.0.1:9478" doc:"Manager listen address"`
	ExternalURL               string `env:"RWM_EXTERNAL_URL" doc:"Public manager URL"`
	StateDir                  string `env:"RWM_STATE_DIR" default:".runwisp-manager" doc:"Manager state directory"`
	LogLevel                  string `env:"RWM_LOG_LEVEL" default:"info" doc:"Log level"`
	LogFormat                 string `env:"RWM_LOG_FORMAT" default:"auto" doc:"Log format"`
	TLSCertFile               string `env:"RWM_TLS_CERT_FILE" doc:"TLS certificate file"`
	TLSKeyFile                string `env:"RWM_TLS_KEY_FILE" doc:"TLS private key file" secret:"true"`
	TrustedProxies            string `env:"RWM_TRUSTED_PROXIES" doc:"Trusted proxy CIDRs"`
	RunWispSocket             string `env:"RWM_RUNWISP_SOCKET" doc:"RunWisp Unix socket"`
	RunWispDataDir            string `env:"RWM_RUNWISP_DATA_DIR" default:".runwisp" doc:"RunWisp data directory"`
	RunWispURL                string `env:"RWM_RUNWISP_URL" doc:"RunWisp HTTP URL"`
	RunWispPassword           string `env:"RWM_RUNWISP_PASSWORD" doc:"RunWisp HTTP password" secret:"true"`
	RunWispCAFile             string `env:"RWM_RUNWISP_CA_FILE" doc:"RunWisp CA file"`
	RunWispInsecureSkipVerify bool   `env:"RWM_RUNWISP_INSECURE_SKIP_VERIFY" doc:"Skip RunWisp TLS verification"`
	RunWispConfig             string `env:"RWM_RUNWISP_CONFIG" doc:"RunWisp configuration path"`
	RunWispBin                string `env:"RWM_RUNWISP_BIN" default:"runwisp" doc:"RunWisp executable"`
	StrictVersion             bool   `env:"RWM_STRICT_VERSION" doc:"Refuse daemon and binary version mismatch"`
	InsecureCookies           bool   `env:"RWM_INSECURE_COOKIES" doc:"Allow insecure external cookies"`
	EditEnabled               bool   `env:"RWM_EDIT_ENABLED" default:"true" doc:"Enable manager editing"`
	AuthMode                  string `env:"RWM_AUTH_MODE" default:"oidc" doc:"Authentication mode"`
	OIDCIssuerURL             string `env:"RWM_OIDC_ISSUER_URL" doc:"OIDC issuer URL"`
	OIDCClientID              string `env:"RWM_OIDC_CLIENT_ID" doc:"OIDC client ID"`
	OIDCClientSecret          string `env:"RWM_OIDC_CLIENT_SECRET" secret:"true" doc:"OIDC client secret"`
	OIDCScopes                string `env:"RWM_OIDC_SCOPES" default:"openid profile email" doc:"OIDC scopes"`
	OIDCClaimUsername         string `env:"RWM_OIDC_CLAIM_USERNAME" default:"preferred_username" doc:"OIDC username claim"`
	OIDCClaimName             string `env:"RWM_OIDC_CLAIM_NAME" default:"name" doc:"OIDC name claim"`
	OIDCClaimEmail            string `env:"RWM_OIDC_CLAIM_EMAIL" default:"email" doc:"OIDC email claim"`
	OIDCClaimGroups           string `env:"RWM_OIDC_CLAIM_GROUPS" default:"groups" doc:"OIDC groups claim"`
	OIDCRoleAdminGroups       string `env:"RWM_OIDC_ROLE_ADMIN_GROUPS" doc:"OIDC admin groups"`
	OIDCRoleEditorGroups      string `env:"RWM_OIDC_ROLE_EDITOR_GROUPS" doc:"OIDC editor groups"`
	OIDCRoleOperatorGroups    string `env:"RWM_OIDC_ROLE_OPERATOR_GROUPS" doc:"OIDC operator groups"`
	OIDCRoleViewerGroups      string `env:"RWM_OIDC_ROLE_VIEWER_GROUPS" doc:"OIDC viewer groups"`
	OIDCDefaultRole           string `env:"RWM_OIDC_DEFAULT_ROLE" doc:"Default OIDC role"`
	OIDCAllowedDomains        string `env:"RWM_OIDC_ALLOWED_DOMAINS" doc:"Allowed email domains"`
	OIDCAllowedEmails         string `env:"RWM_OIDC_ALLOWED_EMAILS" doc:"Allowed email addresses"`
	OIDCRequireEmailVerified  bool   `env:"RWM_OIDC_REQUIRE_EMAIL_VERIFIED" default:"true" doc:"Require verified email"`
	OIDCUserinfoClaims        bool   `env:"RWM_OIDC_USERINFO_CLAIMS" doc:"Merge OIDC userinfo claims"`
	OIDCExtraAuthParams       string `env:"RWM_OIDC_EXTRA_AUTH_PARAMS" doc:"Extra OIDC authorization parameters"`
	OIDCCAFile                string `env:"RWM_OIDC_CA_FILE" doc:"OIDC CA file"`
	OIDCInsecureSkipTLSVerify bool   `env:"RWM_OIDC_INSECURE_SKIP_TLS_VERIFY" doc:"Skip OIDC TLS verification"`
	SessionTTL                string `env:"RWM_SESSION_TTL" default:"12h" doc:"Absolute session lifetime"`
	SessionIdle               string `env:"RWM_SESSION_IDLE" default:"1h" doc:"Idle session lifetime"`
	SessionSecret             string `env:"RWM_SESSION_SECRET" secret:"true" doc:"Session HMAC secret"`
	Landing                   string `env:"RWM_LANDING" default:"/" doc:"Post-login landing path"`
}

// Load reads the process environment, applying defaults and secret _FILE values.
func Load(environ []string) (Config, error) {
	c := Config{}
	fields, names := fieldsByEnv()
	values := make(map[string]string, len(environ))
	for _, item := range environ {
		key, value, ok := strings.Cut(item, "=")
		if ok {
			values[key] = value
		}
	}
	var problems []string
	for key := range values {
		if strings.HasPrefix(key, "RWM_") && fields[key] == nil && fields[key+"_FILE"] == nil {
			problems = append(problems, fmt.Sprintf("unknown variable %s (did you mean %s?)", key, closest(key, names)))
		}
	}
	rv := reflect.ValueOf(&c).Elem()
	rt := rv.Type()
	for i := 0; i < rt.NumField(); i++ {
		field := rt.Field(i)
		key := field.Tag.Get("env")
		value, present := values[key]
		if field.Tag.Get("secret") == "true" {
			if file, ok := values[key+"_FILE"]; ok {
				data, err := os.ReadFile(file)
				if err != nil {
					problems = append(problems, fmt.Sprintf("%s_FILE: read secret: %v", key, err))
				} else {
					value = strings.TrimRight(string(data), "\r\n")
					present = true
				}
			}
		}
		if !present {
			value, present = field.Tag.Lookup("default")
		}
		if present {
			if err := setValue(rv.Field(i), value); err != nil {
				problems = append(problems, fmt.Sprintf("%s: %v", key, err))
			}
		}
	}
	problems = append(problems, c.validate()...)
	if len(problems) > 0 {
		sort.Strings(problems)
		return c, errors.New(strings.Join(problems, "\n"))
	}
	return c, nil
}

func (c Config) validate() []string {
	var problems []string
	if c.RunWispSocket != "" && c.RunWispURL != "" {
		problems = append(problems, "RWM_RUNWISP_SOCKET and RWM_RUNWISP_URL are mutually exclusive")
	}
	if c.TLSCertFile != "" && c.TLSKeyFile == "" {
		problems = append(problems, "RWM_TLS_CERT_FILE requires RWM_TLS_KEY_FILE")
	}
	if c.RunWispURL != "" {
		if u, err := url.Parse(c.RunWispURL); err != nil || u.Scheme != "http" && u.Scheme != "https" || u.Host == "" {
			problems = append(problems, "RWM_RUNWISP_URL must be an absolute HTTP(S) URL")
		}
	}
	if c.ExternalURL != "" {
		u, err := url.Parse(c.ExternalURL)
		loopback := err == nil && (u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1" || u.Hostname() == "::1")
		if err != nil || !u.IsAbs() || u.Host == "" || (u.Scheme != "https" && !(loopback || c.InsecureCookies)) {
			problems = append(problems, "RWM_EXTERNAL_URL must be an absolute https:// URL unless loopback or RWM_INSECURE_COOKIES=true")
		}
	}
	if c.AuthMode != "oidc" && c.AuthMode != "disabled" {
		problems = append(problems, "RWM_AUTH_MODE must be oidc or disabled")
	}
	if c.InsecureCookies && c.ExternalURL != "" && strings.HasPrefix(strings.ToLower(c.ExternalURL), "https://") {
		problems = append(problems, "RWM_INSECURE_COOKIES cannot be used with an https RWM_EXTERNAL_URL")
	}
	if c.AuthMode == "disabled" && c.Listen != "" && !isLoopbackListen(c.Listen) {
		problems = append(problems, "RWM_AUTH_MODE=disabled requires a loopback RWM_LISTEN")
	}
	if c.AuthMode == "oidc" && (c.OIDCIssuerURL != "" || c.OIDCClientID != "" || c.OIDCClientSecret != "" || c.SessionSecret != "") {
		problems = append(problems, c.validateOIDC(false)...)
	}
	for _, value := range splitCSV(c.TrustedProxies) {
		_, network, err := net.ParseCIDR(value)
		if err != nil {
			problems = append(problems, fmt.Sprintf("RWM_TRUSTED_PROXIES: invalid CIDR %q", value))
		} else if network.String() == "0.0.0.0/0" || network.String() == "::/0" {
			problems = append(problems, fmt.Sprintf("RWM_TRUSTED_PROXIES: %q is not allowed", value))
		}
	}
	return problems
}

// ValidateAuth returns strict authentication configuration errors. Load keeps
// the no-environment baseline usable for existing local commands; serve and
// check-config call this method before starting an authenticated service.
func (c Config) ValidateAuth() []string { return c.validateOIDC(true) }

func (c Config) validateOIDC(strict bool) []string {
	var problems []string
	if c.AuthMode == "disabled" {
		if !isLoopbackListen(c.Listen) {
			problems = append(problems, "RWM_AUTH_MODE=disabled requires a loopback RWM_LISTEN")
		}
		return problems
	}
	if strict || c.OIDCIssuerURL != "" || c.OIDCClientID != "" || c.OIDCClientSecret != "" || c.SessionSecret != "" {
		if c.ExternalURL == "" {
			problems = append(problems, "RWM_EXTERNAL_URL is required in oidc mode")
		}
		if c.OIDCIssuerURL == "" {
			problems = append(problems, "RWM_OIDC_ISSUER_URL is required in oidc mode")
		}
		if c.OIDCClientID == "" {
			problems = append(problems, "RWM_OIDC_CLIENT_ID is required in oidc mode")
		}
		if c.OIDCClientSecret == "" {
			problems = append(problems, "RWM_OIDC_CLIENT_SECRET or _FILE is required in oidc mode")
		}
		if len([]byte(c.SessionSecret)) < 32 {
			problems = append(problems, "RWM_SESSION_SECRET must contain at least 32 bytes")
		}
		if c.OIDCDefaultRole != "" && !validRole(c.OIDCDefaultRole) {
			problems = append(problems, "RWM_OIDC_DEFAULT_ROLE must be viewer, operator, editor, or admin")
		}
		if c.OIDCRoleAdminGroups == "" && c.OIDCRoleEditorGroups == "" && c.OIDCRoleOperatorGroups == "" && c.OIDCRoleViewerGroups == "" && c.OIDCDefaultRole == "" {
			problems = append(problems, "OIDC role mapping requires at least one role group or RWM_OIDC_DEFAULT_ROLE")
		}
		if _, err := time.ParseDuration(c.SessionTTL); err != nil {
			problems = append(problems, "RWM_SESSION_TTL must be a duration")
		}
		if _, err := time.ParseDuration(c.SessionIdle); err != nil {
			problems = append(problems, "RWM_SESSION_IDLE must be a duration")
		}
	}
	return problems
}

func validRole(role string) bool {
	return role == "viewer" || role == "operator" || role == "editor" || role == "admin"
}

func isLoopbackListen(address string) bool {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return false
	}
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func setValue(dst reflect.Value, value string) error {
	switch dst.Kind() {
	case reflect.String:
		dst.SetString(value)
	case reflect.Bool:
		v, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("invalid boolean %q", value)
		}
		dst.SetBool(v)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		v, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid integer %q", value)
		}
		dst.SetInt(v)
	default:
		return fmt.Errorf("unsupported configuration type %s", dst.Type())
	}
	return nil
}

func splitCSV(value string) []string {
	var out []string
	for _, item := range strings.Split(value, ",") {
		if item = strings.TrimSpace(item); item != "" {
			out = append(out, item)
		}
	}
	return out
}

func fieldsByEnv() (map[string]*reflect.StructField, []string) {
	result := map[string]*reflect.StructField{}
	var names []string
	t := reflect.TypeOf(Config{})
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		result[f.Tag.Get("env")] = &f
		names = append(names, f.Tag.Get("env"))
		if f.Tag.Get("secret") == "true" {
			result[f.Tag.Get("env")+"_FILE"] = &f
		}
	}
	return result, names
}

func closest(input string, choices []string) string {
	best, score := "RWM_LISTEN", 1<<30
	for _, choice := range choices {
		if d := levenshtein(input, choice); d < score {
			best, score = choice, d
		}
	}
	return best
}
func levenshtein(a, b string) int {
	prev := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i, ar := range a {
		cur := make([]int, len(b)+1)
		cur[0] = i + 1
		for j, br := range b {
			cost := 0
			if ar != br {
				cost = 1
			}
			cur[j+1] = min(cur[j]+1, prev[j+1]+1, prev[j]+cost)
		}
		prev = cur
	}
	return prev[len(b)]
}

// Effective returns a redacted map suitable for check-config output.
func (c Config) Effective() map[string]string {
	out := map[string]string{}
	v := reflect.ValueOf(c)
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		value := fmt.Sprint(v.Field(i).Interface())
		if f.Tag.Get("secret") == "true" && value != "" {
			value = "***"
		}
		out[f.Tag.Get("env")] = value
	}
	return out
}

// SocketPath returns the configured socket or the conventional data directory socket.
func (c Config) SocketPath() string {
	if c.RunWispSocket != "" {
		return c.RunWispSocket
	}
	return c.RunWispDataDir + string(os.PathSeparator) + "runwisp.sock"
}

// TrustedProxyNetworks parses the trusted proxy list.
func (c Config) TrustedProxyNetworks() ([]*net.IPNet, error) {
	var out []*net.IPNet
	for _, value := range splitCSV(c.TrustedProxies) {
		_, network, err := net.ParseCIDR(value)
		if err != nil {
			return nil, fmt.Errorf("parse trusted proxy %q: %w", value, err)
		}
		out = append(out, network)
	}
	return out, nil
}

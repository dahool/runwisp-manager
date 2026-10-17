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

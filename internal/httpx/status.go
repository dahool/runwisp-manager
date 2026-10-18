package httpx

import (
	"encoding/json"
	"net"
	"net/http"
	"strings"

	"github.com/runwisp/runwisp-manager/internal/daemon"
)

type Status struct {
	Transport    daemon.Transport    `json:"transport"`
	Daemon       daemon.DaemonInfo   `json:"daemon"`
	Capabilities daemon.Capabilities `json:"capabilities"`
	Warnings     []string            `json:"warnings,omitempty"`
	LastError    string              `json:"lastError,omitempty"`
}

func statusHandler(discovery *daemon.Discovery) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		value := Status{}
		if discovery != nil {
			value.Transport = discovery.Capabilities.Transport
			value.Daemon = discovery.Daemon
			value.Daemon.Version = discovery.Capabilities.DaemonVersion
			value.Capabilities = discovery.Capabilities
			value.Warnings = discovery.Capabilities.Warnings
			if discovery.LastError != nil {
				value.LastError = discovery.LastError.Error()
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(value)
	}
}
func readyHandler(discovery *daemon.Discovery) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if discovery == nil || discovery.LastError != nil || !discovery.Capabilities.ConfigReadable {
			problem(w, http.StatusServiceUnavailable, "RunWisp unreachable or configuration unreadable")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ready"})
	}
}

// RealIP trusts forwarding headers only when the direct peer is in trusted CIDRs.
func RealIP(networks []*net.IPNet) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			host, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				host = r.RemoteAddr
			}
			ip := net.ParseIP(host)
			trusted := false
			for _, network := range networks {
				if ip != nil && network.Contains(ip) {
					trusted = true
					break
				}
			}
			if trusted {
				if forwarded := strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-For"), ",")[0]); net.ParseIP(forwarded) != nil {
					_, port, err := net.SplitHostPort(r.RemoteAddr)
					if err != nil {
						port = "0"
					}
					r.RemoteAddr = net.JoinHostPort(forwarded, port)
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

package proxy

import (
	"net/http"
	"path"
	"strings"
)

type Class string

const (
	Public  Class = "public"
	Denied  Class = "denied"
	Synth   Class = "synth"
	Read    Class = "read"
	Operate Class = "operate"
	Apply   Class = "apply"
)

func minimumRole(class Class) string {
	switch class {
	case Read:
		return "viewer"
	case Operate:
		return "operator"
	case Apply:
		return "editor"
	default:
		return ""
	}
}

func rank(role string) int {
	return map[string]int{"viewer": 1, "operator": 2, "editor": 3, "admin": 4}[role]
}

func classify(method, requestPath string, public []string, exposeMetrics bool) Class {
	clean := path.Clean("/" + requestPath)
	for _, prefix := range public {
		prefix = "/" + strings.Trim(strings.TrimSpace(prefix), "/")
		if clean == prefix || strings.HasPrefix(clean, prefix+"/") {
			return Public
		}
	}
	if clean == "/api/auth/status" && method == http.MethodGet {
		return Synth
	}
	if clean == "/metrics" && exposeMetrics && method == http.MethodGet {
		return Read
	}
	if strings.HasPrefix(clean, "/api/local/") || clean == "/api/daemon/identity" || strings.HasPrefix(clean, "/api/auth/") || clean == "/metrics" {
		return Denied
	}
	if clean == "/api/daemon/reload" && method == http.MethodPost {
		return Apply
	}
	if isOperate(method, clean) {
		return Operate
	}
	if method == http.MethodGet || method == http.MethodHead {
		if strings.HasPrefix(clean, "/api/") || isSPAPath(clean) {
			return Read
		}
	}
	return Denied
}

func isOperate(method, clean string) bool {
	if !strings.HasPrefix(clean, "/api/") {
		return false
	}
	if method != http.MethodPost && method != http.MethodDelete && method != http.MethodPatch {
		return false
	}
	for _, suffix := range []string{"/run", "/start", "/stop", "/restart", "/pause", "/resume"} {
		if strings.HasSuffix(clean, suffix) {
			return strings.HasPrefix(clean, "/api/tasks/")
		}
	}
	return method == http.MethodDelete && strings.HasPrefix(clean, "/api/runs/") || method == http.MethodPost && clean == "/api/notifications/read"
}

func isSPAPath(clean string) bool {
	return clean == "/" || clean == "/notifications" || clean == "/favicon.ico" || strings.HasPrefix(clean, "/tasks/") || strings.HasPrefix(clean, "/runs/") || strings.HasPrefix(clean, "/_app/") || strings.HasPrefix(clean, "/assets/")
}

package main

import (
	"fmt"
	"os"
	"reflect"
	"strings"
)

type config struct {
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
	Landing                   string `env:"RWM_LANDING" default:"runwisp" doc:"Landing target (runwisp or manager)"`
	ProxyPublicPaths          string `env:"RWM_PROXY_PUBLIC_PATHS" default:"/health,/api/hooks/" doc:"Unauthenticated proxy paths"`
	ProxyExposeMetrics        bool   `env:"RWM_PROXY_EXPOSE_METRICS" doc:"Expose the daemon metrics endpoint"`
	Bridge                    bool   `env:"RWM_BRIDGE" default:"true" doc:"Inject the manager bridge into RunWisp HTML"`
	BridgePosition            string `env:"RWM_BRIDGE_POSITION" default:"bottom-right" doc:"Bridge pill position"`
}

func main() {
	t := reflect.TypeOf(config{})
	var b strings.Builder
	b.WriteString("# Environment variables\n\nGenerated by `go generate ./internal/config`.\n\n| Variable | Default | Secret | Description |\n| --- | --- | --- | --- |\n")
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		def := f.Tag.Get("default")
		if def == "" {
			def = "—"
		}
		secret := "no"
		if f.Tag.Get("secret") == "true" {
			secret = "yes (also supports `_FILE`)"
		}
		fmt.Fprintf(&b, "| `%s` | `%s` | %s | %s |\n", f.Tag.Get("env"), def, secret, f.Tag.Get("doc"))
	}
	if err := os.WriteFile("../../docs/ENVIRONMENT.md", []byte(b.String()), 0o644); err != nil {
		panic(err)
	}
}

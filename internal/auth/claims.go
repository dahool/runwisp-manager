package auth

import (
	"errors"
	"strings"
	"time"

	"github.com/runwisp/runwisp-manager/internal/config"
)

type Identity struct {
	Sub, Username, Name, Email, Role string
	Groups                           []string
	ExpiresAt                        time.Time
	IDTokenHint                      string
}

var roleRank = map[string]int{"viewer": 1, "operator": 2, "editor": 3, "admin": 4}

func ResolveIdentity(c config.Config, claims Claims) (Identity, error) {
	identity := Identity{Sub: stringClaim(claims, "sub"), Username: stringClaim(claims, c.OIDCClaimUsername), Name: stringClaim(claims, c.OIDCClaimName), Email: strings.ToLower(strings.TrimSpace(stringClaim(claims, c.OIDCClaimEmail))), Groups: normalizeGroups(claims[c.OIDCClaimGroups])}
	if identity.Username == "" {
		identity.Username = identity.Email
	}
	if identity.Username == "" || identity.Sub == "" {
		return Identity{}, errors.New("oidc identity is missing subject or username")
	}
	if c.OIDCRequireEmailVerified && !boolClaim(claims["email_verified"]) {
		return Identity{}, errors.New("email address is not verified")
	}
	if !allowed(identity.Email, c.OIDCAllowedDomains, c.OIDCAllowedEmails) {
		return Identity{}, errors.New("email address is not allowed")
	}
	groups := map[string]struct{}{}
	for _, group := range identity.Groups {
		groups[strings.ToLower(group)] = struct{}{}
	}
	for _, role := range []string{"admin", "editor", "operator", "viewer"} {
		if intersects(groups, roleGroups(c, role)) {
			identity.Role = role
			return identity, nil
		}
	}
	if validRole(c.OIDCDefaultRole) {
		identity.Role = c.OIDCDefaultRole
		return identity, nil
	}
	return Identity{}, errors.New("identity has no mapped role")
}

func roleGroups(c config.Config, role string) []string {
	var value string
	switch role {
	case "admin":
		value = c.OIDCRoleAdminGroups
	case "editor":
		value = c.OIDCRoleEditorGroups
	case "operator":
		value = c.OIDCRoleOperatorGroups
	case "viewer":
		value = c.OIDCRoleViewerGroups
	}
	return lowerCSV(value)
}

func normalizeGroups(value any) []string {
	var raw []string
	switch v := value.(type) {
	case string:
		raw = strings.FieldsFunc(v, func(r rune) bool { return r == ',' || r == ' ' || r == '\t' || r == '\n' })
	case []string:
		raw = v
	case []any:
		for _, item := range v {
			if text, ok := item.(string); ok {
				raw = append(raw, text)
			}
		}
	}
	seen := map[string]struct{}{}
	groups := make([]string, 0, len(raw))
	for _, group := range raw {
		group = strings.TrimSpace(group)
		if group == "" {
			continue
		}
		if _, ok := seen[group]; ok {
			continue
		}
		seen[group] = struct{}{}
		groups = append(groups, group)
	}
	return groups
}

func allowed(email, domains, emails string) bool {
	if domains == "" && emails == "" {
		return true
	}
	for _, candidate := range lowerCSV(emails) {
		if email == candidate {
			return true
		}
	}
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return false
	}
	for _, domain := range lowerCSV(domains) {
		if parts[1] == strings.TrimPrefix(domain, "@") {
			return true
		}
	}
	return false
}

func intersects(groups map[string]struct{}, wanted []string) bool {
	for _, group := range wanted {
		if _, ok := groups[group]; ok {
			return true
		}
	}
	return false
}
func lowerCSV(value string) []string {
	var out []string
	for _, item := range strings.Split(value, ",") {
		if item = strings.ToLower(strings.TrimSpace(item)); item != "" {
			out = append(out, item)
		}
	}
	return out
}
func stringClaim(claims Claims, name string) string {
	value, _ := claims[name].(string)
	return strings.TrimSpace(value)
}
func boolClaim(value any) bool {
	switch v := value.(type) {
	case bool:
		return v
	case string:
		return strings.EqualFold(v, "true")
	}
	return false
}
func rank(role string) int       { return roleRank[role] }
func validRole(role string) bool { return roleRank[role] > 0 }

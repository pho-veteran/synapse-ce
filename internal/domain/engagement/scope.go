package engagement

import (
	"net/netip"
	"path"
	"path/filepath"
	"strings"
)

// Scope defines what is in and out of bounds for an engagement.
// Scope is enforced in the execution layer, never as a single bypassable prompt/hook.
type Scope struct {
	InScope    []Target
	OutOfScope []Target
}

// TargetKind enumerates the kinds of assets an engagement can target.
type TargetKind string

const (
	TargetDomain TargetKind = "domain"
	TargetIP     TargetKind = "ip"
	TargetCIDR   TargetKind = "cidr"
	TargetURL    TargetKind = "url"
	TargetRepo   TargetKind = "repo"
	TargetImage  TargetKind = "image"
)

// Target is a single in-scope or out-of-scope asset.
type Target struct {
	Kind  TargetKind
	Value string
}

// Validate reports whether the target is a well-formed scope entry. Network
// targets use the same canonical parsing contract used at authorization and
// process-execution boundaries.
func (t Target) Validate() error {
	_, err := NormalizeTarget(t, true)
	return err
}

// Allows reports whether a raw target value is permitted by this scope. It is a
// convenience for value-only callers (SCA repo/path/URL targets): the value is
// matched exactly, plus host-centric against any host/CIDR/wildcard scope entry.
// Out-of-scope matches always win. Prefer AllowsTarget for kind-aware requests.
func (s Scope) Allows(value string) bool {
	return s.AllowsTarget(Target{Kind: InferTargetKind(value), Value: value})
}

// AllowsTarget reports whether the requested target is permitted (host-centric,
// CIDR/wildcard-aware). Matching is value-exact first (preserves SCA repo/image
// semantics), then kind-aware against each scope entry. A URL scope authorizes
// its exact canonical scheme, host, and effective port; use a domain scope for
// an intentionally host-wide authorization. Out-of-scope always wins over
// in-scope (fail closed). Enforced server-side before any tool runs.
func (s Scope) AllowsTarget(req Target) bool {
	if strings.TrimSpace(req.Value) == "" {
		return false
	}
	for _, t := range s.OutOfScope {
		if matchScopeTarget(t, req) {
			return false
		}
	}
	for _, t := range s.InScope {
		if matchScopeTarget(t, req) {
			return true
		}
	}
	return false
}

// matchScopeTarget reports whether a requested target matches a single scope
// entry. Repo/image values retain exact comparison; network kinds are parsed and
// normalized before comparison so malformed authorities fail closed.
func matchScopeTarget(scopeT, req Target) bool {
	if strings.TrimSpace(scopeT.Value) == "" || strings.TrimSpace(req.Value) == "" {
		return false
	}
	if scopeT.Kind == TargetRepo && req.Kind == TargetRepo && localPathContains(scopeT.Value, req.Value) {
		return true
	}
	if (scopeT.Kind == TargetRepo || scopeT.Kind == TargetImage) &&
		strings.EqualFold(strings.TrimSpace(scopeT.Value), strings.TrimSpace(req.Value)) {
		return true
	}

	scopeT, err := NormalizeTarget(scopeT, true)
	if err != nil {
		return false
	}

	switch scopeT.Kind {
	case TargetCIDR:
		pfx, err := netip.ParsePrefix(scopeT.Value)
		if err != nil {
			return false
		}
		ip, ok := addrOf(req.Value)
		return ok && pfx.Contains(ip)
	case TargetIP:
		a, err := netip.ParseAddr(scopeT.Value)
		if err != nil {
			return false
		}
		b, ok := addrOf(req.Value)
		return ok && a == b
	case TargetDomain:
		return domainMatches(scopeT.Value, hostOf(req.Value))
	case TargetURL:
		if req.Kind != TargetURL {
			return false
		}
		reqURL, err := NormalizeURL(req.Value)
		if err != nil {
			return false
		}
		scopeURL, err := NormalizeURL(scopeT.Value)
		return err == nil && scopeURL.Scheme == reqURL.Scheme && scopeURL.Host == reqURL.Host && scopeURL.Port == reqURL.Port
	}
	return false
}

func localPathContains(parent, child string) bool {
	p, ok := cleanLocalScopePath(parent)
	if !ok {
		return false
	}
	c, ok := cleanLocalScopePath(child)
	if !ok {
		return false
	}
	return c == p || strings.HasPrefix(c, strings.TrimRight(p, "/")+"/")
}

func cleanLocalScopePath(v string) (string, bool) {
	v = strings.TrimSpace(v)
	if v == "" {
		return "", false
	}
	slash := filepath.ToSlash(v)
	abs := strings.HasPrefix(slash, "/") ||
		(len(slash) >= 3 && ((slash[0] >= 'a' && slash[0] <= 'z') || (slash[0] >= 'A' && slash[0] <= 'Z')) && slash[1] == ':' && slash[2] == '/')
	if !abs {
		return "", false
	}
	clean := path.Clean(slash)
	if len(clean) >= 2 && clean[1] == ':' {
		clean = strings.ToLower(clean)
	}
	return clean, true
}

// domainMatches reports whether host is covered by a canonical domain scope
// pattern. A `*.example.com` pattern matches any subdomain (not the bare apex);
// a plain `example.com` pattern matches that exact host only.
func domainMatches(pattern, host string) bool {
	if host == "" {
		return false
	}
	if strings.HasPrefix(pattern, "*.") {
		suffix := pattern[1:]
		return strings.HasSuffix(host, suffix) && len(host) > len(suffix)
	}
	return pattern == host
}

// hostOf extracts a canonical host from a URL, endpoint, or bare host/IP. It
// performs no DNS resolution; malformed values return an empty host so matching
// fails closed.
func hostOf(value string) string {
	v := strings.TrimSpace(value)
	if v == "" {
		return ""
	}
	if strings.Contains(v, "://") {
		identity, err := NormalizeURL(v)
		if err != nil {
			return ""
		}
		return identity.Host
	}
	if host, _, _, err := NormalizeEndpoint(v); err == nil {
		return host
	}
	host, err := NormalizeHost(v)
	if err != nil {
		return ""
	}
	return host
}

// addrOf parses the host of a target value as an IP address (ok=false if it is a
// hostname rather than a literal IP). Hostname<->IP reconciliation (DNS) is
// intentionally NOT done here: a pure domain matcher must not resolve names – that
// would add a DNS-rebinding / SSRF surface.
func addrOf(value string) (netip.Addr, bool) {
	a, err := netip.ParseAddr(hostOf(value))
	if err != nil {
		return netip.Addr{}, false
	}
	return a, true
}

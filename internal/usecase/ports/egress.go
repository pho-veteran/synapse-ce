package ports

import "net/netip"

// EgressRule is one egress decision: allow or deny traffic to Net on Ports (empty Ports
// = all ports). Rules are matched in order; an EgressPolicy is default-DENY, so only
// explicit allow rules open anything and deny rules (out-of-scope) are ordered first.
type EgressRule struct {
	Allow bool
	Net   netip.Prefix
	Ports []uint16 // nil/empty = all ports
}

// EgressPolicy is an ordered, default-deny egress ruleset compiled from an engagement
// scope. It lives in ports so both the usecase compiler that produces it
// and the infrastructure applier that enforces it depend only inward on it. Domains can
// only be enforced once resolved, so they are carried separately for run-start
// resolution + pinning – never matched on the hostname string.
// DomainRule defers a hostname rule until the sandbox setup resolves and pins
// it. Ports follows EgressRule semantics: empty permits every port; otherwise it
// constrains each resolved IP to the declared TCP/UDP ports.
type DomainRule struct {
	Host  string
	Ports []uint16
}

// EgressPolicy is an ordered, default-deny egress ruleset. AllowDomains and
// DenyDomains remain for hostname-wide callers; AllowDomainRules and
// DenyDomainRules preserve URL scope's effective-port constraint until resolution.
type EgressPolicy struct {
	Rules            []EgressRule
	AllowDomains     []string // hostname-wide in-scope domains to resolve + allow
	DenyDomains      []string // hostname-wide out-of-scope domains to resolve + deny
	AllowDomainRules []DomainRule
	DenyDomainRules  []DomainRule
}

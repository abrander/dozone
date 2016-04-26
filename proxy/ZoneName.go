package proxy

import (
	"strings"
)

type (
	// ZoneName is a zone name without the traling dot.
	ZoneName string
)

// NewZoneName will return a new ZoneName. If zoneName contains a trailing dot,
// it will be removed.
func NewZoneName(zoneName string) ZoneName {
	return ZoneName(strings.Trim(zoneName, "."))
}

// FQDN will return a fully qualified domain name (in a DNS sense) with a
// trailing dot. If hostname is @, it will be specialcased to simply return
// the zonename.
func (z ZoneName) FQDN(hostname string) string {
	return z.String(hostname) + "."
}

// String behaves like FQDN but doesn't add the trailing dot.
func (z ZoneName) String(hostname string) string {
	if hostname == "" {
		return string(z)
	}

	if hostname == "@" {
		return string(z)
	}

	return hostname + "." + string(z)
}

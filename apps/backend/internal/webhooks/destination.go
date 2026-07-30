package webhooks

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
)

type Resolver interface {
	LookupIPAddr(context.Context, string) ([]net.IPAddr, error)
}

type DestinationPolicy struct {
	Environment  string
	AllowedHosts []string
	Resolver     Resolver
}

type UnsafeDestinationError struct {
	Reason string
}

func (e *UnsafeDestinationError) Error() string {
	return "unsafe webhook destination: " + e.Reason
}

func (p DestinationPolicy) Validate(ctx context.Context, rawURL string) (*url.URL, error) {
	parsed, err := url.ParseRequestURI(strings.TrimSpace(rawURL))
	if err != nil ||
		parsed.Scheme != "https" ||
		parsed.Host == "" ||
		parsed.User != nil ||
		parsed.Fragment != "" {
		return nil, &UnsafeDestinationError{Reason: "an absolute HTTPS URL without credentials or fragments is required"}
	}
	host := strings.ToLower(strings.TrimSuffix(parsed.Hostname(), "."))
	if host == "" || !isASCII(host) {
		return nil, &UnsafeDestinationError{Reason: "hostname is invalid"}
	}
	if strings.EqualFold(strings.TrimSpace(p.Environment), "staging") && len(p.AllowedHosts) == 0 {
		return nil, &UnsafeDestinationError{Reason: "staging webhook allowlist is empty"}
	}
	if len(p.AllowedHosts) > 0 && !hostAllowed(host, p.AllowedHosts) {
		return nil, &UnsafeDestinationError{Reason: "hostname is not in the environment allowlist"}
	}
	if isForbiddenHostname(host) {
		return nil, &UnsafeDestinationError{Reason: "local and metadata hostnames are blocked"}
	}

	if literal := net.ParseIP(host); literal != nil {
		if !isPublicIP(literal) {
			return nil, &UnsafeDestinationError{Reason: "private or special-purpose IP addresses are blocked"}
		}
		return parsed, nil
	}
	resolver := p.Resolver
	if resolver == nil {
		resolver = net.DefaultResolver
	}
	addresses, err := resolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, fmt.Errorf("resolve webhook destination: %w", err)
	}
	if len(addresses) == 0 {
		return nil, fmt.Errorf("resolve webhook destination: no addresses returned")
	}
	for _, address := range addresses {
		if !isPublicIP(address.IP) {
			return nil, &UnsafeDestinationError{Reason: "hostname resolves to a private or special-purpose address"}
		}
	}
	return parsed, nil
}

func IsUnsafeDestination(err error) bool {
	var unsafe *UnsafeDestinationError
	return errors.As(err, &unsafe)
}

func hostAllowed(host string, allowed []string) bool {
	for _, candidate := range allowed {
		candidate = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(candidate), "."))
		if candidate == host {
			return true
		}
		if strings.HasPrefix(candidate, "*.") {
			suffix := strings.TrimPrefix(candidate, "*")
			if strings.HasSuffix(host, suffix) && host != strings.TrimPrefix(suffix, ".") {
				return true
			}
		}
	}
	return false
}

func isForbiddenHostname(host string) bool {
	if host == "localhost" ||
		strings.HasSuffix(host, ".localhost") ||
		strings.HasSuffix(host, ".local") ||
		host == "metadata.google.internal" ||
		host == "instance-data" ||
		strings.HasSuffix(host, ".internal") {
		return true
	}
	return false
}

func isPublicIP(ip net.IP) bool {
	if ip == nil ||
		ip.IsUnspecified() ||
		ip.IsLoopback() ||
		ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsMulticast() {
		return false
	}
	for _, network := range specialUseNetworks {
		if network.Contains(ip) {
			return false
		}
	}
	return true
}

var specialUseNetworks = mustNetworks(
	"0.0.0.0/8",
	"100.64.0.0/10",
	"192.0.0.0/24",
	"192.0.2.0/24",
	"198.18.0.0/15",
	"198.51.100.0/24",
	"203.0.113.0/24",
	"240.0.0.0/4",
	"::/128",
	"100::/64",
	"2001:db8::/32",
	"fc00::/7",
	"fec0::/10",
	"fe80::/10",
)

func mustNetworks(values ...string) []*net.IPNet {
	result := make([]*net.IPNet, 0, len(values))
	for _, value := range values {
		_, network, err := net.ParseCIDR(value)
		if err != nil {
			panic(err)
		}
		result = append(result, network)
	}
	return result
}

func isASCII(value string) bool {
	for _, character := range value {
		if character > 127 {
			return false
		}
	}
	return true
}

package realip

import (
	"fmt"
	"net"
	"net/http"
	"strings"
)

var privateNets []*net.IPNet

func init() {
	cidrs := []string{
		// IPv4 Private
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		// IPv4 Link-Local
		"169.254.0.0/16",
		// IPv4 Shared Address Space (RFC 6598)
		"100.64.0.0/10",
		// IPv4 Benchmarking (RFC 2544)
		"198.18.0.0/15",
		// IPv6 Unique Local Addresses (ULA)
		"fc00::/7",
		// IPv6 Link-local
		"fe80::/10",
	}

	for _, cidr := range cidrs {
		_, network, err := net.ParseCIDR(cidr)
		if err == nil {
			privateNets = append(privateNets, network)
		}
	}
}

// Get extracts the "real" client IP from the request.
// It prefers the first public IP found scanning headers right-to-left,
// falls back to the first valid IP seen in headers, then to RemoteAddr.
func Get(r *http.Request) (string, error) {
	var firstValidIP string

	for _, header := range []string{"X-Forwarded-For", "X-Real-Ip"} {
		hv := r.Header.Get(header)
		if hv == "" {
			continue
		}

		parts := strings.Split(hv, ",")
		// First pass: left → right to find first valid IP for fallback
		for _, ipStr := range parts {
			ipStr = strings.TrimSpace(ipStr)
			ip := net.ParseIP(ipStr)
			if ip == nil {
				continue
			}
			if firstValidIP == "" {
				firstValidIP = ipStr
			}
		}

		// Second pass: right → left to pick first public IP
		for i := len(parts) - 1; i >= 0; i-- {
			ipStr := strings.TrimSpace(parts[i])
			ip := net.ParseIP(ipStr)
			if ip == nil {
				continue
			}
			if ip.IsGlobalUnicast() && !isPrivateSubnet(ip) {
				return ipStr, nil
			}
		}
	}

	// fallback to first valid IP (even if private)
	if firstValidIP != "" {
		return firstValidIP, nil
	}

	// Fallback: RemoteAddr
	remote := r.RemoteAddr
	if host, _, err := net.SplitHostPort(remote); err == nil {
		remote = host
	}
	if ip := net.ParseIP(remote); ip != nil {
		return remote, nil
	}

	return "", fmt.Errorf("no valid IP found in request: %q", r.RemoteAddr)
}

func isPrivateSubnet(ip net.IP) bool {
	for _, n := range privateNets {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

// IsPrivateIP returns true if the IP address is in a private subnet.
// This is useful for validating that a request came from a trusted proxy.
func IsPrivateIP(ip net.IP) bool {
	return isPrivateSubnet(ip)
}

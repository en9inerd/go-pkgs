package middleware

import (
	"net"
	"net/http"
	"slices"
	"strings"

	"github.com/en9inerd/go-pkgs/realip"
)

// RealIP is a middleware that sets a http.Request's RemoteAddr to the results
// of parsing either the X-Forwarded-For or X-Real-IP headers.
//
// This middleware should only be used if you can trust the headers sent with the request.
// If reverse proxies are configured to pass along arbitrary header values from the client,
// or if this middleware is used without a reverse proxy, malicious clients could set anything
// as X-Forwarded-For header and attack the server in various ways.
//
// For a secure version that validates proxy IPs, use RealIPWithTrustedProxies.
func RealIP(h http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		if rip, err := realip.Get(r); err == nil {
			r.RemoteAddr = rip
		}
		h.ServeHTTP(w, r)
	}

	return http.HandlerFunc(fn)
}

// RealIPWithTrustedProxies is a secure version of RealIP that only trusts
// X-Forwarded-For and X-Real-IP headers when the request comes from a trusted proxy.
// This prevents IP spoofing attacks by validating that the RemoteAddr is from a trusted source.
//
// trustedProxies can be:
//   - nil or empty: Only trust headers if RemoteAddr is a private IP (assumes behind reverse proxy).
//     This default behavior is safe if:
//   - Your server is always behind a reverse proxy, AND
//   - Direct client connections from private networks are not possible.
//   - List of CIDR blocks: Only trust headers if RemoteAddr matches one of the CIDRs
//   - List of IP addresses: Only trust headers if RemoteAddr matches one of the IPs
//
// Example usage:
//
//	// Explicit trusted proxies (most secure)
//	middleware.RealIPWithTrustedProxies([]string{"10.0.0.1", "10.0.0.2"}, handler)
//
//	// Trust all private IPs (safe if behind reverse proxy)
//	middleware.RealIPWithTrustedProxies(nil, handler)
func RealIPWithTrustedProxies(trustedProxies []string, h http.Handler) http.Handler {
	var trustedNets []*net.IPNet
	var trustedIPs []net.IP

	for _, proxy := range trustedProxies {
		if strings.Contains(proxy, "/") {
			_, network, err := net.ParseCIDR(proxy)
			if err == nil {
				trustedNets = append(trustedNets, network)
			}
		} else {
			ip := net.ParseIP(proxy)
			if ip != nil {
				trustedIPs = append(trustedIPs, ip)
			}
		}
	}

	fn := func(w http.ResponseWriter, r *http.Request) {
		remoteIPStr := r.RemoteAddr
		if host, _, err := net.SplitHostPort(remoteIPStr); err == nil {
			remoteIPStr = host
		}
		remoteIP := net.ParseIP(remoteIPStr)
		if remoteIP == nil {
			h.ServeHTTP(w, r)
			return
		}

		isTrusted := false

		if len(trustedProxies) == 0 {
			isTrusted = realip.IsPrivateIP(remoteIP)
		} else {
			isTrusted = slices.ContainsFunc(trustedIPs, func(trustedIP net.IP) bool {
				return remoteIP.Equal(trustedIP)
			})
			if !isTrusted {
				isTrusted = slices.ContainsFunc(trustedNets, func(network *net.IPNet) bool {
					return network.Contains(remoteIP)
				})
			}
		}

		if isTrusted {
			if rip, err := realip.Get(r); err == nil {
				r.RemoteAddr = rip
			}
		}

		h.ServeHTTP(w, r)
	}

	return http.HandlerFunc(fn)
}

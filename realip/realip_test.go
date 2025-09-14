package realip

import (
	"net"
	"net/http"
	"testing"
)

func newRequest(headers map[string]string, remoteAddr string) *http.Request {
	r := &http.Request{
		Header:     make(http.Header),
		RemoteAddr: remoteAddr,
	}
	for k, v := range headers {
		r.Header.Set(k, v)
	}
	return r
}

func TestGet(t *testing.T) {
	tests := []struct {
		name       string
		headers    map[string]string
		remoteAddr string
		wantIP     string
		wantErr    bool
	}{
		{"PublicIPInXForwardedFor", map[string]string{"X-Forwarded-For": "192.168.0.1, 8.8.8.8"}, "10.0.0.1:12345", "8.8.8.8", false},
		{"FallsBackToFirstPrivateIP", map[string]string{"X-Forwarded-For": "192.168.1.10, 10.0.0.5"}, "203.0.113.1:8080", "192.168.1.10", false},
		{"UsesXRealIPIfPresent", map[string]string{"X-Real-Ip": "8.8.4.4"}, "127.0.0.1:12345", "8.8.4.4", false},
		{"FallsBackToRemoteAddr", nil, "203.0.113.99:5678", "203.0.113.99", false},
		{"InvalidRemoteAddr", nil, "not-an-ip", "", true},
		{"MultiplePublicIPsInXForwardedFor", map[string]string{"X-Forwarded-For": "192.168.0.1, 8.8.8.8, 1.1.1.1"}, "10.0.0.1:12345", "1.1.1.1", false},
		{"HeaderWithInvalidIPs", map[string]string{"X-Forwarded-For": "invalid-ip, also-bad, 8.8.8.8"}, "10.0.0.1:12345", "8.8.8.8", false},
		{"EmptyHeaders", map[string]string{"X-Forwarded-For": "", "X-Real-Ip": ""}, "203.0.113.5:4567", "203.0.113.5", false},
		{"IPv6PublicAndPrivateMix", map[string]string{"X-Forwarded-For": "fc00::1, 2001:4860:4860::8888"}, "[fe80::1]:1234", "2001:4860:4860::8888", false},
		{"RemoteAddrWithoutPort", nil, "203.0.113.77", "203.0.113.77", false},
		{"HeaderInvalidRemoteValid", map[string]string{"X-Forwarded-For": "not-an-ip"}, "8.8.4.4:1234", "8.8.4.4", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newRequest(tt.headers, tt.remoteAddr)
			ip, err := Get(r)
			if (err != nil) != tt.wantErr {
				t.Fatalf("unexpected error: %v", err)
			}
			if ip != tt.wantIP {
				t.Errorf("expected %s, got %s", tt.wantIP, ip)
			}
		})
	}
}

func TestIsPrivateSubnet(t *testing.T) {
	cases := []struct {
		ip       string
		expected bool
	}{
		{"192.168.1.1", true},
		{"10.0.0.1", true},
		{"172.16.5.5", true},
		{"100.64.0.1", true},
		{"169.254.0.5", true},
		{"198.18.0.1", true},
		{"fc00::1", true},
		{"fe80::1", true},
		{"8.8.8.8", false},
		{"2001:4860:4860::8888", false},
	}

	for _, c := range cases {
		ip := net.ParseIP(c.ip)
		if got := isPrivateSubnet(ip); got != c.expected {
			t.Errorf("ip %s: expected %v, got %v", c.ip, c.expected, got)
		}
	}
}

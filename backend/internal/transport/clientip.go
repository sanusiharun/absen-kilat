package transport

import (
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"strings"
)

// ClientIPResolver menentukan IP klien yang dipakai untuk rate limit auth.
// Header X-Forwarded-For hanya dipercaya bila koneksi berasal dari trusted
// proxy yang dikonfigurasi; selain itu RemoteAddr yang dipakai (I17).
type ClientIPResolver struct {
	trusted []netip.Prefix
}

// NewClientIPResolver mem-parsing daftar proxy terpercaya (dipisah koma, IP
// atau CIDR, mis. "10.0.0.0/8,172.18.0.1"). Daftar kosong berarti tidak ada
// proxy yang dipercaya — aman secara default.
func NewClientIPResolver(raw string) (*ClientIPResolver, error) {
	resolver := &ClientIPResolver{}
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if strings.Contains(part, "/") {
			prefix, err := netip.ParsePrefix(part)
			if err != nil {
				return nil, fmt.Errorf("trusted proxy %q: %w", part, err)
			}
			resolver.trusted = append(resolver.trusted, prefix.Masked())
			continue
		}
		addr, err := netip.ParseAddr(part)
		if err != nil {
			return nil, fmt.Errorf("trusted proxy %q: %w", part, err)
		}
		resolver.trusted = append(resolver.trusted, netip.PrefixFrom(addr, addr.BitLen()))
	}
	return resolver, nil
}

// Resolve mengembalikan IP klien: RemoteAddr bila peer tidak terpercaya, atau
// hop non-terpercaya paling kanan dari X-Forwarded-For bila peer terpercaya.
func (c *ClientIPResolver) Resolve(r *http.Request) string {
	remote := hostOnly(r.RemoteAddr)
	if c == nil || !c.isTrusted(remote) {
		return remote
	}
	hops := strings.Split(r.Header.Get("X-Forwarded-For"), ",")
	for i := len(hops) - 1; i >= 0; i-- {
		hop := strings.TrimSpace(hops[i])
		if hop == "" {
			continue
		}
		addr, err := netip.ParseAddr(stripZone(hop))
		if err != nil {
			// Header rusak: jangan menebak, pakai peer yang terpercaya.
			return remote
		}
		if !c.isTrusted(addr.String()) {
			return addr.String()
		}
	}
	return remote
}

func (c *ClientIPResolver) isTrusted(raw string) bool {
	addr, err := netip.ParseAddr(stripZone(raw))
	if err != nil {
		return false
	}
	for _, prefix := range c.trusted {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}

// hostOnly strips the port from a RemoteAddr, tolerating bare IPs and IPv6.
func hostOnly(remoteAddr string) string {
	if host, _, err := net.SplitHostPort(remoteAddr); err == nil {
		return host
	}
	return remoteAddr
}

func stripZone(host string) string {
	if i := strings.IndexByte(host, '%'); i >= 0 {
		return host[:i]
	}
	return host
}

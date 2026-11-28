package transport

import (
	"net/http/httptest"
	"testing"
)

// I17: klien tak-terpercaya yang memalsukan X-Forwarded-For tetap dibatasi
// berdasarkan RemoteAddr.
func TestClientIPResolver_UntrustedPeerIgnoresForwardedHeader(t *testing.T) {
	resolver, err := NewClientIPResolver("")
	if err != nil {
		t.Fatalf("resolver: %v", err)
	}
	r := httptest.NewRequest("POST", "/auth/login", nil)
	r.RemoteAddr = "198.51.100.7:41234"
	r.Header.Set("X-Forwarded-For", "203.0.113.250")

	if got := resolver.Resolve(r); got != "198.51.100.7" {
		t.Fatalf("got %q, want RemoteAddr 198.51.100.7", got)
	}
}

// Proxy terpercaya: hop non-terpercaya paling kanan dari XFF yang dipakai.
func TestClientIPResolver_TrustedProxyUsesForwardedChain(t *testing.T) {
	resolver, err := NewClientIPResolver("10.0.0.0/8, 172.18.0.1")
	if err != nil {
		t.Fatalf("resolver: %v", err)
	}

	r := httptest.NewRequest("POST", "/auth/login", nil)
	r.RemoteAddr = "10.1.2.3:5555"
	r.Header.Set("X-Forwarded-For", "203.0.113.5")
	if got := resolver.Resolve(r); got != "203.0.113.5" {
		t.Fatalf("got %q, want 203.0.113.5", got)
	}

	// Rantai: klien memalsukan hop kiri; hop kanan (yang ditambahkan proxy)
	// adalah klien sebenarnya.
	r.Header.Set("X-Forwarded-For", "1.1.1.1, 203.0.113.9")
	if got := resolver.Resolve(r); got != "203.0.113.9" {
		t.Fatalf("got %q, want 203.0.113.9", got)
	}

	// Hop proxy internal dilewati.
	r.Header.Set("X-Forwarded-For", "203.0.113.9, 10.9.9.9")
	if got := resolver.Resolve(r); got != "203.0.113.9" {
		t.Fatalf("got %q, want 203.0.113.9 (trusted hop skipped)", got)
	}

	// Header rusak: jatuh ke peer, bukan menebak.
	r.Header.Set("X-Forwarded-For", "not-an-ip")
	if got := resolver.Resolve(r); got != "10.1.2.3" {
		t.Fatalf("got %q, want peer 10.1.2.3", got)
	}
}

func TestClientIPResolver_InvalidConfigRejected(t *testing.T) {
	if _, err := NewClientIPResolver("10.0.0.0/99"); err == nil {
		t.Fatal("expected error for invalid CIDR")
	}
	if _, err := NewClientIPResolver("bukan-ip"); err == nil {
		t.Fatal("expected error for invalid IP")
	}
}

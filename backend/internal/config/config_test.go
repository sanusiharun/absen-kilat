package config

import (
	"testing"
	"time"
)

func TestConfig_ReuseGraceDefault(t *testing.T) {
	t.Setenv("REFRESH_REUSE_GRACE", "")
	t.Setenv("REFRESH_REUSE_GRACE_SECONDS", "")
	cfg := Load()
	if err := cfg.ValidateIdentity(); err != nil {
		t.Fatalf("default grace harus valid: %v", err)
	}
	if cfg.ReuseGrace != 10*time.Second {
		t.Fatalf("default grace harus 10s, dapat %s", cfg.ReuseGrace)
	}
}

func TestConfig_ReuseGraceParsed(t *testing.T) {
	t.Setenv("REFRESH_REUSE_GRACE", "25s")
	t.Setenv("REFRESH_REUSE_GRACE_SECONDS", "")
	cfg := Load()
	if err := cfg.ValidateIdentity(); err != nil {
		t.Fatalf("25s harus valid: %v", err)
	}
	if cfg.ReuseGrace != 25*time.Second {
		t.Fatalf("grace harus 25s, dapat %s", cfg.ReuseGrace)
	}
}

func TestConfig_ReuseGraceZeroIsStrict(t *testing.T) {
	t.Setenv("REFRESH_REUSE_GRACE", "0s")
	t.Setenv("REFRESH_REUSE_GRACE_SECONDS", "")
	cfg := Load()
	if err := cfg.ValidateIdentity(); err != nil {
		t.Fatalf("0s harus valid: %v", err)
	}
	if cfg.ReuseGrace != 0 {
		t.Fatalf("grace harus 0, dapat %s", cfg.ReuseGrace)
	}
}

func TestConfig_GraceBoundsRejected(t *testing.T) {
	cases := []struct{ name, env, legacy string }{
		{"negatif", "-5s", ""},
		{"di atas maksimum", "61s", ""},
		{"tak bisa diparse", "sepuluh", ""},
		{"legacy negatif", "", "-1"},
		{"legacy di atas maksimum", "", "120"},
		{"legacy tak bisa diparse", "", "10s"},
		{"legacy overflow int64 maksimum", "", "9223372036854775807"},
		{"legacy overflow 2^55", "", "36028797018963968"},
		{"legacy overflow jadi 30s palsu", "", "36028797018963998"},
		{"legacy di bawah int64 minimum", "", "-9223372036854775809"},
		{"durasi string overflow", "9999999999999999999s", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("REFRESH_REUSE_GRACE", tc.env)
			t.Setenv("REFRESH_REUSE_GRACE_SECONDS", tc.legacy)
			if err := Load().ValidateIdentity(); err == nil {
				t.Fatalf("grace %q/%q harus ditolak saat start", tc.env, tc.legacy)
			}
		})
	}
}

func TestConfig_GraceLegacySecondsAlias(t *testing.T) {
	t.Setenv("REFRESH_REUSE_GRACE", "")
	t.Setenv("REFRESH_REUSE_GRACE_SECONDS", "30")
	cfg := Load()
	if err := cfg.ValidateIdentity(); err != nil {
		t.Fatalf("alias legacy 30 harus valid: %v", err)
	}
	if cfg.ReuseGrace != 30*time.Second {
		t.Fatalf("grace harus 30s, dapat %s", cfg.ReuseGrace)
	}
}

func TestConfig_NewVarWinsOverLegacy(t *testing.T) {
	t.Setenv("REFRESH_REUSE_GRACE", "5s")
	t.Setenv("REFRESH_REUSE_GRACE_SECONDS", "45")
	cfg := Load()
	if cfg.ReuseGrace != 5*time.Second {
		t.Fatalf("REFRESH_REUSE_GRACE harus menang, dapat %s", cfg.ReuseGrace)
	}
}

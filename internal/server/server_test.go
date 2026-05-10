package server

import "testing"

func TestDefaultConfig_DefaultsWhenEnvUnset(t *testing.T) {
	t.Setenv("KUMO_HOST", "")
	t.Setenv("KUMO_PORT", "")

	cfg := DefaultConfig()

	if cfg.Host != "0.0.0.0" {
		t.Errorf("Host = %q, want 0.0.0.0", cfg.Host)
	}

	if cfg.Port != 4566 {
		t.Errorf("Port = %d, want 4566", cfg.Port)
	}
}

func TestDefaultConfig_HonorsEnv(t *testing.T) {
	t.Setenv("KUMO_HOST", "127.0.0.1")
	t.Setenv("KUMO_PORT", "18080")

	cfg := DefaultConfig()

	if cfg.Host != "127.0.0.1" {
		t.Errorf("Host = %q, want 127.0.0.1", cfg.Host)
	}

	if cfg.Port != 18080 {
		t.Errorf("Port = %d, want 18080", cfg.Port)
	}
}

func TestDefaultConfig_IgnoresUnparseablePort(t *testing.T) {
	t.Setenv("KUMO_PORT", "not-a-number")

	cfg := DefaultConfig()

	if cfg.Port != 4566 {
		t.Errorf("Port = %d, want default 4566 when KUMO_PORT is unparseable", cfg.Port)
	}
}

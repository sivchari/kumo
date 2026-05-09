package rds

import (
	"strings"
	"testing"
)

// TestEndpointFor_DefaultLooksLikeAWS preserves the historical
// behaviour: with no env override, the hostname matches the AWS
// shape and the port is the engine default.
func TestEndpointFor_DefaultLooksLikeAWS(t *testing.T) {
	clearEnv(t)

	addr, port := endpointFor("postgres", "my-db", 5432)
	if !strings.HasSuffix(addr, ".us-east-1.rds.amazonaws.com") {
		t.Fatalf("default address: got %q, want AWS-style suffix", addr)
	}

	if port != 5432 {
		t.Fatalf("default port: got %d, want 5432", port)
	}
}

// TestEndpointFor_GlobalOverride redirects every engine to the same
// (address, port). Useful when the developer only runs one DB.
func TestEndpointFor_GlobalOverride(t *testing.T) {
	clearEnv(t)
	t.Setenv("KUMO_RDS_ENDPOINT_ADDRESS", "127.0.0.1")
	t.Setenv("KUMO_RDS_ENDPOINT_PORT", "5433")

	addr, port := endpointFor("postgres", "my-db", 5432)
	if addr != "127.0.0.1" || port != 5433 {
		t.Fatalf("global override: got (%s, %d), want (127.0.0.1, 5433)", addr, port)
	}
}

// TestEndpointFor_PerEnginePrecedence — the engine-specific var wins
// over the global one. Lets a developer route Postgres to one process
// and MySQL to another.
func TestEndpointFor_PerEnginePrecedence(t *testing.T) {
	clearEnv(t)
	t.Setenv("KUMO_RDS_ENDPOINT_ADDRESS", "global.example")
	t.Setenv("KUMO_RDS_ENDPOINT_ADDRESS_POSTGRES", "pg.local")
	t.Setenv("KUMO_RDS_ENDPOINT_PORT_POSTGRES", "15432")

	pgAddr, pgPort := endpointFor("postgres", "x", 5432)
	if pgAddr != "pg.local" || pgPort != 15432 {
		t.Fatalf("postgres: got (%s, %d), want (pg.local, 15432)", pgAddr, pgPort)
	}

	mysqlAddr, mysqlPort := endpointFor("mysql", "x", 3306)
	if mysqlAddr != "global.example" || mysqlPort != 3306 {
		t.Fatalf("mysql falls back to global: got (%s, %d), want (global.example, 3306)",
			mysqlAddr, mysqlPort)
	}
}

// TestEndpointFor_DashedEngineNormalised — `aurora-postgresql` looks
// for `KUMO_RDS_ENDPOINT_ADDRESS_AURORA_POSTGRESQL`, not the dashed
// form (env vars can't have dashes on every shell).
func TestEndpointFor_DashedEngineNormalised(t *testing.T) {
	clearEnv(t)
	t.Setenv("KUMO_RDS_ENDPOINT_ADDRESS_AURORA_POSTGRESQL", "aurora.local")

	addr, _ := endpointFor("aurora-postgresql", "x", 5432)
	if addr != "aurora.local" {
		t.Fatalf("dashed engine: got %q, want aurora.local", addr)
	}
}

// TestEndpointFor_PartialOverride — setting only the address keeps
// the engine default port (and vice-versa).
func TestEndpointFor_PartialOverride(t *testing.T) {
	clearEnv(t)
	t.Setenv("KUMO_RDS_ENDPOINT_ADDRESS", "127.0.0.1")

	_, port := endpointFor("postgres", "x", 5432)
	if port != 5432 {
		t.Fatalf("partial override should keep default port: got %d", port)
	}
}

// clearEnv resets every KUMO_RDS_ENDPOINT_* variable. t.Setenv with
// an empty value does the right thing here — Go's testing package
// restores it on cleanup either way.
func clearEnv(t *testing.T) {
	t.Helper()

	for _, k := range []string{
		"KUMO_RDS_ENDPOINT_ADDRESS",
		"KUMO_RDS_ENDPOINT_PORT",
		"KUMO_RDS_ENDPOINT_ADDRESS_POSTGRES",
		"KUMO_RDS_ENDPOINT_PORT_POSTGRES",
		"KUMO_RDS_ENDPOINT_ADDRESS_MYSQL",
		"KUMO_RDS_ENDPOINT_PORT_MYSQL",
		"KUMO_RDS_ENDPOINT_ADDRESS_AURORA_POSTGRESQL",
	} {
		t.Setenv(k, "")
	}
}

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

// TestEndpointFor_GlobalBackend redirects every engine to the same
// host:port. Useful when the developer only runs one DB.
func TestEndpointFor_GlobalBackend(t *testing.T) {
	clearEnv(t)
	t.Setenv("KUMO_RDS_BACKEND", "127.0.0.1:5433")

	addr, port := endpointFor("postgres", "my-db", 5432)
	if addr != "127.0.0.1" || port != 5433 {
		t.Fatalf("global backend: got (%s, %d), want (127.0.0.1, 5433)", addr, port)
	}
}

// TestEndpointFor_PerEngineBackend routes different engines to different
// host:port pairs.
func TestEndpointFor_PerEngineBackend(t *testing.T) {
	clearEnv(t)
	t.Setenv("KUMO_RDS_BACKEND", "postgres=pg.local:15432,mysql=mysql.local:13306")

	pgAddr, pgPort := endpointFor("postgres", "x", 5432)
	if pgAddr != "pg.local" || pgPort != 15432 {
		t.Fatalf("postgres: got (%s, %d), want (pg.local, 15432)", pgAddr, pgPort)
	}

	mysqlAddr, mysqlPort := endpointFor("mysql", "x", 3306)
	if mysqlAddr != "mysql.local" || mysqlPort != 13306 {
		t.Fatalf("mysql: got (%s, %d), want (mysql.local, 13306)",
			mysqlAddr, mysqlPort)
	}
}

// TestEndpointFor_DashedEngineNormalised accepts either dashes or
// underscores in engine names.
func TestEndpointFor_DashedEngineNormalised(t *testing.T) {
	clearEnv(t)
	t.Setenv("KUMO_RDS_BACKEND", "aurora_postgresql=aurora.local:5432")

	addr, port := endpointFor("aurora-postgresql", "x", 5432)
	if addr != "aurora.local" || port != 5432 {
		t.Fatalf("dashed engine: got (%s, %d), want (aurora.local, 5432)", addr, port)
	}
}

// TestEndpointFor_InvalidBackendFallsBack ignores malformed backend values.
func TestEndpointFor_InvalidBackendFallsBack(t *testing.T) {
	clearEnv(t)
	t.Setenv("KUMO_RDS_BACKEND", "http://127.0.0.1:5432")

	addr, port := endpointFor("postgres", "x", 5432)
	if !strings.HasSuffix(addr, ".us-east-1.rds.amazonaws.com") || port != 5432 {
		t.Fatalf("invalid backend fallback: got (%s, %d), want AWS-style address and 5432", addr, port)
	}
}

// clearEnv resets the RDS backend variable. t.Setenv with
// an empty value does the right thing here — Go's testing package
// restores it on cleanup either way.
func clearEnv(t *testing.T) {
	t.Helper()

	t.Setenv("KUMO_RDS_BACKEND", "")
}

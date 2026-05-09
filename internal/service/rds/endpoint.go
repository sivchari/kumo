package rds

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// endpointFor returns the (address, port) pair to expose for a DB
// instance / cluster of the given engine.
//
// kumo's primary use case is integration testing IaC against an
// emulator, so the data plane (actual SQL) usually has to land on a
// real database the developer is running locally. Two env-var levels
// of override let the caller redirect:
//
//	KUMO_RDS_ENDPOINT_ADDRESS_<ENGINE>   per-engine address (e.g. _POSTGRES)
//	KUMO_RDS_ENDPOINT_PORT_<ENGINE>      per-engine port
//	KUMO_RDS_ENDPOINT_ADDRESS            global fallback address
//	KUMO_RDS_ENDPOINT_PORT               global fallback port
//
// Without overrides, behaviour is unchanged from upstream: an AWS-
// shaped hostname is returned (`<id>.<rnd>.us-east-1.rds.amazonaws.com`)
// and the per-engine default port is used. The hostname looks real
// but doesn't resolve, which is what callers using kumo purely for
// control-plane testing expect today.
//
// Per-engine takes precedence over global. If only the address (or
// only the port) is overridden, the other side falls back through
// the same chain.
func endpointFor(engine, identifier string, defaultPort int32) (string, int32) {
	address := lookupEndpointEnv("ADDRESS", engine)
	port := lookupEndpointEnv("PORT", engine)

	addr := address
	if addr == "" {
		addr = fmt.Sprintf("%s.%s.%s.rds.amazonaws.com", identifier, generateID(), defaultRegion)
	}

	pp := defaultPort

	if port != "" {
		if parsed, err := strconv.ParseInt(port, 10, 32); err == nil && parsed > 0 {
			pp = int32(parsed)
		}
	}

	return addr, pp
}

// lookupEndpointEnv consults the per-engine env var first, then the
// engine-less fallback. Returns "" when neither is set.
func lookupEndpointEnv(field, engine string) string {
	if engine != "" {
		key := "KUMO_RDS_ENDPOINT_" + field + "_" + envEngineKey(engine)
		if v := os.Getenv(key); v != "" {
			return v
		}
	}

	return os.Getenv("KUMO_RDS_ENDPOINT_" + field)
}

// envEngineKey normalises an engine string into the upper-case form
// the env var uses. `aurora-postgresql` → `AURORA_POSTGRESQL`.
func envEngineKey(engine string) string {
	return strings.ToUpper(strings.ReplaceAll(engine, "-", "_"))
}

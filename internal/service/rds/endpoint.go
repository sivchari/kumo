package rds

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
)

// endpointFor returns the (address, port) pair to expose for a DB
// instance / cluster of the given engine.
//
// kumo's primary use case is integration testing IaC against an
// emulator, so the data plane (actual SQL) usually has to land on a
// real database the developer is running locally. KUMO_RDS_BACKEND can
// redirect endpoints in either of these forms:
//
//	KUMO_RDS_BACKEND=127.0.0.1:5432
//	KUMO_RDS_BACKEND=postgres=127.0.0.1:5432,mysql=127.0.0.1:3306
//
// Without overrides, behaviour is unchanged from upstream: an AWS-
// shaped hostname is returned (`<id>.<rnd>.us-east-1.rds.amazonaws.com`)
// and the per-engine default port is used. The hostname looks real
// but doesn't resolve, which is what callers using kumo purely for
// control-plane testing expect today.
//
// Per-engine entries take precedence over a global host:port entry when
// both are present.
func endpointFor(engine, identifier string, defaultPort int32) (string, int32) {
	if backend, ok := backendEndpointFor(engine); ok {
		return backend.address, backend.port
	}

	return fmt.Sprintf("%s.%s.%s.rds.amazonaws.com", identifier, generateID(), defaultRegion), defaultPort
}

type backendEndpoint struct {
	address string
	port    int32
}

func backendEndpointFor(engine string) (backendEndpoint, bool) {
	raw := os.Getenv("KUMO_RDS_BACKEND")
	if raw == "" {
		return backendEndpoint{}, false
	}

	engineKey := envEngineKey(engine)

	var global *backendEndpoint

	for _, entry := range strings.Split(raw, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}

		key, target, hasEngine := strings.Cut(entry, "=")
		if !hasEngine {
			if backend, ok := parseBackendEndpoint(entry); ok {
				global = &backend
			}

			continue
		}

		if envEngineKey(strings.TrimSpace(key)) != engineKey {
			continue
		}

		if backend, ok := parseBackendEndpoint(strings.TrimSpace(target)); ok {
			return backend, true
		}
	}

	if global != nil {
		return *global, true
	}

	return backendEndpoint{}, false
}

func parseBackendEndpoint(raw string) (backendEndpoint, bool) {
	if strings.Contains(raw, "://") {
		return backendEndpoint{}, false
	}

	host, portText, err := net.SplitHostPort(raw)
	if err != nil || host == "" {
		return backendEndpoint{}, false
	}

	port, err := strconv.ParseInt(portText, 10, 32)
	if err != nil || port <= 0 || port > 65535 {
		return backendEndpoint{}, false
	}

	return backendEndpoint{address: host, port: int32(port)}, true
}

// envEngineKey normalises an engine string into the upper-case form
// used in backend entries. `aurora-postgresql` → `AURORA_POSTGRESQL`.
func envEngineKey(engine string) string {
	return strings.ToUpper(strings.ReplaceAll(engine, "-", "_"))
}

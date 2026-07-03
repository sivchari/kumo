package execapi

import (
	"fmt"
	"os"
)

// DefaultBaseURL is the kumo server URL used for in-process self-calls
// (execute-api -> Lambda invoke) when KUMO_HOST/KUMO_PORT are not set.
const DefaultBaseURL = "http://localhost:4566"

// ResolveBaseURL builds the kumo server base URL from KUMO_HOST / KUMO_PORT,
// mirroring the convention used by other services that self-call kumo
// endpoints (e.g. eventbridge -> lambda).
func ResolveBaseURL() string {
	if host := os.Getenv("KUMO_HOST"); host != "" {
		port := os.Getenv("KUMO_PORT")
		if port == "" {
			port = "4566"
		}

		return fmt.Sprintf("http://%s:%s", host, port)
	}

	if port := os.Getenv("KUMO_PORT"); port != "" {
		return fmt.Sprintf("http://localhost:%s", port)
	}

	return DefaultBaseURL
}

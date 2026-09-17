package lambda

import (
	"net/http"
	"strings"

	"github.com/sivchari/kumo/internal/service"
	"github.com/sivchari/kumo/internal/service/execapi"
)

var _ service.FunctionURLHandler = (*Service)(nil)

const arnAccountIndex = 4

// HandleFunctionURL serves a request that arrived on <url-id>.lambda-url.<host>.
// It reports false when no function owns urlID so the router can refuse the
// request. Authentication, CORS and the invocation follow the Lambda function
// URL data plane; RESPONSE_STREAM URLs are served buffered because kumo's
// Invoke is synchronous.
func (s *Service) HandleFunctionURL(w http.ResponseWriter, r *http.Request, urlID string) bool {
	cfg, functionName, err := s.storage.LookupFunctionURL(r.Context(), urlID)
	if err != nil {
		return false
	}

	origin := r.Header.Get(headerOrigin)

	if cfg.Cors != nil && isCORSPreflight(r) {
		if preflightAllowed(cfg.Cors, r) {
			writeCORSPreflight(w, cfg.Cors, origin)
		} else {
			writeFunctionURLForbidden(w)
		}

		return true
	}

	req := &execapi.FunctionURLRequest{URLID: cfg.URLID, AccountID: arnAccount(cfg.FunctionArn)}

	if cfg.AuthType == authTypeAWSIAM {
		identity, ok := sigV4Identity(r, req.AccountID)
		if !ok {
			writeFunctionURLForbidden(w)

			return true
		}

		req.Authorizer = &execapi.FunctionURLAuthorizer{IAM: identity}
	}

	if cfg.Cors != nil {
		addCORSHeaders(w.Header(), cfg.Cors, origin)
	}

	execapi.ServeFunctionURL(w, r, s.invokeBaseURL, functionName, req)

	return true
}

// writeFunctionURLForbidden is the 403 Lambda returns for an unsigned request
// to an AWS_IAM URL and for a preflight the CORS policy rejects.
func writeFunctionURLForbidden(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	_, _ = w.Write([]byte(`{"Message":"Forbidden"}`))
}

// arnAccount returns the account of arn:partition:service:region:account:...
func arnAccount(arn string) string {
	parts := strings.Split(arn, ":")
	if len(parts) <= arnAccountIndex {
		return ""
	}

	return parts[arnAccountIndex]
}

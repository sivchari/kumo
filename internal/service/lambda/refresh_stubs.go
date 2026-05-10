package lambda

import (
	"net/http"
	"strings"
)

// ListVersionsByFunction returns a single $LATEST entry for any existing
// function. terraform-provider-aws calls this on every refresh of
// aws_lambda_function and apply errors immediately after CreateFunction
// without it. Versions are not modeled in storage; the response only
// includes the canonical $LATEST entry that AWS always returns.
func (s *Service) ListVersionsByFunction(w http.ResponseWriter, r *http.Request) {
	name := extractFunctionNameForListChild(r.URL.Path, "versions")
	if name == "" {
		writeFunctionError(w, "InvalidParameterValueException", "FunctionName is required", http.StatusBadRequest)

		return
	}

	fn, err := s.storage.GetFunction(r.Context(), name)
	if err != nil {
		writeFunctionError(w, "ResourceNotFoundException", err.Error(), http.StatusNotFound)

		return
	}

	writeJSONResponse(w, http.StatusOK, listVersionsByFunctionResponse{
		Versions: []functionConfigurationVersion{
			{
				FunctionName: fn.FunctionName,
				FunctionArn:  fn.FunctionArn,
				Runtime:      fn.Runtime,
				Role:         fn.Role,
				Handler:      fn.Handler,
				Version:      "$LATEST",
				LastModified: fn.LastModified.UTC().Format("2006-01-02T15:04:05.000+0000"),
			},
		},
	})
}

// ListAliases returns an empty Aliases list. terraform-provider-aws calls
// this on every refresh of aws_lambda_function. Aliases are not modeled.
func (s *Service) ListAliases(w http.ResponseWriter, r *http.Request) {
	if extractFunctionNameForListChild(r.URL.Path, "aliases") == "" {
		writeFunctionError(w, "InvalidParameterValueException", "FunctionName is required", http.StatusBadRequest)

		return
	}

	writeJSONResponse(w, http.StatusOK, listAliasesResponse{Aliases: []aliasConfiguration{}})
}

// GetFunctionCodeSigningConfig reports no code-signing config for any
// function. terraform-provider-aws reads this on every refresh.
func (s *Service) GetFunctionCodeSigningConfig(w http.ResponseWriter, r *http.Request) {
	name := extractFunctionNameForListChild(r.URL.Path, "code-signing-config")
	if name == "" {
		writeFunctionError(w, "InvalidParameterValueException", "FunctionName is required", http.StatusBadRequest)

		return
	}

	writeJSONResponse(w, http.StatusOK, getFunctionCodeSigningConfigResponse{FunctionName: name})
}

// ListFunctionEventInvokeConfigs returns an empty list.
func (s *Service) ListFunctionEventInvokeConfigs(w http.ResponseWriter, r *http.Request) {
	if extractFunctionNameForListChild(r.URL.Path, "event-invoke-config") == "" {
		writeFunctionError(w, "InvalidParameterValueException", "FunctionName is required", http.StatusBadRequest)

		return
	}

	writeJSONResponse(w, http.StatusOK, listFunctionEventInvokeConfigsResponse{
		FunctionEventInvokeConfigs: []map[string]any{},
	})
}

// GetPolicy reports no resource policy for any function. AWS returns 404
// when a function has no attached policy; terraform-provider-aws expects
// that and treats it as "no policy".
func (s *Service) GetPolicy(w http.ResponseWriter, r *http.Request) {
	name := extractFunctionNameForListChild(r.URL.Path, "policy")
	if name == "" {
		writeFunctionError(w, "InvalidParameterValueException", "FunctionName is required", http.StatusBadRequest)

		return
	}

	if _, err := s.storage.GetFunction(r.Context(), name); err != nil {
		writeFunctionError(w, "ResourceNotFoundException", err.Error(), http.StatusNotFound)

		return
	}

	writeFunctionError(w, "ResourceNotFoundException", "The resource you requested does not exist.", http.StatusNotFound)
}

// extractFunctionNameForListChild returns the function name from a path
// like /.../functions/{name}/<child>. Empty if the shape does not match.
func extractFunctionNameForListChild(path, child string) string {
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	for i, p := range parts {
		if p == pathSegmentFunctions && i+2 < len(parts) && parts[i+2] == child {
			return parts[i+1]
		}
	}

	return ""
}

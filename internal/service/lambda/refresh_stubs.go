package lambda

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
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

// GetPolicy returns the resource policy for a function. AWS returns 404
// when a function has no attached policy; terraform-provider-aws expects
// that and treats it as "no policy".
func (s *Service) GetPolicy(w http.ResponseWriter, r *http.Request) {
	name := extractFunctionNameForListChild(r.URL.Path, "policy")
	if name == "" {
		writeFunctionError(w, "InvalidParameterValueException", "FunctionName is required", http.StatusBadRequest)

		return
	}

	fn, err := s.storage.GetFunction(r.Context(), name)
	if err != nil {
		writeFunctionError(w, "ResourceNotFoundException", err.Error(), http.StatusNotFound)

		return
	}

	if fn.Policy == nil || len(fn.Policy.Statements) == 0 {
		writeFunctionError(w, "ResourceNotFoundException", "The resource you requested does not exist.", http.StatusNotFound)

		return
	}

	policyJSON, err := json.Marshal(fn.Policy)
	if err != nil {
		writeFunctionError(w, ErrServiceException, "Internal server error", http.StatusInternalServerError)

		return
	}

	writeJSONResponse(w, http.StatusOK, getPolicyResponse{
		Policy:     string(policyJSON),
		RevisionID: "default",
	})
}

// AddPermission adds a permission to a Lambda function's resource policy.
//
//nolint:funlen // Permission handling with validation and policy construction.
func (s *Service) AddPermission(w http.ResponseWriter, r *http.Request) {
	name := extractFunctionNameForListChild(r.URL.Path, "policy")
	if name == "" {
		writeFunctionError(w, ErrInvalidParameterValue, "FunctionName is required", http.StatusBadRequest)

		return
	}

	var req addPermissionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeFunctionError(w, ErrInvalidParameterValue, "Invalid request body", http.StatusBadRequest)

		return
	}

	if req.StatementID == "" {
		writeFunctionError(w, ErrInvalidParameterValue, "StatementId is required", http.StatusBadRequest)

		return
	}

	if req.Action == "" {
		writeFunctionError(w, ErrInvalidParameterValue, "Action is required", http.StatusBadRequest)

		return
	}

	if req.Principal == "" {
		writeFunctionError(w, ErrInvalidParameterValue, "Principal is required", http.StatusBadRequest)

		return
	}

	fn, err := s.storage.GetFunction(r.Context(), name)
	if err != nil {
		handleGetFunctionError(w, err)

		return
	}

	stmt := &PolicyStatement{
		Sid:       req.StatementID,
		Effect:    "Allow",
		Principal: map[string]string{"Service": req.Principal},
		Action:    req.Action,
		Resource:  fn.FunctionArn,
	}

	if req.SourceArn != "" {
		stmt.Condition = map[string]any{
			"ArnLike": map[string]string{
				"AWS:SourceArn": req.SourceArn,
			},
		}
	}

	if req.SourceAccount != "" {
		if stmt.Condition == nil {
			stmt.Condition = make(map[string]any)
		}

		stmt.Condition["StringEquals"] = map[string]string{
			"AWS:SourceAccount": req.SourceAccount,
		}
	}

	if err := s.storage.AddPermission(r.Context(), name, stmt); err != nil {
		handleFunctionError(w, err)

		return
	}

	stmtJSON, err := json.Marshal(stmt)
	if err != nil {
		writeFunctionError(w, ErrServiceException, "Internal server error", http.StatusInternalServerError)

		return
	}

	writeJSONResponse(w, http.StatusCreated, addPermissionResponse{
		Statement: string(stmtJSON),
	})
}

// RemovePermission removes a permission from a Lambda function's resource policy.
func (s *Service) RemovePermission(w http.ResponseWriter, r *http.Request) {
	name, statementID := extractFunctionNameAndStatementID(r.URL.Path)
	if name == "" || statementID == "" {
		writeFunctionError(w, ErrInvalidParameterValue, "FunctionName and StatementId are required", http.StatusBadRequest)

		return
	}

	if err := s.storage.RemovePermission(r.Context(), name, statementID); err != nil {
		handleFunctionError(w, err)

		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ListTags returns the tags for a Lambda function identified by its ARN.
// The AWS API path is GET /2017-03-31/tags/{ARN}.
func (s *Service) ListTags(w http.ResponseWriter, r *http.Request) {
	arn := extractARNFromTagsPath(r.URL.Path)
	if arn == "" {
		writeFunctionError(w, ErrInvalidParameterValue, "Resource ARN is required", http.StatusBadRequest)

		return
	}

	fn, err := s.storage.GetFunctionByARN(r.Context(), arn)
	if err != nil {
		handleGetFunctionError(w, err)

		return
	}

	tags := fn.Tags
	if tags == nil {
		tags = make(map[string]string)
	}

	writeJSONResponse(w, http.StatusOK, listTagsResponse{Tags: tags})
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

// extractFunctionNameAndStatementID extracts function name and statement ID
// from paths like /.../functions/{name}/policy/{statementId}.
func extractFunctionNameAndStatementID(path string) (string, string) {
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	for i, p := range parts {
		if p == pathSegmentFunctions && i+3 < len(parts) && parts[i+2] == "policy" {
			return parts[i+1], parts[i+3]
		}
	}

	return "", ""
}

// extractARNFromTagsPath extracts the ARN from a path like
// /lambda/2017-03-31/tags/{arn} or /2017-03-31/tags/{arn}.
// The ARN is URL-encoded in the path and contains colons.
func extractARNFromTagsPath(path string) string {
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	for i, p := range parts {
		if p == "tags" && i+1 < len(parts) {
			// The ARN may span multiple segments if it contains slashes,
			// but Lambda ARNs use colons. Join remaining parts.
			raw := strings.Join(parts[i+1:], "/")

			decoded, err := url.PathUnescape(raw)
			if err != nil {
				return raw
			}

			return decoded
		}
	}

	return ""
}

// policyID returns a deterministic policy ID for a function.
func policyID(functionName string) string {
	return fmt.Sprintf("%s-policy", functionName)
}

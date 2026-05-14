package lambda

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"
)

const pathSegmentFunctions = "functions"

// CreateFunction handles the CreateFunction API.
func (s *Service) CreateFunction(w http.ResponseWriter, r *http.Request) {
	var req CreateFunctionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeFunctionError(w, ErrInvalidParameterValue, "Invalid request body", http.StatusBadRequest)

		return
	}

	if req.FunctionName == "" {
		writeFunctionError(w, ErrInvalidParameterValue, "FunctionName is required", http.StatusBadRequest)

		return
	}

	if req.Role == "" {
		writeFunctionError(w, ErrInvalidParameterValue, "Role is required", http.StatusBadRequest)

		return
	}

	fn, err := s.storage.CreateFunction(r.Context(), &req)
	if err != nil {
		var lambdaErr *FunctionError
		if errors.As(err, &lambdaErr) {
			status := http.StatusBadRequest
			if lambdaErr.Type == ErrResourceConflict {
				status = http.StatusConflict
			}

			writeFunctionError(w, lambdaErr.Type, lambdaErr.Message, status)

			return
		}

		writeFunctionError(w, ErrServiceException, "Internal server error", http.StatusInternalServerError)

		return
	}

	resp := functionToCreateResponse(fn)
	writeJSONResponse(w, http.StatusCreated, resp)
}

// GetFunction handles the GetFunction API.
func (s *Service) GetFunction(w http.ResponseWriter, r *http.Request) {
	functionName := extractFunctionName(r.URL.Path)
	if functionName == "" {
		writeFunctionError(w, ErrInvalidParameterValue, "FunctionName is required", http.StatusBadRequest)

		return
	}

	fn, err := s.storage.GetFunction(r.Context(), functionName)
	if err != nil {
		var lambdaErr *FunctionError
		if errors.As(err, &lambdaErr) {
			status := http.StatusBadRequest
			if lambdaErr.Type == ErrResourceNotFound {
				status = http.StatusNotFound
			}

			writeFunctionError(w, lambdaErr.Type, lambdaErr.Message, status)

			return
		}

		writeFunctionError(w, ErrServiceException, "Internal server error", http.StatusInternalServerError)

		return
	}

	resp := &GetFunctionResponse{
		Configuration: functionToConfiguration(fn),
		Code: &FunctionCodeLocation{
			RepositoryType: "S3",
			Location:       s.baseURL + "/lambda-code/" + functionName,
		},
	}

	writeJSONResponse(w, http.StatusOK, resp)
}

// GetFunctionConfiguration handles GET /functions/{name}/configuration.
// Returns only the configuration portion of GetFunction.
func (s *Service) GetFunctionConfiguration(w http.ResponseWriter, r *http.Request) {
	functionName := extractFunctionName(r.URL.Path)
	if functionName == "" {
		writeFunctionError(w, ErrInvalidParameterValue, "FunctionName is required", http.StatusBadRequest)

		return
	}

	fn, err := s.storage.GetFunction(r.Context(), functionName)
	if err != nil {
		handleGetFunctionError(w, err)

		return
	}

	writeJSONResponse(w, http.StatusOK, functionToConfiguration(fn))
}

// DeleteFunction handles the DeleteFunction API.
func (s *Service) DeleteFunction(w http.ResponseWriter, r *http.Request) {
	functionName := extractFunctionName(r.URL.Path)
	if functionName == "" {
		writeFunctionError(w, ErrInvalidParameterValue, "FunctionName is required", http.StatusBadRequest)

		return
	}

	err := s.storage.DeleteFunction(r.Context(), functionName)
	if err != nil {
		var lambdaErr *FunctionError
		if errors.As(err, &lambdaErr) {
			status := http.StatusBadRequest
			if lambdaErr.Type == ErrResourceNotFound {
				status = http.StatusNotFound
			}

			writeFunctionError(w, lambdaErr.Type, lambdaErr.Message, status)

			return
		}

		writeFunctionError(w, ErrServiceException, "Internal server error", http.StatusInternalServerError)

		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ListFunctions handles the ListFunctions API.
func (s *Service) ListFunctions(w http.ResponseWriter, r *http.Request) {
	marker := r.URL.Query().Get("Marker")
	maxItemsStr := r.URL.Query().Get("MaxItems")

	maxItems := 50

	if maxItemsStr != "" {
		if parsed, err := strconv.Atoi(maxItemsStr); err == nil {
			maxItems = parsed
		}
	}

	functions, nextMarker, err := s.storage.ListFunctions(r.Context(), marker, maxItems)
	if err != nil {
		writeFunctionError(w, ErrServiceException, "Internal server error", http.StatusInternalServerError)

		return
	}

	configs := make([]*FunctionConfiguration, 0, len(functions))
	for _, fn := range functions {
		configs = append(configs, functionToConfiguration(fn))
	}

	resp := &ListFunctionsResponse{
		Functions:  configs,
		NextMarker: nextMarker,
	}

	writeJSONResponse(w, http.StatusOK, resp)
}

// UpdateFunctionCode handles the UpdateFunctionCode API.
func (s *Service) UpdateFunctionCode(w http.ResponseWriter, r *http.Request) {
	functionName := extractFunctionNameFromCodePath(r.URL.Path)
	if functionName == "" {
		writeFunctionError(w, ErrInvalidParameterValue, "FunctionName is required", http.StatusBadRequest)

		return
	}

	var req UpdateFunctionCodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeFunctionError(w, ErrInvalidParameterValue, "Invalid request body", http.StatusBadRequest)

		return
	}

	fn, err := s.storage.UpdateFunctionCode(r.Context(), functionName, &req)
	if err != nil {
		var lambdaErr *FunctionError
		if errors.As(err, &lambdaErr) {
			status := http.StatusBadRequest
			if lambdaErr.Type == ErrResourceNotFound {
				status = http.StatusNotFound
			}

			writeFunctionError(w, lambdaErr.Type, lambdaErr.Message, status)

			return
		}

		writeFunctionError(w, ErrServiceException, "Internal server error", http.StatusInternalServerError)

		return
	}

	resp := functionToCreateResponse(fn)
	writeJSONResponse(w, http.StatusOK, resp)
}

// UpdateFunctionConfiguration handles the UpdateFunctionConfiguration API.
func (s *Service) UpdateFunctionConfiguration(w http.ResponseWriter, r *http.Request) {
	functionName := extractFunctionNameFromConfigPath(r.URL.Path)
	if functionName == "" {
		writeFunctionError(w, ErrInvalidParameterValue, "FunctionName is required", http.StatusBadRequest)

		return
	}

	var req UpdateFunctionConfigurationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeFunctionError(w, ErrInvalidParameterValue, "Invalid request body", http.StatusBadRequest)

		return
	}

	fn, err := s.storage.UpdateFunctionConfiguration(r.Context(), functionName, &req)
	if err != nil {
		var lambdaErr *FunctionError
		if errors.As(err, &lambdaErr) {
			status := http.StatusBadRequest
			if lambdaErr.Type == ErrResourceNotFound {
				status = http.StatusNotFound
			}

			writeFunctionError(w, lambdaErr.Type, lambdaErr.Message, status)

			return
		}

		writeFunctionError(w, ErrServiceException, "Internal server error", http.StatusInternalServerError)

		return
	}

	resp := functionToCreateResponse(fn)
	writeJSONResponse(w, http.StatusOK, resp)
}

// Invoke handles the Invoke API.
//
//nolint:funlen // Invoke handles multiple code paths (sync, async, stub).
func (s *Service) Invoke(w http.ResponseWriter, r *http.Request) {
	functionName := extractFunctionNameFromInvokePath(r.URL.Path)
	if functionName == "" {
		writeFunctionError(w, ErrInvalidParameterValue, "FunctionName is required", http.StatusBadRequest)

		return
	}

	fn, err := s.storage.GetFunction(r.Context(), functionName)
	if err != nil {
		handleGetFunctionError(w, err)

		return
	}

	payload, err := io.ReadAll(r.Body)
	if err != nil {
		writeFunctionError(w, ErrInvalidParameterValue, "Failed to read request body", http.StatusBadRequest)

		return
	}

	invocationType := r.Header.Get("X-Amz-Invocation-Type")
	if invocationType == "" {
		invocationType = "RequestResponse"
	}

	// When no InvokeEndpoint is configured the function is treated as an
	// echo stub: the invocation is accepted and the input payload is
	// returned as-is. This lets SDK callers exercise functions created
	// without kumo's InvokeEndpoint extension and still receive a
	// meaningful response payload, which matches the expectation of
	// tests that invoke Lambda functions via the standard AWS SDK.
	if fn.InvokeEndpoint == "" {
		switch invocationType {
		case "DryRun":
			writeInvokeHeaders(w)
			w.WriteHeader(http.StatusNoContent)
		case "Event":
			writeInvokeHeaders(w)
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte("{}"))
		default:
			writeInvokeHeaders(w)
			w.WriteHeader(http.StatusOK)

			if len(payload) == 0 {
				_, _ = w.Write([]byte("null"))
			} else {
				_, _ = w.Write(payload)
			}
		}

		return
	}

	switch invocationType {
	case "DryRun":
		writeInvokeHeaders(w)
		w.WriteHeader(http.StatusNoContent)
	case "Event":
		s.invokeAsync(functionName, fn.InvokeEndpoint, payload)
		writeInvokeHeaders(w)
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte("{}"))
	default:
		s.invokeSync(r.Context(), w, fn.InvokeEndpoint, payload)
	}
}

// handleGetFunctionError writes error response for GetFunction errors.
func handleGetFunctionError(w http.ResponseWriter, err error) {
	var lambdaErr *FunctionError
	if errors.As(err, &lambdaErr) {
		status := http.StatusBadRequest
		if lambdaErr.Type == ErrResourceNotFound {
			status = http.StatusNotFound
		}

		writeFunctionError(w, lambdaErr.Type, lambdaErr.Message, status)

		return
	}

	writeFunctionError(w, ErrServiceException, "Internal server error", http.StatusInternalServerError)
}

// writeInvokeHeaders writes common invoke response headers.
func writeInvokeHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Amz-Executed-Version", "$LATEST")
	w.Header().Set("X-Amz-Request-Id", uuid.New().String())
}

// invokeAsync invokes the function asynchronously.
func (s *Service) invokeAsync(functionName, endpoint string, payload []byte) {
	payloadCopy := make([]byte, len(payload))
	copy(payloadCopy, payload)

	go func() {
		req, err := http.NewRequestWithContext(
			context.Background(),
			http.MethodPost,
			endpoint,
			bytes.NewReader(payloadCopy),
		)
		if err != nil {
			slog.Error("async invoke failed to create request", "function", functionName, "error", err)

			return
		}

		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			slog.Error("async invoke failed", "function", functionName, "error", err)

			return
		}

		_ = resp.Body.Close()
	}()
}

// invokeSync invokes the function synchronously and writes the response.
func (s *Service) invokeSync(ctx context.Context, w http.ResponseWriter, endpoint string, payload []byte) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		writeFunctionError(w, ErrServiceException,
			fmt.Sprintf("Failed to create request: %v", err), http.StatusInternalServerError)

		return
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		writeFunctionError(w, ErrServiceException,
			fmt.Sprintf("Failed to invoke endpoint: %v", err), http.StatusInternalServerError)

		return
	}

	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		writeFunctionError(w, ErrServiceException,
			"Failed to read response from endpoint", http.StatusInternalServerError)

		return
	}

	writeInvokeHeaders(w)
	w.WriteHeader(http.StatusOK)

	if len(respBody) == 0 {
		_, _ = w.Write([]byte("null"))
	} else {
		_, _ = w.Write(respBody)
	}
}

// extractFunctionName extracts function name from URL paths like:
//
//   - /lambda/2015-03-31/functions/{name}              (SDK BaseEndpoint = .../lambda)
//   - /2015-03-31/functions/{name}                     (terraform-provider-aws, single endpoint)
//   - /lambda/2015-03-31/functions/{name}/code         (sub-resources accepted as well)
//
// Routes are registered for both prefixes, so the helper finds the
// "functions" segment regardless of where it appears in the path. The
// trailing /code, /configuration, /invocations sub-resources are tolerated
// — the dedicated FromXPath helpers below assert which sub-resource was
// matched.
func extractFunctionName(path string) string {
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	for i, p := range parts {
		if p == pathSegmentFunctions && i+1 < len(parts) {
			return parts[i+1]
		}
	}

	return ""
}

// extractFunctionNameFromCodePath returns the function name when the path
// ends in /functions/{name}/code; "" if the shape does not match.
func extractFunctionNameFromCodePath(path string) string {
	return extractFunctionNameFromSubresource(path, "code")
}

// extractFunctionNameFromConfigPath returns the function name when the path
// ends in /functions/{name}/configuration; "" if the shape does not match.
func extractFunctionNameFromConfigPath(path string) string {
	return extractFunctionNameFromSubresource(path, "configuration")
}

// extractFunctionNameFromInvokePath returns the function name when the path
// ends in /functions/{name}/invocations; "" if the shape does not match.
func extractFunctionNameFromInvokePath(path string) string {
	return extractFunctionNameFromSubresource(path, "invocations")
}

// extractFunctionNameFromSubresource returns the function name when the
// path matches /functions/{name}/<sub>, accepting any leading prefix.
func extractFunctionNameFromSubresource(path, sub string) string {
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	for i, p := range parts {
		if p == pathSegmentFunctions && i+2 < len(parts) && parts[i+2] == sub {
			return parts[i+1]
		}
	}

	return ""
}

// functionToCreateResponse converts a Function to CreateFunctionResponse.
func functionToCreateResponse(fn *Function) *CreateFunctionResponse {
	return &CreateFunctionResponse{
		FunctionName:    fn.FunctionName,
		FunctionArn:     fn.FunctionArn,
		Runtime:         fn.Runtime,
		Role:            fn.Role,
		Handler:         fn.Handler,
		CodeSize:        fn.CodeSize,
		Description:     fn.Description,
		Timeout:         fn.Timeout,
		MemorySize:      fn.MemorySize,
		LastModified:    fn.LastModified.Format("2006-01-02T15:04:05.000+0000"),
		CodeSha256:      fn.CodeSha256,
		Version:         fn.Version,
		State:           fn.State,
		StateReason:     fn.StateReason,
		StateReasonCode: fn.StateReasonCode,
		PackageType:     fn.PackageType,
		Architectures:   fn.Architectures,
		Environment:     fn.Environment,
	}
}

// functionToConfiguration converts a Function to FunctionConfiguration.
func functionToConfiguration(fn *Function) *FunctionConfiguration {
	return &FunctionConfiguration{
		FunctionName:    fn.FunctionName,
		FunctionArn:     fn.FunctionArn,
		Runtime:         fn.Runtime,
		Role:            fn.Role,
		Handler:         fn.Handler,
		CodeSize:        fn.CodeSize,
		Description:     fn.Description,
		Timeout:         fn.Timeout,
		MemorySize:      fn.MemorySize,
		LastModified:    fn.LastModified.Format("2006-01-02T15:04:05.000+0000"),
		CodeSha256:      fn.CodeSha256,
		Version:         fn.Version,
		State:           fn.State,
		StateReason:     fn.StateReason,
		StateReasonCode: fn.StateReasonCode,
		PackageType:     fn.PackageType,
		Architectures:   fn.Architectures,
		Environment:     fn.Environment,
	}
}

// writeJSONResponse writes a JSON response.
func writeJSONResponse(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Amzn-Requestid", uuid.New().String())
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeFunctionError writes a Lambda error response.
func writeFunctionError(w http.ResponseWriter, errType, message string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Amzn-Requestid", uuid.New().String())
	w.Header().Set("X-Amzn-Errortype", errType)
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"Type":    errType,
		"Message": message,
	})
}

// CreateEventSourceMapping handles the CreateEventSourceMapping API.
func (s *Service) CreateEventSourceMapping(w http.ResponseWriter, r *http.Request) {
	var req CreateEventSourceMappingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeFunctionError(w, ErrInvalidParameterValue, "Invalid request body", http.StatusBadRequest)

		return
	}

	if req.FunctionName == "" {
		writeFunctionError(w, ErrInvalidParameterValue, "FunctionName is required", http.StatusBadRequest)

		return
	}

	mapping, err := s.storage.CreateEventSourceMapping(r.Context(), &req)
	if err != nil {
		handleFunctionError(w, err)

		return
	}

	writeJSONResponse(w, http.StatusCreated, mapping)
}

// GetEventSourceMapping handles the GetEventSourceMapping API.
func (s *Service) GetEventSourceMapping(w http.ResponseWriter, r *http.Request) {
	mappingUUID := extractEventSourceMappingUUID(r.URL.Path)
	if mappingUUID == "" {
		writeFunctionError(w, ErrInvalidParameterValue, "UUID is required", http.StatusBadRequest)

		return
	}

	mapping, err := s.storage.GetEventSourceMapping(r.Context(), mappingUUID)
	if err != nil {
		handleFunctionError(w, err)

		return
	}

	writeJSONResponse(w, http.StatusOK, mapping)
}

// DeleteEventSourceMapping handles the DeleteEventSourceMapping API.
func (s *Service) DeleteEventSourceMapping(w http.ResponseWriter, r *http.Request) {
	mappingUUID := extractEventSourceMappingUUID(r.URL.Path)
	if mappingUUID == "" {
		writeFunctionError(w, ErrInvalidParameterValue, "UUID is required", http.StatusBadRequest)

		return
	}

	mapping, err := s.storage.GetEventSourceMapping(r.Context(), mappingUUID)
	if err != nil {
		handleFunctionError(w, err)

		return
	}

	if err := s.storage.DeleteEventSourceMapping(r.Context(), mappingUUID); err != nil {
		handleFunctionError(w, err)

		return
	}

	// Return the mapping with state set to Deleting
	mapping.State = "Deleting"
	writeJSONResponse(w, http.StatusOK, mapping)
}

// ListEventSourceMappings handles the ListEventSourceMappings API.
func (s *Service) ListEventSourceMappings(w http.ResponseWriter, r *http.Request) {
	functionName := r.URL.Query().Get("FunctionName")
	eventSourceArn := r.URL.Query().Get("EventSourceArn")
	marker := r.URL.Query().Get("Marker")

	maxItems := 100

	if maxItemsStr := r.URL.Query().Get("MaxItems"); maxItemsStr != "" {
		if parsed, err := strconv.Atoi(maxItemsStr); err == nil {
			maxItems = parsed
		}
	}

	mappings, nextMarker, err := s.storage.ListEventSourceMappings(r.Context(), functionName, eventSourceArn, marker, maxItems)
	if err != nil {
		writeFunctionError(w, ErrServiceException, "Internal server error", http.StatusInternalServerError)

		return
	}

	resp := &ListEventSourceMappingsResponse{
		EventSourceMappings: mappings,
		NextMarker:          nextMarker,
	}

	writeJSONResponse(w, http.StatusOK, resp)
}

// UpdateEventSourceMapping handles the UpdateEventSourceMapping API.
func (s *Service) UpdateEventSourceMapping(w http.ResponseWriter, r *http.Request) {
	mappingUUID := extractEventSourceMappingUUID(r.URL.Path)
	if mappingUUID == "" {
		writeFunctionError(w, ErrInvalidParameterValue, "UUID is required", http.StatusBadRequest)

		return
	}

	var req UpdateEventSourceMappingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeFunctionError(w, ErrInvalidParameterValue, "Invalid request body", http.StatusBadRequest)

		return
	}

	mapping, err := s.storage.UpdateEventSourceMapping(r.Context(), mappingUUID, &req)
	if err != nil {
		handleFunctionError(w, err)

		return
	}

	writeJSONResponse(w, http.StatusOK, mapping)
}

// handleFunctionError handles FunctionError and writes appropriate response.
func handleFunctionError(w http.ResponseWriter, err error) {
	var lambdaErr *FunctionError
	if errors.As(err, &lambdaErr) {
		status := http.StatusBadRequest
		if lambdaErr.Type == ErrResourceNotFound {
			status = http.StatusNotFound
		}

		writeFunctionError(w, lambdaErr.Type, lambdaErr.Message, status)

		return
	}

	writeFunctionError(w, ErrServiceException, "Internal server error", http.StatusInternalServerError)
}

// extractEventSourceMappingUUID extracts UUID from paths like:
//
//   - /lambda/2015-03-31/event-source-mappings/{UUID}  (SDK BaseEndpoint = .../lambda)
//   - /2015-03-31/event-source-mappings/{UUID}          (terraform-provider-aws, single endpoint)
//
// Routes are registered under both prefixes, so the helper finds the
// "event-source-mappings" segment regardless of where it appears in the path.
func extractEventSourceMappingUUID(path string) string {
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	for i, p := range parts {
		if p == "event-source-mappings" && i+1 < len(parts) {
			return parts[i+1]
		}
	}

	return ""
}
